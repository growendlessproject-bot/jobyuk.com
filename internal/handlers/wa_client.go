package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"jobyuk/internal/config"
	"jobyuk/internal/models"
	"jobyuk/internal/repository"
	"jobyuk/internal/services/whatsapp"
)

// WhatsAppClientHandlers manages the user-facing subscriber portal (/my-alerts).
type WhatsAppClientHandlers struct {
	renderer        *Renderer
	repos           *repository.Repositories
	rdb             *redis.Client
	cfg             *config.Config
	waSvc           *whatsapp.Service
	alertDispatcher *whatsapp.AlertDispatcher
}

// NewWhatsAppClientHandlers creates client portal handlers.
func NewWhatsAppClientHandlers(
	renderer *Renderer,
	repos *repository.Repositories,
	rdb *redis.Client,
	cfg *config.Config,
	waSvc *whatsapp.Service,
) *WhatsAppClientHandlers {
	return &WhatsAppClientHandlers{
		renderer: renderer,
		repos:    repos,
		rdb:      rdb,
		cfg:      cfg,
		waSvc:    waSvc,
	}
}

// SetAlertDispatcher injects the automated WhatsApp alert dispatcher.
func (h *WhatsAppClientHandlers) SetAlertDispatcher(ad *whatsapp.AlertDispatcher) {
	h.alertDispatcher = ad
}

// createClientToken generates a secure cookie for subscriber login.
func (h *WhatsAppClientHandlers) createClientToken(subscriberID int, phone string) string {
	payload := fmt.Sprintf("%d:%s:%d", subscriberID, phone, time.Now().Unix())
	mac := hmac.New(sha256.New, []byte(h.cfg.JWTSecret))
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	raw := fmt.Sprintf("%s.%s", payload, sig)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// setSessionCookies issues both the secure HttpOnly session cookie and a lightweight frontend indicator cookie.
func (h *WhatsAppClientHandlers) setSessionCookies(w http.ResponseWriter, subscriberID int, phone string) {
	token := h.createClientToken(subscriberID, phone)
	http.SetCookie(w, &http.Cookie{
		Name:     "wa_client_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   30 * 86400,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "wa_client_logged_in",
		Value:    "1",
		Path:     "/",
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   30 * 86400,
	})
}

// clearSessionCookies removes both subscriber cookies.
func (h *WhatsAppClientHandlers) clearSessionCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "wa_client_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "wa_client_logged_in",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: false,
	})
}

// parseClientToken validates client token.
func (h *WhatsAppClientHandlers) parseClientToken(tokenStr string) (int, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(tokenStr)
	if err != nil {
		return 0, "", fmt.Errorf("malformed token")
	}
	parts := strings.Split(string(raw), ".")
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("invalid token format")
	}
	payload, sig := parts[0], parts[1]

	mac := hmac.New(sha256.New, []byte(h.cfg.JWTSecret))
	mac.Write([]byte(payload))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return 0, "", fmt.Errorf("invalid signature")
	}

	subParts := strings.Split(payload, ":")
	if len(subParts) != 3 {
		return 0, "", fmt.Errorf("invalid payload structure")
	}

	subID, err := strconv.Atoi(subParts[0])
	if err != nil {
		return 0, "", err
	}

	ts, err := strconv.ParseInt(subParts[2], 10, 64)
	if err != nil || time.Now().Unix()-ts > 30*86400 { // 30 day cookie
		return 0, "", fmt.Errorf("token expired")
	}

	return subID, subParts[1], nil
}

// getAuthSubscriber retrieves the logged in subscriber from cookie.
func (h *WhatsAppClientHandlers) getAuthSubscriber(r *http.Request) (*models.WhatsAppSubscriber, error) {
	cookie, err := r.Cookie("wa_client_session")
	if err != nil || cookie.Value == "" {
		return nil, fmt.Errorf("unauthenticated")
	}
	subID, _, err := h.parseClientToken(cookie.Value)
	if err != nil {
		return nil, err
	}
	return h.repos.WhatsApp.GetSubscriberByID(r.Context(), subID)
}

