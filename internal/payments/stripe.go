package payments

import (
	"context"
	"fmt"
	"strings"

	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/checkout/session"
	"github.com/stripe/stripe-go/v78/webhook"
)

type StripeClient struct {
	successURL    string
	cancelURL     string
	webhookSecret string
	livemode      bool
}

func NewStripeClient(secretKey, webhookSecret, successURL, cancelURL string) *StripeClient {
	stripe.Key = secretKey
	return &StripeClient{
		successURL:    successURL,
		cancelURL:     cancelURL,
		webhookSecret: webhookSecret,
		livemode: strings.HasPrefix(secretKey, "sk_live_"),
	}
}

func (s *StripeClient) CreateCheckoutSession(ctx context.Context, amountCents int, email *string) (sessionID, url string, err error) {
	params := &stripe.CheckoutSessionParams{
		Mode: stripe.String(string(stripe.CheckoutSessionModePayment)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Quantity: stripe.Int64(1),
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String("usd"),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String("ISF Donation"),
					},
					UnitAmount: stripe.Int64(int64(amountCents)),
				},
			},
		},
		SuccessURL: stripe.String(s.successURL + "?session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:  stripe.String(s.cancelURL),
	}
	if email != nil && *email != "" {
		params.CustomerEmail = stripe.String(*email)
	}
	params.Context = ctx

	sess, err := session.New(params)
	if err != nil {
		return "", "", fmt.Errorf("stripe checkout: %w", err)
	}
	return sess.ID, sess.URL, nil
}

func (s *StripeClient) VerifyWebhook(payload []byte, signatureHeader string) (stripe.Event, error) {
	return webhook.ConstructEvent(payload, signatureHeader, s.webhookSecret)
}

func (s *StripeClient) Livemode() bool { return s.livemode }

func (s *StripeClient) GetSession(ctx context.Context, sessionID string) (paymentStatus, paymentIntentID string, livemode bool, err error) {
	params := &stripe.CheckoutSessionParams{}
	params.Context = ctx
	sess, err := session.Get(sessionID, params)
	if err != nil {
		return "", "", false, fmt.Errorf("stripe get session: %w", err)
	}
	var pi string
	if sess.PaymentIntent != nil {
		pi = sess.PaymentIntent.ID
	}
	return string(sess.PaymentStatus), pi, sess.Livemode, nil
}
