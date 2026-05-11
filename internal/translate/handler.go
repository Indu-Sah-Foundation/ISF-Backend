package translate

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"isf-backend/internal/httperr"
)

type Handler struct {
	client *Client
}

func NewHandler(client *Client) *Handler {
	return &Handler{client: client}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.GET("/translate/languages", h.languages)
}

func (h *Handler) languages(c *gin.Context) {
	langs, err := h.client.Languages(c.Request.Context())
	if err != nil {
		httperr.Respond(c, httperr.ErrUpstream, err)
		return
	}
	c.JSON(http.StatusOK, langs)
}
