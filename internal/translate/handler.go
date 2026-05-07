package translate

import (
	"github.com/gin-gonic/gin"
	"net/http"
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
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, langs)
}
