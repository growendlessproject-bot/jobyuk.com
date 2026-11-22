package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/redis/go-redis/v9"

	"jobyuk/internal/config"
	"jobyuk/internal/models"
	"jobyuk/internal/repository"
)

// PinURLRegex checks for standard Pinterest URLs and pin.it shortlinks.
var pinURLRegex = regexp.MustCompile(`(?i)^(https?:\/\/)?((([a-zA-Z0-9-]+\.)?pinterest\.(com|[a-z]{2,3}(\.[a-z]{2})?)\/pin\/[0-9a-zA-Z_-]+)|(pin\.it\/[0-9a-zA-Z_-]+))`)

// PinterestHandlers manages the user-facing pin-login, 1-by-1 link dashboard, and admin manager.
type PinterestHandlers struct {
	renderer *Renderer
	repos    *repository.Repositories
	rdb      *redis.Client
	cfg      *config.Config
}

// NewPinterestHandlers creates new Pinterest handlers with Redis rate limiting.
func NewPinterestHandlers(renderer *Renderer, repos *repository.Repositories, rdb *redis.Client, cfg *config.Config) *PinterestHandlers {
	return &PinterestHandlers{
		renderer: renderer,
		repos:    repos,
		rdb:      rdb,
		cfg:      cfg,
	}
}

// createSignedToken creates a tamper-proof session token for the 6-digit pin user with timestamp.
func (h *PinterestHandlers) createSignedToken(userID int, accessCode string) string {
	payload := fmt.Sprintf("%d:%s:%d", userID, accessCode, time.Now().Unix())
	mac := hmac.New(sha256.New, []byte(h.cfg.JWTSecret))
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	raw := fmt.Sprintf("%s.%s", payload, sig)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// setPinSessionCookie sets the 15-minute inactivity session cookie for a Pinterest poster.
func (h *PinterestHandlers) setPinSessionCookie(w http.ResponseWriter, userID int, accessCode string) {
	token := h.createSignedToken(userID, accessCode)
	http.SetCookie(w, &http.Cookie{
		Name:     "pin_session",
		Value:    token,
		Path:     "/",
		MaxAge:   900, // 15 minutes
		Expires:  time.Now().Add(15 * time.Minute),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   !h.cfg.IsDev(),
	})
}

// clearPinSessionCookie removes the pin session cookie.
func (h *PinterestHandlers) clearPinSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "pin_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   !h.cfg.IsDev(),
	})
}

// parseSignedToken verifies the session token and enforces the 15-minute inactivity check.
func (h *PinterestHandlers) parseSignedToken(tokenStr string) (int, error) {
	rawBytes, err := base64.RawURLEncoding.DecodeString(tokenStr)
	if err != nil {
		return 0, fmt.Errorf("invalid token format")
	}
	parts := strings.Split(string(rawBytes), ".")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid token segments")
	}
	payload := parts[0]
	sig := parts[1]

	mac := hmac.New(sha256.New, []byte(h.cfg.JWTSecret))
	mac.Write([]byte(payload))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return 0, fmt.Errorf("invalid token signature")
	}

	payloadParts := strings.Split(payload, ":")
	if len(payloadParts) < 3 {
		return 0, fmt.Errorf("invalid token payload")
	}

	issuedAt, err := strconv.ParseInt(payloadParts[2], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid token timestamp")
	}

	// Enforce 15 minutes of inactivity (900 seconds)
	if time.Now().Unix()-issuedAt > 15*60 {
		return 0, fmt.Errorf("session_expired_inactivity")
	}

	userID, err := strconv.Atoi(payloadParts[0])
	if err != nil {
		return 0, err
	}
	return userID, nil
}

// getAuthUser extracts and verifies the logged in Pinterest user from cookie (read-only).
func (h *PinterestHandlers) getAuthUser(r *http.Request) (*models.PinterestUser, error) {
	cookie, err := r.Cookie("pin_session")
	if err != nil || cookie.Value == "" {
		return nil, fmt.Errorf("not authenticated")
	}
	userID, err := h.parseSignedToken(cookie.Value)
	if err != nil {
		return nil, err
	}
	user, err := h.repos.Pinterest.GetPinterestUserByID(r.Context(), userID)
	if err != nil || !user.IsActive {
		return nil, fmt.Errorf("user not found or inactive")
	}
	return user, nil
}

