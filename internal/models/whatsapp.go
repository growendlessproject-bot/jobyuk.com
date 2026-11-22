package models

import (
	"time"
)

// WhatsAppSubscriber represents a paid job alert subscriber.
type WhatsAppSubscriber struct {
	ID                  int        `json:"id" db:"id"`
	PhoneNumber         string     `json:"phone_number" db:"phone_number"` // E.164 format e.g. +447123456789
	FullName            string     `json:"full_name" db:"full_name"`
	Email               string     `json:"email" db:"email"`
	PlanType            string     `json:"plan_type" db:"plan_type"` // 'monthly', '6months', 'yearly', 'admin_pass'
	PlanPriceGBP        float64    `json:"plan_price_gbp" db:"plan_price_gbp"`
	Status              string     `json:"status" db:"status"` // 'active', 'paused', 'expired', 'cancelled'
	MaxNumberChanges    int        `json:"max_number_changes" db:"max_number_changes"`
	NumberChangesUsed   int        `json:"number_changes_used" db:"number_changes_used"`
	LastNumberChangedAt *time.Time `json:"last_number_changed_at" db:"last_number_changed_at"`
	Categories          []string   `json:"categories" db:"categories"`
	Locations           []string   `json:"locations" db:"locations"`
	MinSalary           int        `json:"min_salary" db:"min_salary"`
	AlertMode           string     `json:"alert_mode" db:"alert_mode"` // 'instant', 'daily_digest'
	Notes               string     `json:"notes" db:"notes"`
	CreatedAt           time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at" db:"updated_at"`
	ExpiresAt           time.Time  `json:"expires_at" db:"expires_at"`
	PINHash             string     `json:"-" db:"pin_hash"`
	FailedPINAttempts   int        `json:"failed_pin_attempts" db:"failed_pin_attempts"`
	LockedUntil         *time.Time `json:"locked_until,omitempty" db:"locked_until"`

	Source              string     `json:"source" db:"source"`                 // 'homepage_trial', 'portal', 'admin'
	TrialEndsAt         *time.Time `json:"trial_ends_at,omitempty" db:"trial_ends_at"`
	PaymentStatus       string     `json:"payment_status" db:"payment_status"` // 'unpaid', 'paid', 'trial', 'free_pass'

	// Derived / Helpers
	RemainingChanges    int        `json:"remaining_changes"`
	CooldownActive      bool       `json:"cooldown_active"`
	CooldownEndsIn      string     `json:"cooldown_ends_in,omitempty"`
	DaysRemaining       int        `json:"days_remaining"`
	IsTrial             bool       `json:"is_trial"`
	IsTrialActive       bool       `json:"is_trial_active"`
	TrialHoursRemaining int        `json:"trial_hours_remaining"`
	IsPaywalled         bool       `json:"is_paywalled"`
}

// WhatsAppNumberHistory records every phone number change for audit.
type WhatsAppNumberHistory struct {
	ID           int       `json:"id" db:"id"`
	SubscriberID int       `json:"subscriber_id" db:"subscriber_id"`
	OldPhone     string    `json:"old_phone" db:"old_phone"`
	NewPhone     string    `json:"new_phone" db:"new_phone"`
	ChangedAt    time.Time `json:"changed_at" db:"changed_at"`
	IPAddress    string    `json:"ip_address" db:"ip_address"`
}

