package storage

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"isf-backend/internal/httperr"
)

type Handler struct {
	client *Client
}

func NewHandler(c *Client) *Handler { return &Handler{client: c} }

// Allowed MIME types → file extension we'll use in the blob name.
var allowedTypes = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
	"image/gif":  "gif",
}

const (
	maxUploadBytes = 10 * 1024 * 1024 // 10 MB
	sasTTL         = 10 * time.Minute
)

// All routes here require admin auth — caller wires in the middleware.
func (h *Handler) RegisterRoutes(r *gin.Engine, adminMW ...gin.HandlerFunc) {
	g := r.Group("/admin/images", adminMW...)
	g.POST("/sas", h.createSAS)
}

type sasRequest struct {
	Filename    string `json:"filename"     binding:"required,max=200"`
	ContentType string `json:"content_type" binding:"required"`
	SizeBytes   int64  `json:"size_bytes"   binding:"required,gt=0"`
}

// POST /admin/images/sas
//
// Body:  { filename, content_type, size_bytes }
// Reply: { upload_url, public_url, blob_name, expires_at }
func (h *Handler) createSAS(c *gin.Context) {
	var req sasRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, err.Error())
		return
	}

	ext, ok := allowedTypes[strings.ToLower(req.ContentType)]
	if !ok {
		httperr.BadRequest(c, "unsupported content type (allowed: image/jpeg, image/png, image/webp, image/gif)")
		return
	}
	if req.SizeBytes > maxUploadBytes {
		httperr.BadRequest(c, "image too large (max 10 MB)")
		return
	}

	// Build a unique, predictable blob name:
	//   2026/05/11/<uuid>-<safe-filename>.<ext>
	// Date prefix makes the container easy to browse chronologically.
	base := sanitizeFilename(req.Filename)
	blobName := time.Now().UTC().Format("2006/01/02") + "/" +
		uuid.New().String() + "-" + base + "." + ext

	res, err := h.client.GenerateUploadSAS(blobName, sasTTL)
	if err != nil {
		httperr.Respond(c, httperr.ErrInternal, err)
		return
	}

	c.JSON(http.StatusOK, res)
}

var unsafeChars = regexp.MustCompile(`[^a-z0-9]+`)

func sanitizeFilename(s string) string {
	s = strings.ToLower(s)
	// Strip extension (we re-attach a canonical one based on content type).
	if i := strings.LastIndex(s, "."); i > 0 {
		s = s[:i]
	}
	s = unsafeChars.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 80 {
		s = s[:80]
	}
	if s == "" {
		s = "image"
	}
	return s
}
