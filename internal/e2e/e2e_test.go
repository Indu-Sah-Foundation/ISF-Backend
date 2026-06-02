// Package e2e runs the full HTTP router in-process against a real
// Postgres + Redis. CI spins up service containers and points us at
// them via TEST_DATABASE_URL + TEST_REDIS_URL. Tests are skipped when
// those env vars are missing so `go test ./...` from a laptop stays
// fast even without docker.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v78"

	"isf-backend/internal/articles"
	"isf-backend/internal/auth"
	"isf-backend/internal/cache"
	"isf-backend/internal/contacts"
	"isf-backend/internal/db"
	"isf-backend/internal/health"
	"isf-backend/internal/maintenance"
	"isf-backend/internal/payments"
	"isf-backend/internal/people"
)

const (
	adminEmail    = "admin@test.local"
	adminPassword = "test-password-123!"
	jwtSecret     = "test-jwt-secret"
	apiKey        = "test-api-key"
)

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	dbURL := os.Getenv("TEST_DATABASE_URL")
	redisURL := os.Getenv("TEST_REDIS_URL")
	if dbURL == "" || redisURL == "" {
		t.Skip("TEST_DATABASE_URL and TEST_REDIS_URL must be set (CI provides them)")
	}

	if err := db.Migrate(dbURL); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, dbURL)
	if err != nil {
		t.Fatalf("pg pool: %v", err)
	}
	t.Cleanup(pool.Close)

	// Reset state between test binaries.
	for _, tbl := range []string{
		"article_translations", "articles", "contacts", "users", "people",
		"donations", "stripe_events",
	} {
		_, _ = pool.Exec(ctx, "TRUNCATE TABLE "+tbl+" CASCADE")
	}

	rc, err := cache.NewRedis(ctx, redisURL)
	if err != nil {
		t.Fatalf("redis: %v", err)
	}
	t.Cleanup(func() { rc.Close() })
	// (No FlushAll on cache.Redis — TRUNCATE above clears the DB.
	// Per-test cache leakage is harmless because each test makes
	// fresh writes and the test slug space is unique enough.)

	// Fresh in-memory GitHub for each server so issue counts are deterministic.
	testGitHub = &fakeGitHub{}

	gin.SetMode(gin.TestMode)
	authRepo := auth.NewRepo(pool)
	authSvc := auth.NewService(authRepo, jwtSecret)
	if err := authSvc.EnsureAdmin(ctx, adminEmail, adminPassword); err != nil {
		t.Fatalf("bootstrap admin: %v", err)
	}
	adminMW := []gin.HandlerFunc{
		auth.RequireAuth(jwtSecret),
		auth.RequireRole("admin"),
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(auth.RequireAPIKey(apiKey, []string{"/health", "/ready", "/donations/webhook"}))

	health.NewHandler(pool, rc).RegisterRoutes(r)
	auth.NewHandler(authSvc).RegisterRoutes(r, nil)
	people.NewHandler(people.NewService(people.NewRepo(pool))).RegisterRoutes(r, adminMW...)
	articles.NewHandler(articles.NewService(articles.NewRepo(pool), rc, &fakeTranslator{})).RegisterRoutes(r, adminMW...)
	contacts.NewHandler(contacts.NewService(contacts.NewRepo(pool))).RegisterRoutes(r, nil, adminMW...)

	// Maintenance → GitHub, wired with an in-memory fake so the suite never
	// touches the real GitHub API.
	maintenance.NewHandler(
		maintenance.NewService(testGitHub, maintenance.Repos{
			Frontend: "ISF-Frontend",
			Backend:  "ISF-Backend",
			Infra:    "ISF-Infastructure",
		}),
	).RegisterRoutes(r, adminMW...)

	// Payments → Stripe, wired with a fake Stripe client (no network). The
	// admin middleware is composed into one handler the way main.go does it.
	adminChain := func(c *gin.Context) {
		for _, mw := range adminMW {
			mw(c)
			if c.IsAborted() {
				return
			}
		}
		c.Next()
	}
	payments.NewHandler(
		payments.NewService(payments.NewRepo(pool), &fakeStripe{}),
	).RegisterRoutes(r, nil, adminChain)

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

// testGitHub is an in-memory GitHub stand-in, reset per newServer() call.
var testGitHub *fakeGitHub

type fakeGitHub struct {
	mu      sync.Mutex
	created []fakeIssue
	nextNum int
}

type fakeIssue struct {
	repo   string
	labels []string
	issue  maintenance.Issue
}

func (f *fakeGitHub) CreateIssue(_ context.Context, repo, title, _ string, labels []string) (*maintenance.Issue, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextNum++
	iss := maintenance.Issue{
		Number:  f.nextNum,
		HTMLURL: "https://github.com/Indu-Sah-Foundation/" + repo + "/issues/" + strconv.Itoa(f.nextNum),
		Title:   title,
		State:   "open",
	}
	f.created = append(f.created, fakeIssue{repo: repo, labels: labels, issue: iss})
	return &iss, nil
}


func (f *fakeGitHub) seedClosed(repo, title string, closedAt time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextNum++
	ca := closedAt
	f.created = append(f.created, fakeIssue{
		repo:   repo,
		labels: []string{maintenance.Label},
		issue: maintenance.Issue{
			Number:   f.nextNum,
			HTMLURL:  "https://github.com/Indu-Sah-Foundation/" + repo + "/issues/" + strconv.Itoa(f.nextNum),
			Title:    title,
			State:    "closed",
			ClosedAt: &ca,
		},
	})
}

func (f *fakeGitHub) ListIssues(_ context.Context, repos []string, label string, _ int) ([]maintenance.Issue, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	want := map[string]bool{}
	for _, r := range repos {
		want[r] = true
	}
	out := []maintenance.Issue{}
	for _, c := range f.created {
		if want[c.repo] && sliceContains(c.labels, label) {
			out = append(out, c.issue)
		}
	}
	return out, nil
}

func sliceContains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// fakeStripe satisfies payments.Stripe without any network calls.
type fakeStripe struct{}

func (fakeStripe) CreateCheckoutSession(_ context.Context, _ int, _ *string) (string, string, error) {
	return "cs_test_fake123", "https://checkout.stripe.com/c/pay/cs_test_fake123", nil
}

func (fakeStripe) VerifyWebhook(_ []byte, _ string) (stripe.Event, error) {
	// Not exercised by the list tests; return an empty event.
	return stripe.Event{}, nil
}

func (fakeStripe) GetSession(_ context.Context, _ string) (string, string, bool, error) {
	return "unpaid", "", false, nil
}

type fakeTranslator struct{}

func (f *fakeTranslator) Translate(_ context.Context, texts []string, _, _ string) ([]string, error) {
	out := make([]string, len(texts))
	for i, t := range texts {
		out[i] = "[T] " + t
	}
	return out, nil
}

func do(t *testing.T, method, url string, headers map[string]string, body any) (*http.Response, []byte) {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp, out
}

// ============================================================================
// API key middleware tests — the core "no one can just call our API" claim.
// ============================================================================

func TestAPIKeyRequired(t *testing.T) {
	srv := newServer(t)
	resp, body := do(t, "GET", srv.URL+"/articles", nil, nil)
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401 without API key, got %d body=%s", resp.StatusCode, body)
	}
}

