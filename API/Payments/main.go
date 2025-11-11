// package main

// import (
// 	"crypto/hmac"
// 	"crypto/sha256"
// 	"encoding/hex"
// 	"encoding/json"
// 	"fmt"
// 	"log"
// 	"net/http"
// 	"os"
// 	"strconv"
// 	"strings"
// 	"time"

// 	"github.com/gin-gonic/gin"
// 	"github.com/joho/godotenv"
// 	"github.com/stripe/stripe-go/v78"
// 	"github.com/stripe/stripe-go/v78/checkout/session"
// 	"github.com/stripe/stripe-go/v78/webhook"
// )

// var (
// 	stripeKey string
// 	webhookSecret string
// )

// type DonationRequest struct {
// 	Amount int64 `json:"amount" binding:"required,min=100"`;
// 	Email    string `json:"email" binding:"required,email"`
// 	Name     string `json:"name" binding:"omitempty"`
// }


// func init() {
// 	if err := godotenv.Load(); err != nil {
// 		log.Println("No .env file found")
// 	}
// 	stripeKey = os.Getenv("STRIPE_SECRET_KEY")
// 	webhookSecret = os.Getenv("STRIPE_WEBHOOK_SECRET")
// 	stripe.Key = stripeKey
// }

// func createCheckoutSession(c *gin.Context) {
// 	var req DonationRequest
// 	if err := c.ShouldBindJSON(&req); err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
// 		return
// 	}

// 	params := &stripe.CheckoutSessionParams{
// 		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
// 		Mode:               stripe.String("payment"), // One-time; use "subscription" for recurring
// 		LineItems: []*stripe.CheckoutSessionLineItemParams{
// 			{
// 				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
// 					Currency:     stripe.String("usd"),
// 					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
// 						Name: stripe.String("Donation to [Your Foundation]"),
// 					},
// 					UnitAmount: stripe.Int64(req.Amount),
// 				},
// 				Quantity: stripe.Int64(1),
// 			},
// 		},
// 		SuccessURL: stripe.String("https://your-foundation-site.com/success?session_id={CHECKOUT_SESSION_ID}"),
// 		CancelURL:  stripe.String("https://your-foundation-site.com/cancel"),
// 		CustomerEmail: stripe.String(req.Email),
// 	}

// 	Metadata: map[string]string{
// 		"donor_name": req.Name,
// 	},

// }