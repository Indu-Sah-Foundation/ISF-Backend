package gallery

import (
	"time"

	"github.com/google/uuid"
)

// Item is a single photo on the public /gallery page. `Src` is the
// public Azure Blob URL produced by the SAS upload endpoint — never an
// inline data: URL. Cleanup jobs use the SrcsForArticle / SrcsForGallery
// helpers (in the cleanup module) to determine which blobs are still in
// use vs orphaned.
type Item struct {
	ID        uuid.UUID `json:"id"`
	Src       string    `json:"src"`
	Title     string    `json:"title"`
	Caption   string    `json:"caption"`
	Size      string    `json:"size"` // "S" | "M" | "L" | "XL"
	Tags      []string  `json:"tags"`
	Position  int       `json:"position"`
	Published bool      `json:"published"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateItemRequest struct {
	Src       string   `json:"src" binding:"required,url"`
	Title     string   `json:"title"`
	Caption   string   `json:"caption"`
	Size      string   `json:"size" binding:"omitempty,oneof=S M L XL"`
	Tags      []string `json:"tags"`
	Position  int      `json:"position"`
	Published *bool    `json:"published,omitempty"`
}

type UpdateItemRequest struct {
	Src       *string   `json:"src,omitempty"`
	Title     *string   `json:"title,omitempty"`
	Caption   *string   `json:"caption,omitempty"`
	Size      *string   `json:"size,omitempty" binding:"omitempty,oneof=S M L XL"`
	Tags      *[]string `json:"tags,omitempty"`
	Position  *int      `json:"position,omitempty"`
	Published *bool     `json:"published,omitempty"`
}

type Tag struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateTagRequest struct {
<<<<<<< HEAD
	Name     string `json:"name" binding:"required,min=1,max=50"`
=======
	Name     string `json:"name" binding:"required,min=1,max=120"`
>>>>>>> 3e4e8d0 (contact API: migrations)
	Position int    `json:"position"`
}
