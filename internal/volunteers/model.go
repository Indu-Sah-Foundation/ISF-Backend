package volunteers

import (
	"time"

	"github.com/google/uuid"
)

type Volunteer struct {
	ID        uuid.UUID `json:"id"`
	Kind      string    `json:"kind"`
	Name      string    `json:"name"`
	Bio       string    `json:"bio"`
	ImageURL  string    `json:"image_url"`
	Position  int       `json:"position"`
	Published bool      `json:"published"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateVolunteerRequest struct {
	Kind      string `json:"kind" binding:"required,oneof=team volunteer_field research_field"`
	Name      string `json:"name" binding:"required,min=1,max=300"`
	Bio       string `json:"bio"`
	ImageURL  string `json:"image_url"`
	Position  int    `json:"position"`
	Published *bool  `json:"published,omitempty"`
}

type UpdateVolunteerRequest struct {
	Kind      *string `json:"kind,omitempty" binding:"omitempty,oneof=team volunteer_field research_field"`
	Name      *string `json:"name,omitempty"`
	Bio       *string `json:"bio,omitempty"`
	ImageURL  *string `json:"image_url,omitempty"`
	Position  *int    `json:"position,omitempty"`
	Published *bool   `json:"published,omitempty"`
}
