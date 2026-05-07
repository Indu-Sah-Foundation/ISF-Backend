package articles

import (
	"time"

	"github.com/google/uuid"
)

type Article struct {
	ID          uuid.UUID  `json:"id"`
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	BodyMD      string     `json:"body_md"`
	SourceLang  string     `json:"source_lang"`
	PublishedAt *time.Time `json:"published_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type CreateArticleRequest struct {
	Slug       string `json:"slug" binding:"required,min=1,max=200"`
	Title      string `json:"title" binding:"required,min=1,max=500"`
	BodyMD     string `json:"body_md" binding:"required,min=1"`
	SourceLang string `json:"source_lang" binding:"omitempty,len=2"`
	Publish    bool   `json:"publish"`
}

type UpdateArticleRequest struct {
	Title   *string `json:"title,omitempty"`
	BodyMD  *string `json:"body_md,omitempty"`
	Publish *bool   `json:"publish,omitempty"`
}
