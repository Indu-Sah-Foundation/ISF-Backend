package team

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"isf-backend/internal/httperr"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterRoutes mounts /team — public read, admin write. Note the existing
// /people route is a different resource (contacts/donors).
func (h *Handler) RegisterRoutes(r *gin.Engine, adminMW ...gin.HandlerFunc) {
	g := r.Group("/team")
	g.GET("", h.list)

	admin := g.Group("", adminMW...)
	admin.POST("", h.create)
	admin.PUT("/:id", h.update)
	admin.DELETE("/:id", h.delete)
}

func (h *Handler) list(c *gin.Context) {
	includeUnpublished := c.Query("include_unpublished") == "true"
	kind := c.Query("kind") // "" | "founder" | "advisor_intl" | "advisor_nat" | "board"
	items, err := h.svc.List(c.Request.Context(), includeUnpublished, kind)
	if err != nil {
		httperr.Respond(c, httperr.ErrInternal, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "count": len(items)})
}

func (h *Handler) create(c *gin.Context) {
	var req CreateMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, err.Error())
		return
	}
	m, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		httperr.Respond(c, httperr.ErrInternal, err)
		return
	}
	c.JSON(http.StatusCreated, m)
}

func (h *Handler) update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.Respond(c, httperr.ErrBadRequest.With("invalid id"), nil)
		return
	}
	var req UpdateMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, err.Error())
		return
	}
	m, err := h.svc.Update(c.Request.Context(), id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httperr.Respond(c, httperr.ErrNotFound.With("team member not found"), nil)
			return
		}
		httperr.Respond(c, httperr.ErrInternal, err)
		return
	}
	c.JSON(http.StatusOK, m)
}

func (h *Handler) delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.Respond(c, httperr.ErrBadRequest.With("invalid id"), nil)
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			httperr.Respond(c, httperr.ErrNotFound.With("team member not found"), nil)
			return
		}
		httperr.Respond(c, httperr.ErrInternal, err)
		return
	}
	c.Status(http.StatusNoContent)
}
