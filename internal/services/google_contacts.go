package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// GoogleContactsToken stores the OAuth 2.0 access & refresh tokens.
type GoogleContactsToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
	TokenType    string    `json:"token_type"`
	AccountEmail string    `json:"account_email,omitempty"`
}

// GoogleContactsService manages Google People API synchronization for contacts.
type GoogleContactsService struct {
	clientID     string
	clientSecret string
	redirectURI  string
	tokenPath    string
	token        *GoogleContactsToken
	mu           sync.RWMutex
	httpClient   *http.Client
}

// NewGoogleContactsService initializes the Google Contacts Sync Service.
func NewGoogleContactsService(clientID, clientSecret, redirectURI string) *GoogleContactsService {
	svc := &GoogleContactsService{
		clientID:     strings.TrimSpace(clientID),
		clientSecret: strings.TrimSpace(clientSecret),
		redirectURI:  strings.TrimSpace(redirectURI),
		tokenPath:    filepath.Join("data", "google_contacts_token.json"),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}

	_ = os.MkdirAll("data", 0755)
	svc.loadToken()
	return svc
}

// loadToken reads stored token from data/google_contacts_token.json.
func (s *GoogleContactsService) loadToken() {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.tokenPath)
	if err != nil {
		return
	}

	var tok GoogleContactsToken
	if err := json.Unmarshal(data, &tok); err == nil && tok.RefreshToken != "" {
		s.token = &tok
		slog.Info("Google Contacts API token loaded successfully", "has_refresh", tok.RefreshToken != "", "account", tok.AccountEmail)
	}
}

// saveToken persists token to disk.
func (s *GoogleContactsService) saveToken(tok *GoogleContactsToken) error {
	s.mu.Lock()
	s.token = tok
	s.mu.Unlock()

	data, err := json.MarshalIndent(tok, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.tokenPath, data, 0600)
}

// IsConnected returns true if Google Contacts has an active authorized token.
func (s *GoogleContactsService) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.token != nil && s.token.RefreshToken != ""
}

// GetAccountEmail returns the authorized Google account email if known.
func (s *GoogleContactsService) GetAccountEmail() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.token != nil {
		return s.token.AccountEmail
	}
	return ""
}

// GetAuthURL generates the Google OAuth 2.0 consent authorization URL.
func (s *GoogleContactsService) GetAuthURL() string {
	scope := "https://www.googleapis.com/auth/contacts https://www.googleapis.com/auth/userinfo.email"
	return fmt.Sprintf(
		"https://accounts.google.com/o/oauth2/v2/auth?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&access_type=offline&prompt=consent",
		url.QueryEscape(s.clientID),
		url.QueryEscape(s.redirectURI),
		url.QueryEscape(scope),
	)
}

// ExchangeCode trades the OAuth 2.0 authorization code for access and refresh tokens.
func (s *GoogleContactsService) ExchangeCode(ctx context.Context, code string) error {
	vals := url.Values{}
	vals.Set("client_id", s.clientID)
	vals.Set("client_secret", s.clientSecret)
	vals.Set("code", code)
	vals.Set("grant_type", "authorization_code")
	vals.Set("redirect_uri", s.redirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/token", strings.NewReader(vals.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("token exchange network error: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("google token exchange failed (%d): %s", resp.StatusCode, string(body))
	}

	var raw struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return fmt.Errorf("failed to parse token response: %w", err)
	}

	if raw.RefreshToken == "" {
		s.mu.RLock()
		if s.token != nil && s.token.RefreshToken != "" {
			raw.RefreshToken = s.token.RefreshToken
		}
		s.mu.RUnlock()
	}

	tok := &GoogleContactsToken{
		AccessToken:  raw.AccessToken,
		RefreshToken: raw.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(raw.ExpiresIn-60) * time.Second),
		TokenType:    raw.TokenType,
	}

	// Try fetching account email
	email := s.fetchAccountEmail(ctx, tok.AccessToken)
	tok.AccountEmail = email

	if err := s.saveToken(tok); err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}

	slog.Info("Google Contacts OAuth 2.0 Connected Successfully!", "email", email)
	return nil
}