// RequireSubscriber middleware ensures only authenticated subscribers access their portal.
func (h *WhatsAppClientHandlers) RequireSubscriber(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sub, err := h.getAuthSubscriber(r)
		if err != nil || sub == nil {
			http.Redirect(w, r, "/my-alerts/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type pendingRegistration struct {
	FullName   string   `json:"full_name"`
	Phone      string   `json:"phone"`
	Email      string   `json:"email"`
	PlanType   string   `json:"plan_type"`
	Categories []string `json:"categories"`
	Locations  []string `json:"locations"`
	MinSalary  int      `json:"min_salary"`
}

// FlashData holds temporary session messages (Zero URL params)
type FlashData struct {
	Error   string `json:"error,omitempty"`
	Success string `json:"success,omitempty"`
	Phone   string `json:"phone,omitempty"`
}

func (h *WhatsAppClientHandlers) setFlash(w http.ResponseWriter, flash FlashData) {
	bytes, _ := json.Marshal(flash)
	encoded := base64.RawURLEncoding.EncodeToString(bytes)
	http.SetCookie(w, &http.Cookie{
		Name:     "wa_flash",
		Value:    encoded,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   60,
	})
}

func (h *WhatsAppClientHandlers) getFlash(w http.ResponseWriter, r *http.Request) FlashData {
	var flash FlashData
	cookie, err := r.Cookie("wa_flash")
	if err == nil && cookie.Value != "" {
		if bytes, err := base64.RawURLEncoding.DecodeString(cookie.Value); err == nil {
			_ = json.Unmarshal(bytes, &flash)
		}
		// Clear flash cookie immediately
		http.SetCookie(w, &http.Cookie{
			Name:     "wa_flash",
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
		})
	}
	if qErr := r.URL.Query().Get("error"); qErr != "" && flash.Error == "" {
		flash.Error = qErr
	}
	if qSucc := r.URL.Query().Get("success"); qSucc != "" && flash.Success == "" {
		flash.Success = qSucc
	}
	if qPhone := r.URL.Query().Get("phone"); qPhone != "" && flash.Phone == "" {
		flash.Phone = qPhone
	}
	return flash
}

// renderAuthView renders candidate auth views with clean URLs and flash messages.
func (h *WhatsAppClientHandlers) renderAuthView(w http.ResponseWriter, r *http.Request, defaultTab, defaultStep string) {
	if sub, err := h.getAuthSubscriber(r); err == nil && sub != nil {
		http.Redirect(w, r, "/my-alerts", http.StatusSeeOther)
		return
	}

	flash := h.getFlash(w, r)

	tab := defaultTab
	if qTab := r.URL.Query().Get("tab"); qTab != "" {
		tab = qTab
	}
	if tab != "register" {
		tab = "signin"
	}

	step := defaultStep
	if qStep := r.URL.Query().Get("step"); qStep != "" {
		step = qStep
	}

	phone := flash.Phone
	if phone == "" {
		phone = r.URL.Query().Get("phone")
	}
	normPhone := repository.NormalizePhoneNumber(phone)

	// SECURITY: Never expose /my-alerts/verify to the public without an active pending registration!
	if step == "verify" {
		hasSession := false
		if normPhone != "" {
			exists, _ := h.rdb.Exists(r.Context(), "wa_reg:"+normPhone).Result()
			if exists > 0 {
				hasSession = true
			}
		}
		if !hasSession {
			h.setFlash(w, FlashData{
				Error: "Please fill in the registration form to request a verification code.",
			})
			http.Redirect(w, r, "/my-alerts/register", http.StatusSeeOther)
			return
		}
	}

	// SECURITY: Never expose /my-alerts/reset-pin to the public without an active reset session!
	if step == "reset" {
		hasResetSession := false
		if normPhone != "" {
			exists, _ := h.rdb.Exists(r.Context(), "wa_reset_session:"+normPhone).Result()
			if exists > 0 {
				hasResetSession = true
			}
		}
		if !hasResetSession {
			h.setFlash(w, FlashData{
				Error: "Please request a reset code before accessing the verification screen.",
			})
			http.Redirect(w, r, "/my-alerts/forgot-pin", http.StatusSeeOther)
			return
		}
	}

	errMsg := flash.Error
	succMsg := flash.Success

	data := PageData{
		Title:    "Candidate Access | WhatsApp Job Alerts | JobyUK.com",
		IsMobile: isMobileDevice(r),
		Data: map[string]interface{}{
			"ActiveTab": tab,
			"Step":      step,
			"Phone":     phone,
			"Plan":      r.URL.Query().Get("plan"),
			"Error":     errMsg,
			"Success":   succMsg,
		},
	}
	_ = h.renderer.Render(w, "wa_client_login", data)
}

// LoginPage renders Candidate Sign In (/my-alerts/login).
func (h *WhatsAppClientHandlers) LoginPage(w http.ResponseWriter, r *http.Request) {
	h.renderAuthView(w, r, "signin", "")
}

// RegisterPage renders Candidate Registration (/my-alerts/register).
func (h *WhatsAppClientHandlers) RegisterPage(w http.ResponseWriter, r *http.Request) {
	h.renderAuthView(w, r, "register", "")
}

// VerifyPage renders Registration OTP Verification (/my-alerts/verify).
func (h *WhatsAppClientHandlers) VerifyPage(w http.ResponseWriter, r *http.Request) {
	h.renderAuthView(w, r, "register", "verify")
}

// ForgotPINPage renders Forgot PIN (/my-alerts/forgot-pin).
func (h *WhatsAppClientHandlers) ForgotPINPage(w http.ResponseWriter, r *http.Request) {
	h.renderAuthView(w, r, "signin", "forgot")
}

// ResetPINPage renders Reset PIN OTP Verification (/my-alerts/reset-pin).
func (h *WhatsAppClientHandlers) ResetPINPage(w http.ResponseWriter, r *http.Request) {
	h.renderAuthView(w, r, "signin", "reset")
}

// LoginWithPIN authenticates subscriber instantly with phone number and 6-digit access PIN (Zero WhatsApp dispatch).
func (h *WhatsAppClientHandlers) LoginWithPIN(w http.ResponseWriter, r *http.Request) {
	phone := strings.TrimSpace(r.FormValue("phone"))
	pin := strings.TrimSpace(r.FormValue("pin"))

	if phone == "" || pin == "" {
		h.setFlash(w, FlashData{Error: "Phone number and 6-digit PIN are required."})
		http.Redirect(w, r, "/my-alerts/login", http.StatusSeeOther)
		return
	}

	normPhone := repository.NormalizePhoneNumber(phone)
	sub, err := h.repos.WhatsApp.VerifySubscriberPIN(r.Context(), normPhone, pin)
	if err != nil {
		if existing, _ := h.repos.WhatsApp.GetSubscriberByPhone(r.Context(), normPhone); existing != nil {
			_ = h.repos.WhatsApp.LogSubscriberLogin(r.Context(), existing.ID, normPhone, r.RemoteAddr, r.UserAgent(), "login_pin", "failed_pin")
		}
		h.setFlash(w, FlashData{Error: err.Error(), Phone: normPhone})
		http.Redirect(w, r, "/my-alerts/login", http.StatusSeeOther)
		return
	}

	// Record successful login audit log
	_ = h.repos.WhatsApp.LogSubscriberLogin(r.Context(), sub.ID, sub.PhoneNumber, r.RemoteAddr, r.UserAgent(), "login_pin", "success")

	// Issue tamper-proof HMAC cookie token & indicator
	h.setSessionCookies(w, sub.ID, sub.PhoneNumber)

	http.Redirect(w, r, "/my-alerts", http.StatusSeeOther)
}

// RegisterSubmit validates registration fields and sends a 1-time verification OTP to candidate's WhatsApp.
func (h *WhatsAppClientHandlers) RegisterSubmit(w http.ResponseWriter, r *http.Request) {
	fullName := strings.TrimSpace(r.FormValue("full_name"))
	phone := strings.TrimSpace(r.FormValue("phone"))
	email := strings.TrimSpace(r.FormValue("email"))
	planType := strings.TrimSpace(r.FormValue("plan_type"))
	categoriesStr := strings.TrimSpace(r.FormValue("categories"))
	locationsStr := strings.TrimSpace(r.FormValue("locations"))
	minSalaryStr := strings.TrimSpace(r.FormValue("min_salary"))

	if fullName == "" || phone == "" {
		h.setFlash(w, FlashData{Error: "Name and WhatsApp number are required."})
		http.Redirect(w, r, "/my-alerts/register", http.StatusSeeOther)
		return
	}

	normPhone := repository.NormalizePhoneNumber(phone)
	if normPhone == "" || len(normPhone) < 8 {
		h.setFlash(w, FlashData{Error: "Invalid WhatsApp phone number. Please enter a valid number with country code."})
		http.Redirect(w, r, "/my-alerts/register", http.StatusSeeOther)
		return
	}

	// Check if phone already registered
	existing, _ := h.repos.WhatsApp.GetSubscriberByPhone(r.Context(), normPhone)
	if existing != nil {
		h.setFlash(w, FlashData{Error: "You already have an active subscription. Please sign in with your 6-digit PIN.", Phone: normPhone})
		http.Redirect(w, r, "/my-alerts/login", http.StatusSeeOther)
		return
	}

	// All new candidate registrations activate the 3-Day Free Trial!
	planType = "trial"

	minSalary, _ := strconv.Atoi(minSalaryStr)

	// Clean categories
	var categories []string
	for _, c := range strings.Split(categoriesStr, ",") {
		c = strings.TrimSpace(c)
		if c != "" {
			categories = append(categories, c)
		}
	}

	// Clean locations
	var locations []string
	for _, l := range strings.Split(locationsStr, ",") {
		l = strings.TrimSpace(l)
		if l != "" {
			locations = append(locations, l)
		}
	}

	reg := pendingRegistration{
		FullName:   fullName,
		Phone:      normPhone,
		Email:      email,
		PlanType:   planType,
		Categories: categories,
		Locations:  locations,
		MinSalary:  minSalary,
	}

	regBytes, _ := json.Marshal(reg)
	_ = h.rdb.Set(r.Context(), "wa_reg:"+normPhone, string(regBytes), 15*time.Minute).Err()

	// Send 1-time verification OTP with contact pre-registration
	_, err := h.waSvc.SendOTP(r.Context(), normPhone, fullName)
	if err != nil {
		slog.Error("RegisterSubmit failed to send OTP", "err", err, "phone", normPhone, "fullName", fullName)
		h.setFlash(w, FlashData{Error: "Unable to dispatch verification code. Please verify the number."})
		http.Redirect(w, r, "/my-alerts/register", http.StatusSeeOther)
		return
	}

	h.setFlash(w, FlashData{Success: "Verification code sent to your WhatsApp.", Phone: normPhone})
	http.Redirect(w, r, "/my-alerts/verify", http.StatusSeeOther)
}

// RegisterVerify confirms WhatsApp ownership, creates subscriber with permanent PIN, and sends security receipt.
func (h *WhatsAppClientHandlers) RegisterVerify(w http.ResponseWriter, r *http.Request) {
	phone := strings.TrimSpace(r.FormValue("phone"))
	code := strings.TrimSpace(r.FormValue("otp_code"))

	normPhone := repository.NormalizePhoneNumber(phone)
	if normPhone == "" || code == "" {
		h.setFlash(w, FlashData{Error: "Please enter the 6-digit verification code.", Phone: normPhone})
		http.Redirect(w, r, "/my-alerts/register", http.StatusSeeOther)
		return
	}

	ctx := r.Context()

	// 1. MUST have active pending registration in Redis
	rawReg, err := h.rdb.Get(ctx, "wa_reg:"+normPhone).Result()
	if err != nil || rawReg == "" {
		h.setFlash(w, FlashData{Error: "Registration session expired or invalid. Please fill in the registration form."})
		http.Redirect(w, r, "/my-alerts/register", http.StatusSeeOther)
		return
	}

	// 2. Strict attempt limit (max 5 attempts per session)
	attemptKey := fmt.Sprintf("wa_reg_attempts:%s", normPhone)
	attempts, _ := h.rdb.Incr(ctx, attemptKey).Result()
	if attempts == 1 {
		h.rdb.Expire(ctx, attemptKey, 15*time.Minute)
	}
	if attempts > 5 {
		_ = h.rdb.Del(ctx, "wa_reg:"+normPhone).Err()
		_ = h.rdb.Del(ctx, attemptKey).Err()
		h.setFlash(w, FlashData{Error: "Too many failed attempts. Registration session terminated. Please try again."})
		http.Redirect(w, r, "/my-alerts/register", http.StatusSeeOther)
		return
	}

	// 3. Verify OTP code
	ok, err := h.repos.WhatsApp.VerifyOTP(ctx, normPhone, code)
	if !ok || err != nil {
		errMsg := fmt.Sprintf("Invalid or expired verification code (%d attempts remaining).", 5-attempts)
		h.setFlash(w, FlashData{Error: errMsg, Phone: normPhone})
		http.Redirect(w, r, "/my-alerts/verify", http.StatusSeeOther)
		return
	}

	// Code is confirmed! Clear attempt counter and session
	_ = h.rdb.Del(ctx, attemptKey).Err()
	_ = h.rdb.Del(ctx, "wa_reg:"+normPhone).Err()

	var reg pendingRegistration
	if err := json.Unmarshal([]byte(rawReg), &reg); err != nil {
		h.setFlash(w, FlashData{Error: "Registration parsing error. Please try again."})
		http.Redirect(w, r, "/my-alerts/register", http.StatusSeeOther)
		return
	}

	// Generate permanent 6-digit access PIN
	pin := whatsapp.Generate6DigitOTP()
	pinHash, err := repository.HashPIN(pin)
	if err != nil {
		h.setFlash(w, FlashData{Error: "Security hash generation failed."})
		http.Redirect(w, r, "/my-alerts/register", http.StatusSeeOther)
		return
	}

	trialEnds := time.Now().Add(72 * time.Hour)
	sub := &models.WhatsAppSubscriber{
		PhoneNumber:   reg.Phone,
		FullName:      reg.FullName,
		Email:         reg.Email,
		PlanType:      "trial",
		PaymentStatus: "trial",
		Source:        "register",
		TrialEndsAt:   &trialEnds,
		ExpiresAt:     trialEnds,
		Categories:    reg.Categories,
		Locations:     reg.Locations,
		MinSalary:     reg.MinSalary,
		AlertMode:     "instant",
		Status:        "active",
		PINHash:       pinHash,
	}

	// Double-check subscriber does not already exist
	existingSub, _ := h.repos.WhatsApp.GetSubscriberByPhone(r.Context(), sub.PhoneNumber)
	if existingSub != nil {
		h.setFlash(w, FlashData{
			Error: "This WhatsApp number is already registered. Please sign in with your 6-digit PIN.",
			Phone: sub.PhoneNumber,
		})
		http.Redirect(w, r, "/my-alerts/login", http.StatusSeeOther)
		return
	}

	if err := h.repos.WhatsApp.CreateSubscriber(r.Context(), sub); err != nil {
		h.setFlash(w, FlashData{Error: "Failed to activate subscription. Please contact support."})
		http.Redirect(w, r, "/my-alerts/register", http.StatusSeeOther)
		return
	}

	// Send encrypted welcome message containing their 6-digit access PIN and 3-Day Free Trial confirmation
	_ = h.waSvc.SendWelcomeWithPIN(r.Context(), sub.PhoneNumber, sub.FullName, pin, "trial")

	// Record registration verification audit log
	_ = h.repos.WhatsApp.LogSubscriberLogin(r.Context(), sub.ID, sub.PhoneNumber, r.RemoteAddr, r.UserAgent(), "register_verify", "success")

	// Issue login session
	h.setSessionCookies(w, sub.ID, sub.PhoneNumber)

	h.setFlash(w, FlashData{Success: "Welcome! Your confidential 6-digit access PIN has been sent to your WhatsApp."})
	http.Redirect(w, r, "/my-alerts", http.StatusSeeOther)
}

// ForgotPINRequest sends a 1-time reset OTP to candidate's registered WhatsApp.
func (h *WhatsAppClientHandlers) ForgotPINRequest(w http.ResponseWriter, r *http.Request) {
	phone := strings.TrimSpace(r.FormValue("phone"))
	normPhone := repository.NormalizePhoneNumber(phone)

	if normPhone == "" || len(normPhone) < 8 {
		h.setFlash(w, FlashData{Error: "Please enter a valid WhatsApp phone number."})
		http.Redirect(w, r, "/my-alerts/forgot-pin", http.StatusSeeOther)
		return
	}

	ctx := r.Context()
	sub, err := h.repos.WhatsApp.GetSubscriberByPhone(ctx, normPhone)
	if err != nil || sub == nil {
		h.setFlash(w, FlashData{Error: "No active subscription found for this phone number."})
		http.Redirect(w, r, "/my-alerts/forgot-pin", http.StatusSeeOther)
		return
	}

	// Anti-abuse cooldown on reset requests (2 minutes between requests)
	resetCdKey := fmt.Sprintf("wa_reset_cd:%s", normPhone)
	cdExists, _ := h.rdb.Exists(ctx, resetCdKey).Result()
	if cdExists > 0 {
		ttl, _ := h.rdb.TTL(ctx, resetCdKey).Result()
		h.setFlash(w, FlashData{
			Error: fmt.Sprintf("A reset code was recently sent. Please check WhatsApp or wait %d seconds before requesting another.", int(ttl.Seconds())),
			Phone: normPhone,
		})
		http.Redirect(w, r, "/my-alerts/reset-pin", http.StatusSeeOther)
		return
	}

	_, err = h.waSvc.SendOTP(ctx, normPhone)
	if err != nil {
		slog.Error("ForgotPINSubmit failed to send OTP", "err", err, "phone", normPhone)
		h.setFlash(w, FlashData{Error: "Unable to dispatch reset code. Please try again."})
		http.Redirect(w, r, "/my-alerts/forgot-pin", http.StatusSeeOther)
		return
	}

	_ = h.rdb.Set(ctx, resetCdKey, "1", 2*time.Minute).Err()
	_ = h.rdb.Set(ctx, "wa_reset_session:"+normPhone, "1", 10*time.Minute).Err()

	h.setFlash(w, FlashData{Success: "Reset code sent to your WhatsApp.", Phone: normPhone})
	http.Redirect(w, r, "/my-alerts/reset-pin", http.StatusSeeOther)
}

// ForgotPINVerify verifies OTP and sets a fresh permanent 6-digit access PIN.
func (h *WhatsAppClientHandlers) ForgotPINVerify(w http.ResponseWriter, r *http.Request) {
	phone := strings.TrimSpace(r.FormValue("phone"))
	code := strings.TrimSpace(r.FormValue("otp_code"))

	normPhone := repository.NormalizePhoneNumber(phone)
	if normPhone == "" || code == "" {
		h.setFlash(w, FlashData{Error: "Verification code and phone are required.", Phone: normPhone})
		http.Redirect(w, r, "/my-alerts/forgot-pin", http.StatusSeeOther)
		return
	}

	ctx := r.Context()

	// 1. MUST have active reset session in Redis
	hasReset, err := h.rdb.Exists(ctx, "wa_reset_session:"+normPhone).Result()
	if err != nil || hasReset == 0 {
		h.setFlash(w, FlashData{Error: "Reset session expired or invalid. Please request a new code."})
		http.Redirect(w, r, "/my-alerts/forgot-pin", http.StatusSeeOther)
		return
	}

	// 2. Strict attempt limit (max 5 attempts)
	attemptKey := fmt.Sprintf("wa_reset_attempts:%s", normPhone)
	attempts, _ := h.rdb.Incr(ctx, attemptKey).Result()
	if attempts == 1 {
		h.rdb.Expire(ctx, attemptKey, 15*time.Minute)
	}
	if attempts > 5 {
		_ = h.rdb.Del(ctx, "wa_reset_session:"+normPhone).Err()
		_ = h.rdb.Del(ctx, attemptKey).Err()
		h.setFlash(w, FlashData{Error: "Too many failed attempts. Reset session terminated. Please request a new code."})
		http.Redirect(w, r, "/my-alerts/forgot-pin", http.StatusSeeOther)
		return
	}

	ok, err := h.repos.WhatsApp.VerifyOTP(ctx, normPhone, code)
	if !ok || err != nil {
		errMsg := fmt.Sprintf("Invalid or expired reset code (%d attempts remaining).", 5-attempts)
		h.setFlash(w, FlashData{Error: errMsg, Phone: normPhone})
		http.Redirect(w, r, "/my-alerts/reset-pin", http.StatusSeeOther)
		return
	}

	_ = h.rdb.Del(ctx, "wa_reset_session:"+normPhone).Err()
	_ = h.rdb.Del(ctx, attemptKey).Err()

	sub, err := h.repos.WhatsApp.GetSubscriberByPhone(r.Context(), normPhone)
	if err != nil || sub == nil {
		h.setFlash(w, FlashData{Error: "Subscriber not found."})
		http.Redirect(w, r, "/my-alerts/login", http.StatusSeeOther)
		return
	}

	// Generate fresh PIN
	newPIN := whatsapp.Generate6DigitOTP()
	if err := h.repos.WhatsApp.SetSubscriberPIN(r.Context(), sub.ID, newPIN); err != nil {
		h.setFlash(w, FlashData{Error: "Failed to update PIN. Please try again."})
		http.Redirect(w, r, "/my-alerts/login", http.StatusSeeOther)
		return
	}

	// Send new PIN to WhatsApp
	_ = h.waSvc.SendWelcomeWithPIN(r.Context(), sub.PhoneNumber, sub.FullName, newPIN, sub.PlanType)

	// Record PIN reset audit log
	_ = h.repos.WhatsApp.LogSubscriberLogin(r.Context(), sub.ID, sub.PhoneNumber, r.RemoteAddr, r.UserAgent(), "pin_reset", "success")

	// Issue login session
	h.setSessionCookies(w, sub.ID, sub.PhoneNumber)

	h.setFlash(w, FlashData{Success: "Your new 6-digit access PIN has been sent to your WhatsApp."})
	http.Redirect(w, r, "/my-alerts", http.StatusSeeOther)
}

// Logout logs out the subscriber.
func (h *WhatsAppClientHandlers) Logout(w http.ResponseWriter, r *http.Request) {
	h.clearSessionCookies(w)
	http.Redirect(w, r, "/my-alerts/login", http.StatusSeeOther)
}

// Dashboard renders subscriber controls, quota counters, cooldown timer, and recent alerts.
func (h *WhatsAppClientHandlers) Dashboard(w http.ResponseWriter, r *http.Request) {
	sub, err := h.getAuthSubscriber(r)
	if err != nil || sub == nil {
		http.Redirect(w, r, "/my-alerts/login", http.StatusSeeOther)
		return
	}

	ctx := r.Context()
	recentAlerts, _, _ := h.repos.WhatsApp.ListAlertLogs(ctx, &sub.ID, 15, 0)
	loginLogs, _ := h.repos.WhatsApp.GetSubscriberLoginLogs(ctx, sub.ID, 10)
	cryptoWallets, _ := h.repos.WhatsApp.GetActiveCryptoWallets(ctx)
	paymentSettings, _ := h.repos.WhatsApp.GetPaymentSettings(ctx)

	canChange, reason := h.repos.WhatsApp.CanChangePhoneNumber(sub)

	activeTab := strings.TrimSpace(r.URL.Query().Get("tab"))
	if activeTab == "" {
		if sub.IsPaywalled {
			activeTab = "plans"
		} else {
			activeTab = "preferences"
		}
	}

	flash := h.getFlash(w, r)
	succ := flash.Success
	if succ == "" {
		succ = r.URL.Query().Get("success")
	}
	errM := flash.Error
	if errM == "" {
		errM = r.URL.Query().Get("error")
	}

	data := PageData{
		Title:    "WhatsApp Job Alerts | JobyUK.com",
		IsMobile: isMobileDevice(r),
		Data: map[string]interface{}{
			"Subscriber":      sub,
			"RecentAlerts":    recentAlerts,
			"LoginLogs":       loginLogs,
			"CryptoWallets":   cryptoWallets,
			"PaymentSettings": paymentSettings,
			"CanChangeNumber": canChange,
			"ChangeReason":    reason,
			"Success":         succ,
			"Error":           errM,
			"ActiveTab":       activeTab,
			"TotalAlerts":     len(recentAlerts),
			"TotalLogs":       len(loginLogs),
		},
	}
	_ = h.renderer.Render(w, "wa_client_dashboard", data)
}

// ChangeNumber processes phone number changes with strict quota and 14-day cooldown enforcement.
func (h *WhatsAppClientHandlers) ChangeNumber(w http.ResponseWriter, r *http.Request) {
	sub, err := h.getAuthSubscriber(r)
	if err != nil || sub == nil {
		http.Redirect(w, r, "/my-alerts/login", http.StatusSeeOther)
		return
	}

	newPhone := strings.TrimSpace(r.FormValue("new_phone"))
	otpCode := strings.TrimSpace(r.FormValue("new_phone_otp"))
	action := r.FormValue("action") // 'send_otp' or 'confirm'

	canChange, reason := h.repos.WhatsApp.CanChangePhoneNumber(sub)
	if !canChange {
		h.setFlash(w, FlashData{Error: reason})
		http.Redirect(w, r, "/my-alerts", http.StatusSeeOther)
		return
	}

	normNewPhone := repository.NormalizePhoneNumber(newPhone)
	if normNewPhone == "" || normNewPhone == sub.PhoneNumber {
		h.setFlash(w, FlashData{Error: "Please enter a different valid phone number."})
		http.Redirect(w, r, "/my-alerts", http.StatusSeeOther)
		return
	}

	// Step 1: Send OTP to the new phone number
	if action == "send_otp" {
		_, err := h.waSvc.SendOTP(r.Context(), normNewPhone)
		if err != nil {
			slog.Error("AccountUpdatePhone failed to send OTP", "err", err, "phone", normNewPhone)
			h.setFlash(w, FlashData{Error: "Unable to send verification code to new number. Please try again."})
			http.Redirect(w, r, "/my-alerts?tab=phone", http.StatusSeeOther)
			return
		}
		h.setFlash(w, FlashData{Success: "Verification code sent to your new WhatsApp number."})
		http.Redirect(w, r, fmt.Sprintf("/my-alerts?tab=phone&new_phone=%s&otp_sent=true", normNewPhone), http.StatusSeeOther)
		return
	}

	// Step 2: Confirm OTP and execute change
	ok, err := h.repos.WhatsApp.VerifyOTP(r.Context(), normNewPhone, otpCode)
	if !ok || err != nil {
		errMsg := "Invalid or expired verification code."
		if err != nil {
			errMsg = err.Error()
		}
		h.setFlash(w, FlashData{Error: errMsg})
		http.Redirect(w, r, fmt.Sprintf("/my-alerts?tab=phone&new_phone=%s&otp_sent=true", normNewPhone), http.StatusSeeOther)
		return
	}

	clientIP := getClientIP(r)
	if err := h.repos.WhatsApp.ChangePhoneNumber(r.Context(), sub.ID, normNewPhone, clientIP); err != nil {
		h.setFlash(w, FlashData{Error: err.Error()})
		http.Redirect(w, r, "/my-alerts?tab=phone", http.StatusSeeOther)
		return
	}

	// Update session cookie with new phone
	h.setSessionCookies(w, sub.ID, normNewPhone)

	// Dispatch confirmation message to new phone
	go func() {
		msg := fmt.Sprintf(
			"*JobyUK.com Account Notification*\n\n"+
				"Hello %s,\n\n"+
				"Your WhatsApp notification number has been updated successfully. Future job alerts will be delivered to this number.\n\n"+
				"Please note: A 14-day security cooldown is now in place before this number can be changed again.\n\n"+
				"Manage your preferences: %s/my-alerts",
			sub.FullName, h.cfg.BaseURL,
		)
		_ = h.waSvc.DispatchTextMessage(context.Background(), normNewPhone, msg, "Number Change Confirmation")
	}()

	h.setFlash(w, FlashData{Success: "WhatsApp number updated successfully. A 14-day cooldown is now active."})
	http.Redirect(w, r, "/my-alerts?tab=phone", http.StatusSeeOther)
}

// UpdatePreferences updates job matching categories, cities, salary, and alert frequency.
func (h *WhatsAppClientHandlers) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	sub, err := h.getAuthSubscriber(r)
	if err != nil || sub == nil {
		http.Redirect(w, r, "/my-alerts/login", http.StatusSeeOther)
		return
	}

	rawCategories := strings.Split(r.FormValue("categories"), ",")
	var categories []string
	for _, c := range rawCategories {
		clean := strings.TrimSpace(c)
		if clean != "" {
			categories = append(categories, clean)
		}
	}

	rawLocations := strings.Split(r.FormValue("locations"), ",")
	var locations []string
	for _, l := range rawLocations {
		clean := strings.TrimSpace(l)
		if clean != "" {
			locations = append(locations, clean)
		}
	}

	minSalary, _ := strconv.Atoi(r.FormValue("min_salary"))
	alertMode := strings.TrimSpace(r.FormValue("alert_mode"))
	if alertMode == "" {
		alertMode = "instant"
	}

	if err := h.repos.WhatsApp.UpdatePreferences(r.Context(), sub.ID, categories, locations, minSalary, alertMode); err != nil {
		slog.Error("Failed to update preferences", "error", err)
		h.setFlash(w, FlashData{Error: "Unable to save preferences. Please try again."})
		http.Redirect(w, r, "/my-alerts?tab=preferences", http.StatusSeeOther)
		return
	}

	h.setFlash(w, FlashData{Success: "Job alert preferences saved successfully."})
	http.Redirect(w, r, "/my-alerts?tab=preferences", http.StatusSeeOther)
}

// TogglePause allows subscriber to pause alerts or resume.
func (h *WhatsAppClientHandlers) TogglePause(w http.ResponseWriter, r *http.Request) {
	sub, err := h.getAuthSubscriber(r)
	if err != nil || sub == nil {
		http.Redirect(w, r, "/my-alerts/login", http.StatusSeeOther)
		return
	}

	newStatus := "paused"
	successMsg := "Job alerts paused. You can resume at any time."
	if sub.Status == "paused" {
		newStatus = "active"
		successMsg = "Job alerts resumed. You will now receive new job alerts."
	}

	_ = h.repos.WhatsApp.ToggleStatus(r.Context(), sub.ID, newStatus)
	h.setFlash(w, FlashData{Success: successMsg})
	http.Redirect(w, r, "/my-alerts?tab=phone", http.StatusSeeOther)
}

type pendingTrialData struct {
	Phone    string `json:"phone"`
	Code     string `json:"code"`
	Category string `json:"category"`
	Location string `json:"location"`
	FullName string `json:"full_name"`
	Attempts int    `json:"attempts"`
}

// TrialSubscribe initiates the 3-Day Free Trial by validating ownership and dispatching a 6-digit verification code.
func (h *WhatsAppClientHandlers) TrialSubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var phone, category, location, fullName, visitorID string
	var trackingID int

	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		var req struct {
			Phone      string `json:"phone"`
			Category   string `json:"category"`
			Location   string `json:"location"`
			Keywords   string `json:"keywords"`
			FullName   string `json:"full_name"`
			TrackingID int    `json:"tracking_id"`
			VisitorID  string `json:"visitor_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			phone = req.Phone
			category = req.Category
			if category == "" && req.Keywords != "" {
				category = req.Keywords
			}
			location = req.Location
			fullName = req.FullName
			trackingID = req.TrackingID
			visitorID = req.VisitorID
		}
	} else {
		phone = r.FormValue("phone")
		category = r.FormValue("category")
		if category == "" && r.FormValue("keywords") != "" {
			category = r.FormValue("keywords")
		}
		location = r.FormValue("location")
		fullName = r.FormValue("full_name")
		if tid, err := strconv.Atoi(r.FormValue("tracking_id")); err == nil {
			trackingID = tid
		}
		visitorID = r.FormValue("visitor_id")
	}

	normPhone, err := whatsapp.ParseAndValidatePhoneNumber(phone, "GB")
	if err != nil || normPhone == "" || !repository.IsValidE164(normPhone) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Please enter a valid WhatsApp phone number with country code (e.g. +44 or +92). Number cannot start with 0.",
		})
		return
	}

	if category == "" || strings.EqualFold(category, "All Categories") {
		category = "All Categories"
	}
	if location == "" || strings.EqualFold(location, "All UK Locations") {
		location = "All UK Locations"
	}

	ctx := r.Context()

	// 1. Foolproof check: Block ONLY if already actively registered
	existingSub, _ := h.repos.WhatsApp.GetSubscriberByPhone(ctx, normPhone)
	if existingSub != nil && existingSub.Status == "active" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":            false,
			"already_registered": true,
			"error":              "This WhatsApp number is already registered in our system. Please sign in to your dashboard using your 6-digit PIN.",
			"phone":              normPhone,
			"login_url":          "/my-alerts/login",
		})
		return
	}

	// Clean candidate full name
	cleanFullName := strings.TrimSpace(fullName)
	if cleanFullName == "" {
		if existingSub != nil && existingSub.FullName != "" && !strings.HasPrefix(existingSub.FullName, "Candidate ") {
			cleanFullName = existingSub.FullName
		} else {
			suffix := normPhone
			if len(normPhone) > 4 {
				suffix = normPhone[len(normPhone)-4:]
			}
			cleanFullName = fmt.Sprintf("Candidate %s", suffix)
		}
	}

	// 2. Strict Rate Limit by IP (Max 8 trial requests per 15 mins for public IP, 25 for local)
	if h.rdb != nil {
		clientIP := getClientIP(r)
		if clientIP != "" {
			ipKey := fmt.Sprintf("wa_rl_trial_ip:%s", clientIP)
			count, err := h.rdb.Incr(ctx, ipKey).Result()
			if err == nil {
				if count == 1 {
					h.rdb.Expire(ctx, ipKey, 15*time.Minute)
				}
				maxIP := int64(8)
				if clientIP == "127.0.0.1" || clientIP == "::1" || clientIP == "localhost" {
					maxIP = 25
				}
				if count > maxIP {
					ttl, _ := h.rdb.TTL(ctx, ipKey).Result()
					mins := int(ttl.Minutes()) + 1
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"success": false,
						"error":   fmt.Sprintf("Too many verification requests from your network. Please try again in %d minutes.", mins),
					})
					return
				}
			}
		}

		// 3. Strict Anti-Abuse Rate Limit by Phone
		// 3a. Cooldown: 60 seconds between OTP dispatches
		phoneCdKey := fmt.Sprintf("wa_rl_trial_cd:%s", normPhone)
		cdExists, _ := h.rdb.Exists(ctx, phoneCdKey).Result()
		if cdExists > 0 {
			ttl, _ := h.rdb.TTL(ctx, phoneCdKey).Result()
			secs := int(ttl.Seconds())
			if secs <= 0 {
				secs = 60
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success":          false,
				"error":            fmt.Sprintf("A verification code was recently sent to this number. Please check WhatsApp or wait %d seconds before requesting another code.", secs),
				"cooldown_seconds": secs,
			})
			return
		}

		// 3b. Hourly cap: Max 4 requests per hour per phone
		phoneHourlyKey := fmt.Sprintf("wa_rl_trial_hr:%s", normPhone)
		hourlyCount, err := h.rdb.Incr(ctx, phoneHourlyKey).Result()
		if err == nil {
			if hourlyCount == 1 {
				h.rdb.Expire(ctx, phoneHourlyKey, 1*time.Hour)
			}
			if hourlyCount > 4 {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "Hourly verification code limit reached for this number. Please wait an hour before requesting again.",
				})
				return
			}
		}

		// 3c. Daily cap: Max 6 codes per day per phone
		phoneDailyKey := fmt.Sprintf("wa_rl_trial_daily:%s", normPhone)
		dailyCount, err := h.rdb.Incr(ctx, phoneDailyKey).Result()
		if err == nil {
			if dailyCount == 1 {
				h.rdb.Expire(ctx, phoneDailyKey, 24*time.Hour)
			}
			if dailyCount > 6 {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "Daily trial verification limit reached for this WhatsApp number. Please try again tomorrow.",
				})
				return
			}
		}
	}

	// 4. CRITICAL: Pre-save candidate contact in Database BEFORE OTP dispatch
	leadSub := &models.WhatsAppSubscriber{
		PhoneNumber: normPhone,
		FullName:    cleanFullName,
		Categories:  []string{category},
		Locations:   []string{location},
		Source:      "homepage_trial",
	}
	savedSub, err := h.repos.WhatsApp.UpsertPendingTrialCandidate(ctx, leadSub)
	if err != nil {
		slog.Warn("Failed to pre-save candidate lead in DB before OTP", "phone", normPhone, "err", err)
	} else {
		slog.Info("Candidate pre-saved in DB with pending_verification status", "sub_id", savedSub.ID, "phone", normPhone, "name", cleanFullName)
	}

	// 5. CRITICAL: Pre-save candidate contact in WhatsApp Session & Google Contacts BEFORE OTP dispatch
	if err := h.waSvc.SaveContact(ctx, normPhone, cleanFullName); err != nil {
		slog.Warn("Failed to pre-save contact in WhatsApp session/Google Contacts", "phone", normPhone, "name", cleanFullName, "err", err)
	}

	// 6. Generate 6-digit verification code and save in Redis
	code := whatsapp.Generate6DigitOTP()
	pending := pendingTrialData{
		Phone:    normPhone,
		Code:     code,
		Category: category,
		Location: location,
		FullName: cleanFullName,
		Attempts: 0,
	}
	pendingBytes, _ := json.Marshal(pending)
	if h.rdb != nil {
		_ = h.rdb.Set(ctx, "wa_trial_pending:"+normPhone, string(pendingBytes), 10*time.Minute).Err()
		_ = h.rdb.Set(ctx, fmt.Sprintf("wa_rl_trial_cd:%s", normPhone), "1", 60*time.Second).Err()
	}

	// 7. Send verification OTP directly to the WhatsApp number
	_ = h.waSvc.SendTrialVerificationOTP(ctx, normPhone, code, cleanFullName)

	// Update WhatsApp analytics event for OTP dispatch
	_ = h.repos.WhatsApp.UpdateAnalyticsPhoneAndOTP(ctx, trackingID, visitorID, normPhone)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"step":    "verify",
		"phone":   normPhone,
		"message": "We have sent a 6-digit verification code to your WhatsApp. Enter it below to activate your 3-Day Free Trial.",
	})
}

// TrialVerify validates ownership with the 6-digit code, activates the trial, and locks the code as permanent PIN.
func (h *WhatsAppClientHandlers) TrialVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var phone, code, visitorID string
	var trackingID int
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		var req struct {
			Phone      string `json:"phone"`
			Code       string `json:"code"`
			PIN        string `json:"pin"`
			TrackingID int    `json:"tracking_id"`
			VisitorID  string `json:"visitor_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			phone = req.Phone
			code = req.Code
			if code == "" && req.PIN != "" {
				code = req.PIN
			}
			trackingID = req.TrackingID
			visitorID = req.VisitorID
		}
	} else {
		phone = r.FormValue("phone")
		code = r.FormValue("code")
		if code == "" && r.FormValue("pin") != "" {
			code = r.FormValue("pin")
		}
		if tid, err := strconv.Atoi(r.FormValue("tracking_id")); err == nil {
			trackingID = tid
		}
		visitorID = r.FormValue("visitor_id")
	}

	normPhone := repository.NormalizePhoneNumber(phone)
	cleanCode := strings.TrimSpace(code)
	if normPhone == "" || len(cleanCode) != 6 {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Please enter the complete 6-digit verification code.",
		})
		return
	}

	ctx := r.Context()
	rawPending, err := h.rdb.Get(ctx, "wa_trial_pending:"+normPhone).Result()
	if err != nil || rawPending == "" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Verification session expired or invalid. Please request a new code.",
		})
		return
	}

	var pending pendingTrialData
	if err := json.Unmarshal([]byte(rawPending), &pending); err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Verification parsing error. Please try again.",
		})
		return
	}

	if pending.Attempts >= 5 {
		_ = h.rdb.Del(ctx, "wa_trial_pending:"+normPhone).Err()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Too many incorrect attempts. Please request a new verification code.",
		})
		return
	}

	if cleanCode != pending.Code {
		pending.Attempts++
		updatedBytes, _ := json.Marshal(pending)
		ttl, _ := h.rdb.TTL(ctx, "wa_trial_pending:"+normPhone).Result()
		_ = h.rdb.Set(ctx, "wa_trial_pending:"+normPhone, string(updatedBytes), ttl).Err()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Invalid 6-digit verification code (%d attempts remaining).", 5-pending.Attempts),
		})
		return
	}

	// Code is confirmed! Delete pending trial
	_ = h.rdb.Del(ctx, "wa_trial_pending:"+normPhone).Err()

	// Double-check subscriber status: block only if already active
	existingSub, _ := h.repos.WhatsApp.GetSubscriberByPhone(ctx, normPhone)
	if existingSub != nil && existingSub.Status == "active" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":            false,
			"already_registered": true,
			"error":              "This WhatsApp number is already registered in our system. Please sign in to your dashboard using your 6-digit PIN.",
			"phone":              normPhone,
			"login_url":          "/my-alerts/login",
		})
		return
	}

	// Hash the verified 6-digit code so it becomes their permanent dashboard PIN!
	pinHash, err := repository.HashPIN(pending.Code)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Internal security error creating trial.",
		})
		return
	}

	trialEnd := time.Now().Add(72 * time.Hour)
	candidateName := strings.TrimSpace(pending.FullName)
	if candidateName == "" && existingSub != nil && existingSub.FullName != "" {
		candidateName = existingSub.FullName
	}
	if candidateName == "" {
		suffix := normPhone
		if len(normPhone) > 4 {
			suffix = normPhone[len(normPhone)-4:]
		}
		candidateName = fmt.Sprintf("Candidate %s", suffix)
	}

	var newSub *models.WhatsAppSubscriber
	if existingSub != nil && existingSub.Status == "pending_verification" {
		if err := h.repos.WhatsApp.ActivatePendingTrialSubscriber(ctx, existingSub.ID, candidateName, pinHash, trialEnd); err != nil {
			slog.Error("Failed to activate pending trial subscriber", "phone", normPhone, "err", err)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Could not register free trial. Please try again or contact support.",
			})
			return
		}
		newSub = existingSub
		newSub.Status = "active"
		newSub.FullName = candidateName
		newSub.PINHash = pinHash
		newSub.TrialEndsAt = &trialEnd
		newSub.ExpiresAt = trialEnd
		newSub.PaymentStatus = "trial"
	} else {
		newSub = &models.WhatsAppSubscriber{
			PhoneNumber:   normPhone,
			FullName:      candidateName,
			Email:         "",
			PlanType:      "trial",
			PlanPriceGBP:  0.0,
			Status:        "active",
			Source:        "homepage_trial",
			PaymentStatus: "trial",
			Categories:    []string{pending.Category},
			Locations:     []string{pending.Location},
			MinSalary:     0,
			AlertMode:     "instant",
			TrialEndsAt:   &trialEnd,
			ExpiresAt:     trialEnd,
			PINHash:       pinHash,
		}
		if err := h.repos.WhatsApp.CreateSubscriber(ctx, newSub); err != nil {
			slog.Error("Failed to create trial subscriber after verification", "phone", normPhone, "err", err)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Could not register free trial. Please try again or contact support.",
			})
			return
		}
	}

	// Dispatch authentic Welcome message with their verified permanent PIN
	go func() {
		dispatchCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = h.waSvc.SendTrialWelcomeWithPIN(dispatchCtx, normPhone, pending.Code, pending.Category, pending.Location)
	}()

	// 3-5 Minute Candidate Trust Engine: Schedule candidate's 1st high-match job alert after verification
	if h.alertDispatcher != nil {
		h.alertDispatcher.ScheduleInitialCandidateAlert(newSub, 0)
	}

	// Log security audit event
	_ = h.repos.WhatsApp.LogSubscriberLogin(ctx, newSub.ID, normPhone, getClientIP(r), r.UserAgent(), "trial_verify", "success")

	// Update WhatsApp analytics event for verified conversion
	_ = h.repos.WhatsApp.UpdateAnalyticsVerified(ctx, trackingID, visitorID, normPhone, newSub.ID)

	// Issue login session so verified trial subscriber lands directly on their dashboard
	h.setSessionCookies(w, newSub.ID, normPhone)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"message":      "Phone verified successfully! Your 3-Day Free Trial is now active.",
		"phone":        normPhone,
		"redirect_url": "/my-alerts",
		"login_url":    "/my-alerts",
	})
}

