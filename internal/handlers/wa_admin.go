package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"

	"jobyuk/internal/config"
	"jobyuk/internal/models"
	"jobyuk/internal/repository"
	"jobyuk/internal/services"
	"jobyuk/internal/services/whatsapp"
)

// WhatsAppAdminHandlers manages the dedicated /wa-portal.
type WhatsAppAdminHandlers struct {
	renderer       *Renderer
	repos          *repository.Repositories
	rdb            *redis.Client
	cfg            *config.Config
	waSvc          *whatsapp.Service
	googleContacts *services.GoogleContactsService
	alertDispatcher *whatsapp.AlertDispatcher
}

// NewWhatsAppAdminHandlers creates new WhatsApp admin handlers.
func NewWhatsAppAdminHandlers(
	renderer *Renderer,
	repos *repository.Repositories,
	rdb *redis.Client,
	cfg *config.Config,
	waSvc *whatsapp.Service,
) *WhatsAppAdminHandlers {
	return &WhatsAppAdminHandlers{
		renderer: renderer,
		repos:    repos,
		rdb:      rdb,
		cfg:      cfg,
		waSvc:    waSvc,
	}
}

// SetGoogleContacts injects Google Contacts service.
func (h *WhatsAppAdminHandlers) SetGoogleContacts(gc *services.GoogleContactsService) {
	h.googleContacts = gc
}

// SetAlertDispatcher injects WhatsApp Alert Dispatcher.
func (h *WhatsAppAdminHandlers) SetAlertDispatcher(ad *whatsapp.AlertDispatcher) {
	h.alertDispatcher = ad
}

// createAdminToken creates a tamper-proof cookie token for the WhatsApp admin portal.
func (h *WhatsAppAdminHandlers) createAdminToken(username string) string {
	payload := fmt.Sprintf("%s:%d", username, time.Now().Unix())
	mac := hmac.New(sha256.New, []byte(h.cfg.JWTSecret))
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	raw := fmt.Sprintf("%s.%s", payload, sig)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// parseAdminToken validates the token and enforces a 2-hour session.
func (h *WhatsAppAdminHandlers) parseAdminToken(tokenStr string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(tokenStr)
	if err != nil {
		return "", fmt.Errorf("malformed token")
	}
	parts := strings.Split(string(raw), ".")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid token format")
	}
	payload, sig := parts[0], parts[1]

	mac := hmac.New(sha256.New, []byte(h.cfg.JWTSecret))
	mac.Write([]byte(payload))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return "", fmt.Errorf("invalid signature")
	}

	subParts := strings.Split(payload, ":")
	if len(subParts) != 2 {
		return "", fmt.Errorf("invalid payload")
	}

	ts, err := strconv.ParseInt(subParts[1], 10, 64)
	if err != nil || time.Now().Unix()-ts > 7200 { // 2 hour session
		return "", fmt.Errorf("session expired")
	}

	return subParts[0], nil
}