// fetchAccountEmail retrieves user's email address from userinfo endpoint.
func (s *GoogleContactsService) fetchAccountEmail(ctx context.Context, accessToken string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return ""
	}
	defer resp.Body.Close()

	var info struct {
		Email string `json:"email"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&info)
	return info.Email
}

// getValidAccessToken returns an active access token, refreshing if necessary.
func (s *GoogleContactsService) getValidAccessToken(ctx context.Context) (string, error) {
	s.mu.RLock()
	tok := s.token
	s.mu.RUnlock()

	if tok == nil || tok.RefreshToken == "" {
		return "", fmt.Errorf("google contacts not connected")
	}

	if tok.AccessToken != "" && time.Now().Before(tok.Expiry) {
		return tok.AccessToken, nil
	}

	// Token expired, refresh it
	vals := url.Values{}
	vals.Set("client_id", s.clientID)
	vals.Set("client_secret", s.clientSecret)
	vals.Set("refresh_token", tok.RefreshToken)
	vals.Set("grant_type", "refresh_token")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/token", strings.NewReader(vals.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token refresh network error: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token refresh failed (%d): %s", resp.StatusCode, string(body))
	}

	var raw struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", fmt.Errorf("failed to parse refresh response: %w", err)
	}

	tok.AccessToken = raw.AccessToken
	tok.Expiry = time.Now().Add(time.Duration(raw.ExpiresIn-60) * time.Second)

	_ = s.saveToken(tok)
	return tok.AccessToken, nil
}

// CreateContact creates or updates a contact directly in Google Contacts (People API).
// Once created, Android phones logged into this Google Account sync it automatically in seconds!
func (s *GoogleContactsService) CreateContact(ctx context.Context, phone, fullName string) error {
	accessToken, err := s.getValidAccessToken(ctx)
	if err != nil {
		return err
	}

	phone = strings.TrimSpace(phone)
	fullName = strings.TrimSpace(fullName)
	if fullName == "" {
		suffix := phone
		if len(phone) > 4 {
			suffix = phone[len(phone)-4:]
		}
		fullName = fmt.Sprintf("Candidate %s", suffix)
	}

	firstName := fullName
	lastName := "JobyUK.com"
	if parts := strings.SplitN(fullName, " ", 2); len(parts) == 2 {
		firstName = parts[0]
		lastName = parts[1]
	}

	payload := map[string]interface{}{
		"names": []map[string]interface{}{
			{
				"givenName":   firstName,
				"familyName":  lastName,
				"displayName": fmt.Sprintf("%s (JobyUK.com)", fullName),
			},
		},
		"phoneNumbers": []map[string]interface{}{
			{
				"value": phone,
				"type":  "mobile",
			},
		},
		"organizations": []map[string]interface{}{
			{
				"name":  "JobyUK.com Candidate",
				"title": "Job Alerts Candidate",
			},
		},
		"biographies": []map[string]interface{}{
			{
				"value":       "JobyUK.com Candidate auto-synced via Google People API",
				"contentType": "TEXT_PLAIN",
			},
		},
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://people.googleapis.com/v1/people:createContact", bytes.NewReader(jsonBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("create contact network error: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		slog.Warn("Google People API createContact warning", "status", resp.StatusCode, "response", string(respBody), "phone", phone)
		return fmt.Errorf("google people api error (%d): %s", resp.StatusCode, string(respBody))
	}

	slog.Info("Contact Synced Successfully to Google Contacts!", "name", fullName, "phone", phone)
	return nil
}

// Disconnect removes stored Google OAuth tokens.
func (s *GoogleContactsService) Disconnect() error {
	s.mu.Lock()
	s.token = nil
	s.mu.Unlock()
	_ = os.Remove(s.tokenPath)
	slog.Info("Google Contacts disconnected")
	return nil
}
