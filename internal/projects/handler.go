package projects

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
	g := r.Group("/projects")

	g.GET("", h.list)
	g.GET("/:slug", h.getBySlug)

	admin := g.Group("", adminMW...)
	admin.POST("", h.create)
	admin.PUT("/:id", h.update)
	admin.DELETE("/:id", h.delete)
}

func (h *Handler) list(c *gin.Context) {
	includeUnpublished := c.Query("include_unpublished") == "true"
	kind := c.Query("kind") 
	items, err := h.svc.List(c.Request.Context(), includeUnpublished, kind)
	if err != nil {
		httperr.Respond(c, httperr.ErrInternal, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "count": len(items)})
}

func (h *Handler) getBySlug(c *gin.Context) {
	p, err := h.svc.GetBySlug(c.Request.Context(), c.Param("slug"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httperr.Respond(c, httperr.ErrNotFound.With("project not found"), nil)
			return
		}
		httperr.Respond(c, httperr.ErrInternal, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *Handler) create(c *gin.Context) {
	var req CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, err.Error())
		return
	}
	p, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		httperr.Respond(c, httperr.ErrInternal, err)
		return
	}
	c.JSON(http.StatusCreated, p)
}

func (h *Handler) update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.Respond(c, httperr.ErrBadRequest.With("invalid id"), nil)
		return
	}
	var req UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, err.Error())
		return
	}
	p, err := h.svc.Update(c.Request.Context(), id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httperr.Respond(c, httperr.ErrNotFound.With("project not found"), nil)
			return
		}
		httperr.Respond(c, httperr.ErrInternal, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *Handler) delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.Respond(c, httperr.ErrBadRequest.With("invalid id"), nil)
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			httperr.Respond(c, httperr.ErrNotFound.With("project not found"), nil)
			return
		}
		httperr.Respond(c, httperr.ErrInternal, err)
		return
	}
	c.Status(http.StatusNoContent)
}
