package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"isf-backend/internal/httperr"
)

func prettyValidationError(err error) string {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		return "invalid request body"
	}
	msgs := make([]string, 0, len(ve))
	for _, fe := range ve {
		field := strings.ToLower(fe.Field())
		switch fe.Tag() {
		case "required":
			msgs = append(msgs, field+" is required")
		case "email":
			msgs = append(msgs, field+" must be a valid email")
		case "min":
			msgs = append(msgs, fmt.Sprintf("%s must be at least %s characters", field, fe.Param()))
		case "max":
			msgs = append(msgs, fmt.Sprintf("%s must be at most %s characters", field, fe.Param()))
		default:
			msgs = append(msgs, fmt.Sprintf("%s is invalid", field))
		}
	}
	return strings.Join(msgs, "; ")
}

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
		httperr.BadRequest(c, prettyValidationError(err))
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
