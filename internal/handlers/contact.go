package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"jobyuk/internal/models"
)

// generateContactFormToken generates a secure tamper-proof timestamp token.
func (h *Handlers) generateContactFormToken() string {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(h.cfg.CSRFSecret))
	mac.Write([]byte(ts))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s.%s", ts, sig)
}

// verifyContactFormToken verifies that the token wasn't tampered with and measures elapsed time.
func (h *Handlers) verifyContactFormToken(token string) (time.Duration, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return 0, false
	}
	tsStr, sig := parts[0], parts[1]

	mac := hmac.New(sha256.New, []byte(h.cfg.CSRFSecret))
	mac.Write([]byte(tsStr))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return 0, false
	}

	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return 0, false
	}

	elapsed := time.Since(time.Unix(ts, 0))
	return elapsed, true
}

// getFormValue extracts a parameter from FormValue, PostForm, or MultipartForm safely.
func getFormValue(r *http.Request, key string) string {
	if val := r.FormValue(key); val != "" {
		return val
	}
	if r.PostForm != nil {
		if val := r.PostForm.Get(key); val != "" {
			return val
		}
	}
	if r.MultipartForm != nil && r.MultipartForm.Value != nil {
		if vs := r.MultipartForm.Value[key]; len(vs) > 0 {
			return vs[0]
		}
	}
	return ""
}