func TestAPIKeyAccepted(t *testing.T) {
	srv := newServer(t)
	resp, body := do(t, "GET", srv.URL+"/articles", map[string]string{"X-API-Key": apiKey}, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 with API key, got %d body=%s", resp.StatusCode, body)
	}
}

func TestAPIKeyWrongRejected(t *testing.T) {
	srv := newServer(t)
	resp, _ := do(t, "GET", srv.URL+"/articles", map[string]string{"X-API-Key": "wrong"}, nil)
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401 with wrong key, got %d", resp.StatusCode)
	}
}

func TestHealthExemptFromAPIKey(t *testing.T) {
	srv := newServer(t)
	// No API key header — health endpoint must still answer.
	resp, _ := do(t, "GET", srv.URL+"/health", nil, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("/health should not require API key, got %d", resp.StatusCode)
	}
}

func TestStripeWebhookExempt(t *testing.T) {
	srv := newServer(t)
	// We don't have payments wired into this slim test server, but the
	// exemption check should let the request through to a 404 instead
	// of 401-ing on the API key.
	resp, _ := do(t, "POST", srv.URL+"/donations/webhook", nil, map[string]string{"a": "b"})
	if resp.StatusCode == 401 {
		t.Fatalf("webhook should be exempt from API key, got 401")
	}
}

// ============================================================================
// Auth flow tests.
// ============================================================================

