package projects

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Project struct {
	ID           uuid.UUID `json:"id"`
	Slug         string    `json:"slug"`
	Kind         string    `json:"kind"`           // "current" | "upcoming"
	Title        string    `json:"title"`
	Label        string    `json:"label"`          // eyebrow above title
	Lede         string    `json:"lede"`           // intro paragraph
	ImageURL     string    `json:"image_url"`
	ImageVariant string    `json:"image_variant"`  // "default" | "alt"
	Blocks       Blocks    `json:"blocks"`
	Position     int       `json:"position"`
	Published    bool      `json:"published"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Blocks []Block


type Block struct {
	Type    string      `json:"type"`
	Heading string      `json:"heading,omitempty"`
	Body    string      `json:"body,omitempty"`
	Items   interface{} `json:"items,omitempty"`
}

func (b *Blocks) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*b = Blocks{}
		return nil
	}
	var raw []Block
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*b = raw
	return nil
}

type CreateProjectRequest struct {
	Slug         string `json:"slug" binding:"required,min=1,max=200"`
	Kind         string `json:"kind" binding:"required,oneof=current upcoming"`
	Title        string `json:"title" binding:"required,min=1,max=300"`
	Label        string `json:"label" binding:"omitempty,max=120"`
	Lede         string `json:"lede"`
	ImageURL     string `json:"image_url"`
	ImageVariant string `json:"image_variant" binding:"omitempty,oneof=default alt"`
	Blocks       Blocks `json:"blocks"`
	Position     int    `json:"position"`
	Published    *bool  `json:"published,omitempty"`
}

type UpdateProjectRequest struct {
	Kind         *string `json:"kind,omitempty" binding:"omitempty,oneof=current upcoming"`
	Title        *string `json:"title,omitempty"`
	Label        *string `json:"label,omitempty"`
	Lede         *string `json:"lede,omitempty"`
	ImageURL     *string `json:"image_url,omitempty"`
	ImageVariant *string `json:"image_variant,omitempty" binding:"omitempty,oneof=default alt"`
	Blocks       *Blocks `json:"blocks,omitempty"`
	Position     *int    `json:"position,omitempty"`
	Published    *bool   `json:"published,omitempty"`
}
