package payments

import (
	"time"

	"github.com/google/uuid"
)

const (
	StatusPending  = "pending"
	StatusPaid     = "paid"
	StatusFailed   = "failed"
	StatusRefunded = "refunded"
)

type Donation struct {
	ID                    uuid.UUID `json:"id"`
	AmountCents           int       `json:"amount_cents"`
	Currency              string    `json:"currency"`
	Email                 *string   `json:"email"`
	Name                  *string   `json:"name"`
	Status                string    `json:"status"`
	Livemode              bool      `json:"livemode"`
	StripeSessionID       string    `json:"stripe_session_id"`
	StripePaymentIntentID *string   `json:"stripe_payment_intent_id,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type CheckoutRequest struct {
	AmountCents int     `json:"amount_cents" binding:"required,min=100,max=10000000"`
	Email       *string `json:"email,omitempty" binding:"omitempty,email"`
	Name        *string `json:"name,omitempty" binding:"omitempty,max=200"`
}

type CheckoutResponse struct {
	URL       string `json:"url"`
	SessionID string `json:"session_id"`
}
