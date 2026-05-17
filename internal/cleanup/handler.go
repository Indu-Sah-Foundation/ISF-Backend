package cleanup

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"isf-backend/internal/httperr"
)

type Handler struct {
	cleanup *Cleanup
}

func NewHandler(c *Cleanup) *Handler { return &Handler{cleanup: c} }

func (h *Handler) RegisterRoutes(r *gin.Engine, adminMW ...gin.HandlerFunc) {
	// Both endpoints are admin-only.
	r.GET("/admin/images/cleanup", append(adminMW, h.preview)...) // dry-run
	r.POST("/admin/images/cleanup", append(adminMW, h.run)...)    // actually delete
}

// preview returns the orphan list WITHOUT deleting.
func (h *Handler) preview(c *gin.Context) {
	out, err := h.cleanup.Run(c.Request.Context(), true)
	if err != nil {
		httperr.Respond(c, httperr.ErrInternal, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// run deletes all orphans and returns the summary.
func (h *Handler) run(c *gin.Context) {
	out, err := h.cleanup.Run(c.Request.Context(), false)
	if err != nil {
		httperr.Respond(c, httperr.ErrInternal, err)
		return
	}
	c.JSON(http.StatusOK, out)
}