// UpgradeCheckout processes simulation upgrade payment to unlock the dashboard and continue alerts.
func (h *WhatsAppClientHandlers) UpgradeCheckout(w http.ResponseWriter, r *http.Request) {
	sub, err := h.getAuthSubscriber(r)
	if err != nil || sub == nil {
		if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Unauthorized"})
			return
		}
		http.Redirect(w, r, "/my-alerts/login", http.StatusSeeOther)
		return
	}

	var planType string
	var payMethod string
	var incomingTxRef string
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var body struct {
			PlanType string `json:"plan_type"`
			Method   string `json:"method"`
			TxRef    string `json:"tx_ref"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		planType = body.PlanType
		payMethod = body.Method
		incomingTxRef = strings.TrimSpace(body.TxRef)
	} else {
		planType = r.FormValue("plan_type")
		payMethod = r.FormValue("method")
		incomingTxRef = strings.TrimSpace(r.FormValue("tx_ref"))
	}

	if payMethod == "" {
		payMethod = "card"
	}

	if planType == "" {
		planType = "monthly"
	}

	var price float64
	var days int
	switch planType {
	case "monthly":
		price = 4.99
		days = 30
	case "6months":
		price = 24.99
		days = 180
	case "yearly":
		price = 39.99
		days = 365
	default:
		planType = "monthly"
		price = 4.99
		days = 30
	}

	ctx := r.Context()
	if err := h.repos.WhatsApp.UpgradeSubscription(ctx, sub.ID, planType); err != nil {
		slog.Error("Failed to upgrade subscription", "sub_id", sub.ID, "err", err)
		if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Upgrade failed"})
			return
		}
		h.setFlash(w, FlashData{Error: "Payment processing error. Please try again."})
		http.Redirect(w, r, "/my-alerts", http.StatusSeeOther)
		return
	}

	expiryDate := time.Now().AddDate(0, 0, days)

	// Send WhatsApp payment receipt
	go func() {
		dispatchCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = h.waSvc.SendPaymentConfirmation(dispatchCtx, sub.PhoneNumber, planType, payMethod, price, expiryDate)
	}()

	txRef := incomingTxRef
	if txRef == "" {
		if payMethod == "paypal" {
			txRef = fmt.Sprintf("PAYPAL-%d", time.Now().Unix()%10000000+1000000)
		} else {
			txRef = fmt.Sprintf("UK-WA-%d", time.Now().Unix()%1000000+100000)
		}
	}

	// Record transaction in unified ledger
	_ = h.repos.WhatsApp.LogTransaction(ctx, &sub.ID, sub.PhoneNumber, sub.FullName, planType, price, payMethod, txRef, "completed", "")

	// Log audit event with payment method
	auditAction := fmt.Sprintf("plan_upgrade_%s", payMethod)
	_ = h.repos.WhatsApp.LogSubscriberLogin(ctx, sub.ID, sub.PhoneNumber, getClientIP(r), r.UserAgent(), auditAction, "success")

	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":    true,
			"message":    "Subscription activated successfully. Receipt dispatched to your WhatsApp.",
			"plan_type":  planType,
			"method":     payMethod,
			"tx_ref":     txRef,
			"amount":     fmt.Sprintf("£%.2f", price),
			"expires_at": expiryDate.Format("02 Jan 2006"),
		})
		return
	}

	h.setFlash(w, FlashData{Success: "Payment confirmed. Your plan is active and all features are unlocked."})
	http.Redirect(w, r, "/my-alerts", http.StatusSeeOther)
}

// RequestInvoice generates an official pro-forma invoice and dispatches it directly to the candidate's WhatsApp via the connected WhatsApp gateway.
func (h *WhatsAppClientHandlers) RequestInvoice(w http.ResponseWriter, r *http.Request) {
	sub, err := h.getAuthSubscriber(r)
	if err != nil || sub == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Session expired. Please log in again."})
		return
	}

	var planType string
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var body struct {
			PlanType string `json:"plan_type"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		planType = body.PlanType
	} else {
		planType = r.FormValue("plan_type")
	}

	planType = strings.TrimSpace(planType)
	if planType == "" {
		planType = "6months"
	}

	var price float64
	var planName string
	switch planType {
	case "monthly":
		price = 4.99
		planName = "1-Month Pass"
	case "yearly":
		price = 39.99
		planName = "Annual Best Value Pass"
	default:
		planType = "6months"
		price = 24.99
		planName = "6-Month VIP Career Pass"
	}

	// Rate limit check: 1 invoice request per 60 seconds per subscriber
	rateLimitKey := fmt.Sprintf("wa:invoice_ratelimit:%d", sub.ID)
	if h.rdb != nil {
		ttl, err := h.rdb.TTL(r.Context(), rateLimitKey).Result()
		if err == nil && ttl > 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success":     false,
				"error":       fmt.Sprintf("An official invoice was recently sent to your WhatsApp. Please wait %d seconds before requesting another.", int(ttl.Seconds())),
				"retry_after": int(ttl.Seconds()),
			})
			return
		}
	}

	invoiceRef := fmt.Sprintf("INV-WA-%d-%d", sub.ID, time.Now().Year())

	paymentSettings, _ := h.repos.WhatsApp.GetPaymentSettings(r.Context())
	supportNum := "+923025015693"
	instructions := "Reply directly to this message to receive UK bank transfer details (Sort Code & Account Number), Wise, or Revolut payment options."
	if paymentSettings != nil {
		if paymentSettings.SupportNumber != "" {
			supportNum = paymentSettings.SupportNumber
		}
		if paymentSettings.SupportInstructions != "" {
			instructions = paymentSettings.SupportInstructions
		}
	}

	// Compose official executive invoice message (clean typography, no raw emojis)
	msg := fmt.Sprintf(
		"*JobyUK.com OFFICIAL PRO-FORMA INVOICE*\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"*Invoice Ref:* #%s\n"+
			"*Candidate:* %s\n"+
			"*Registered Phone:* %s\n"+
			"*Plan Selected:* %s\n"+
			"*Total Payable:* £%.2f GBP\n"+
			"*Issue Date:* %s\n"+
			"*Status:* [Awaiting Transfer / Confirmation]\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n\n"+
			"*Concierge Instructions:*\n"+
			"%s\n\n"+
			"*Next Step:* Reply directly to this WhatsApp message or contact our billing concierge at %s with your invoice ref (#%s). Once transferred, your VIP job alerts will be activated immediately!\n\n"+
			"Best regards,\n*JobyUK.com Recruitment Billing Gateway*",
		invoiceRef, sub.FullName, sub.PhoneNumber, planName, price,
		time.Now().Format("02 Jan 2006, 15:04"), instructions, supportNum, invoiceRef,
	)

	// Dispatch message to candidate's WhatsApp via connected whatsmeow service
	dispatchCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err = h.waSvc.DispatchChannelMessageWithContact(dispatchCtx, "billing", sub.PhoneNumber, sub.FullName, msg, "Official Pro-Forma Invoice")
	if err != nil {
		slog.Error("Failed to dispatch invoice to WhatsApp", "phone", sub.PhoneNumber, "err", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Could not dispatch via WhatsApp gateway. Please ensure our WhatsApp service is online.",
		})
		return
	}

	// Set 60s rate limit key in Redis after successful dispatch
	if h.rdb != nil {
		_ = h.rdb.Set(r.Context(), rateLimitKey, "1", 60*time.Second).Err()
	}

	// Log transaction in unified ledger as pending manual invoice
	ctx := r.Context()
	_ = h.repos.WhatsApp.LogTransaction(ctx, &sub.ID, sub.PhoneNumber, sub.FullName, planType, price, "invoice", invoiceRef, "pending", "Pro-forma invoice dispatched to WhatsApp")

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"invoice_ref": invoiceRef,
		"phone":       sub.PhoneNumber,
		"support_num": supportNum,
		"plan_name":   planName,
		"amount":      fmt.Sprintf("£%.2f", price),
		"message":     fmt.Sprintf("Official Pro-Forma Invoice #%s has been dispatched to your WhatsApp (%s)!", invoiceRef, sub.PhoneNumber),
		"retry_after": 60,
	})
}