// getAuthUserAndRenew verifies auth and slides/renews the 15-minute inactivity session for active requests.
func (h *PinterestHandlers) getAuthUserAndRenew(w http.ResponseWriter, r *http.Request) (*models.PinterestUser, error) {
	cookie, err := r.Cookie("pin_session")
	if err != nil || cookie.Value == "" {
		return nil, fmt.Errorf("not authenticated")
	}
	userID, err := h.parseSignedToken(cookie.Value)
	if err != nil {
		if err.Error() == "session_expired_inactivity" {
			h.clearPinSessionCookie(w)
		}
		return nil, err
	}
	user, err := h.repos.Pinterest.GetPinterestUserByID(r.Context(), userID)
	if err != nil || !user.IsActive {
		h.clearPinSessionCookie(w)
		return nil, fmt.Errorf("user not found or inactive")
	}

	// Active user: continuously renew 15-minute sliding inactivity window
	h.setPinSessionCookie(w, user.ID, user.AccessCode)
	return user, nil
}

// LoginPage renders the dedicated 6-digit access code login screen with rate limit cooldown checking.
func (h *PinterestHandlers) LoginPage(w http.ResponseWriter, r *http.Request) {
	if user, err := h.getAuthUser(r); err == nil && user != nil {
		http.Redirect(w, r, "/pin-dashboard", http.StatusSeeOther)
		return
	} else if err != nil && err.Error() == "session_expired_inactivity" {
		h.clearPinSessionCookie(w)
	}

	ip := getClientIP(r)
	key := fmt.Sprintf("pin_login_fail:%s", ip)
	ctx := r.Context()

	var cooldownSeconds int
	var isLocked bool
	var remainingAttempts int = 5

	if h.rdb != nil {
		fails, err := h.rdb.Get(ctx, key).Int()
		if err == nil {
			if fails >= 5 {
				isLocked = true
				ttl, _ := h.rdb.TTL(ctx, key).Result()
				cooldownSeconds = int(ttl.Seconds())
				if cooldownSeconds < 0 {
					cooldownSeconds = 0
				}
			} else {
				remainingAttempts = 5 - fails
			}
		}
	}

	errorMsg := r.URL.Query().Get("error")
	data := PageData{
		Title:    "Pinterest Poster Portal Login | JobyUK.com",
		IsMobile: isMobileDevice(r),
		Data: map[string]interface{}{
			"Error":             errorMsg,
			"IsLocked":          isLocked,
			"CooldownSeconds":   cooldownSeconds,
			"RemainingAttempts": remainingAttempts,
		},
	}

	w.Header().Set("Vary", "User-Agent")
	if err := h.renderer.Render(w, "pin_login", data); err != nil {
		slog.Error("Failed to render pin login template", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// LoginSubmit processes the 6-digit access code submission with bulletproof rate limiting.
func (h *PinterestHandlers) LoginSubmit(w http.ResponseWriter, r *http.Request) {
	ip := getClientIP(r)
	key := fmt.Sprintf("pin_login_fail:%s", ip)
	ctx := r.Context()

	// Check if already in cooldown
	if h.rdb != nil {
		fails, err := h.rdb.Get(ctx, key).Int()
		if err == nil && fails >= 5 {
			ttl, _ := h.rdb.TTL(ctx, key).Result()
			cooldownSec := int(ttl.Seconds())
			if isAJAX(r) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				fmt.Fprintf(w, `{"success":false,"is_locked":true,"cooldown_seconds":%d,"error":"Too many failed attempts. Security cooldown active."}`, cooldownSec)
				return
			}
			http.Redirect(w, r, fmt.Sprintf("/pin-login?error=Too+many+failed+attempts.+Please+wait+%d+seconds.", cooldownSec), http.StatusSeeOther)
			return
		}
	}

	code := strings.TrimSpace(r.FormValue("access_code"))
	if code == "" {
		for i := 0; i < 6; i++ {
			d := strings.TrimSpace(r.FormValue(fmt.Sprintf("otp_%d", i)))
			if d == "" {
				d = strings.TrimSpace(r.FormValue(fmt.Sprintf("digit_%d", i)))
			}
			code += d
		}
	}
	code = strings.TrimSpace(code)

	if len(code) != 6 {
		if isAJAX(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"success":false,"error":"Please enter all 6 digits."}`)
			return
		}
		http.Redirect(w, r, "/pin-login?error=Please+enter+all+6+digits.", http.StatusSeeOther)
		return
	}

	user, err := h.repos.Pinterest.GetPinterestUserByCode(r.Context(), code)
	if err != nil || user == nil {
		// Increment failure counter in Redis
		var remainingAttempts int = 4
		var cooldownSec int = 0
		var isLocked bool = false

		if h.rdb != nil {
			fails, _ := h.rdb.Incr(ctx, key).Result()
			if fails == 1 {
				h.rdb.Expire(ctx, key, 15*time.Minute)
			}
			if fails >= 5 {
				isLocked = true
				ttl, _ := h.rdb.TTL(ctx, key).Result()
				cooldownSec = int(ttl.Seconds())
			} else {
				remainingAttempts = 5 - int(fails)
			}
		}

		if isAJAX(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintf(w, `{"success":false,"is_locked":%t,"remaining_attempts":%d,"cooldown_seconds":%d,"error":"Invalid 6-digit access code."}`,
				isLocked, remainingAttempts, cooldownSec)
			return
		}

		if isLocked {
			http.Redirect(w, r, fmt.Sprintf("/pin-login?error=Too+many+failed+attempts.+Security+cooldown+active+for+%d+seconds.", cooldownSec), http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/pin-login?error=Invalid+access+code.+%d+attempts+remaining.", remainingAttempts), http.StatusSeeOther)
		return
	}

	// 1. Run automatic suspension check for 5 missed days
	isSuspended, reason, err := h.repos.Pinterest.CheckAndApplyAutoSuspension(r.Context(), user.ID)
	if err == nil && isSuspended {
		if reason == "" {
			reason = "Suspended account please contact admin."
		}
		if isAJAX(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(w, `{"success":false,"is_suspended":true,"error":"%s"}`, reason)
			return
		}
		http.Redirect(w, r, "/pin-login?error="+url.QueryEscape(reason), http.StatusSeeOther)
		return
	}

	// 2. Deactivated / Suspended user check (if manually deactivated by admin)
	if !user.IsActive {
		suspendedMsg := user.DeactivationReason
		if suspendedMsg == "" {
			suspendedMsg = "Suspended account please contact admin."
		}
		if isAJAX(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(w, `{"success":false,"is_suspended":true,"error":"%s"}`, suspendedMsg)
			return
		}
		http.Redirect(w, r, "/pin-login?error="+url.QueryEscape(suspendedMsg), http.StatusSeeOther)
		return
	}

	// Login successful - Reset failure counter
	if h.rdb != nil {
		_ = h.rdb.Del(ctx, key)
	}

	// Set 15-minute inactivity session cookie
	h.setPinSessionCookie(w, user.ID, user.AccessCode)

	if isAJAX(r) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"success":true,"redirect":"/pin-dashboard"}`)
		return
	}

	http.Redirect(w, r, "/pin-dashboard", http.StatusSeeOther)
}

// Logout clears the session cookie and redirects to login.
func (h *PinterestHandlers) Logout(w http.ResponseWriter, r *http.Request) {
	h.clearPinSessionCookie(w)
	http.Redirect(w, r, "/pin-login", http.StatusSeeOther)
}

// Heartbeat responds to active client watchdog pings, sliding the 15-minute pin session token.
func (h *PinterestHandlers) Heartbeat(w http.ResponseWriter, r *http.Request) {
	user, err := h.getAuthUserAndRenew(w, r)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, `{"status":"expired","error":"%s"}`, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","authenticated":true,"user_id":%d,"timestamp":%d}`, user.ID, time.Now().Unix())
}

// Dashboard renders the 1-by-1 job link dashboard and progress meter for the user.
func (h *PinterestHandlers) Dashboard(w http.ResponseWriter, r *http.Request) {
	user, err := h.getAuthUserAndRenew(w, r)
	if err != nil {
		if err.Error() == "session_expired_inactivity" {
			http.Redirect(w, r, "/pin-login?error=Your+session+has+expired+due+to+15+minutes+of+inactivity.+Please+log+in+again.", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/pin-login", http.StatusSeeOther)
		return
	}

	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "active"
	}

	progress, err := h.repos.Pinterest.GetUserTodayProgress(r.Context(), user.ID)
	if err != nil {
		slog.Error("Failed to get Pinterest user progress", "user_id", user.ID, "error", err)
	}

	// Get recent submission history for the user
	history, totalHistory, _ := h.repos.Pinterest.GetUserSubmissions(r.Context(), user.ID, 50, 0)

	limit := user.DailyLimit
	if progress != nil && progress.DailyLimit > 0 {
		limit = progress.DailyLimit
	}
	if limit <= 0 {
		limit = 5
	}
	optimalSlots := GetOptimalPostingSlots(limit)

	data := PageData{
		Title:    "Daily Pin Dispatch Workspace | JobyUK.com",
		IsMobile: isMobileDevice(r),
		Data: map[string]interface{}{
			"User":         user,
			"Progress":     progress,
			"History":      history,
			"TotalHistory": totalHistory,
			"OptimalSlots": optimalSlots,
			"OptimalCount": len(optimalSlots),
			"DailyLimit":   limit,
			"Tab":          tab,
			"BaseURL":      h.cfg.BaseURL,
			"Success":      r.URL.Query().Get("success"),
			"Error":        r.URL.Query().Get("error"),
		},
	}

	w.Header().Set("Vary", "User-Agent")
	if err := h.renderer.Render(w, "pin_dashboard", data); err != nil {
		slog.Error("Failed to render pin dashboard template", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// SubmitPin handles validating and saving the Pinterest Pin URL for the current job.
func (h *PinterestHandlers) SubmitPin(w http.ResponseWriter, r *http.Request) {
	user, err := h.getAuthUserAndRenew(w, r)
	if err != nil {
		errMsg := "Session expired. Please log in again."
		if err.Error() == "session_expired_inactivity" {
			errMsg = "Your session has expired due to 15 minutes of inactivity. Please log in again."
		}
		if isAJAX(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintf(w, `{"success":false,"error":"%s"}`, errMsg)
			return
		}
		http.Redirect(w, r, "/pin-login?error="+url.QueryEscape(errMsg), http.StatusSeeOther)
		return
	}

	jobIDStr := strings.TrimSpace(r.FormValue("job_id"))
	jobID, err := strconv.ParseInt(jobIDStr, 10, 64)
	if err != nil || jobID <= 0 {
		// Smart fallback: look up current active assigned job for user
		progress, pErr := h.repos.Pinterest.GetUserTodayProgress(r.Context(), user.ID)
		if pErr == nil && progress != nil && progress.CurrentJob != nil {
			jobID = progress.CurrentJob.ID
		}
	}
	if jobID <= 0 {
		if isAJAX(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"success":false,"error":"No active job found to submit pin."}`)
			return
		}
		http.Redirect(w, r, "/pin-dashboard?error=No+active+job+found", http.StatusSeeOther)
		return
	}

	pinURL := strings.TrimSpace(r.FormValue("pin_url"))

	// Strict Pinterest URL validation
	if !pinURLRegex.MatchString(pinURL) {
		errorMsg := "Invalid Pinterest Pin URL. Please paste a valid Pinterest pin link (e.g. https://www.pinterest.com/pin/... or https://pin.it/...)"
		if isAJAX(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"success":false,"error":"%s"}`, errorMsg)
			return
		}
		http.Redirect(w, r, "/pin-dashboard?error="+errorMsg, http.StatusSeeOther)
		return
	}

	// Submit and record assignment
	if err := h.repos.Pinterest.SubmitJobPin(r.Context(), user.ID, jobID, pinURL); err != nil {
		slog.Error("Failed to submit Pinterest pin", "user_id", user.ID, "job_id", jobID, "error", err)
		if isAJAX(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"success":false,"error":"%s"}`, err.Error())
			return
		}
		http.Redirect(w, r, "/pin-dashboard?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}

	if isAJAX(r) {
		// Get updated progress
		progress, _ := h.repos.Pinterest.GetUserTodayProgress(r.Context(), user.ID)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"success":true,"message":"Pin submitted successfully!","today_completed":%d,"daily_limit":%d,"is_completed":%t}`,
			progress.TodayCompleted, progress.DailyLimit, progress.IsCompleted)
		return
	}

	http.Redirect(w, r, "/pin-dashboard?success=Pin+submitted+successfully!+Next+job+unlocked.", http.StatusSeeOther)
}

// UpdateHandle saves or updates the user's personal Pinterest account handle.
func (h *PinterestHandlers) UpdateHandle(w http.ResponseWriter, r *http.Request) {
	user, err := h.getAuthUserAndRenew(w, r)
	if err != nil {
		errMsg := "Session expired. Please log in again."
		if err.Error() == "session_expired_inactivity" {
			errMsg = "Your session has expired due to 15 minutes of inactivity. Please log in again."
		}
		if isAJAX(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintf(w, `{"success":false,"error":"%s"}`, errMsg)
			return
		}
		http.Redirect(w, r, "/pin-login?error="+url.QueryEscape(errMsg), http.StatusSeeOther)
		return
	}

	handle := strings.TrimSpace(r.FormValue("pinterest_handle"))
	if err := h.repos.Pinterest.UpdatePinterestHandle(r.Context(), user.ID, handle); err != nil {
		slog.Error("Failed to update Pinterest handle", "user_id", user.ID, "error", err)
		if isAJAX(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"success":false,"error":"Failed to save handle."}`)
			return
		}
		http.Redirect(w, r, "/pin-dashboard?error=Failed+to+save+Pinterest+handle", http.StatusSeeOther)
		return
	}

	if isAJAX(r) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"success":true,"message":"Pinterest handle updated successfully!","handle":"%s"}`, handle)
		return
	}
	http.Redirect(w, r, "/pin-dashboard?success=Pinterest+handle+saved+successfully!", http.StatusSeeOther)
}

// EditPin updates the Pinterest Pin URL of a previously submitted assignment.
func (h *PinterestHandlers) EditPin(w http.ResponseWriter, r *http.Request) {
	user, err := h.getAuthUserAndRenew(w, r)
	if err != nil {
		errMsg := "Session expired. Please log in again."
		if err.Error() == "session_expired_inactivity" {
			errMsg = "Your session has expired due to 15 minutes of inactivity. Please log in again."
		}
		if isAJAX(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintf(w, `{"success":false,"error":"%s"}`, errMsg)
			return
		}
		http.Redirect(w, r, "/pin-login?error="+url.QueryEscape(errMsg), http.StatusSeeOther)
		return
	}

	assignmentIDStr := r.FormValue("assignment_id")
	assignmentID, err := strconv.ParseInt(assignmentIDStr, 10, 64)
	if err != nil || assignmentID <= 0 {
		if isAJAX(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"success":false,"error":"Invalid submission ID."}`)
			return
		}
		http.Redirect(w, r, "/pin-dashboard?tab=history&error=Invalid+submission+ID", http.StatusSeeOther)
		return
	}

	pinURL := strings.TrimSpace(r.FormValue("pin_url"))
	if !pinURLRegex.MatchString(pinURL) {
		errorMsg := "Invalid Pinterest Pin URL. Please paste a valid Pinterest pin link (e.g. https://www.pinterest.com/pin/... or https://pin.it/...)"
		if isAJAX(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"success":false,"error":"%s"}`, errorMsg)
			return
		}
		http.Redirect(w, r, "/pin-dashboard?tab=history&error="+errorMsg, http.StatusSeeOther)
		return
	}

	if err := h.repos.Pinterest.UpdateSubmissionPinURL(r.Context(), user.ID, assignmentID, pinURL); err != nil {
		slog.Error("Failed to update submission pin URL", "user_id", user.ID, "assignment_id", assignmentID, "error", err)
		if isAJAX(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"success":false,"error":"%s"}`, err.Error())
			return
		}
		http.Redirect(w, r, "/pin-dashboard?tab=history&error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}

	if isAJAX(r) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"success":true,"message":"Pin URL updated successfully!","pin_url":"%s"}`, pinURL)
		return
	}
	http.Redirect(w, r, "/pin-dashboard?tab=history&success=Pin+URL+updated+successfully!", http.StatusSeeOther)
}

