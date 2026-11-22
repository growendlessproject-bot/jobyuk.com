package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"
)

const (
	csrfCookieName = "csrf_token"
	csrfFieldName  = "csrf_token"
	csrfHeaderName = "X-CSRF-Token"
	csrfMaxAge     = 3600 // 1 hour
)

type csrfContextKey string
const CSRFTokenKey csrfContextKey = "csrf_token"

// CSRFProtect returns middleware that enforces CSRF tokens on state-changing requests.
// It generates a token on GET requests and validates it on POST/PUT/DELETE.
func CSRFProtect(secret string, isDev bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				// Safe methods: ensure a CSRF cookie exists
				var token string
				cookie, err := r.Cookie(csrfCookieName)
				if err != nil || !validateCSRFToken(cookie.Value, secret) {
					// Generate and set a new CSRF cookie
					token = generateCSRFToken(secret)
					http.SetCookie(w, &http.Cookie{
						Name:     csrfCookieName,
						Value:    token,
						Path:     "/",
						MaxAge:   csrfMaxAge,
						HttpOnly: false, // JS needs to read this for AJAX
						Secure:   !isDev,
						SameSite: http.SameSiteStrictMode,
					})
				} else {
					token = cookie.Value
				}
				ctx := context.WithValue(r.Context(), CSRFTokenKey, token)
				next.ServeHTTP(w, r.WithContext(ctx))

			case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
				// State-changing methods: validate CSRF token
				submittedToken := r.FormValue(csrfFieldName)
				if submittedToken == "" && r.MultipartForm != nil && len(r.MultipartForm.Value[csrfFieldName]) > 0 {
					submittedToken = r.MultipartForm.Value[csrfFieldName][0]
				}
				if submittedToken == "" {
					submittedToken = r.Header.Get(csrfHeaderName)
				}
				if submittedToken == "" {
					submittedToken = r.URL.Query().Get(csrfFieldName)
				}
				if submittedToken == "" {
					if cookie, err := r.Cookie(csrfCookieName); err == nil {
						submittedToken = cookie.Value
					}
				}

				if submittedToken == "" {
					http.Error(w, "CSRF validation failed: missing token", http.StatusForbidden)
					return
				}

				if !validateCSRFToken(submittedToken, secret) {
					http.Error(w, "CSRF validation failed: invalid token", http.StatusForbidden)
					return
				}

				next.ServeHTTP(w, r)

			default:
				next.ServeHTTP(w, r)
			}
		})
	}
}

// generateCSRFToken creates an HMAC-signed token with embedded timestamp.
func generateCSRFToken(secret string) string {
	// Generate 16 bytes of randomness
	randomBytes := make([]byte, 16)
	_, _ = rand.Read(randomBytes)
	nonce := hex.EncodeToString(randomBytes)

	// Embed timestamp for expiry checking
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	payload := nonce + "|" + timestamp

	// Sign with HMAC-SHA256
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	signature := hex.EncodeToString(mac.Sum(nil))

	return payload + "|" + signature
}

// validateCSRFToken verifies the HMAC signature and checks expiry.
func validateCSRFToken(token, secret string) bool {
	// Token format: nonce|timestamp|signature
	parts := splitCSRFToken(token)
	if parts == nil {
		return false
	}

	nonce, timestamp, signature := parts[0], parts[1], parts[2]

	// Verify signature
	payload := nonce + "|" + timestamp
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		return false
	}

	// Check expiry (1 hour)
	var ts int64
	_, err := fmt.Sscanf(timestamp, "%d", &ts)
	if err != nil {
		return false
	}
	if time.Now().Unix()-ts > csrfMaxAge {
		return false
	}

	return true
}

// splitCSRFToken splits a token into its 3 parts.
func splitCSRFToken(token string) []string {
	first := -1
	second := -1
	for i, c := range token {
		if c == '|' {
			if first == -1 {
				first = i
			} else if second == -1 {
				second = i
				break
			}
		}
	}
	if first == -1 || second == -1 {
		return nil
	}
	return []string{token[:first], token[first+1 : second], token[second+1:]}
}

// GetCSRFToken retrieves the current CSRF token from the request context or cookie.
// Returns empty string if no token is set.
func GetCSRFToken(r *http.Request) string {
	if val, ok := r.Context().Value(CSRFTokenKey).(string); ok {
		return val
	}
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}
