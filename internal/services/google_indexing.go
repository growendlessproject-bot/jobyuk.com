package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type googleCredentials struct {
	Type        string `json:"type"`
	PrivateKey  string `json:"private_key"`
	ClientEmail string `json:"client_email"`
	TokenURI    string `json:"token_uri"`
}

// GoogleIndexingService handles notifying Google about new, updated, or deleted URLs.
type GoogleIndexingService struct {
	creds       *googleCredentials
	enabled     bool
	token       string
	tokenExpiry time.Time
	mu          sync.Mutex
	client      *http.Client
}

// NewGoogleIndexingService initializes the Google Indexing API service.
func NewGoogleIndexingService(credentialsJSON string) *GoogleIndexingService {
	svc := &GoogleIndexingService{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	var rawBytes []byte

	// 1. If credentialsJSON is empty, check standard file locations
	if credentialsJSON == "" {
		candidates := []string{
			"google-credentials.json",
			"google-key.json",
			"/var/www/JobyUK.com/google-credentials.json",
			"/var/www/JobyUK.com/google-key.json",
		}
		for _, path := range candidates {
			if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
				rawBytes = data
				slog.Info("Loaded Google credentials from file", "path", path)
				break
			}
		}
	} else {
		// 2. Check if credentialsJSON is a file path on disk
		if data, err := os.ReadFile(credentialsJSON); err == nil && len(data) > 0 {
			rawBytes = data
			slog.Info("Loaded Google credentials from path", "path", credentialsJSON)
		} else {
			// 3. Otherwise try direct bytes (or base64)
			rawBytes = []byte(credentialsJSON)
		}
	}

	if len(rawBytes) == 0 {
		slog.Warn("Google credentials not found. Indexing API integrations are disabled.")
		return svc
	}

	var creds googleCredentials
	if err := json.Unmarshal(rawBytes, &creds); err != nil {
		slog.Error("Failed to parse Google credentials JSON. Indexing API disabled.", "error", err)
		return svc
	}

	if creds.PrivateKey == "" || creds.ClientEmail == "" || creds.TokenURI == "" {
		slog.Error("Google credentials missing required fields (private_key, client_email, token_uri). Indexing API disabled.")
		return svc
	}

	svc.creds = &creds
	svc.enabled = true
	slog.Info("Google Indexing API service initialized successfully.", "client_email", creds.ClientEmail)
	return svc
}

// IsEnabled returns true if Google Indexing service is configured and ready.
func (s *GoogleIndexingService) IsEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enabled
}

// GetClientEmail returns the configured service account email.
func (s *GoogleIndexingService) GetClientEmail() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.creds != nil {
		return s.creds.ClientEmail
	}
	return ""
}

// SaveAndReload writes the JSON credentials to disk and activates the service in memory.
func (s *GoogleIndexingService) SaveAndReload(rawBytes []byte) error {
	var creds googleCredentials
	if err := json.Unmarshal(rawBytes, &creds); err != nil {
		return fmt.Errorf("invalid JSON format: %w", err)
	}

	if creds.PrivateKey == "" || creds.ClientEmail == "" || creds.TokenURI == "" {
		return fmt.Errorf("JSON is missing required fields (private_key, client_email, token_uri)")
	}

	// Write to disk in current directory and production path if available
	_ = os.WriteFile("google-credentials.json", rawBytes, 0600)
	if _, err := os.Stat("/var/www/JobyUK.com"); err == nil {
		_ = os.WriteFile("/var/www/JobyUK.com/google-credentials.json", rawBytes, 0600)
	}

	s.mu.Lock()
	s.creds = &creds
	s.enabled = true
	s.token = ""
	s.tokenExpiry = time.Time{}
	s.mu.Unlock()

	slog.Info("Google Indexing API credentials updated and activated successfully", "client_email", creds.ClientEmail)
	return nil
}

// PublishUpdate notifies Google of a new or updated URL.
func (s *GoogleIndexingService) PublishUpdate(ctx context.Context, targetURL string) error {
	return s.notify(ctx, targetURL, "URL_UPDATED")
}

// PublishDelete notifies Google of a deleted or expired URL.
func (s *GoogleIndexingService) PublishDelete(ctx context.Context, targetURL string) error {
	return s.notify(ctx, targetURL, "URL_DELETED")
}

func (s *GoogleIndexingService) notify(ctx context.Context, targetURL, notificationType string) error {
	if !s.enabled {
		return nil
	}

	token, err := s.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get Google API access token: %w", err)
	}

	payload := map[string]string{
		"url":  targetURL,
		"type": notificationType,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://indexing.googleapis.com/v3/urlNotifications:publish", bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		slog.Error("Google Indexing API call failed", "status", resp.Status, "response", errResp, "url", targetURL, "type", notificationType)
		return fmt.Errorf("google indexing API returned status %s", resp.Status)
	}

	slog.Info("Notified Google Indexing API of URL action", "url", targetURL, "type", notificationType)
	return nil
}

func (s *GoogleIndexingService) getAccessToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If token is still valid (with a 2-minute buffer), return it
	if s.token != "" && time.Now().Add(2*time.Minute).Before(s.tokenExpiry) {
		return s.token, nil
	}

	// Generate JWT assertion for OAuth2 assertion profile
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(s.creds.PrivateKey))
	if err != nil {
		return "", fmt.Errorf("failed to parse RSA private key: %w", err)
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   s.creds.ClientEmail,
		"scope": "https://www.googleapis.com/auth/indexing",
		"aud":   s.creds.TokenURI,
		"exp":   now.Add(1 * time.Hour).Unix(),
		"iat":   now.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	assertion, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign assertion: %w", err)
	}

	// Post assertion to request token
	resp, err := s.client.PostForm(s.creds.TokenURI, url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {assertion},
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch access token from Google: status %s", resp.Status)
	}

	var res struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}

	s.token = res.AccessToken
	s.tokenExpiry = now.Add(time.Duration(res.ExpiresIn) * time.Second)

	return s.token, nil
}