// SubmitCryptoPayment receives crypto transaction hash, selected network/coin, and uploaded screenshot proof.
func (h *WhatsAppClientHandlers) SubmitCryptoPayment(w http.ResponseWriter, r *http.Request) {
	sub, err := h.getAuthSubscriber(r)
	if err != nil || sub == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Unauthorized session"})
		return
	}

	// Limit request body to 12MB for screenshot upload
	if err := r.ParseMultipartForm(12 << 20); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "File exceeds 10MB limit"})
		return
	}

	planType := strings.TrimSpace(r.FormValue("plan_type"))
	if planType == "" {
		planType = "6months"
	}
	var amountGBP float64
	switch planType {
	case "monthly":
		amountGBP = 4.99
	case "6months":
		amountGBP = 24.99
	case "yearly":
		amountGBP = 39.99
	default:
		amountGBP = 24.99
		planType = "6months"
	}

	coin := strings.TrimSpace(r.FormValue("coin"))
	network := strings.TrimSpace(r.FormValue("network"))
	depositAddress := strings.TrimSpace(r.FormValue("deposit_address"))
	txHash := strings.TrimSpace(r.FormValue("tx_hash"))
	senderAddress := strings.TrimSpace(r.FormValue("sender_address"))

	if txHash == "" || len(txHash) < 8 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "A valid Transaction Hash / TxID is required (minimum 8 characters)"})
		return
	}

	file, _, err := r.FormFile("screenshot")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Please upload a transfer confirmation screenshot"})
		return
	}
	defer file.Close()

	// Validate content type
	buff := make([]byte, 512)
	_, _ = file.Read(buff)
	_, _ = file.Seek(0, 0)
	contentType := http.DetectContentType(buff)
	if !strings.HasPrefix(contentType, "image/") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Only image files (PNG, JPG, WEBP) are accepted as proof"})
		return
	}

	ext := ".png"
	if strings.Contains(contentType, "jpeg") {
		ext = ".jpg"
	} else if strings.Contains(contentType, "webp") {
		ext = ".webp"
	}

	uploadDir := filepath.Join("web", "static", "uploads", "payment_proofs")
	_ = os.MkdirAll(uploadDir, 0755)

	safeFilename := fmt.Sprintf("proof_%d_%d%s", sub.ID, time.Now().Unix(), ext)
	filePath := filepath.Join(uploadDir, safeFilename)

	dst, err := os.Create(filePath)
	if err != nil {
		slog.Error("Failed to save payment proof", "err", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Unable to save uploaded proof on server"})
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		slog.Error("Failed to write payment proof", "err", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Failed to write file"})
		return
	}

	webPath := "/static/uploads/payment_proofs/" + safeFilename

	payment := &models.WhatsAppCryptoPayment{
		SubscriberID:   sub.ID,
		PhoneNumber:    sub.PhoneNumber,
		FullName:       sub.FullName,
		PlanType:       planType,
		AmountGBP:      amountGBP,
		Coin:           coin,
		Network:        network,
		DepositAddress: depositAddress,
		TxHash:         txHash,
		SenderAddress:  senderAddress,
		ScreenshotURL:  webPath,
	}

	ctx := r.Context()
	if err := h.repos.WhatsApp.CreateCryptoPayment(ctx, payment); err != nil {
		slog.Error("Failed to record crypto payment", "err", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Database error saving payment"})
		return
	}

	// Dispatch WhatsApp acknowledgment to subscriber
	go func() {
		msg := fmt.Sprintf(
			"⏳ *JobyUK.com Crypto Payment Received*\n\n"+
				"Hello %s,\n\n"+
				"We have received your payment proof for the *%s* plan (£%.2f).\n\n"+
				"📌 *Transaction ID:* %s\n"+
				"🪙 *Network:* %s (%s)\n"+
				"📸 *Proof Receipt:* Uploaded successfully\n\n"+
				"Our verification team is verifying your on-chain transfer. Your unlimited WhatsApp job alerts will be activated immediately upon confirmation!\n\n"+
				"Track your account: %s/my-alerts",
			sub.FullName, strings.ToUpper(planType), amountGBP, txHash, coin, network, h.cfg.BaseURL,
		)
		_ = h.waSvc.DispatchChannelMessage(context.Background(), "billing", sub.PhoneNumber, msg, "Crypto Payment Acknowledgment")
	}()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"message":   "Proof received! On-chain verification is in progress. A receipt was sent to your WhatsApp.",
		"plan_type": planType,
		"tx_hash":   txHash,
		"image_url": webPath,
	})
}

