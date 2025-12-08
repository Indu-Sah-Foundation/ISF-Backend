package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/checkout/session"
	"github.com/stripe/stripe-go/v78/webhook"
)

var (
	stripeKey     string
	webhookSecret string
)

type DonationRequest struct {
	Amount       int64  `json:"amount" binding:"required,min=100"`
	Email        string `json:"email" binding:"required,email"`
	Name         string `json:"name" binding:"omitempty"`
	DonationType string `json:"donation_type" binding:"required"` // "preset" or "custom"
}

type DonationAmountsResponse struct {
	PresetAmounts []DonationOption `json:"preset_amounts"`
}

type DonationOption struct {
	Amount      int64  `json:"amount"`
	DisplayName string `json:"display_name"`
}

func init() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}
	stripeKey = os.Getenv("STRIPE_SECRET_KEY")
	if stripeKey == "" {
		log.Fatal("STRIPE_SECRET_KEY environment variable is not set")
	}
	webhookSecret = os.Getenv("STRIPE_WEBHOOK_SECRET")
	if webhookSecret == "" {
		log.Fatal("STRIPE_WEBHOOK_SECRET environment variable is not set")
	}
	stripe.Key = stripeKey
}

func main() {
	r := gin.Default()

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// Get available donation amounts
	r.GET("/donations/amounts", getDonationAmounts)

	// Create a checkout session for donation
	r.POST("/donations/checkout", createCheckoutSession)

	// Webhook endpoint for Stripe events
	r.POST("/donations/webhook", handleStripeWebhook)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Payments API listening on :%s\n", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func getDonationAmounts(c *gin.Context) {
	amounts := []DonationOption{
		{Amount: 100, DisplayName: "$1.00"},
		{Amount: 500, DisplayName: "$5.00"},
		{Amount: 1000, DisplayName: "$10.00"},
		{Amount: 2500, DisplayName: "$25.00"},
		{Amount: 10000, DisplayName: "$100.00"},
	}

	c.JSON(http.StatusOK, DonationAmountsResponse{
		PresetAmounts: amounts,
	})
}

func createCheckoutSession(c *gin.Context) {
	var req DonationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate amount
	if req.Amount < 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be at least $1.00 (100 cents)"})
		return
	}

	// For preset donations, validate against allowed amounts
	if req.DonationType == "preset" {
		validAmounts := map[int64]bool{
			100:   true,
			500:   true,
			1000:  true,
			2500:  true,
			10000: true,
		}
		if !validAmounts[req.Amount] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid preset amount"})
			return
		}
	}

	// Format the amount for display
	dollarAmount := float64(req.Amount) / 100.0

	// Get success/cancel URLs from environment or use defaults
	successURL := os.Getenv("DONATION_SUCCESS_URL")
	if successURL == "" {
		successURL = "https://isf.example.com/donation/success?session_id={CHECKOUT_SESSION_ID}"
	}

	cancelURL := os.Getenv("DONATION_CANCEL_URL")
	if cancelURL == "" {
		cancelURL = "https://isf.example.com/donation/cancel"
	}

	params := &stripe.CheckoutSessionParams{
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		Mode:               stripe.String("payment"),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String("usd"),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String("Donation to ISF"),
					},
					UnitAmount: stripe.Int64(req.Amount),
				},
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL:    stripe.String(successURL),
		CancelURL:     stripe.String(cancelURL),
		CustomerEmail: stripe.String(req.Email),
		Metadata: map[string]string{
			"donor_name":      req.Name,
			"donation_amount": fmt.Sprintf("$%.2f", dollarAmount),
			"donation_type":   req.DonationType,
		},
	}

	sess, err := session.New(params)
	if err != nil {
		log.Printf("session.New error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create checkout session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id": sess.ID,
		"url":        sess.URL,
	})
}

func handleStripeWebhook(c *gin.Context) {
	const MaxBodyBytes = int64(65536)
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxBodyBytes)

	body, err := c.GetRawData()
	if err != nil {
		log.Printf("error reading body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "request body too large"})
		return
	}

	sig := c.GetHeader("Stripe-Signature")
	event, err := webhook.ConstructEvent(body, sig, webhookSecret)
	if err != nil {
		log.Printf("webhook signature verification failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid signature"})
		return
	}

	switch event.Type {
	case "checkout.session.completed":
		var checkoutSession stripe.CheckoutSession
		err := json.Unmarshal(event.Data.Raw, &checkoutSession)
		if err != nil {
			log.Printf("error parsing checkout session: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "error parsing session"})
			return
		}

		log.Printf("Donation received: %s from %s (Amount: %s)",
			checkoutSession.ID,
			checkoutSession.CustomerEmail,
			checkoutSession.Metadata["donor_name"],
		)

		// TODO: Handle donation in your database/system
		// - Save to database
		// - Send confirmation email
		// - Update donor records, etc.

	case "payment_intent.payment_failed":
		var intent stripe.PaymentIntent
		err := json.Unmarshal(event.Data.Raw, &intent)
		if err != nil {
			log.Printf("error parsing payment intent: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "error parsing intent"})
			return
		}

		log.Printf("Payment failed for intent: %s - Reason: %v",
			intent.ID,
			intent.LastPaymentError,
		)

		// TODO: Handle failed payment
		// - Send notification email to donor
		// - Log failure details
		// - etc.

	default:
		log.Printf("Unhandled event type: %s", event.Type)
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}