// -------------------------------------------------------------
// Admin Handlers for Pinterest Management (/admin/pinterest)
// -------------------------------------------------------------

// AdminPinterestManager renders the complete Pinterest user and reporting manager.
func (h *PinterestHandlers) AdminPinterestManager(w http.ResponseWriter, r *http.Request) {
	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "users"
	}

	// Automatically evaluate missed days and suspend inactive posters
	_ = h.repos.Pinterest.RunAllUsersAutoSuspension(r.Context())

	users, err := h.repos.Pinterest.ListPinterestUsers(r.Context())
	if err != nil {
		slog.Error("Failed to list Pinterest users", "error", err)
	}

	selectedUserID, _ := strconv.Atoi(r.URL.Query().Get("user_id"))
	var selectedUser *models.PinterestUser
	var userSubmissions []models.PinterestAssignment
	var userTotalSubmissions int
	var userAuditReports []models.PinterestDailyAuditItem

	if selectedUserID > 0 {
		selectedUser, _ = h.repos.Pinterest.GetPinterestUserByID(r.Context(), selectedUserID)
		userSubmissions, userTotalSubmissions, _ = h.repos.Pinterest.GetAdminAllSubmissions(r.Context(), selectedUserID, 100, 0)
		userAuditReports, _ = h.repos.Pinterest.GetAdminDailyAuditReportForUser(r.Context(), selectedUserID, 30)
		if tab == "" || tab == "users" {
			tab = "user_report"
		}
	}

	auditReports, err := h.repos.Pinterest.GetAdminDailyAuditReport(r.Context(), 14)
	if err != nil {
		slog.Error("Failed to get daily audit reports", "error", err)
	}

	submissions, totalSubmissions, err := h.repos.Pinterest.GetAdminAllSubmissions(r.Context(), 0, 50, 0)
	if err != nil {
		slog.Error("Failed to get submissions", "error", err)
	}

	data := PageData{
		Title: "Pinterest Team & Pin Operations - JobyUK.com Admin",
		Data: map[string]interface{}{
			"Users":                users,
			"SelectedUser":         selectedUser,
			"SelectedUserID":       selectedUserID,
			"UserSubmissions":      userSubmissions,
			"UserTotalSubmissions": userTotalSubmissions,
			"UserAuditReports":     userAuditReports,
			"AuditReports":         auditReports,
			"Submissions":          submissions,
			"TotalSubmissions":     totalSubmissions,
			"Tab":                  tab,
			"BaseURL":              h.cfg.BaseURL,
			"CreatedUser":          r.URL.Query().Get("created_user"),
			"CreatedCode":          r.URL.Query().Get("created_code"),
			"Success":              r.URL.Query().Get("success"),
			"Error":                r.URL.Query().Get("error"),
		},
	}

	if err := h.renderer.RenderAdmin(w, r, "admin/pinterest_admin", data); err != nil {
		slog.Error("Failed to render admin/pinterest_admin template", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// AdminCreateUser creates a new Pinterest user with an auto-generated 6-digit code.
func (h *PinterestHandlers) AdminCreateUser(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))
	phone := strings.TrimSpace(r.FormValue("phone"))
	dailyLimit, _ := strconv.Atoi(r.FormValue("daily_limit"))
	if dailyLimit <= 0 {
		dailyLimit = 5
	}

	if name == "" || email == "" {
		http.Redirect(w, r, "/admin-lol/pinterest?tab=users&error=Name+and+Email+are+required", http.StatusSeeOther)
		return
	}

	user := &models.PinterestUser{
		Name:       name,
		Email:      email,
		Phone:      phone,
		DailyLimit: dailyLimit,
	}

	if err := h.repos.Pinterest.CreatePinterestUser(r.Context(), user); err != nil {
		slog.Error("Failed to create Pinterest user", "error", err)
		http.Redirect(w, r, "/admin-lol/pinterest?tab=users&error=Failed+to+create+user:+"+err.Error(), http.StatusSeeOther)
		return
	}

	redirectURL := fmt.Sprintf("/admin-lol/pinterest?tab=users&created_user=%s&created_code=%s&success=User+created+successfully!+Share+the+6-digit+code+with+the+user.",
		user.Name, user.AccessCode)
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// AdminUpdateUserQuota updates the daily link limit for a user.
func (h *PinterestHandlers) AdminUpdateUserQuota(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	dailyLimit, _ := strconv.Atoi(r.FormValue("daily_limit"))
	if dailyLimit <= 0 {
		dailyLimit = 5
	}

	if err := h.repos.Pinterest.UpdatePinterestUserQuota(r.Context(), id, dailyLimit); err != nil {
		slog.Error("Failed to update user quota", "id", id, "error", err)
		http.Redirect(w, r, "/admin-lol/pinterest?tab=users&error=Failed+to+update+quota", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin-lol/pinterest?tab=users&success=Daily+limit+updated+successfully", http.StatusSeeOther)
}

// AdminRegenerateCode generates a new 6-digit access code for the user.
func (h *PinterestHandlers) AdminRegenerateCode(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	newCode, err := h.repos.Pinterest.RegeneratePinterestCode(r.Context(), id)
	if err != nil {
		slog.Error("Failed to regenerate access code", "id", id, "error", err)
		http.Redirect(w, r, "/admin-lol/pinterest?tab=users&error=Failed+to+regenerate+code", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/admin-lol/pinterest?tab=users&created_code=%s&success=New+6-digit+code+generated:+%s", newCode, newCode), http.StatusSeeOther)
}

// AdminToggleUser toggles active/inactive state of a Pinterest user.
func (h *PinterestHandlers) AdminToggleUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	if err := h.repos.Pinterest.TogglePinterestUserActive(r.Context(), id); err != nil {
		slog.Error("Failed to toggle user status", "id", id, "error", err)
		http.Redirect(w, r, "/admin-lol/pinterest?tab=users&error=Failed+to+update+status", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin-lol/pinterest?tab=users&success=User+status+updated+successfully", http.StatusSeeOther)
}

// AdminDeleteUser permanently deletes a Pinterest user and their database records.
func (h *PinterestHandlers) AdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	if err := h.repos.Pinterest.DeletePinterestUser(r.Context(), id); err != nil {
		slog.Error("Failed to delete Pinterest user", "id", id, "error", err)
		http.Redirect(w, r, "/admin-lol/pinterest?tab=users&error=Failed+to+delete+poster", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin-lol/pinterest?tab=users&success=Poster+deleted+and+record+permanently+removed+from+database", http.StatusSeeOther)
}

// isAJAX helper checks if request expects JSON.
func isAJAX(r *http.Request) bool {
	return r.Header.Get("Accept") == "application/json" ||
		r.Header.Get("X-Requested-With") == "XMLHttpRequest" ||
		strings.Contains(r.Header.Get("Content-Type"), "application/json")
}

// GetOptimalPostingSlots dynamically calculates N best publishing time slots based on user's daily pin limit.
func GetOptimalPostingSlots(limit int) []models.OptimalPostingSlot {
	if limit <= 0 {
		limit = 5 // default fallback
	}

	// Curated high-precision presets for 1 to 5
	switch limit {
	case 1:
		return []models.OptimalPostingSlot{
			{
				SlotNumber:  1,
				Title:       "🔥 Prime Peak",
				UKTime:      "07:30 PM",
				PKTime:      "11:30 PM",
				Description: "Highest UK job application & active search window.",
				IsPrime:     true,
				Badge:       "Mega Peak",
			},
		}
	case 2:
		return []models.OptimalPostingSlot{
			{
				SlotNumber:  1,
				Title:       "Lunch Break",
				UKTime:      "12:45 PM",
				PKTime:      "04:45 PM",
				Description: "Mid-day mobile lunch browsing.",
				IsPrime:     false,
				Badge:       "Mid-Day",
			},
			{
				SlotNumber:  2,
				Title:       "🔥 Prime Peak",
				UKTime:      "07:30 PM",
				PKTime:      "11:30 PM",
				Description: "Highest traffic & application conversion.",
				IsPrime:     true,
				Badge:       "Top Traffic",
			},
		}
	case 3:
		return []models.OptimalPostingSlot{
			{
				SlotNumber:  1,
				Title:       "Morning Commute",
				UKTime:      "08:30 AM",
				PKTime:      "12:30 PM",
				Description: "Breakfast & morning travel search.",
				IsPrime:     false,
				Badge:       "Morning",
			},
			{
				SlotNumber:  2,
				Title:       "Lunch Break",
				UKTime:      "01:15 PM",
				PKTime:      "05:15 PM",
				Description: "Mid-day casual job browsing.",
				IsPrime:     false,
				Badge:       "Mid-Day",
			},
			{
				SlotNumber:  3,
				Title:       "🔥 Prime Peak",
				UKTime:      "07:30 PM",
				PKTime:      "11:30 PM",
				Description: "Highest traffic & application conversion.",
				IsPrime:     true,
				Badge:       "Top Traffic",
			},
		}
	case 4:
		return []models.OptimalPostingSlot{
			{
				SlotNumber:  1,
				Title:       "Morning Commute",
				UKTime:      "08:30 AM",
				PKTime:      "12:30 PM",
				Description: "Breakfast & morning travel search.",
				IsPrime:     false,
				Badge:       "Morning",
			},
			{
				SlotNumber:  2,
				Title:       "Lunch Break",
				UKTime:      "12:45 PM",
				PKTime:      "04:45 PM",
				Description: "Mid-day casual job browsing.",
				IsPrime:     false,
				Badge:       "Mid-Day",
			},
			{
				SlotNumber:  3,
				Title:       "Afternoon Commute",
				UKTime:      "05:00 PM",
				PKTime:      "09:00 PM",
				Description: "Work closing & homeward travel search.",
				IsPrime:     false,
				Badge:       "Commute",
			},
			{
				SlotNumber:  4,
				Title:       "🔥 Prime Peak",
				UKTime:      "08:00 PM",
				PKTime:      "12:00 AM",
				Description: "Highest traffic & application conversion.",
				IsPrime:     true,
				Badge:       "Top Traffic",
			},
		}
	case 5:
		return []models.OptimalPostingSlot{
			{
				SlotNumber:  1,
				Title:       "Morning Commute",
				UKTime:      "08:30 AM",
				PKTime:      "12:30 PM",
				Description: "Morning commute & breakfast search.",
				IsPrime:     false,
				Badge:       "Morning",
			},
			{
				SlotNumber:  2,
				Title:       "Lunch Break",
				UKTime:      "12:45 PM",
				PKTime:      "04:45 PM",
				Description: "Lunch break casual job browsing.",
				IsPrime:     false,
				Badge:       "Mid-Day",
			},
			{
				SlotNumber:  3,
				Title:       "Afternoon Commute",
				UKTime:      "04:30 PM",
				PKTime:      "08:30 PM",
				Description: "Work closing & evening travel search.",
				IsPrime:     false,
				Badge:       "Commute",
			},
			{
				SlotNumber:  4,
				Title:       "🔥 Prime Peak",
				UKTime:      "07:15 PM",
				PKTime:      "11:15 PM",
				Description: "Highest traffic & application conversion.",
				IsPrime:     true,
				Badge:       "Top Traffic",
			},
			{
				SlotNumber:  5,
				Title:       "🔥 Night Peak",
				UKTime:      "09:30 PM",
				PKTime:      "01:30 AM",
				Description: "Late night browsing before sleep.",
				IsPrime:     true,
				Badge:       "High Intent",
			},
		}
	}

	// For limit >= 6 (e.g. 6, 8, 10, 15, 20, etc.), generate evenly distributed slots from 08:00 to 22:30 UK:
	startMinutes := 8 * 60       // 08:00 AM
	endMinutes := 22*60 + 30     // 10:30 PM
	totalSpan := endMinutes - startMinutes
	step := float64(totalSpan) / float64(limit-1)

	slots := make([]models.OptimalPostingSlot, limit)
	for i := 0; i < limit; i++ {
		currentMin := startMinutes + int(float64(i)*step+0.5)
		// Round to nearest 5 minutes
		currentMin = ((currentMin + 2) / 5) * 5
		if currentMin > endMinutes {
			currentMin = endMinutes
		}

		ukHour := currentMin / 60
		ukMin := currentMin % 60

		// Pakistan is +4 hours ahead in BST (UTC+1 to UTC+5)
		pkHour := (ukHour + 4) % 24
		pkMin := ukMin

		ukTimeStr := format12Hour(ukHour, ukMin)
		pkTimeStr := format12Hour(pkHour, pkMin)

		title, desc, isPrime, badge := getSlotContext(ukHour)

		slots[i] = models.OptimalPostingSlot{
			SlotNumber:  i + 1,
			Title:       title,
			UKTime:      ukTimeStr,
			PKTime:      pkTimeStr,
			Description: desc,
			IsPrime:     isPrime,
			Badge:       badge,
		}
	}
	return slots
}

func format12Hour(hour, minute int) string {
	period := "AM"
	displayHour := hour
	if displayHour >= 12 {
		period = "PM"
		if displayHour > 12 {
			displayHour -= 12
		}
	}
	if displayHour == 0 {
		displayHour = 12
	}
	return fmt.Sprintf("%02d:%02d %s", displayHour, minute, period)
}

func getSlotContext(ukHour int) (title string, desc string, isPrime bool, badge string) {
	switch {
	case ukHour < 10:
		return "Morning Commute", "Breakfast & morning commute job search.", false, "Morning"
	case ukHour < 12:
		return "Mid-Morning", "Active desktop job exploration.", false, "Active"
	case ukHour < 14:
		return "Lunch Break", "Mid-day mobile lunch browsing.", false, "Lunch"
	case ukHour < 16:
		return "Afternoon Focus", "Casual afternoon job scanning.", false, "Mid-Day"
	case ukHour < 18:
		return "Evening Commute", "Work wrap-up & homeward commute.", false, "Commute"
	case ukHour < 19:
		return "Early Evening", "Dinner & post-work browsing.", false, "Evening"
	case ukHour < 21:
		return "🔥 Prime Peak", "Highest UK traffic & application submissions.", true, "Top Traffic"
	case ukHour < 23:
		return "🔥 Night Peak", "High-intent quiet job applications.", true, "High Intent"
	default:
		return "Late Night", "Night owl job searchers.", false, "Night"
	}
}