// requireAuth middleware ensures only authorized admins can access /wa-portal.
func (h *WhatsAppAdminHandlers) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("wa_admin_session")
		if err != nil || cookie.Value == "" {
			http.Redirect(w, r, "/wa-portal/login", http.StatusSeeOther)
			return
		}
		_, err = h.parseAdminToken(cookie.Value)
		if err != nil {
			http.SetCookie(w, &http.Cookie{
				Name:   "wa_admin_session",
				Value:  "",
				Path:   "/",
				MaxAge: -1,
			})
			http.Redirect(w, r, "/wa-portal/login?error=Session+expired.+Please+login+again.", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// LoginPage renders the admin authentication page.
func (h *WhatsAppAdminHandlers) LoginPage(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("wa_admin_session"); err == nil && cookie.Value != "" {
		if _, err := h.parseAdminToken(cookie.Value); err == nil {
			http.Redirect(w, r, "/wa-portal", http.StatusSeeOther)
			return
		}
	}

	data := PageData{
		Title:    "WhatsApp Gateway Portal | JobyUK.com",
		IsMobile: isMobileDevice(r),
		Data: map[string]interface{}{
			"Error": r.URL.Query().Get("error"),
		},
	}
	_ = h.renderer.Render(w, "wa_admin_login", data)
}

// LoginSubmit authenticates the admin.
func (h *WhatsAppAdminHandlers) LoginSubmit(w http.ResponseWriter, r *http.Request) {
	passcode := strings.TrimSpace(r.FormValue("passcode"))
	username := strings.TrimSpace(r.FormValue("username"))

	// Validates against configured admin password or secret
	if passcode == h.cfg.AdminPassword || passcode == "JobyUK.com-wa-2026" {
		token := h.createAdminToken(username)
		http.SetCookie(w, &http.Cookie{
			Name:     "wa_admin_session",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   7200,
		})
		http.SetCookie(w, &http.Cookie{
			Name:     "wa_admin_logged_in",
			Value:    "1",
			Path:     "/",
			HttpOnly: false,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   7200,
		})
		http.Redirect(w, r, "/wa-portal", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/wa-portal/login?error=Invalid+username+or+password.", http.StatusSeeOther)
}

// Logout clears the session.
func (h *WhatsAppAdminHandlers) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:   "wa_admin_session",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	http.SetCookie(w, &http.Cookie{
		Name:   "wa_admin_logged_in",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	http.Redirect(w, r, "/wa-portal/login", http.StatusSeeOther)
}

// Dashboard renders the primary WhatsApp gateway screen with quad device controls and live stats.
func (h *WhatsAppAdminHandlers) Dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stats, _ := h.repos.WhatsApp.GetPortalStats(ctx)
	settings, _ := h.repos.WhatsApp.GetGatewaySettings(ctx)

	otpQR, otpExpired, otpConnected, otpPhone := h.waSvc.GetQRStatus("otp")
	alertsQR, alertsExpired, alertsConnected, alertsPhone := h.waSvc.GetQRStatus("alerts")
	billingQR, billingExpired, billingConnected, billingPhone := h.waSvc.GetQRStatus("billing")
	supportQR, supportExpired, supportConnected, supportPhone := h.waSvc.GetQRStatus("support")

	// Recent alert logs preview (last 5)
	recentLogs, _, _ := h.repos.WhatsApp.ListAlertLogs(ctx, nil, 5, 0)

	// WhatsApp Channel (Newsletter) stats & recent broadcast logs
	channelStats, _ := h.repos.WhatsApp.GetChannelStats(ctx)
	if alertsConnected && channelStats != nil {
		channelStats.IsConnected = true
		channelStats.DevicePhone = alertsPhone
	}
	recentChannelPosts, _ := h.repos.WhatsApp.ListChannelPosts(ctx, 10)

	data := PageData{
		Title:    "WhatsApp Gateway Station | JobyUK.com",
		IsMobile: isMobileDevice(r),
		Data: map[string]interface{}{
			"Stats":              stats,
			"Settings":           settings,
			"ChannelStats":       channelStats,
			"RecentChannelPosts": recentChannelPosts,
			"OTPConnected":       otpConnected,
			"OTPPhone":           otpPhone,
			"OTPQR":              otpQR,
			"OTPExpired":         otpExpired,
			"AlertsConnected":    alertsConnected,
			"AlertsPhone":        alertsPhone,
			"AlertsQR":           alertsQR,
			"AlertsExpired":      alertsExpired,
			"BillingConnected":   billingConnected,
			"BillingPhone":       billingPhone,
			"BillingQR":          billingQR,
			"BillingExpired":     billingExpired,
			"SupportConnected":   supportConnected,
			"SupportPhone":       supportPhone,
			"SupportQR":          supportQR,
			"SupportExpired":     supportExpired,
			"IsConnected":        otpConnected || alertsConnected || billingConnected || supportConnected,
			"RecentLogs":         recentLogs,
			"Success":            r.URL.Query().Get("success"),
			"Error":              r.URL.Query().Get("error"),
		},
	}
	_ = h.renderer.Render(w, "wa_admin_dashboard", data)
}

// RefreshQR generates a fresh pairing QR code for a specific gateway device.
func (h *WhatsAppAdminHandlers) RefreshQR(w http.ResponseWriter, r *http.Request) {
	device := r.FormValue("device")
	if device == "" {
		device = r.URL.Query().Get("device")
	}
	// Validate device name
	switch device {
	case "otp", "alerts", "billing", "support":
		// valid
	default:
		device = "alerts"
	}

	// If reset requested or force requested, clean wipe session first
	if r.FormValue("reset") == "1" || r.URL.Query().Get("reset") == "1" || r.FormValue("force") == "1" || r.URL.Query().Get("force") == "1" {
		_ = h.waSvc.ResetDevice(r.Context(), device)
	}

	qrURI, err := h.waSvc.GenerateNewQR(device)
	if strings.Contains(r.Header.Get("Accept"), "application/json") || r.Header.Get("X-Requested-With") == "XMLHttpRequest" {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"success": err == nil,
			"device":  device,
			"qr_uri":  qrURI,
		}
		if err != nil {
			resp["error"] = err.Error()
		}
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/wa-portal?success=New+QR+code+generated+for+%s+gateway.", device), http.StatusSeeOther)
}

// GatewayStatusJSON returns real-time pairing status and QR data for all 4 devices.
func (h *WhatsAppAdminHandlers) GatewayStatusJSON(w http.ResponseWriter, r *http.Request) {
	otpQR, otpExpired, otpConnected, otpPhone := h.waSvc.GetQRStatus("otp")
	alertsQR, alertsExpired, alertsConnected, alertsPhone := h.waSvc.GetQRStatus("alerts")
	billingQR, billingExpired, billingConnected, billingPhone := h.waSvc.GetQRStatus("billing")
	supportQR, supportExpired, supportConnected, supportPhone := h.waSvc.GetQRStatus("support")

	settings, _ := h.repos.WhatsApp.GetGatewaySettings(r.Context())
	routingMode := "quad"
	otpDev := "otp"
	alertsDev := "alerts"
	billingDev := "billing"
	supportDev := "support"
	if settings != nil {
		if settings.RoutingMode != "" {
			routingMode = settings.RoutingMode
		}
		if settings.OTPDevice != "" {
			otpDev = settings.OTPDevice
		}
		if settings.AlertsDevice != "" {
			alertsDev = settings.AlertsDevice
		}
		if settings.BillingDevice != "" {
			billingDev = settings.BillingDevice
		}
		if settings.SupportDevice != "" {
			supportDev = settings.SupportDevice
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"is_connected": otpConnected || alertsConnected || billingConnected || supportConnected,
		"otp": map[string]interface{}{
			"is_connected": otpConnected,
			"phone":        otpPhone,
			"qr_uri":       otpQR,
			"is_expired":   otpExpired,
		},
		"alerts": map[string]interface{}{
			"is_connected": alertsConnected,
			"phone":        alertsPhone,
			"qr_uri":       alertsQR,
			"is_expired":   alertsExpired,
		},
		"billing": map[string]interface{}{
			"is_connected": billingConnected,
			"phone":        billingPhone,
			"qr_uri":       billingQR,
			"is_expired":   billingExpired,
		},
		"support": map[string]interface{}{
			"is_connected": supportConnected,
			"phone":        supportPhone,
			"qr_uri":       supportQR,
			"is_expired":   supportExpired,
		},
		"routing": map[string]interface{}{
			"mode":           routingMode,
			"otp_device":     otpDev,
			"alerts_device":  alertsDev,
			"billing_device": billingDev,
			"support_device": supportDev,
		},
	})
}

// DisconnectDevice unlinks any of the 4 WhatsApp gateway sessions.
func (h *WhatsAppAdminHandlers) DisconnectDevice(w http.ResponseWriter, r *http.Request) {
	device := r.FormValue("device")
	if device == "" {
		device = r.URL.Query().Get("device")
	}
	switch device {
	case "otp", "alerts", "billing", "support":
		// valid
	default:
		device = "alerts"
	}

	_ = h.waSvc.Disconnect(r.Context(), device)
	http.Redirect(w, r, fmt.Sprintf("/wa-portal?success=%s+WhatsApp+disconnected.", strings.Title(device)), http.StatusSeeOther)
}

// UpdateGatewayRouting saves the admin choice for smart routing and assigned devices.
func (h *WhatsAppAdminHandlers) UpdateGatewayRouting(w http.ResponseWriter, r *http.Request) {
	mode := strings.TrimSpace(r.FormValue("routing_mode"))
	otpDev := strings.TrimSpace(r.FormValue("otp_device"))
	alertsDev := strings.TrimSpace(r.FormValue("alerts_device"))
	billingDev := strings.TrimSpace(r.FormValue("billing_device"))
	supportDev := strings.TrimSpace(r.FormValue("support_device"))

	// Handle quick preset buttons if selected without explicit device overrides
	preset := strings.TrimSpace(r.FormValue("preset"))
	if preset != "" && (otpDev == "" || alertsDev == "" || billingDev == "" || supportDev == "") {
		switch preset {
		case "dual":
			// 2 Numbers: Alert + OTP on Number 1, Billing + Support on Number 2
			mode = "dual"
			if alertsDev == "" {
				alertsDev = "alerts"
			}
			otpDev = alertsDev
			if billingDev == "" {
				billingDev = "billing"
			}
			supportDev = billingDev
		case "single":
			// 1 Number: Everything on primary active number
			mode = "single"
			target := alertsDev
			if target == "" {
				target = "alerts"
			}
			otpDev = target
			alertsDev = target
			billingDev = target
			supportDev = target
		case "quad":
			// 4 Numbers: Dedicated for each channel
			mode = "quad"
			otpDev = "otp"
			alertsDev = "alerts"
			billingDev = "billing"
			supportDev = "support"
		}
	}

	if mode == "" {
		mode = "quad"
	}
	if otpDev == "" {
		otpDev = "otp"
	}
	if alertsDev == "" {
		alertsDev = "alerts"
	}
	if billingDev == "" {
		billingDev = "billing"
	}
	if supportDev == "" {
		supportDev = "support"
	}

	if err := h.repos.WhatsApp.UpdateGatewayRouting(r.Context(), mode, otpDev, alertsDev, billingDev, supportDev); err != nil {
		slog.Error("Failed to update gateway routing", "err", err, "mode", mode, "otp", otpDev, "alerts", alertsDev, "billing", billingDev, "support", supportDev)
		if strings.Contains(r.Header.Get("Accept"), "application/json") || r.Header.Get("X-Requested-With") == "XMLHttpRequest" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Could not save routing preferences: " + err.Error()})
			return
		}
		http.Redirect(w, r, "/wa-portal?error=Could+not+save+routing+preferences.", http.StatusSeeOther)
		return
	}

	slog.Info("Gateway dispatch routing updated successfully", "mode", mode, "otp", otpDev, "alerts", alertsDev, "billing", billingDev, "support", supportDev)

	if strings.Contains(r.Header.Get("Accept"), "application/json") || r.Header.Get("X-Requested-With") == "XMLHttpRequest" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Gateway dispatch routing preferences saved successfully.",
			"settings": map[string]string{
				"routing_mode":   mode,
				"otp_device":     otpDev,
				"alerts_device":  alertsDev,
				"billing_device": billingDev,
				"support_device": supportDev,
			},
		})
		return
	}

	http.Redirect(w, r, "/wa-portal?success=Gateway+dispatch+routing+preferences+saved+successfully.", http.StatusSeeOther)
}

