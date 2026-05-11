// Package middleware holds Gin middleware shared across the app.
package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// IPRateLimiter caps requests per second from each unique client IP.
//
// Use:
//   loginLimiter := middleware.NewIPRateLimiter(rate.Every(time.Second), 5)
//   r.POST("/auth/login", loginLimiter.Middleware(), loginHandler)
//
// "rate.Every(time.Second), 5" means: refill 1 token per second, burst up to 5.
// Each request consumes 1 token. When empty, requests get 429.
type IPRateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     rate.Limit
	burst    int
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewIPRateLimiter creates a limiter that allows `burst` requests immediately
// and refills at `r` tokens per second. Stale visitor entries (>10min idle)
// are GC'd hourly to keep memory bounded.
func NewIPRateLimiter(r rate.Limit, burst int) *IPRateLimiter {
	l := &IPRateLimiter{
		visitors: make(map[string]*visitor),
		rate:     r,
		burst:    burst,
	}
	go l.cleanupLoop()
	return l
}

func (l *IPRateLimiter) get(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	v, ok := l.visitors[ip]
	if !ok {
		lim := rate.NewLimiter(l.rate, l.burst)
		l.visitors[ip] = &visitor{limiter: lim, lastSeen: time.Now()}
		return lim
	}
	v.lastSeen = time.Now()
	return v.limiter
}

func (l *IPRateLimiter) cleanupLoop() {
	for {
		time.Sleep(time.Hour)
		l.mu.Lock()
		cutoff := time.Now().Add(-10 * time.Minute)
		for ip, v := range l.visitors {
			if v.lastSeen.Before(cutoff) {
				delete(l.visitors, ip)
			}
		}
		l.mu.Unlock()
	}
}

// Middleware returns the Gin middleware function.
func (l *IPRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// ClientIP respects the X-Forwarded-For header; trusted proxies must
		// be configured on the gin.Engine for this to be safe behind App Service.
		ip := c.ClientIP()
		if !l.get(ip).Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			return
		}
		c.Next()
	}
}
