package main

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidateDonationAmount validates that the amount is within acceptable range
func ValidateDonationAmount(amount int64) bool {
	return amount >= 100 // Minimum $1.00
}

// ValidateEmail checks if email format is valid
func ValidateEmail(email string) bool {
	if email == "" {
		return false
	}
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return re.MatchString(strings.ToLower(email))
}

// IsPresetAmount checks if the amount matches one of the preset donation amounts
func IsPresetAmount(amount int64) bool {
	presetAmounts := map[int64]bool{
		100:   true, // $1.00
		500:   true, // $5.00
		1000:  true, // $10.00
		2500:  true, // $25.00
		10000: true, // $100.00
	}
	return presetAmounts[amount]
}

// FormatAmountAsUSD converts cents to USD string format
func FormatAmountAsUSD(cents int64) string {
	dollars := float64(cents) / 100.0
	if cents%100 == 0 {
		return fmt.Sprintf("$%.0f.00", dollars)
	}
	return fmt.Sprintf("$%.2f", dollars)
}
