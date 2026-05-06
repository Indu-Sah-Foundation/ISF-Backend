package people

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"net/http"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	g := r.Group("/people")
	g.POST("", h.create)
	g.GET("", h.list)
	g.GET("/:id", h.get)
}

func (h *Handler) create(c *gin.Context) {
	var req CreatePersonRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		return
	}
}
