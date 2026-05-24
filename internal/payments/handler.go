package payments

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"isf-backend/internal/httperr"
)

func friendlyValidationError(err error) string {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) || len(ve) == 0 {
		return err.Error()
	}
	fe := ve[0]
	switch fe.Field() {
	case "AmountCents":
		switch fe.Tag() {
		case "required":
			return "Please choose a donation amount."
		case "min":
			return "The minimum donation is $1."
		case "max":
			return "Donations are capped at $100,000. For larger gifts, please contact us directly."
		}
		return "Donation amount is invalid."
	case "Email":
		return "Please enter a valid email address."
	}
	return fmt.Sprintf("%s is invalid (%s).", fe.Field(), fe.Tag())
}

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(r *gin.Engine, checkoutLimiter, adminMW gin.HandlerFunc) {
	g := r.Group("/donations")
	g.GET("/amounts", h.amounts)

	// Apply rate limit ONLY to /checkout to discourage Stripe-spamming bots.
	if checkoutLimiter != nil {
		g.POST("/checkout", checkoutLimiter, h.checkout)
	} else {
		g.POST("/checkout", h.checkout)
	}

	g.POST("/webhook", h.webhook)

	if adminMW != nil {
		admin := g.Group("", adminMW)
		admin.GET("", h.list)
	} else {
		g.GET("", h.list)
	}
}

func (h *Handler) amounts(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"currency":      "usd",
		"amounts_cents": []int{1000, 2500, 5000, 10000, 25000},
	})
}

func (h *Handler) checkout(c *gin.Context) {
	var req CheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, friendlyValidationError(err))
		return
	}
	resp, err := h.svc.Checkout(c.Request.Context(), req)
	if err != nil {
		// Stripe API failed -- 502 (upstream) and log internally
		httperr.Respond(c, httperr.ErrUpstream, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) webhook(c *gin.Context) {
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		httperr.Respond(c, httperr.ErrBadRequest.With("read body"), err)
		return
	}
	sig := c.GetHeader("Stripe-Signature")
	if err := h.svc.HandleWebhook(c.Request.Context(), payload, sig); err != nil {
		// Return 400 on signature failure (Stripe won't retry forever) and log.
		httperr.Respond(c, httperr.ErrBadRequest.With("webhook rejected"), err)
		return
	}
	c.Status(http.StatusOK)
}

func (h *Handler) list(c *gin.Context) {
	status := c.Query("status")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	out, err := h.svc.List(c.Request.Context(), status, limit, offset)
	if err != nil {
		httperr.Respond(c, httperr.ErrInternal, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items":  out,
		"limit":  limit,
		"offset": offset,
		"count":  len(out),
	})
}
