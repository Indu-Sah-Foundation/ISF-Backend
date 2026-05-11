// Package health exposes liveness and readiness endpoints.
//
// /health     -> liveness: is the process up? Used by App Service / orchestrator.
// /health/db  -> dependency: can we reach Postgres?
// /health/redis -> dependency: can we reach Redis?
// /ready      -> readiness: ALL dependencies healthy. Use for Front Door / LB.
//
// Liveness should never fail unless the process is hosed. Readiness fails
// during startup and during dep outages, so traffic gets routed away.
package health

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"isf-backend/internal/cache"
)

type Handler struct {
	pool  *pgxpool.Pool
	cache *cache.Redis // concrete type since we use Ping()
}

func NewHandler(pool *pgxpool.Pool, c *cache.Redis) *Handler {
	return &Handler{pool: pool, cache: c}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.GET("/health", h.live)
	r.GET("/health/db", h.db)
	r.GET("/health/redis", h.redis)
	r.GET("/ready", h.ready)
}

// live returns 200 as long as the process can answer HTTP. Liveness probe.
func (h *Handler) live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// db pings Postgres. 503 if unreachable.
func (h *Handler) db(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if err := h.pool.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "down",
			"check":  "postgres",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "check": "postgres"})
}

// redis pings Redis. 503 if unreachable.
func (h *Handler) redis(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if err := h.cache.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "down",
			"check":  "redis",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "check": "redis"})
}

// ready checks all dependencies in parallel. 503 if any fails. Readiness probe.
func (h *Handler) ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	type result struct {
		name string
		ok   bool
	}
	checks := make(chan result, 2)

	go func() {
		checks <- result{"postgres", h.pool.Ping(ctx) == nil}
	}()
	go func() {
		checks <- result{"redis", h.cache.Ping(ctx) == nil}
	}()

	statuses := map[string]string{}
	allOK := true
	for i := 0; i < 2; i++ {
		r := <-checks
		if r.ok {
			statuses[r.name] = "ok"
		} else {
			statuses[r.name] = "down"
			allOK = false
		}
	}

	if allOK {
		c.JSON(http.StatusOK, gin.H{"status": "ready", "checks": statuses})
		return
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "checks": statuses})
}