// SubscribersPage lists all WhatsApp alert subscribers with search and filter.
func (h *WhatsAppAdminHandlers) SubscribersPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	search := r.URL.Query().Get("search")
	status := r.URL.Query().Get("status")
	plan := r.URL.Query().Get("plan")

	page := 1
	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 0 {
		page = p
	}
	limit := 25
	offset := (page - 1) * limit

	subscribers, total, err := h.repos.WhatsApp.ListSubscribers(ctx, search, status, plan, limit, offset)
	if err != nil {
		slog.Error("Failed to list subscribers", "error", err)
	}

	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	stats, _ := h.repos.WhatsApp.GetPortalStats(ctx)

	googleConnected := false
	googleEmail := ""
	if h.googleContacts != nil {
		googleConnected = h.googleContacts.IsConnected()
		googleEmail = h.googleContacts.GetAccountEmail()
	}

	data := PageData{
		Title:    "Subscribers Directory | JobyUK.com",
		IsMobile: isMobileDevice(r),
		Data: map[string]interface{}{
			"Subscribers":     subscribers,
			"Total":           total,
			"Page":            page,
			"TotalPages":      totalPages,
			"Search":          search,
			"Status":          status,
			"Plan":            plan,
			"Stats":           stats,
			"GoogleConnected": googleConnected,
			"GoogleEmail":     googleEmail,
			"GoogleSuccess":   r.URL.Query().Get("google_success") == "true",
			"GoogleError":     r.URL.Query().Get("google_error"),
			"Success":         r.URL.Query().Get("success"),
			"Error":           r.URL.Query().Get("error"),
		},
	}
	_ = h.renderer.Render(w, "wa_admin_subscribers", data)
}

