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
	List(ctx context.Context, status string, livemodeOnly bool, limit, offset int) ([]Donation, error)
	PendingSessions(ctx context.Context, livemodeOnly bool) ([]string, error)
}

type Stripe interface {
	CreateCheckoutSession(ctx context.Context, amountCents int, email *string) (sessionID, url string, err error)
	VerifyWebhook(payload []byte, signatureHeader string) (stripe.Event, error)
	GetSession(ctx context.Context, sessionID string) (paymentStatus, paymentIntentID string, livemode bool, err error)
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

		if sess.PaymentStatus != stripe.CheckoutSessionPaymentStatusPaid &&
			sess.PaymentStatus != stripe.CheckoutSessionPaymentStatusNoPaymentRequired {
			return nil
		}
		return s.recordPaid(ctx, &sess)

	default:

	}

	return nil
}


func (s *Service) recordPaid(ctx context.Context, sess *stripe.CheckoutSession) error {
	var pi string
	if sess.PaymentIntent != nil {
		pi = sess.PaymentIntent.ID
	}
	var email, name *string
	if sess.CustomerDetails != nil {
		if sess.CustomerDetails.Email != "" {
			e := sess.CustomerDetails.Email
			email = &e
		}
		if sess.CustomerDetails.Name != "" {
			n := sess.CustomerDetails.Name
			name = &n
		}
	}
	currency := string(sess.Currency)
	if currency == "" {
		currency = "usd"
	}
	d := &Donation{
		AmountCents:           int(sess.AmountTotal),
		Currency:              currency,
		Email:                 email,
		Name:                  name,
		Status:                StatusPaid,
		StripeSessionID:       sess.ID,
		StripePaymentIntentID: &pi,
	}
	if err := s.repo.Create(ctx, d); err != nil {

		if errors.Is(err, ErrDuplicate) {
			return nil
		}
		return fmt.Errorf("record donation: %w", err)
	}
	return nil
}

func (s *Service) List(ctx context.Context, status string, livemodeOnly bool, limit, offset int) ([]Donation, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.List(ctx, status, livemodeOnly, limit, offset)
}

func (s *Service) Reconcile(ctx context.Context, livemodeOnly bool) (updated int, err error) {
	sessions, err := s.repo.PendingSessions(ctx, livemodeOnly)
	if err != nil {
		return 0, err
	}
	for _, sid := range sessions {
		payStatus, pi, _, err := s.stripe.GetSession(ctx, sid)
		if err != nil {
			continue
		}
		switch payStatus {
		case "paid", "no_payment_required":
			if err := s.repo.MarkPaid(ctx, sid, pi); err == nil {
				updated++
			}
		}
	}
	return updated, nil
}
