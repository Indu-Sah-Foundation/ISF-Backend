package articles

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"isf-backend/internal/httperr"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) RegisterRoutes(r *gin.Engine, adminMW ...gin.HandlerFunc) {
	g := r.Group("/articles")
	g.GET("", h.list)
	g.GET("/:slug", h.get)
	admin := g.Group("", adminMW...)
	admin.POST("", h.create)
	admin.PUT("/:id", h.update)
	admin.DELETE("/:id", h.delete)
}

// parseLimitOffset reads ?limit= and ?offset= query params with safe defaults.
func parseLimitOffset(c *gin.Context, defaultLimit int) (limit, offset int) {
	limit = defaultLimit
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = n
		}
	}
	return
}

func (h *Handler) list(c *gin.Context) {
	limit, offset := parseLimitOffset(c, 20)
	articles, err := h.svc.List(c.Request.Context(), c.Query("include_unpublished") == "true", limit, offset)
	if err != nil {
		httperr.Respond(c, httperr.ErrInternal, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items":  articles,
		"limit":  limit,
		"offset": offset,
		"count":  len(articles),
	})
}

func (h *Handler) create(c *gin.Context) {
	var req CreateArticleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, err.Error())
		return
	}
	a, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		httperr.Respond(c, httperr.ErrInternal, err)
		return
	}
	c.JSON(http.StatusCreated, a)
}

func (h *Handler) get(c *gin.Context) {
	slug := c.Param("slug")
	lang := c.Query("lang")
	a, err := h.svc.Get(c.Request.Context(), slug, lang)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httperr.Respond(c, httperr.ErrNotFound.With("article not found"), nil)
			return
		}
		httperr.Respond(c, httperr.ErrInternal, err)
		return
	}
	c.JSON(http.StatusOK, a)
}

func (h *Handler) update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.Respond(c, httperr.ErrBadRequest.With("invalid id"), nil)
		return
	}
	var req UpdateArticleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, err.Error())
		return
	}
	a, err := h.svc.Update(c.Request.Context(), id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httperr.Respond(c, httperr.ErrNotFound.With("article not found"), nil)
			return
		}
		httperr.Respond(c, httperr.ErrInternal, err)
		return
	}
	c.JSON(http.StatusOK, a)
}

func (h *Handler) delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.Respond(c, httperr.ErrBadRequest.With("invalid id"), nil)
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			httperr.Respond(c, httperr.ErrNotFound.With("article not found"), nil)
			return
		}
		httperr.Respond(c, httperr.ErrInternal, err)
		return
	}
	c.Status(http.StatusNoContent)
}