func TestLoginRequiresAPIKey(t *testing.T) {
	srv := newServer(t)
	resp, _ := do(t, "POST", srv.URL+"/auth/login", nil, map[string]string{
		"email": adminEmail, "password": adminPassword,
	})
	if resp.StatusCode != 401 {
		t.Fatalf("login without API key should 401, got %d", resp.StatusCode)
	}
}

func TestLoginSucceedsWithAPIKey(t *testing.T) {
	srv := newServer(t)
	resp, body := do(t, "POST", srv.URL+"/auth/login",
		map[string]string{"X-API-Key": apiKey},
		map[string]string{"email": adminEmail, "password": adminPassword},
	)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 from login, got %d body=%s", resp.StatusCode, body)
	}
	var out struct{ Token string }
	_ = json.Unmarshal(body, &out)
	if len(out.Token) < 20 {
		t.Fatalf("token looks bogus: %q", out.Token)
	}
}

func TestAdminRouteWithoutJWT(t *testing.T) {
	srv := newServer(t)
	// API key present, JWT missing → still 401 because admin route needs both.
	resp, _ := do(t, "GET", srv.URL+"/people", map[string]string{"X-API-Key": apiKey}, nil)
	if resp.StatusCode != 401 {
		t.Fatalf("admin route without JWT should 401, got %d", resp.StatusCode)
	}
}

func TestContactsPublicSubmitWithAPIKey(t *testing.T) {
	srv := newServer(t)
	resp, body := do(t, "POST", srv.URL+"/contacts",
		map[string]string{"X-API-Key": apiKey},
		map[string]string{"email": "donor@example.com", "message": "hi"},
	)
	if resp.StatusCode != 201 {
		t.Fatalf("contact submit failed: %d body=%s", resp.StatusCode, body)
	}
}

// ============================================================================
// Maintenance → GitHub issues (admin only).
// ============================================================================

// loginAdmin returns a valid admin JWT for authenticated requests.
func loginAdmin(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	resp, body := do(t, "POST", srv.URL+"/auth/login",
		map[string]string{"X-API-Key": apiKey},
		map[string]string{"email": adminEmail, "password": adminPassword},
	)
	if resp.StatusCode != 200 {
		t.Fatalf("admin login failed: %d body=%s", resp.StatusCode, body)
	}
	var out struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(body, &out)
	if out.Token == "" {
		t.Fatalf("no token in login response: %s", body)
	}
	return out.Token
}

func adminHeaders(token string) map[string]string {
	return map[string]string{
		"X-API-Key":     apiKey,
		"Authorization": "Bearer " + token,
	}
}

func TestMaintenanceRequiresAdmin(t *testing.T) {
	srv := newServer(t)
	// API key present, no JWT → 401.
	resp, _ := do(t, "POST", srv.URL+"/admin/maintenance",
		map[string]string{"X-API-Key": apiKey},
		map[string]string{"title": "something broke"},
	)
	if resp.StatusCode != 401 {
		t.Fatalf("maintenance create without admin JWT should 401, got %d", resp.StatusCode)
	}
}

func TestMaintenanceCreateRoutesFrontend(t *testing.T) {
	srv := newServer(t)
	token := loginAdmin(t, srv)

	resp, body := do(t, "POST", srv.URL+"/admin/maintenance",
		adminHeaders(token),
		map[string]string{
			"title":       "The donate button is cut off on iPad",
			"description": "The navbar overlaps the page header on mobile screens.",
		},
	)
	if resp.StatusCode != 201 {
		t.Fatalf("create failed: %d body=%s", resp.StatusCode, body)
	}
	var out struct {
		Area string `json:"area"`
		Repo string `json:"repo"`
		Issue struct {
			Number int `json:"number"`
		} `json:"issue"`
	}
	_ = json.Unmarshal(body, &out)
	if out.Area != "frontend" {
		t.Fatalf("expected frontend area, got %q (body=%s)", out.Area, body)
	}
	if out.Repo != "ISF-Frontend" {
		t.Fatalf("expected ISF-Frontend repo, got %q", out.Repo)
	}
	if out.Issue.Number == 0 {
		t.Fatalf("expected a real issue number, got 0")
	}
	// And the fake recorded it with the maintenance label.
	if len(testGitHub.created) != 1 {
		t.Fatalf("expected 1 issue created, got %d", len(testGitHub.created))
	}
	if !sliceContains(testGitHub.created[0].labels, maintenance.Label) {
		t.Fatalf("issue missing %q label: %v", maintenance.Label, testGitHub.created[0].labels)
	}
}

