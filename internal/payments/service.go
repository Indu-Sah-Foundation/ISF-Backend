package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/stripe/stripe-go/v78"
)

type Repository interface {
	Create(ctx context.Context, d *Donation) error
	MarkPaid(ctx context.Context, sessionID, paymentIntentID string) error
	MarkFailed(ctx context.Context, sessionID string) error
	RecordEvent(ctx context.Context, eventID string) (bool, error)
	List(ctx context.Context, status string, limit, offset int) ([]Donation, error)
}

type Stripe interface {
	CreateCheckoutSession(ctx context.Context, amountCents int, email *string) (sessionID, url string, err error)
	VerifyWebhook(payload []byte, signatureHeader string) (stripe.Event, error)
}

type Service struct {
	repo   Repository
	stripe Stripe
}

func NewService(repo Repository, s Stripe) *Service {
	return &Service{repo: repo, stripe: s}
}

func (s *Service) Checkout(ctx context.Context, req CheckoutRequest) (*CheckoutResponse, error) {
	sessionID, url, err := s.stripe.CreateCheckoutSession(ctx, req.AmountCents, req.Email)
	if err != nil {
		return nil, err
	}
	d := &Donation{
		AmountCents:     req.AmountCents,
		Currency:        "usd",
		Email:           req.Email,
		Name:            req.Name,
		Status:          StatusPending,
		StripeSessionID: sessionID,
	}
	if err := s.repo.Create(ctx, d); err != nil {
		return nil, fmt.Errorf("create donation: %w", err)
	}
	return &CheckoutResponse{URL: url, SessionID: sessionID}, nil
}

func (s *Service) HandleWebhook(ctx context.Context, payload []byte, signature string) error {
	evt, err := s.stripe.VerifyWebhook(payload, signature)
	if err != nil {
		return fmt.Errorf("verify webhook: %w", err)
	}
	first, err := s.repo.RecordEvent(ctx, evt.ID)
	if err != nil {
		return err
	}
	if !first {
		return nil
	}

	switch evt.Type {
	case "checkout.session.completed":
		var sess stripe.CheckoutSession
		if err := json.Unmarshal(evt.Data.Raw, &sess); err != nil {
			return fmt.Errorf("decode checkout session: %w", err)
		}
		var pi string
		if sess.PaymentIntent != nil {
			pi = sess.PaymentIntent.ID
		}
		if err := s.repo.MarkPaid(ctx, sess.ID, pi); err != nil {
			if errors.Is(err, ErrNotFound) {
				return fmt.Errorf("mark paid: donation row not found for session %s", sess.ID)
			}
			return err
		}

	case "checkout.session.async_payment_failed", "checkout.session.expired":
		var sess stripe.CheckoutSession
		if err := json.Unmarshal(evt.Data.Raw, &sess); err != nil {
			return fmt.Errorf("decode checkout session: %w", err)
		}
		_ = s.repo.MarkFailed(ctx, sess.ID)

	default:
	}

	return nil
}

func (s *Service) List(ctx context.Context, status string, limit, offset int) ([]Donation, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.List(ctx, status, limit, offset)
}