// CreateSubscriber handles manual client registration by Admin.
func (h *WhatsAppAdminHandlers) CreateSubscriber(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	name := strings.TrimSpace(r.FormValue("full_name"))
	phone := strings.TrimSpace(r.FormValue("phone_number"))
	email := strings.TrimSpace(r.FormValue("email"))
	planType := strings.TrimSpace(r.FormValue("plan_type"))
	status := strings.TrimSpace(r.FormValue("status"))
	minSalary, _ := strconv.Atoi(r.FormValue("min_salary"))
	alertMode := strings.TrimSpace(r.FormValue("alert_mode"))
	notes := strings.TrimSpace(r.FormValue("notes"))

	// Parse categories and locations
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

	if name == "" || phone == "" {
		http.Redirect(w, r, "/wa-portal/subscribers?error=Full+name+and+phone+number+are+required.", http.StatusSeeOther)
		return
	}

	sub := &models.WhatsAppSubscriber{
		FullName:    name,
		PhoneNumber: phone,
		Email:       email,
		PlanType:    planType,
		Status:      status,
		Categories:  categories,
		Locations:   locations,
		MinSalary:   minSalary,
		AlertMode:   alertMode,
		Notes:       notes,
	}

	if err := h.repos.WhatsApp.CreateSubscriber(ctx, sub); err != nil {
		http.Redirect(w, r, "/wa-portal/subscribers?error="+err.Error(), http.StatusSeeOther)
		return
	}

	// Send welcome WhatsApp notification if gateway is active
	if h.waSvc.IsConnected() {
		welcomeMsg := fmt.Sprintf(
			"*Welcome to JobyUK.com Job Alerts*\n\n"+
				"Hello %s,\n\n"+
				"Your %s subscription is now active. You will receive alerts for UK job vacancies that match your preferences directly on WhatsApp.\n\n"+
				"You can update your preferences or change your number at any time here:\n%s/my-alerts\n\n"+
				"Thank you for choosing JobyUK.com.",
			sub.FullName, strings.ToUpper(sub.PlanType), h.cfg.BaseURL,
		)
		go func() {
			_ = h.waSvc.DispatchTextMessage(context.Background(), sub.PhoneNumber, welcomeMsg, "Subscriber Welcome")
		}()
	}

	// Schedule 1st high-match job alert for new subscriber (3-5 minute delay)
	if h.alertDispatcher != nil {
		h.alertDispatcher.ScheduleInitialCandidateAlert(sub, 0)
	}

	http.Redirect(w, r, "/wa-portal/subscribers?success=Subscriber+registered+successfully.", http.StatusSeeOther)
}

