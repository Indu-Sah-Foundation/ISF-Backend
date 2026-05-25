package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"isf-backend/internal/httperr"
)

// RequireAPIKey is a gateway-level guard that rejects any request not
// carrying the right `X-API-Key` header. Layers UNDER the existing
// JWT admin auth — the key proves "you're our frontend"; the JWT proves
// "you're a logged-in admin". Both are required for admin endpoints.
//
// Threat model: stops casual scrapers, automated scanners, and random
// callers hitting our Translator / Stripe / Blob budgets. It is NOT
// strong protection against a determined attacker who extracts the key
// from the published frontend bundle — for that you'd need WAF rules
// or origin checks at Front Door. The key just raises the floor.
//
// Exempt paths (the key cannot reach them, or we don't control the
// caller):
//   - /health, /health/db, /health/redis, /ready  (Azure probes)
//   - /donations/webhook                          (Stripe POSTs here)
//
// Constant-time compare on the key value to avoid timing leaks.
func RequireAPIKey(expected string, exemptPrefixes []string) gin.HandlerFunc {
	expBytes := []byte(expected)
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		for _, p := range exemptPrefixes {
			if strings.HasPrefix(path, p) {
				c.Next()
				return
			}
		}

		got := c.GetHeader("X-API-Key")
		if got == "" {
			// Also accept `Authorization: ApiKey <key>` for callers
			// that prefer the standard header — Stripe CLI, curl
			// scripts during dev, etc.
			if h := c.GetHeader("Authorization"); strings.HasPrefix(h, "ApiKey ") {
				got = strings.TrimPrefix(h, "ApiKey ")
			}
		}
		if subtle.ConstantTimeCompare([]byte(got), expBytes) != 1 {
			httperr.Respond(c, httperr.ErrUnauthorized.With("API key required"), nil)
			c.Abort()
			return
		}
		c.Next()
	}
}

// EnsureNotEmpty returns an error if the configured key is empty —
// prevents accidentally booting with no protection.
func EnsureAPIKeyNotEmpty(key string) error {
	if strings.TrimSpace(key) == "" {
		return ErrEmptyAPIKey
	}
	return nil
}

// ErrEmptyAPIKey signals that API_KEY was unset / empty at config
// load time. Wired in main.go to fail fast at boot.
var ErrEmptyAPIKey = httperr.ErrInternal.With("API_KEY is required")

// Helper: send a 401 with a friendly body the frontend's ApiError
// machinery surfaces cleanly to the user.
var _ = http.StatusUnauthorized
