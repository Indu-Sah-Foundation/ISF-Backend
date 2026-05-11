package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"isf-backend/internal/httperr"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registers /auth/login. The optional loginLimiter middleware
// applies a rate limit to the login endpoint specifically (brute-force defense).
func (h *Handler) RegisterRoutes(r *gin.Engine, loginLimiter gin.HandlerFunc) {
	if loginLimiter != nil {
		r.POST("/auth/login", loginLimiter, h.login)
	} else {
		r.POST("/auth/login", h.login)
	}
}

func (h *Handler) login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, err.Error())
		return
	}
	token, user, err := h.svc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			httperr.Respond(c, httperr.ErrUnauthorized.With("invalid credentials"), nil)
			return
		}
		httperr.Respond(c, httperr.ErrInternal, err)
		return
	}
	c.JSON(http.StatusOK, LoginResponse{Token: token, User: *user})
}
