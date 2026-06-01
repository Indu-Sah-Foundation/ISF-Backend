package maintenance

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"isf-backend/internal/httperr"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) RegisterRoutes(r *gin.Engine, adminMW ...gin.HandlerFunc) {
	g := r.Group("/admin/maintenance", adminMW...)
	g.POST("", h.create)
	g.GET("", h.list)
}

type createRequest struct {
	Title       string `json:"title" binding:"required,min=3,max=200"`
	Description string `json:"description" binding:"max=5000"`
}

func (h *Handler) create(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "Please enter a short title (at least 3 characters) describing the issue.")
		return
	}
	res, err := h.svc.Create(c.Request.Context(), req.Title, req.Description)
	if err != nil {
		httperr.Respond(c, httperr.ErrUpstream, err)
		return
	}
	c.JSON(http.StatusCreated, res)
}

func (h *Handler) list(c *gin.Context) {
	perRepo, _ := strconv.Atoi(c.DefaultQuery("per_repo", "20"))
	items, err := h.svc.List(c.Request.Context(), perRepo)
	if err != nil {
		httperr.Respond(c, httperr.ErrUpstream, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
