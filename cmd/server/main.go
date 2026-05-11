// Command server is the single entrypoint for the ISF backend.
package main

import (
	"context"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"isf-backend/internal/articles"
	"isf-backend/internal/auth"
	"isf-backend/internal/cache"
	"isf-backend/internal/config"
	"isf-backend/internal/db"
	"isf-backend/internal/health"
	"isf-backend/internal/middleware"
	"isf-backend/internal/payments"
	"isf-backend/internal/people"
	"isf-backend/internal/translate"
)

func main() {
	// 1. Config + migrations + DB pool
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}
	if err := db.Migrate(cfg.DatabaseURL); err != nil {
		log.Fatalf("migration failed: %v", err)
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer pool.Close()

	// 2. Redis
	redisCache, err := cache.NewRedis(ctx, cfg.RedisURL)
	if err != nil {
		log.Fatalf("redis connection failed: %v", err)
	}
	defer redisCache.Close()

	gin.SetMode(cfg.GinMode)

	// 3. Auth wiring + bootstrap admin
	authRepo := auth.NewRepo(pool)
	authSvc := auth.NewService(authRepo, cfg.JWTSecret)

	if cfg.AdminEmail != "" && cfg.AdminPassword != "" {
		if err := authSvc.EnsureAdmin(ctx, cfg.AdminEmail, cfg.AdminPassword); err != nil {
			log.Fatalf("bootstrap admin: %v", err)
		}
	}

	authHandler := auth.NewHandler(authSvc)

	adminMW := []gin.HandlerFunc{
		auth.RequireAuth(cfg.JWTSecret),
		auth.RequireRole("admin"),
	}

	// 4. Domain wiring
	stripeClient := payments.NewStripeClient(
		cfg.StripeSecretKey,
		cfg.StripeWebhookSecret,
		cfg.DonationSuccessURL,
		cfg.DonationCancelURL,
	)
	paymentsRepo := payments.NewRepo(pool)
	paymentsSvc := payments.NewService(paymentsRepo, stripeClient)
	paymentsHandler := payments.NewHandler(paymentsSvc)

	peopleRepo := people.NewRepo(pool)
	peopleSvc := people.NewService(peopleRepo)
	peopleHandler := people.NewHandler(peopleSvc)

	translator := translate.NewClient(cfg.TranslatorEndpoint, cfg.TranslatorKey, cfg.TranslatorRegion)
	translateHandler := translate.NewHandler(translator)

	articleRepo := articles.NewRepo(pool)
	articleSvc := articles.NewService(articleRepo, redisCache, translator)
	articleHandler := articles.NewHandler(articleSvc)

	healthHandler := health.NewHandler(pool, redisCache)

	// 5. Rate limiters
	//    /auth/login: 5 attempts/min per IP -> brute-force defense
	//    /donations/checkout: 30/min per IP -> bot/spam defense
	loginLimiter := middleware.NewIPRateLimiter(rate.Every(12*time.Second), 5).Middleware()
	checkoutLimiter := middleware.NewIPRateLimiter(rate.Every(2*time.Second), 30).Middleware()

	// 6. Router
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requestLogger()) // structured access log -> stdout -> App Insights ingestion

	healthHandler.RegisterRoutes(r)
	authHandler.RegisterRoutes(r, loginLimiter)
	peopleHandler.RegisterRoutes(r, adminMW...)
	articleHandler.RegisterRoutes(r, adminMW...)
	translateHandler.RegisterRoutes(r)
	paymentsHandler.RegisterRoutes(r, checkoutLimiter, gin.HandlerFunc(func(c *gin.Context) {
		// Compose admin chain inline (Gin doesn't accept variadic for groups inside a fn)
		for _, mw := range adminMW {
			mw(c)
			if c.IsAborted() {
				return
			}
		}
		c.Next()
	}))

	// 7. Serve
	addr := ":" + cfg.Port
	log.Printf("listening on %s (mode=%s)", addr, cfg.GinMode)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server crashed: %v", err)
	}
}

// requestLogger writes one line per request in a structured-ish format that
// App Service's automatic Application Insights ingestion picks up cleanly.
// Format: method path status duration_ms client_ip
func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		dur := time.Since(start)
		log.Printf(
			"http method=%s path=%s status=%d dur_ms=%d ip=%s",
			c.Request.Method,
			c.Request.URL.Path,
			c.Writer.Status(),
			dur.Milliseconds(),
			c.ClientIP(),
		)
	}
}
