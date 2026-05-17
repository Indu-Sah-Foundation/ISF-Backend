package team

import (
	"time"

	"github.com/google/uuid"
)

// Member is a single person on the About page (founder, advisor, or board).
// The Kind enum switches which fields the frontend renders:
//
//   founder / advisor_*  -> full PersonCard (role + extras + bio + optional motto)
//   board                -> compact MediaCard (just name + role)
type Member struct {
	ID        uuid.UUID `json:"id"`
	Kind      string    `json:"kind"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	Extras    string    `json:"extras"`
	Bio       string    `json:"bio"`
	Motto     string    `json:"motto"`
	ImageURL  string    `json:"image_url"`
	Position  int       `json:"position"`
	Published bool      `json:"published"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateMemberRequest struct {
	Kind      string `json:"kind" binding:"required,oneof=founder advisor_intl advisor_nat board"`
	Name      string `json:"name" binding:"required,min=1,max=300"`
	Role      string `json:"role" binding:"omitempty,max=500"`
	Extras    string `json:"extras"`
	Bio       string `json:"bio"`
	Motto     string `json:"motto"`
	ImageURL  string `json:"image_url"`
	Position  int    `json:"position"`
	Published *bool  `json:"published,omitempty"`
}

type UpdateMemberRequest struct {
	Kind      *string `json:"kind,omitempty" binding:"omitempty,oneof=founder advisor_intl advisor_nat board"`
	Name      *string `json:"name,omitempty"`
	Role      *string `json:"role,omitempty"`
	Extras    *string `json:"extras,omitempty"`
	Bio       *string `json:"bio,omitempty"`
	Motto     *string `json:"motto,omitempty"`
	ImageURL  *string `json:"image_url,omitempty"`
	Position  *int    `json:"position,omitempty"`
	Published *bool   `json:"published,omitempty"`
}