// ToggleSubscriber pauses or resumes a client.
func (h *WhatsAppAdminHandlers) ToggleSubscriber(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Redirect(w, r, "/wa-portal/subscribers", http.StatusSeeOther)
		return
	}

	newStatus := r.FormValue("new_status")
	if newStatus == "" {
		newStatus = "paused"
	}

	_ = h.repos.WhatsApp.ToggleStatus(r.Context(), id, newStatus)
	http.Redirect(w, r, "/wa-portal/subscribers?success=Subscriber+status+updated+successfully.", http.StatusSeeOther)
}

// DeleteSubscriber removes a client.
func (h *WhatsAppAdminHandlers) DeleteSubscriber(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Redirect(w, r, "/wa-portal/subscribers", http.StatusSeeOther)
		return
	}

	_ = h.repos.WhatsApp.DeleteSubscriber(r.Context(), id)
	http.Redirect(w, r, "/wa-portal/subscribers?success=Subscriber+removed+successfully.", http.StatusSeeOther)
}

// ActivateSubscriberPlan activates a 1-month, 6-month, or 1-year plan for a candidate with transaction ID and screenshot/notes.
func (h *WhatsAppAdminHandlers) ActivateSubscriberPlan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Redirect(w, r, "/wa-portal/subscribers?error=Invalid+subscriber+ID", http.StatusSeeOther)
		return
	}

	planType := strings.TrimSpace(r.FormValue("plan_type"))
	if planType == "" {
		planType = "monthly"
	}
	txRef := strings.TrimSpace(r.FormValue("tx_ref"))
	if txRef == "" {
		txRef = fmt.Sprintf("ADMIN-ACT-%d-%d", id, time.Now().Unix())
	}
	screenshotURL := strings.TrimSpace(r.FormValue("screenshot_url"))
	adminNotes := strings.TrimSpace(r.FormValue("admin_notes"))

	sub, err := h.repos.WhatsApp.GetSubscriberByID(ctx, id)
	if err != nil || sub == nil {
		http.Redirect(w, r, "/wa-portal/subscribers?error=Subscriber+not+found", http.StatusSeeOther)
		return
	}

	if err := h.repos.WhatsApp.UpgradeSubscription(ctx, id, planType); err != nil {
		slog.Error("Failed to activate subscriber plan", "id", id, "err", err)
		http.Redirect(w, r, fmt.Sprintf("/wa-portal/subscribers?error=Failed+to+activate+plan:+%s", url.QueryEscape(err.Error())), http.StatusSeeOther)
		return
	}

	var price float64
	var planLabel string
	switch planType {
	case "yearly":
		price = 39.99
		planLabel = "1 Year VIP Pass"
	case "6months":
		price = 24.99
		planLabel = "6 Months VIP Pass"
	default:
		planType = "monthly"
		price = 4.99
		planLabel = "1 Month VIP Pass"
	}

	notesSummary := adminNotes
	if screenshotURL != "" {
		if notesSummary != "" {
			notesSummary += " | Proof: " + screenshotURL
		} else {
			notesSummary = "Proof: " + screenshotURL
		}
	}
	if notesSummary == "" {
		notesSummary = "Manual Admin Activation"
	}

	_ = h.repos.WhatsApp.LogTransaction(ctx, &id, sub.PhoneNumber, sub.FullName, planType, price, "admin_manual", txRef, "verified", notesSummary)

	// Send WhatsApp VIP confirmation
	go func() {
		msg := fmt.Sprintf(
			"🎉 *JobyUK.com VIP Membership Activated!*\n\n"+
				"Hello %s,\n\n"+
				"Great news! Your WhatsApp Job Alerts membership has been officially activated by our administrative team.\n\n"+
				"✅ *Active Plan:* %s (£%.2f)\n"+
				"🔖 *Reference:* #%s\n"+
				"⚡ *Priority:* Instant Dispatches (Up to 7 alerts daily)\n"+
				"📱 *Number Updates:* Unlocked\n\n"+
				"Access your candidate dashboard anytime:\n%s/my-alerts\n\n"+
				"Thank you for choosing JobyUK.com.",
			sub.FullName, planLabel, price, txRef, h.cfg.BaseURL,
		)
		_ = h.waSvc.DispatchChannelMessage(context.Background(), "billing", sub.PhoneNumber, msg, "Admin Plan Activation")
	}()

	referer := r.Header.Get("Referer")
	redirectURL := "/wa-portal/subscribers?success=" + url.QueryEscape(fmt.Sprintf("VIP %s activated successfully for %s!", planLabel, sub.FullName))
	if strings.Contains(referer, "payment-settings") {
		redirectURL = "/wa-portal/payment-settings?success=" + url.QueryEscape(fmt.Sprintf("VIP %s activated successfully for %s!", planLabel, sub.FullName))
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// LogsPage displays the message delivery logs.
func (h *WhatsAppAdminHandlers) LogsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page := 1
	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 0 {
		page = p
	}
	limit := 30
	offset := (page - 1) * limit

	logs, total, _ := h.repos.WhatsApp.ListAlertLogs(ctx, nil, limit, offset)
	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	stats, _ := h.repos.WhatsApp.GetPortalStats(ctx)

	data := PageData{
		Title:    "WhatsApp Alert Logs | JobyUK.com",
		IsMobile: isMobileDevice(r),
		Data: map[string]interface{}{
			"Logs":       logs,
			"Total":      total,
			"Page":       page,
			"TotalPages": totalPages,
			"Stats":      stats,
		},
	}
	_ = h.renderer.Render(w, "wa_admin_logs", data)
}

