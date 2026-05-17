package volunteers

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

func (h *Handler) RegisterRoutes(r *gin.Engine, adminMW ...gin.HandlerFunc) {
	g := r.Group("/volunteers")
	g.GET("", h.list)

	admin := g.Group("", adminMW...)
	admin.POST("", h.create)
	admin.PUT("/:id", h.update)
	admin.DELETE("/:id", h.delete)
}

func (h *Handler) list(c *gin.Context) {
	includeUnpublished := c.Query("include_unpublished") == "true"
	kind := c.Query("kind") // "" | "team" | "volunteer_field" | "research_field"
	items, err := h.svc.List(c.Request.Context(), includeUnpublished, kind)
	if err != nil {
		httperr.Respond(c, httperr.ErrInternal, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "count": len(items)})
}

func (h *Handler) create(c *gin.Context) {
	var req CreateVolunteerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, err.Error())
		return
	}
	v, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		httperr.Respond(c, httperr.ErrInternal, err)
		return
	}
	c.JSON(http.StatusCreated, v)
}

func (h *Handler) update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.Respond(c, httperr.ErrBadRequest.With("invalid id"), nil)
		return
	}
	var req UpdateVolunteerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, err.Error())
		return
	}
	v, err := h.svc.Update(c.Request.Context(), id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httperr.Respond(c, httperr.ErrNotFound.With("volunteer not found"), nil)
			return
		}
		httperr.Respond(c, httperr.ErrInternal, err)
		return
	}
	c.JSON(http.StatusOK, v)
}

func (h *Handler) delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.Respond(c, httperr.ErrBadRequest.With("invalid id"), nil)
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			httperr.Respond(c, httperr.ErrNotFound.With("volunteer not found"), nil)
			return
		}
		httperr.Respond(c, httperr.ErrInternal, err)
		return
	}
	c.Status(http.StatusNoContent)
}