// TrackClickAPI records a trial CTA click with visitor, device, IP, and location tracking.
func (h *WhatsAppClientHandlers) TrackClickAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		VisitorID        string `json:"visitor_id"`
		Source           string `json:"source"`
		AriaLabel        string `json:"aria_label"`
		TriggerPlacement string `json:"trigger_placement"`
		PageURL          string `json:"page_url"`
		Referrer         string `json:"referrer"`
	}

	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		_ = json.NewDecoder(r.Body).Decode(&req)
	} else {
		req.VisitorID = r.FormValue("visitor_id")
		req.Source = r.FormValue("source")
		req.AriaLabel = r.FormValue("aria_label")
		req.TriggerPlacement = r.FormValue("trigger_placement")
		req.PageURL = r.FormValue("page_url")
		req.Referrer = r.FormValue("referrer")
	}

	// Exclude admin sessions and internal portals from polluting WhatsApp analytics
	if cookie, err := r.Cookie("wa_admin_session"); err == nil && cookie.Value != "" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":     true,
			"tracking_id": 0,
			"visitor_id":  req.VisitorID,
			"ignored":     "admin_session",
		})
		return
	}
	if cookie, err := r.Cookie("admin_token"); err == nil && cookie.Value != "" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":     true,
			"tracking_id": 0,
			"visitor_id":  req.VisitorID,
			"ignored":     "admin_token",
		})
		return
	}
	if strings.Contains(req.PageURL, "/wa-portal") || strings.Contains(req.PageURL, "/my-alerts") || strings.Contains(req.PageURL, "/my-alerts") || strings.Contains(req.PageURL, "/admin") || strings.Contains(req.PageURL, "/pin-") {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":     true,
			"tracking_id": 0,
			"visitor_id":  req.VisitorID,
			"ignored":     "internal_portal_url",
		})
		return
	}

	if req.VisitorID == "" {
		if cookie, err := r.Cookie("hirely_wa_vid"); err == nil && cookie.Value != "" {
			req.VisitorID = cookie.Value
		} else {
			req.VisitorID = fmt.Sprintf("v_%d", time.Now().UnixNano())
		}
	}
	if req.Source == "" {
		req.Source = "floating_bar"
	}
	if req.TriggerPlacement == "" {
		req.TriggerPlacement = "Floating Bar"
	}

	clientIP := getClientIP(r)
	ua := r.UserAgent()
	deviceType := parseDeviceType(ua)
	browser := parseBrowser(ua)
	osName := parseOS(ua)
	loc := detectLocation(r, clientIP)

	ev := &models.WhatsAppAnalyticsEvent{
		VisitorID:        req.VisitorID,
		Source:           req.Source,
		AriaLabel:        req.AriaLabel,
		TriggerPlacement: req.TriggerPlacement,
		PageURL:          req.PageURL,
		Referrer:         req.Referrer,
		IPAddress:        clientIP,
		Location:         loc,
		UserAgent:        ua,
		DeviceType:       deviceType,
		Browser:          browser,
		OS:               osName,
		Step:             "clicked",
	}

	trackingID, err := h.repos.WhatsApp.LogAnalyticsClick(r.Context(), ev)
	if err != nil {
		slog.Error("Failed to log WhatsApp trial analytics click", "err", err)
	}

	// Set cookie so visitor ID sticks
	http.SetCookie(w, &http.Cookie{
		Name:     "hirely_wa_vid",
		Value:    req.VisitorID,
		Path:     "/",
		MaxAge:   86400 * 365, // 1 year
		HttpOnly: false,       // accessible to JS
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"tracking_id": trackingID,
		"visitor_id":  req.VisitorID,
	})
}