// ContactPage renders the public contact page with secure form tokens and CSRF.
func (h *Handlers) ContactPage(w http.ResponseWriter, r *http.Request) {
	formToken := h.generateContactFormToken()
	success := r.URL.Query().Get("success") == "1"
	errorMessage := r.URL.Query().Get("error")

	data := PageData{
		Title:       "Contact JobyUK | Support & Employer Enquiries",
		Description: "Get in touch with JobyUK for job seeker support, employer enquiries or technical help. We aim to reply within one working day.",
		Canonical:   h.cfg.BaseURL + "/contact/",
		Robots:      "index,follow",
		OGTitle:     "Contact JobyUK | Support & Employer Enquiries",
		OGDesc:      "Get in touch with JobyUK for job seeker support, employer enquiries or technical help. We aim to reply within one working day.",
		OGImage:     h.cfg.BaseURL + "/static/images/og-image.jpg",
		OGType:      "website",
		Schema:      h.schema.ContactPageSchema(),
		ActiveNav:   "contact",
		Breadcrumbs: []BreadcrumbData{
			{Name: "Home", URL: "/"},
			{Name: "Contact Us"},
		},
		Data: map[string]interface{}{
			"FormToken": formToken,
			"Success":   success,
			"Error":     errorMessage,
		},
	}

	if err := h.renderPage(w, r, "contact", data); err != nil {
		slog.Error("Failed to render contact page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// ContactSubmit processes incoming contact form submissions with multi-layer bot defense.
func (h *Handlers) ContactSubmit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	isAJAX := r.Header.Get("X-Requested-With") == "XMLHttpRequest" ||
		strings.Contains(r.Header.Get("Accept"), "application/json")

	respondError := func(statusCode int, msg string) {
		if isAJAX {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   msg,
			})
			return
		}
		http.Redirect(w, r, "/contact/?error="+urlQueryEscape(msg), http.StatusSeeOther)
	}

	respondSuccess := func() {
		if isAJAX {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"message": "Thank you! Your message has been received. Our team will review your inquiry and get back to you shortly.",
			})
			return
		}
		http.Redirect(w, r, "/contact/?success=1", http.StatusSeeOther)
	}

	// Enforce strict 1MB max body limit to prevent memory exhaustion / DoS
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	_ = r.ParseMultipartForm(1 << 20)
	_ = r.ParseForm()

	// 1. Decoy Honeypot Trap Check:
	// Hidden honeypot fields catch automated scrapers without triggering browser autofill
	hpWebsite := strings.TrimSpace(getFormValue(r, "contact_hp_website"))
	hpLegacyWebsite := strings.TrimSpace(getFormValue(r, "company_website_url"))
	hpLegacyFax := strings.TrimSpace(getFormValue(r, "fax_number"))
	if hpWebsite != "" || hpLegacyWebsite != "" || hpLegacyFax != "" {
		slog.Warn("Contact form honeypot triggered by bot",
			"ip", getClientIP(r),
			"userAgent", r.UserAgent(),
		)
		// Pretend success so malicious scrapers don't retry or adapt
		respondSuccess()
		return
	}

	// 2. Anti-Bot Velocity Verification:
	// Only drop if submission is inhumanly fast (under 500ms)
	formToken := strings.TrimSpace(getFormValue(r, "form_token"))
	if formToken != "" {
		elapsed, validToken := h.verifyContactFormToken(formToken)
		if validToken && elapsed < 500*time.Millisecond {
			slog.Warn("Contact form submitted too quickly via server token (bot detected)", "ip", getClientIP(r), "elapsed", elapsed)
			respondSuccess()
			return
		}
	}

	// 3. Rate limiting per IP address (Max 20 submissions per 10 minutes to be generous to shared IPs)
	clientIP := getClientIP(r)
	recentCount, err := h.repos.Contact.CountRecentByIP(ctx, clientIP, 10)
	if err == nil && recentCount >= 20 {
		slog.Warn("Contact form rate limit exceeded for IP", "ip", clientIP, "count", recentCount)
		respondError(http.StatusTooManyRequests, "You have submitted multiple inquiries recently. Please wait a few moments before submitting another message.")
		return
	}

	// 4. Input Extraction & Clean Validation
	name := strings.TrimSpace(getFormValue(r, "name"))
	email := strings.TrimSpace(strings.ToLower(getFormValue(r, "email")))
	subject := strings.TrimSpace(getFormValue(r, "subject"))
	inquiryType := strings.TrimSpace(strings.ToLower(getFormValue(r, "inquiry_type")))
	message := strings.TrimSpace(getFormValue(r, "message"))

	// Strip any null bytes to prevent string termination exploits
	name = strings.ReplaceAll(name, "\x00", "")
	email = strings.ReplaceAll(email, "\x00", "")
	subject = strings.ReplaceAll(subject, "\x00", "")
	message = strings.ReplaceAll(message, "\x00", "")

	// Forgiving validations: never block legitimate inquiries
	if name == "" {
		respondError(http.StatusBadRequest, "Please enter your name.")
		return
	}

	if email == "" || !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		respondError(http.StatusBadRequest, "Please enter a valid email address.")
		return
	}

	if message == "" {
		respondError(http.StatusBadRequest, "Please enter your message.")
		return
	}

	// Bound max lengths to prevent database overflow or memory spikes
	if len(name) > 120 {
		name = name[:120]
	}
	if len(email) > 200 {
		email = email[:200]
	}
	if len(subject) > 200 {
		subject = subject[:200]
	}
	if len(message) > 5000 {
		message = message[:5000]
	}

	// Fallback subject if user left it blank
	if subject == "" {
		if len(message) > 40 {
			subject = message[:40] + "..."
		} else {
			subject = "General Inquiry"
		}
	}

	// Whitelist inquiry type
	allowedTypes := map[string]string{
		"general":     "General Inquiry",
		"candidate":   "Candidate Support",
		"employer":    "Employer Vacancies & Hiring",
		"partnership": "Partnership & Press",
		"technical":   "Technical Bug Report",
	}
	if _, ok := allowedTypes[inquiryType]; !ok {
		inquiryType = "general"
	}

	// Sanitize text inputs for storage & presentation safety (escape HTML entities)
	safeName := html.EscapeString(name)
	safeSubject := html.EscapeString(subject)
	safeMessage := html.EscapeString(message)

	inquiry := &models.ContactInquiry{
		Name:        safeName,
		Email:       email,
		Subject:     safeSubject,
		InquiryType: inquiryType,
		Message:     safeMessage,
		IPAddress:   clientIP,
		UserAgent:   html.EscapeString(r.UserAgent()),
		Status:      "unread",
	}

	if err := h.repos.Contact.Create(ctx, inquiry); err != nil {
		slog.Error("Failed to save contact inquiry to database", "error", err)
		respondError(http.StatusInternalServerError, "An unexpected error occurred while saving your inquiry. Please try again later.")
		return
	}

	slog.Info("Contact inquiry successfully recorded",
		"id", inquiry.ID,
		"name", inquiry.Name,
		"email", inquiry.Email,
		"subject", inquiry.Subject,
		"type", inquiry.InquiryType,
	)

	respondSuccess()
}

// urlQueryEscape helper
func urlQueryEscape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteByte(c)
		} else if c == ' ' {
			b.WriteByte('+')
		} else {
			b.WriteString(fmt.Sprintf("%%%02X", c))
		}
	}
	return b.String()
}