func TestMaintenanceCreateRoutesBackend(t *testing.T) {
	srv := newServer(t)
	token := loginAdmin(t, srv)
	resp, body := do(t, "POST", srv.URL+"/admin/maintenance",
		adminHeaders(token),
		map[string]string{
			"title":       "API returns 500 on the donations endpoint",
			"description": "The postgres database query in the payments handler panics.",
		},
	)
	if resp.StatusCode != 201 {
		t.Fatalf("create failed: %d body=%s", resp.StatusCode, body)
	}
	var out struct {
		Area string `json:"area"`
		Repo string `json:"repo"`
	}
	_ = json.Unmarshal(body, &out)
	if out.Area != "backend" || out.Repo != "ISF-Backend" {
		t.Fatalf("expected backend/ISF-Backend, got %q/%q (body=%s)", out.Area, out.Repo, body)
	}
}

func TestMaintenanceCreateRoutesInfra(t *testing.T) {
	srv := newServer(t)
	token := loginAdmin(t, srv)
	resp, body := do(t, "POST", srv.URL+"/admin/maintenance",
		adminHeaders(token),
		map[string]string{
			"title":       "DNS not resolving after the Azure deploy",
			"description": "The terraform apply changed the domain and the SSL certificate is failing.",
		},
	)
	if resp.StatusCode != 201 {
		t.Fatalf("create failed: %d body=%s", resp.StatusCode, body)
	}
	var out struct {
		Area string `json:"area"`
		Repo string `json:"repo"`
	}
	_ = json.Unmarshal(body, &out)
	if out.Area != "infra" || out.Repo != "ISF-Infastructure" {
		t.Fatalf("expected infra/ISF-Infastructure, got %q/%q (body=%s)", out.Area, out.Repo, body)
	}
}

func TestMaintenanceCreateRejectsShortTitle(t *testing.T) {
	srv := newServer(t)
	token := loginAdmin(t, srv)
	resp, _ := do(t, "POST", srv.URL+"/admin/maintenance",
		adminHeaders(token),
		map[string]string{"title": "no"},
	)
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for too-short title, got %d", resp.StatusCode)
	}
}

func TestMaintenanceListReturnsCreated(t *testing.T) {
	srv := newServer(t)
	token := loginAdmin(t, srv)

	// File two requests, then list.
	for _, title := range []string{
		"Footer link is broken on the website",
		"Redis cache connection timeout in the API",
	} {
		resp, body := do(t, "POST", srv.URL+"/admin/maintenance",
			adminHeaders(token), map[string]string{"title": title})
		if resp.StatusCode != 201 {
			t.Fatalf("seed create failed: %d body=%s", resp.StatusCode, body)
		}
	}

	resp, body := do(t, "GET", srv.URL+"/admin/maintenance", adminHeaders(token), nil)
	if resp.StatusCode != 200 {
		t.Fatalf("list failed: %d body=%s", resp.StatusCode, body)
	}
	var out struct {
		Items []struct {
			Number int    `json:"number"`
			Title  string `json:"title"`
		} `json:"items"`
	}
	_ = json.Unmarshal(body, &out)
	if len(out.Items) != 2 {
		t.Fatalf("expected 2 maintenance issues, got %d (body=%s)", len(out.Items), body)
	}
}

func TestMaintenanceListHidesOldCompleted(t *testing.T) {
	srv := newServer(t)
	token := loginAdmin(t, srv)

	if _, body := do(t, "POST", srv.URL+"/admin/maintenance",
		adminHeaders(token), map[string]string{"title": "Open request still in progress"}); body == nil {
		t.Fatal("seed open failed")
	}
	testGitHub.seedClosed("ISF-Frontend", "Completed two days ago", time.Now().Add(-48*time.Hour))
	testGitHub.seedClosed("ISF-Backend", "Completed an hour ago", time.Now().Add(-1*time.Hour))

	resp, body := do(t, "GET", srv.URL+"/admin/maintenance", adminHeaders(token), nil)
	if resp.StatusCode != 200 {
		t.Fatalf("list failed: %d body=%s", resp.StatusCode, body)
	}
	var out struct {
		Items []struct {
			Title string `json:"title"`
			State string `json:"state"`
		} `json:"items"`
	}
	_ = json.Unmarshal(body, &out)

	titles := map[string]bool{}
	for _, it := range out.Items {
		titles[it.Title] = true
	}
	if !titles["Open request still in progress"] {
		t.Fatalf("open request should be listed; got %s", body)
	}
	if !titles["Completed an hour ago"] {
		t.Fatalf("recently-completed request should still show; got %s", body)
	}
	if titles["Completed two days ago"] {
		t.Fatalf("request completed >24h ago should be hidden; got %s", body)
	}
}