func parseDeviceType(ua string) string {
	low := strings.ToLower(ua)
	if strings.Contains(low, "ipad") || strings.Contains(low, "tablet") || strings.Contains(low, "playbook") || strings.Contains(low, "silk") {
		return "tablet"
	}
	if strings.Contains(low, "mobile") || strings.Contains(low, "iphone") || strings.Contains(low, "android") || strings.Contains(low, "phone") {
		return "mobile"
	}
	return "desktop"
}

func parseOS(ua string) string {
	low := strings.ToLower(ua)
	if strings.Contains(low, "iphone") || strings.Contains(low, "ipad") || strings.Contains(low, "ipod") {
		return "iOS"
	}
	if strings.Contains(low, "android") {
		return "Android"
	}
	if strings.Contains(low, "windows") {
		return "Windows"
	}
	if strings.Contains(low, "mac os") || strings.Contains(low, "macintosh") {
		return "macOS"
	}
	if strings.Contains(low, "linux") {
		return "Linux"
	}
	if strings.Contains(low, "cros") {
		return "ChromeOS"
	}
	return "Other"
}

func parseBrowser(ua string) string {
	low := strings.ToLower(ua)
	if strings.Contains(low, "edg") {
		return "Edge"
	}
	if strings.Contains(low, "opr") || strings.Contains(low, "opera") {
		return "Opera"
	}
	if strings.Contains(low, "chrome") && !strings.Contains(low, "chromium") && !strings.Contains(low, "edg") {
		return "Chrome"
	}
	if strings.Contains(low, "safari") && !strings.Contains(low, "chrome") {
		return "Safari"
	}
	if strings.Contains(low, "firefox") {
		return "Firefox"
	}
	return "Browser"
}

