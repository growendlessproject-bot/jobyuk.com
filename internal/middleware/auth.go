package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const AdminUserKey contextKey = "admin_user"

// AdminClaims holds JWT claims for admin authentication.
type AdminClaims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// AdminAuth validates JWT tokens for admin routes.
func AdminAuth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check for JWT in cookie first, then Authorization header
			var tokenString string

			cookie, err := r.Cookie("admin_token")
			if err == nil {
				tokenString = cookie.Value
			} else {
				// Fallback to Authorization header
				auth := r.Header.Get("Authorization")
				if strings.HasPrefix(auth, "Bearer ") {
					tokenString = strings.TrimPrefix(auth, "Bearer ")
				}
			}

			if tokenString == "" {
				http.Redirect(w, r, "/admin-lol/login", http.StatusSeeOther)
				return
			}

			// Parse and validate token
			claims := &AdminClaims{}
			token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				// Validate signing method
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(jwtSecret), nil
			})

			if err != nil || !token.Valid {
				// Clear expired/invalid cookie across all paths
				http.SetCookie(w, &http.Cookie{
					Name:     "admin_token",
					Value:    "",
					Path:     "/",
					MaxAge:   -1,
					HttpOnly: true,
					Secure:   true,
					SameSite: http.SameSiteLaxMode,
				})
				// Specific error message for inactive session expiration
				http.Redirect(w, r, "/admin-lol/login?error=Your+session+has+expired+due+to+15+minutes+of+inactivity.+Please+log+in+again.", http.StatusSeeOther)
				return
			}

			// Active Inactivity Renewal: As long as admin is actively sending requests, slide the 15-min window
			// Re-issue a fresh 15-minute token & cookie on every active page load / interaction
			if refreshedToken, rErr := GenerateAdminToken(claims.Username, claims.Role, jwtSecret); rErr == nil {
				isDev := r.TLS == nil && (strings.HasPrefix(r.Host, "localhost") || strings.HasPrefix(r.Host, "127.0.0.1"))
				SetAdminCookie(w, refreshedToken, isDev)
			}

			// Add claims to context
			ctx := context.WithValue(r.Context(), AdminUserKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GenerateAdminToken creates a signed JWT for an admin user valid for 15 minutes of inactivity.
func GenerateAdminToken(username, role, jwtSecret string) (string, error) {
	claims := &AdminClaims{
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "jobyuk",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtSecret))
}

// SetAdminCookie sets the admin JWT cookie with a 15-minute inactivity lifetime and universal path.
// isDev controls whether the Secure flag is set (false in dev for HTTP localhost).
func SetAdminCookie(w http.ResponseWriter, token string, isDev bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_token",
		Value:    token,
		Path:     "/",
		MaxAge:   900, // 15 minutes
		HttpOnly: true,
		Secure:   !isDev,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearAdminCookie removes the admin authentication JWT cookie across the entire domain.
func ClearAdminCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