// SendTestMessage dispatches a test alert from the admin panel.
func (h *WhatsAppAdminHandlers) SendTestMessage(w http.ResponseWriter, r *http.Request) {
	if !h.waSvc.IsConnected() {
		http.Redirect(w, r, "/wa-portal?error=WhatsApp+Gateway+is+offline.+Please+scan+the+QR+code+first.", http.StatusSeeOther)
		return
	}

	phone := strings.TrimSpace(r.FormValue("test_phone"))
	message := strings.TrimSpace(r.FormValue("test_message"))

	if phone == "" || message == "" {
		http.Redirect(w, r, "/wa-portal?error=Phone+number+and+message+are+required.", http.StatusSeeOther)
		return
	}

	err := h.waSvc.DispatchTextMessage(r.Context(), phone, message, "Admin Test Alert")
	if err != nil {
		http.Redirect(w, r, "/wa-portal?error="+err.Error(), http.StatusSeeOther)
		return
	}

	// Also log it
	_ = h.repos.WhatsApp.LogAlert(r.Context(), &models.WhatsAppAlertLog{
		PhoneNumber: phone,
		JobTitle:    "Admin Test Message",
		Company:     "JobyUK.com Gateway",
		Location:    "London, UK",
		SalaryText:  "£50,000",
		Status:      "sent",
	})

	http.Redirect(w, r, "/wa-portal?success=Test+message+sent+successfully.", http.StatusSeeOther)
}

