// Package httperr provides helpers that translate Go errors into HTTP responses
// without leaking internals (SQL fragments, stack traces, sensitive paths) to clients.
//
// The pattern is:
//   - Internal: log the full error with context for debugging.
//   - External: respond with a generic, user-safe message + stable status code.
package httperr

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// E is a user-facing error: status code + safe public message.
type E struct {
	Status  int
	Message string
}

func (e E) Error() string { return e.Message }

// Common public errors. Use these or .Wrap() in handlers.
var (
	ErrBadRequest    = E{Status: http.StatusBadRequest, Message: "bad request"}
	ErrUnauthorized  = E{Status: http.StatusUnauthorized, Message: "unauthorized"}
	ErrForbidden     = E{Status: http.StatusForbidden, Message: "forbidden"}
	ErrNotFound      = E{Status: http.StatusNotFound, Message: "not found"}
	ErrConflict      = E{Status: http.StatusConflict, Message: "conflict"}
	ErrTooManyReq    = E{Status: http.StatusTooManyRequests, Message: "rate limit exceeded"}
	ErrUpstream      = E{Status: http.StatusBadGateway, Message: "upstream service error"}
	ErrInternal      = E{Status: http.StatusInternalServerError, Message: "internal server error"}
)

// With returns a copy of the public error with a custom user-safe message.
// Use sparingly -- the default messages are intentionally generic.
func (e E) With(msg string) E { return E{Status: e.Status, Message: msg} }

// Respond writes a JSON error response. If pubErr is an E, its status+message
// are used as-is. Otherwise it's treated as an internal error: the real error
// is logged with the context, but a generic 500 is returned.
//
//	httperr.Respond(c, httperr.ErrBadRequest.With("email is required"), nil)
//	httperr.Respond(c, httperr.ErrNotFound, nil)
//	httperr.Respond(c, nil, internalErr)              // -> logs, returns 500
//	httperr.Respond(c, httperr.ErrInternal, dbErr)    // same effect, more explicit
func Respond(c *gin.Context, pubErr error, internalErr error) {
	var e E
	switch {
	case errors.As(pubErr, &e):
		// public error -- expose status + message
	case pubErr != nil:
		// shouldn't happen if callers use E, but be defensive
		e = ErrInternal
	default:
		e = ErrInternal
	}

	if internalErr != nil {
		log.Printf("[%d] %s %s -> %v", e.Status, c.Request.Method, c.Request.URL.Path, internalErr)
	}

	c.JSON(e.Status, gin.H{"error": e.Message})
}

// BadRequest is a shorthand for validator-binding failures. Includes the
// validator's message because that text is for the *client*, not internal.
func BadRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, gin.H{"error": msg})
}
