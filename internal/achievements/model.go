package achievements

import (
	"time"

	"github.com/google/uuid"
)

type Achievement struct {
	ID           uuid.UUID  `json:"id"`
	Slot         string     `json:"slot"`
	Title        string     `json:"title"`
	Organization string     `json:"organization"`
	Place        string     `json:"place"`
	Body         string     `json:"body"`
	ImageURL     string     `json:"image_url"`
	AwardedAt    *time.Time `json:"awarded_at,omitempty"`
	Position     int        `json:"position"`
	Published    bool       `json:"published"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type CreateAchievementRequest struct {
	Slot         string     `json:"slot" binding:"required,min=1,max=200"`
	Title        string     `json:"title" binding:"required,min=1,max=300"`
	Organization string     `json:"organization" binding:"omitempty,max=300"`
	Place        string     `json:"place" binding:"omitempty,max=300"`
	Body         string     `json:"body"`
	ImageURL     string     `json:"image_url"`
	AwardedAt    *time.Time `json:"awarded_at,omitempty"`
	Position     int        `json:"position"`
	Published    *bool      `json:"published,omitempty"`
}

type UpdateAchievementRequest struct {
	Title        *string    `json:"title,omitempty"`
	Organization *string    `json:"organization,omitempty"`
	Place        *string    `json:"place,omitempty"`
	Body         *string    `json:"body,omitempty"`
	ImageURL     *string    `json:"image_url,omitempty"`
	AwardedAt    *time.Time `json:"awarded_at,omitempty"`
	Position     *int       `json:"position,omitempty"`
	Published    *bool      `json:"published,omitempty"`
}