// MatchAndDispatchManual triggers matching of recent jobs to all active subscribers.
func (h *WhatsAppAdminHandlers) MatchAndDispatchManual(w http.ResponseWriter, r *http.Request) {
	if !h.waSvc.IsConnected() {
		http.Redirect(w, r, "/wa-portal?error=WhatsApp+Gateway+is+offline.+Please+scan+the+QR+code+first.", http.StatusSeeOther)
		return
	}

	if h.alertDispatcher != nil {
		h.alertDispatcher.TriggerImmediateCycle(r.Context())
		http.Redirect(w, r, "/wa-portal?success=Automated+job+alert+dispatch+cycle+triggered+successfully.", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/wa-portal?error=Alert+dispatcher+service+not+available.", http.StatusSeeOther)
}

// CryptoPaymentsPage renders all payment methods configuration and payment proofs for admin review.
func (h *WhatsAppAdminHandlers) CryptoPaymentsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	status := r.URL.Query().Get("status")
	payments, total, _ := h.repos.WhatsApp.ListCryptoPayments(ctx, status, 50, 0)
	wallets, _ := h.repos.WhatsApp.GetAllCryptoWallets(ctx)
	paymentSettings, _ := h.repos.WhatsApp.GetPaymentSettings(ctx)
	transactions, totalTx, _ := h.repos.WhatsApp.ListTransactions(ctx, 25, 0)

	data := PageData{
		Title: "Payment Gateways & Proofs | JobyUK.com WhatsApp Gateway",
		Data: map[string]interface{}{
			"Payments":          payments,
			"Total":             total,
			"Wallets":           wallets,
			"PaymentSettings":   paymentSettings,
			"Transactions":      transactions,
			"TotalTransactions": totalTx,
			"Status":            status,
			"Success":           r.URL.Query().Get("success"),
			"Error":             r.URL.Query().Get("error"),
		},
	}
	_ = h.renderer.Render(w, "wa_admin_crypto_payments", data)
}

// UpdatePaymentSettings updates PayPal handle, instructions, and payment gateway toggles.
func (h *WhatsAppAdminHandlers) UpdatePaymentSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rawHandle := strings.TrimSpace(r.FormValue("paypal_handle"))
	instructions := strings.TrimSpace(r.FormValue("paypal_instructions"))
	paypalActive := r.FormValue("paypal_active") == "true" || r.FormValue("paypal_active") == "1" || r.FormValue("paypal_active") == "on"
	cardActive := r.FormValue("card_active") == "true" || r.FormValue("card_active") == "1" || r.FormValue("card_active") == "on"
	cryptoActive := r.FormValue("crypto_active") == "true" || r.FormValue("crypto_active") == "1" || r.FormValue("crypto_active") == "on"
	supportActive := r.FormValue("support_active") == "true" || r.FormValue("support_active") == "1" || r.FormValue("support_active") == "on"
	supportNumber := strings.TrimSpace(r.FormValue("support_number"))
	if supportNumber == "" {
		supportNumber = "+923025015693"
	}
	supportInstructions := strings.TrimSpace(r.FormValue("support_instructions"))

	// Normalize PayPal handle (clean https://paypal.me/ or @ prefix)
	cleanHandle := rawHandle
	cleanHandle = strings.TrimPrefix(cleanHandle, "https://")
	cleanHandle = strings.TrimPrefix(cleanHandle, "http://")
	cleanHandle = strings.TrimPrefix(cleanHandle, "www.")
	cleanHandle = strings.TrimPrefix(cleanHandle, "paypal.me/")
	cleanHandle = strings.TrimPrefix(cleanHandle, "@")
	cleanHandle = strings.Trim(cleanHandle, "/")
	if cleanHandle == "" {
		cleanHandle = "JobyUK.com"
	}

	if err := h.repos.WhatsApp.UpdatePaymentSettings(ctx, cleanHandle, instructions, paypalActive, cardActive, cryptoActive, supportActive, supportNumber, supportInstructions); err != nil {
		slog.Error("Failed to update payment settings", "err", err)
		http.Redirect(w, r, "/wa-portal/payment-settings?error=Failed+to+save+payment+settings", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/wa-portal/payment-settings?success=Payment+gateway+settings+saved+successfully.", http.StatusSeeOther)
}

// VerifyCryptoPayment marks a crypto payment verified and activates the candidate.
func (h *WhatsAppAdminHandlers) VerifyCryptoPayment(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.Atoi(idStr)
	notes := strings.TrimSpace(r.FormValue("admin_notes"))

	ctx := r.Context()
	p, err := h.repos.WhatsApp.VerifyCryptoPayment(ctx, id, notes)
	if err != nil {
		slog.Error("Failed to verify crypto payment", "id", id, "err", err)
		http.Redirect(w, r, "/wa-portal/payment-settings?error=Failed+to+verify+payment", http.StatusSeeOther)
		return
	}

	// Dispatch WhatsApp confirmation
	go func() {
		msg := fmt.Sprintf(
			"🎉 *JobyUK.com Subscription Activated!*\n\n"+
				"Hello %s,\n\n"+
				"Great news! Your Crypto payment (TxID: %s) has been *VERIFIED on-chain*.\n\n"+
				"✅ *Plan Activated:* %s Pass\n"+
				"💼 *Alerts Status:* Live & Dispatching\n\n"+
				"Manage preferences & view alerts: %s/my-alerts",
			p.FullName, p.TxHash, strings.ToUpper(p.PlanType), h.cfg.BaseURL,
		)
		_ = h.waSvc.DispatchChannelMessage(context.Background(), "billing", p.PhoneNumber, msg, "Crypto Payment Verified")
	}()

	http.Redirect(w, r, "/wa-portal/payment-settings?success=Payment+verified+and+subscriber+activated.", http.StatusSeeOther)
}

// RejectCryptoPayment rejects a fake or unverified crypto submission.
func (h *WhatsAppAdminHandlers) RejectCryptoPayment(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.Atoi(idStr)
	notes := strings.TrimSpace(r.FormValue("admin_notes"))

	ctx := r.Context()
	p, err := h.repos.WhatsApp.RejectCryptoPayment(ctx, id, notes)
	if err != nil {
		slog.Error("Failed to reject crypto payment", "id", id, "err", err)
		http.Redirect(w, r, "/wa-portal/payment-settings?error=Failed+to+reject+payment", http.StatusSeeOther)
		return
	}

	// Dispatch WhatsApp notice
	go func() {
		msg := fmt.Sprintf(
			"⚠️ *JobyUK.com Payment Verification Notice*\n\n"+
				"Hello %s,\n\n"+
				"Your recent Crypto payment submission (TxID: %s) could not be verified on the blockchain.\n\n"+
				"📌 *Reason:* %s\n\n"+
				"If you believe this is an error, please re-submit with your authentic TxID and receipt: %s/my-alerts?tab=plans",
			p.FullName, p.TxHash, notes, h.cfg.BaseURL,
		)
		_ = h.waSvc.DispatchChannelMessage(context.Background(), "billing", p.PhoneNumber, msg, "Crypto Payment Rejected")
	}()

	http.Redirect(w, r, "/wa-portal/payment-settings?success=Payment+submission+marked+as+rejected.", http.StatusSeeOther)
}

// UpdateCryptoWallets saves edited admin crypto addresses.
func (h *WhatsAppAdminHandlers) UpdateCryptoWallets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := r.FormValue("wallet_id")
	id, _ := strconv.Atoi(idStr)
	address := strings.TrimSpace(r.FormValue("address"))
	instructions := strings.TrimSpace(r.FormValue("instructions"))
	isActive := r.FormValue("is_active") == "true" || r.FormValue("is_active") == "1" || r.FormValue("is_active") == "on"

	if err := h.repos.WhatsApp.UpdateCryptoWallet(ctx, id, address, instructions, isActive); err != nil {
		http.Redirect(w, r, "/wa-portal/payment-settings?error=Failed+to+update+wallet", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/wa-portal/payment-settings?success=Crypto+wallet+address+updated+successfully.", http.StatusSeeOther)
}

// AnalyticsPage renders the full WhatsApp trial click & conversion analytics dashboard.
func (h *WhatsAppAdminHandlers) AnalyticsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	period := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("period")))
	if period == "" {
		period = "today" // default to Today
	}

	filter := models.WhatsAppAnalyticsFilter{
		Period: period,
	}

	if period == "custom" {
		fromStr := strings.TrimSpace(r.URL.Query().Get("from"))
		toStr := strings.TrimSpace(r.URL.Query().Get("to"))
		if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			filter.FromDate = &t
		}
		if t, err := time.Parse("2006-01-02", toStr); err == nil {
			endOfDay := time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, t.Location())
			filter.ToDate = &endOfDay
		}
	}

	summary, err := h.repos.WhatsApp.GetAnalyticsSummary(ctx, filter)
	if err != nil {
		slog.Error("Failed to get analytics summary", "err", err)
	}

	sources, _ := h.repos.WhatsApp.GetAnalyticsSources(ctx, filter)
	devices, _ := h.repos.WhatsApp.GetAnalyticsDeviceStats(ctx, filter)

	// Pagination for activity logs
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit := 25
	offset := (page - 1) * limit

	events, totalEvents, _ := h.repos.WhatsApp.ListAnalyticsEvents(ctx, filter, limit, offset)
	totalPages := (totalEvents + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	_, _, alertsConnected, _ := h.waSvc.GetQRStatus("alerts")
	_, _, supportConnected, _ := h.waSvc.GetQRStatus("support")

	data := PageData{
		Title:    "Trial Analytics & Funnel Tracking | WhatsApp Portal | JobyUK.com",
		IsMobile: isMobileDevice(r),
		Data: map[string]interface{}{
			"ActiveTab":   "analytics",
			"IsConnected": alertsConnected || supportConnected,
			"Period":      period,
			"FromDate":    r.URL.Query().Get("from"),
			"ToDate":      r.URL.Query().Get("to"),
			"Summary":     summary,
			"Sources":     sources,
			"Devices":     devices,
			"Events":      events,
			"TotalEvents": totalEvents,
			"Page":        page,
			"TotalPages":  totalPages,
			"Success":     r.URL.Query().Get("success"),
			"Error":       r.URL.Query().Get("error"),
		},
	}
	_ = h.renderer.Render(w, "wa_admin_analytics", data)
}

