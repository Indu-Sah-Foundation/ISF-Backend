package people

import (
	"time"

	"github.com/google/uuid"
)

type Person struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type CreatePersonRequest struct {
	Name  string `json:"name" binding:"required,min=1,max=200"`
	Email string `json:"email" binding:"required,email"`
}
