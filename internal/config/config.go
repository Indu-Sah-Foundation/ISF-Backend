// Package config centralizes all environment-variable loading.
// Every other package receives a *Config rather than reading env vars itself.
// This makes the code testable (you can construct a Config in a test) and
// makes it obvious in one file what the app needs to run.
package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config holds every setting the backend needs.
// Field names are exported (capitalized) so other packages can read them.
type Config struct {
	// Port the HTTP server listens on. Azure App Service sets PORT for us.
	Port string

	// Postgres connection string, e.g.
	// postgres://user:pass@host:5432/db?sslmode=require
	DatabaseURL string

	// JWT signing secret. Loaded from Key Vault in production.
	JWTSecret string

	// GinMode is "debug" locally, "release" in production.
	GinMode string

	RedisURL string

	TranslatorEndpoint string
	TranslatorKey      string
	TranslatorRegion   string
	AdminEmail         string
	AdminPassword      string
}

// Load reads environment variables and returns a populated *Config.
// It returns an error
// if any *required* variable is missing
// then fail fast startup rather than crashing on the first request.
func Load() (*Config, error) {
	// Load .env file if it exists (ignore error if not found)
	_ = godotenv.Load()

	cfg := &Config{
		Port:               getEnvOr("PORT", "8080"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		JWTSecret:          os.Getenv("JWT_SECRET"),
		GinMode:            getEnvOr("GIN_MODE", "debug"),
		RedisURL:           os.Getenv("REDIS_URL"),
		TranslatorEndpoint: getEnvOr("TRANSLATOR_ENDPOINT", "https://api.cognitive.microsofttranslator.com"),
		TranslatorKey:      os.Getenv("TRANSLATOR_KEY"),
		TranslatorRegion:   os.Getenv("TRANSLATOR_REGION"),
		AdminEmail:         os.Getenv("ADMIN_EMAIL"),
		AdminPassword:      os.Getenv("ADMIN_PASSWORD"),
	}

	// Validate required fields.
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if cfg.TranslatorKey == "" {
		return nil, fmt.Errorf("TRANSLATOR_KEY is required")
	}
	if cfg.TranslatorRegion == "" {
		return nil, fmt.Errorf("TRANSLATOR_REGION is required")
	}

	return cfg, nil
}

// getEnvOr returns the env var if set, otherwise the fallback.
// Lowercase first letter = unexported = only usable inside this package.
func getEnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
