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
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"isf-backend/internal/articles"
	"isf-backend/internal/auth"
	"isf-backend/internal/cache"
	"isf-backend/internal/contacts"
	"isf-backend/internal/db"
	"isf-backend/internal/health"
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

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
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
