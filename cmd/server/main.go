// Command server is the single entrypoint for the ISF backend.
// It loads configuration, wires up dependencies (DB, cache, etc. -- coming
// in later lessons), and starts the HTTP server.
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"isf-backend/internal/articles"
	"isf-backend/internal/auth"
	"isf-backend/internal/cache"
	"isf-backend/internal/config"
	"isf-backend/internal/db"
	"isf-backend/internal/people"
	"isf-backend/internal/translate"
)

func main() {
	// 1. Load config. If anything required is missing, log.Fatal exits with
	//    status 1 and prints the error -- App Service will mark the container
	//    as unhealthy and we'll see it in logs immediately.
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
		log.Fatalf("database connection failed %v", err)
	}
	defer pool.Close()
	// 2. Configure Gin. "release" mode disables debug logging in prod.
	gin.SetMode(cfg.GinMode)
	authRepo := auth.NewRepo(pool)
	authSvc := auth.NewService(authRepo, cfg.JWTSecret)
	authHandler := auth.NewHandler(authSvc)

	if cfg.AdminEmail != "" && cfg.AdminPassword != "" {
		if err := authSvc.EnsureAdmin(ctx, cfg.AdminEmail, cfg.AdminPassword); err != nil {
			log.Fatalf("bootstrap admin: %v", err)
		}
	}

	adminMW := []gin.HandlerFunc{
		auth.RequireAuth(cfg.JWTSecret),
		auth.RequireRole("admin"),
	}

	// 3. Build the router. gin.Default() includes logger + recovery middleware.
	peopleRepo := people.NewRepo(pool)
	peopleSvc := people.NewService(peopleRepo)
	peopleHandler := people.NewHandler(peopleSvc)

	redisCache, err := cache.NewRedis(ctx, cfg.RedisURL)
	if err != nil {
		log.Fatalf("redis connection failed: %v", err)
	}
	defer redisCache.Close()

	translator := translate.NewClient(cfg.TranslatorEndpoint, cfg.TranslatorKey, cfg.TranslatorRegion)
	translateHandler := translate.NewHandler(translator)

	articleRepo := articles.NewRepo(pool)
	articleSvc := articles.NewService(articleRepo, redisCache, translator)
	articleHandler := articles.NewHandler(articleSvc)

	r := gin.Default()
	authHandler.RegisterRoutes(r)
	peopleHandler.RegisterRoutes(r, adminMW...)
	articleHandler.RegisterRoutes(r, adminMW...)
	translateHandler.RegisterRoutes(r)
	// 4. Routes. Just /health for now -- App Service uses this for readiness
	//    probes, and Front Door uses it to decide if the origin is healthy.
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/health/db", func(c *gin.Context) {
		if err := pool.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "database unavailable", "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 5. Start the server. ":"+port means "listen on all interfaces".
	//    r.Run blocks forever; if it returns, something went wrong.
	addr := ":" + cfg.Port
	log.Printf("listening on %s (mode=%s)", addr, cfg.GinMode)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server crashed: %v", err)
	}
}