// WhatsAppOTP holds temporary 6-digit codes sent directly via WhatsApp.
type WhatsAppOTP struct {
	PhoneNumber string    `json:"phone_number" db:"phone_number"`
	OTPCode     string    `json:"otp_code" db:"otp_code"`
	ExpiresAt   time.Time `json:"expires_at" db:"expires_at"`
	Attempts    int       `json:"attempts" db:"attempts"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// WhatsAppAlertLog records every dispatched job alert.
type WhatsAppAlertLog struct {
	ID           int       `json:"id" db:"id"`
	SubscriberID *int      `json:"subscriber_id,omitempty" db:"subscriber_id"`
	PhoneNumber  string    `json:"phone_number" db:"phone_number"`
	JobID        *int64    `json:"job_id,omitempty" db:"job_id"`
	JobTitle     string    `json:"job_title" db:"job_title"`
	Company      string    `json:"company" db:"company"`
	Location     string    `json:"location" db:"location"`
	SalaryText   string    `json:"salary_text" db:"salary_text"`
	Status       string    `json:"status" db:"status"` // 'sent', 'delivered', 'failed'
	ErrorMessage string    `json:"error_message" db:"error_message"`
	SentAt       time.Time `json:"sent_at" db:"sent_at"`
	SubscriberName string  `json:"subscriber_name,omitempty"`
}

// WhatsAppGatewaySettings stores connection status and daily quotas for 4 dedicated WhatsApp gateways.
type WhatsAppGatewaySettings struct {
	ID                   int        `json:"id" db:"id"`
	IsConnected          bool       `json:"is_connected" db:"is_connected"`
	ConnectedPhone       string     `json:"connected_phone" db:"connected_phone"`
	ConnectedName        string     `json:"connected_name" db:"connected_name"`
	LastConnectedAt      *time.Time `json:"last_connected_at" db:"last_connected_at"`
	DailyMessagesSent    int        `json:"daily_messages_sent" db:"daily_messages_sent"`
	LastCounterReset     time.Time  `json:"last_counter_reset" db:"last_counter_reset"`
	OTPConnected         bool       `json:"otp_connected" db:"otp_connected"`
	OTPPhone             string     `json:"otp_phone" db:"otp_phone"`
	OTPName              string     `json:"otp_name" db:"otp_name"`
	OTPLastConnected     *time.Time `json:"otp_last_connected_at" db:"otp_last_connected_at"`
	BillingConnected     bool       `json:"billing_connected" db:"billing_connected"`
	BillingPhone         string     `json:"billing_phone" db:"billing_phone"`
	BillingName          string     `json:"billing_name" db:"billing_name"`
	BillingLastConnected *time.Time `json:"billing_last_connected_at" db:"billing_last_connected_at"`
	SupportConnected     bool       `json:"support_connected" db:"support_connected"`
	SupportPhone         string     `json:"support_phone" db:"support_phone"`
	SupportName          string     `json:"support_name" db:"support_name"`
	SupportLastConnected *time.Time `json:"support_last_connected_at" db:"support_last_connected_at"`
	RoutingMode          string     `json:"routing_mode" db:"routing_mode"`       // 'quad', 'dual', 'alerts_only', etc.
	OTPDevice            string     `json:"otp_device" db:"otp_device"`
	AlertsDevice         string     `json:"alerts_device" db:"alerts_device"`
	BillingDevice        string     `json:"billing_device" db:"billing_device"`
	SupportDevice        string     `json:"support_device" db:"support_device"`
	UpdatedAt            time.Time  `json:"updated_at" db:"updated_at"`
}

// WhatsAppPortalStats aggregates key dashboard metrics.
type WhatsAppPortalStats struct {
	TotalSubscribers  int `json:"total_subscribers"`
	ActiveSubscribers int `json:"active_subscribers"`
	PausedSubscribers int `json:"paused_subscribers"`
	AlertsSentToday   int `json:"alerts_sent_today"`
	TotalAlertsSent   int `json:"total_alerts_sent"`
}

// SubscriberLoginLog tracks login history and security audit trail for a subscriber.
type SubscriberLoginLog struct {
	ID           int       `json:"id" db:"id"`
	SubscriberID int       `json:"subscriber_id" db:"subscriber_id"`
	PhoneNumber  string    `json:"phone_number" db:"phone_number"`
	IPAddress    string    `json:"ip_address" db:"ip_address"`
	UserAgent    string    `json:"user_agent" db:"user_agent"`
	Action       string    `json:"action" db:"action"` // 'login_pin', 'register_verify', 'pin_reset', 'number_change'
	Status       string    `json:"status" db:"status"` // 'success', 'failed_pin', 'locked'
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// WhatsAppCryptoWallet represents an admin-configured deposit wallet.
type WhatsAppCryptoWallet struct {
	ID           int       `json:"id" db:"id"`
	Coin         string    `json:"coin" db:"coin"`         // 'USDT', 'BTC', 'ETH'
	Network      string    `json:"network" db:"network"`   // 'TRC-20 (Tron)', 'BEP-20', etc.
	Symbol       string    `json:"symbol" db:"symbol"`     // '₮', '₿', 'Ξ'
	Address      string    `json:"address" db:"address"`
	Instructions string    `json:"instructions" db:"instructions"`
	IsActive     bool      `json:"is_active" db:"is_active"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// WhatsAppCryptoPayment represents a submitted crypto payment proof.
type WhatsAppCryptoPayment struct {
	ID             int        `json:"id" db:"id"`
	SubscriberID   int        `json:"subscriber_id" db:"subscriber_id"`
	PhoneNumber    string     `json:"phone_number" db:"phone_number"`
	FullName       string     `json:"full_name" db:"full_name"`
	PlanType       string     `json:"plan_type" db:"plan_type"`
	AmountGBP      float64    `json:"amount_gbp" db:"amount_gbp"`
	Coin           string     `json:"coin" db:"coin"`
	Network        string     `json:"network" db:"network"`
	DepositAddress string     `json:"deposit_address" db:"deposit_address"`
	TxHash         string     `json:"tx_hash" db:"tx_hash"`
	SenderAddress  string     `json:"sender_address" db:"sender_address"`
	ScreenshotURL  string     `json:"screenshot_url" db:"screenshot_url"`
	Status         string     `json:"status" db:"status"` // 'pending', 'verified', 'rejected'
	AdminNotes     string     `json:"admin_notes" db:"admin_notes"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	VerifiedAt     *time.Time `json:"verified_at,omitempty" db:"verified_at"`
}

// WhatsAppPaymentSettings stores admin configuration for PayPal, Crypto, and Card gateways.
type WhatsAppPaymentSettings struct {
	ID                 int       `json:"id" db:"id"`
	PayPalHandle       string    `json:"paypal_handle" db:"paypal_handle"`
	PayPalInstructions string    `json:"paypal_instructions" db:"paypal_instructions"`
	PayPalActive        bool      `json:"paypal_active" db:"paypal_active"`
	CardActive          bool      `json:"card_active" db:"card_active"`
	CryptoActive        bool      `json:"crypto_active" db:"crypto_active"`
	SupportActive       bool      `json:"support_active" db:"support_active"`
	SupportNumber       string    `json:"support_number" db:"support_number"`
	SupportInstructions string    `json:"support_instructions" db:"support_instructions"`
	UpdatedAt           time.Time `json:"updated_at" db:"updated_at"`
}

// WhatsAppTransaction records all processed transactions across Card, PayPal, and verified Crypto.
type WhatsAppTransaction struct {
	ID           int       `json:"id" db:"id"`
	SubscriberID *int      `json:"subscriber_id,omitempty" db:"subscriber_id"`
	PhoneNumber  string    `json:"phone_number" db:"phone_number"`
	FullName     string    `json:"full_name" db:"full_name"`
	PlanType     string    `json:"plan_type" db:"plan_type"`
	AmountGBP    float64   `json:"amount_gbp" db:"amount_gbp"`
	Method       string    `json:"method" db:"method"` // 'card', 'paypal', 'crypto'
	TxRef        string    `json:"tx_ref" db:"tx_ref"`
	Status       string    `json:"status" db:"status"` // 'completed', 'verified', 'pending', 'refunded'
	Notes        string    `json:"notes" db:"notes"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// WhatsAppAnalyticsEvent represents a tracked trial click or conversion event.
type WhatsAppAnalyticsEvent struct {
	ID               int        `json:"id" db:"id"`
	VisitorID        string     `json:"visitor_id" db:"visitor_id"`
	Source           string     `json:"source" db:"source"`
	AriaLabel        string     `json:"aria_label" db:"aria_label"`
	TriggerPlacement string     `json:"trigger_placement" db:"trigger_placement"`
	PageURL          string     `json:"page_url" db:"page_url"`
	Referrer         string     `json:"referrer" db:"referrer"`
	IPAddress        string     `json:"ip_address" db:"ip_address"`
	Location         string     `json:"location" db:"location"`
	UserAgent        string     `json:"user_agent" db:"user_agent"`
	DeviceType       string     `json:"device_type" db:"device_type"`
	Browser          string     `json:"browser" db:"browser"`
	OS               string     `json:"os" db:"os"`
	PhoneNumber      string     `json:"phone_number" db:"phone_number"`
	Step             string     `json:"step" db:"step"` // 'clicked', 'otp_sent', 'verified'
	OTPSent          bool       `json:"otp_sent" db:"otp_sent"`
	OTPVerified      bool       `json:"otp_verified" db:"otp_verified"`
	SubscriberID     *int       `json:"subscriber_id,omitempty" db:"subscriber_id"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at" db:"updated_at"`
}

// WhatsAppAnalyticsSummary represents high-level metrics for analytics.
type WhatsAppAnalyticsSummary struct {
	TotalClicks          int     `json:"total_clicks"`
	UniqueVisitors       int     `json:"unique_visitors"`
	OTPSentCount         int     `json:"otp_sent_count"`
	VerifiedCount        int     `json:"verified_count"`
	ClickToOTPConvRate   float64 `json:"click_to_otp_rate"`
	ClickToVerifiedRate  float64 `json:"click_to_verified_rate"`
	OTPToVerifiedRate    float64 `json:"otp_to_verified_rate"`
}

// WhatsAppSourceStat represents clicks and conversion by placement/aria-label.
type WhatsAppSourceStat struct {
	Source           string  `json:"source"`
	AriaLabel        string  `json:"aria_label"`
	TriggerPlacement string  `json:"trigger_placement"`
	Clicks           int     `json:"clicks"`
	UniqueVisitors   int     `json:"unique_visitors"`
	OTPSent          int     `json:"otp_sent"`
	Verified         int     `json:"verified"`
	ConvRate         float64 `json:"conv_rate"`
}

// WhatsAppDeviceStat represents device breakdown.
type WhatsAppDeviceStat struct {
	DeviceType string  `json:"device_type"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

// WhatsAppAnalyticsFilter defines date filters for analytics.
type WhatsAppAnalyticsFilter struct {
	Period   string     `json:"period"` // 'today', 'yesterday', 'week', 'month', 'custom'
	FromDate *time.Time `json:"from_date,omitempty"`
	ToDate   *time.Time `json:"to_date,omitempty"`
}

// WhatsAppChannelPost records every job posted to the official WhatsApp Channel.
type WhatsAppChannelPost struct {
	ID            int       `json:"id" db:"id"`
	JobID         int64     `json:"job_id" db:"job_id"`
	ChannelInvite string    `json:"channel_invite" db:"channel_invite"`
	ChannelJID    string    `json:"channel_jid" db:"channel_jid"`
	JobTitle      string    `json:"job_title" db:"job_title"`
	Company       string    `json:"company" db:"company"`
	Location      string    `json:"location" db:"location"`
	SalaryText    string    `json:"salary_text" db:"salary_text"`
	Status        string    `json:"status" db:"status"` // 'posted', 'failed'
	ErrorMessage  string    `json:"error_message" db:"error_message"`
	PostedAt      time.Time `json:"posted_at" db:"posted_at"`
	JobSlug       string    `json:"job_slug,omitempty"`
	PostedFrom    string    `json:"posted_from,omitempty"`
}

// WhatsAppChannelStats provides high-level statistics for the WhatsApp Channel Auto-Poster.
type WhatsAppChannelStats struct {
	ChannelInvite    string     `json:"channel_invite"`
	ChannelURL       string     `json:"channel_url"`
	ChannelInviteURL string     `json:"channel_invite_url,omitempty"`
	TodayCount       int        `json:"today_count"`
	DailyMax      int        `json:"daily_max"` // 20
	TotalPosts    int        `json:"total_posts"`
	LastPostedAt  *time.Time `json:"last_posted_at,omitempty"`
	IsConnected   bool       `json:"is_connected"`
	DevicePhone   string     `json:"device_phone"`
}