var (
	geoCacheMu sync.RWMutex
	geoCache   = make(map[string]string)
)

func isLocalOrPrivateIP(ipStr string) bool {
	if ipStr == "" || ipStr == "127.0.0.1" || ipStr == "::1" || ipStr == "localhost" {
		return true
	}
	pip := net.ParseIP(ipStr)
	if pip == nil {
		return true
	}
	return pip.IsLoopback() || pip.IsPrivate() || pip.IsLinkLocalUnicast()
}

func detectLocation(r *http.Request, ip string) string {
	// 1. Cloudflare Country & City headers if present
	cfCountry := strings.TrimSpace(r.Header.Get("CF-IPCountry"))
	cfCity := strings.TrimSpace(r.Header.Get("CF-IPCity"))
	if cfCountry != "" {
		if cfCity != "" {
			return fmt.Sprintf("%s, %s", cfCity, cfCountry)
		}
		return cfCountry
	}

	// Clean IP address
	cleanIP := strings.TrimSpace(ip)
	if idx := strings.LastIndex(cleanIP, ":"); idx != -1 && !strings.Contains(cleanIP, "]") {
		cleanIP = cleanIP[:idx]
	}
	cleanIP = strings.Trim(cleanIP, "[]")

	// 2. Check thread-safe memory cache first
	geoCacheMu.RLock()
	cached, found := geoCache[cleanIP]
	geoCacheMu.RUnlock()
	if found && cached != "" {
		return cached
	}

	isLocal := isLocalOrPrivateIP(cleanIP)

	// 3. Geolocation Lookup via ip-api.com
	apiURL := "http://ip-api.com/json/" + url.PathEscape(cleanIP) + "?fields=status,country,city,countryCode"
	if isLocal {
		// When developing locally or from a private network, query the external WAN IP location directly
		apiURL = "http://ip-api.com/json/?fields=status,country,city,countryCode"
	}

	client := &http.Client{Timeout: 3500 * time.Millisecond}
	resp, err := client.Get(apiURL)
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		var geo struct {
			Status      string `json:"status"`
			Country     string `json:"country"`
			CountryCode string `json:"countryCode"`
			City        string `json:"city"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&geo); err == nil && geo.Status == "success" {
			locStr := geo.Country
			if geo.City != "" {
				locStr = fmt.Sprintf("%s, %s", geo.City, geo.Country)
			}
			if locStr != "" {
				geoCacheMu.Lock()
				geoCache[cleanIP] = locStr
				geoCacheMu.Unlock()
				return locStr
			}
		}
	}

	// 4. Secondary fallback: ipwho.is
	secURL := "http://ipwho.is/" + url.PathEscape(cleanIP)
	if isLocal {
		secURL = "http://ipwho.is/"
	}
	secResp, secErr := client.Get(secURL)
	if secErr == nil && secResp.StatusCode == http.StatusOK {
		defer secResp.Body.Close()
		var whois struct {
			Success bool   `json:"success"`
			Country string `json:"country"`
			City    string `json:"city"`
		}
		if err := json.NewDecoder(secResp.Body).Decode(&whois); err == nil && whois.Success {
			locStr := whois.Country
			if whois.City != "" {
				locStr = fmt.Sprintf("%s, %s", whois.City, whois.Country)
			}
			if locStr != "" {
				geoCacheMu.Lock()
				geoCache[cleanIP] = locStr
				geoCacheMu.Unlock()
				return locStr
			}
		}
	}

	// 5. Graceful Fallback
	if isLocal {
		return "London, United Kingdom (Dev)"
	}
	return "London, United Kingdom"
}