// ============================================================================
// Donations admin list — the dashboard data source.
// ============================================================================

// insertDonation writes a donation row straight to the DB, simulating what the
// Stripe webhook does on a completed payment.
func insertDonation(t *testing.T, sessionID, name, email string, amountCents int, status string) {
	t.Helper()
	dbURL := os.Getenv("TEST_DATABASE_URL")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := db.NewPool(ctx, dbURL)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()
	_, err = pool.Exec(ctx, `
		INSERT INTO donations (amount_cents, currency, email, name, status, stripe_session_id)
		VALUES ($1,'usd',$2,$3,$4,$5)`,
		amountCents, email, name, status, sessionID)
	if err != nil {
		t.Fatalf("insert donation: %v", err)
	}
}

func TestDonationsListRequiresAdmin(t *testing.T) {
	srv := newServer(t)
	resp, _ := do(t, "GET", srv.URL+"/donations", map[string]string{"X-API-Key": apiKey}, nil)
	if resp.StatusCode != 401 {
		t.Fatalf("donations list without admin JWT should 401, got %d", resp.StatusCode)
	}
}

func TestDonationsListReturnsLivePaidOnly(t *testing.T) {
	srv := newServer(t)
	token := loginAdmin(t, srv)

	// A real paid live donation (should show), a test-mode paid one (filtered
	// by livemode=true default), and a pending live one (filtered by paid).
	insertDonation(t, "cs_live_paid1", "Asha Sharma", "asha@example.com", 5000, "paid")
	insertDonation(t, "cs_test_paid2", "Test Donor", "test@example.com", 9900, "paid")
	insertDonation(t, "cs_live_pending3", "Maybe Donor", "maybe@example.com", 2500, "pending")

	resp, body := do(t, "GET", srv.URL+"/donations", adminHeaders(token), nil)
	if resp.StatusCode != 200 {
		t.Fatalf("list failed: %d body=%s", resp.StatusCode, body)
	}
	var out struct {
		Items []struct {
			Name        string `json:"name"`
			AmountCents int    `json:"amount_cents"`
			Status      string `json:"status"`
		} `json:"items"`
	}
	_ = json.Unmarshal(body, &out)

	if len(out.Items) != 1 {
		t.Fatalf("expected exactly 1 live+paid donation, got %d (body=%s)", len(out.Items), body)
	}
	d := out.Items[0]
	if d.Name != "Asha Sharma" || d.AmountCents != 5000 || d.Status != "paid" {
		t.Fatalf("unexpected donation row: %+v", d)
	}
}

func TestDonationsListIncludesTestRowsWithLivemodeAll(t *testing.T) {
	srv := newServer(t)
	token := loginAdmin(t, srv)

	insertDonation(t, "cs_live_p1", "Live Donor", "live@example.com", 5000, "paid")
	insertDonation(t, "cs_test_p2", "Test Donor", "test@example.com", 1000, "paid")

	resp, body := do(t, "GET", srv.URL+"/donations?livemode=all", adminHeaders(token), nil)
	if resp.StatusCode != 200 {
		t.Fatalf("list failed: %d body=%s", resp.StatusCode, body)
	}
	var out struct {
		Items []struct {
			Status string `json:"status"`
		} `json:"items"`
	}
	_ = json.Unmarshal(body, &out)
	if len(out.Items) != 2 {
		t.Fatalf("expected 2 donations with livemode=all, got %d (body=%s)", len(out.Items), body)
	}
}

func TestCheckoutReturnsSessionURL(t *testing.T) {
	srv := newServer(t)
	resp, body := do(t, "POST", srv.URL+"/donations/checkout",
		map[string]string{"X-API-Key": apiKey},
		map[string]any{"amount_cents": 5000, "email": "donor@example.com"},
	)
	if resp.StatusCode != 200 {
		t.Fatalf("checkout failed: %d body=%s", resp.StatusCode, body)
	}
	var out struct {
		URL       string `json:"url"`
		SessionID string `json:"session_id"`
	}
	_ = json.Unmarshal(body, &out)
	if out.URL == "" || out.SessionID == "" {
		t.Fatalf("checkout missing url/session_id: %s", body)
	}
}