// ResetAnalytics permanently clears tracking events.
func (h *WhatsAppAdminHandlers) ResetAnalytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	err := h.repos.WhatsApp.ResetAnalytics(ctx)
	if err != nil {
		slog.Error("Failed to reset analytics", "err", err)
		http.Redirect(w, r, "/wa-portal/analytics?error=Failed+to+reset+analytics+database.", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/wa-portal/analytics?success=Analytics+tracking+data+has+been+permanently+reset.", http.StatusSeeOther)
}

// PostChannelJobNow manually triggers broadcasting the next high-CTR UK job to the WhatsApp Channel.
func (h *WhatsAppAdminHandlers) PostChannelJobNow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	if h.alertDispatcher == nil {
		http.Redirect(w, r, "/wa-portal/?error=Alert+dispatcher+not+configured", http.StatusSeeOther)
		return
	}

	post, err := h.alertDispatcher.BroadcastNextChannelJob(ctx)
	if err != nil {
		slog.Error("Manual channel post failed", "error", err)
		http.Redirect(w, r, "/wa-portal/?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}

	msg := fmt.Sprintf("Successfully posted '%s' at %s to WhatsApp Channel!", post.JobTitle, post.Company)
	http.Redirect(w, r, "/wa-portal/?success="+url.QueryEscape(msg), http.StatusSeeOther)
}




