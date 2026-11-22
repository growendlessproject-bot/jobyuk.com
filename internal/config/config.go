package config

import (
	"fmt"
	"os"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// Server
	Port    string
	BaseURL string
	Env     string // "development" or "production"

	// Database
	DatabaseURL string

	// Redis
	RedisURL string

	// Reed API
	ReedAPIKey string

	// Security
	JWTSecret  string
	CSRFSecret string

	// Admin
	AdminUsername string
	AdminPassword string

	// Google Indexing API
	GoogleCredentialsJSON string

	// Analytics
	AnalyticsSalt string

	// Features
	EnableWhatsAppAlerts bool
	IndexNowKey          string
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		Port:         getEnv("PORT", "8080"),
		BaseURL:      getEnv("BASE_URL", "http://localhost:8080"),
		Env:          getEnv("ENV", "development"),
		DatabaseURL:  getEnv("DATABASE_URL", "postgres://jobyuk:jobyuk@localhost:5432/jobyuk?sslmode=disable"),
		RedisURL:     getEnv("REDIS_URL", "redis://localhost:6379/0"),
		ReedAPIKey:   os.Getenv("REED_API_KEY"),
		JWTSecret:    getEnv("JWT_SECRET", "change-me-in-production-please"),
		CSRFSecret:   getEnv("CSRF_SECRET", "change-me-csrf-secret-please"),
		AdminUsername: getEnv("ADMIN_USERNAME", "admin"),
		AdminPassword: getEnv("ADMIN_PASSWORD", ""),
		GoogleCredentialsJSON: os.Getenv("GOOGLE_CREDENTIALS_JSON"),
		AnalyticsSalt:        getEnv("ANALYTICS_SALT", "jobyuk-default-analytics-salt-key"),
		IndexNowKey:          getEnv("INDEXNOW_KEY", "ab4225315c73f110cb98d2abd9911d76"),
	}

	// Validate required config
	if cfg.ReedAPIKey == "" {
		return nil, fmt.Errorf("REED_API_KEY environment variable is required")
	}

	if cfg.Env == "production" {
		if cfg.JWTSecret == "change-me-in-production-please" {
			return nil, fmt.Errorf("JWT_SECRET must be set in production")
		}
		if cfg.AdminPassword == "" {
			return nil, fmt.Errorf("ADMIN_PASSWORD must be set in production")
		}
	}

	return cfg, nil
}

// IsDev returns true if running in development mode.
func (c *Config) IsDev() bool {
	return c.Env == "development"
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
