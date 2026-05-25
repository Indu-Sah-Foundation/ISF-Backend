package contacts

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("not found")


type Contact struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Message   string    `json:"message"`
	IP        string    `json:"ip"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateRequest struct {
	Email   string `json:"email" binding:"required,email,max=320"`
	Message string `json:"message" binding:"required,min=1,max=5000"`
}

type UpdateRequest struct {
	Read *bool `json:"read,omitempty"`
}
