package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"jobyuk/internal/models"
)

// WhatsAppRepository handles database queries for subscribers, OTPs, logs, and gateway settings.
type WhatsAppRepository struct {
	db            *pgxpool.Pool
	routingMu     sync.RWMutex
	cachedRouting *SavedGatewayRouting
}

// NewWhatsAppRepository creates a new WhatsApp repository instance.
func NewWhatsAppRepository(db *pgxpool.Pool) *WhatsAppRepository {
	repo := &WhatsAppRepository{db: db}
	// Pre-load routing settings from disk into memory on startup
	_ = repo.loadRoutingFromFile()
	return repo
}

// AutoMigrate ensures all WhatsApp engine tables, columns, constraints, and indexes exist idempotently.
func (r *WhatsAppRepository) AutoMigrate(ctx context.Context) error {
	queries := []string{
		// 1. WhatsApp Subscribers Table
		`CREATE TABLE IF NOT EXISTS whatsapp_subscribers (
			id SERIAL PRIMARY KEY,
			phone_number VARCHAR(30) UNIQUE NOT NULL,
			full_name VARCHAR(150) NOT NULL,
			email VARCHAR(255),
			plan_type VARCHAR(30) NOT NULL DEFAULT 'monthly',
			plan_price_gbp NUMERIC(8,2) DEFAULT 9.99,
			status VARCHAR(50) NOT NULL DEFAULT 'active',
			max_number_changes INT NOT NULL DEFAULT 1,
			number_changes_used INT NOT NULL DEFAULT 0,
			last_number_changed_at TIMESTAMPTZ,
			categories TEXT[] DEFAULT '{}',
			locations TEXT[] DEFAULT '{}',
			min_salary INT DEFAULT 0,
			alert_mode VARCHAR(20) DEFAULT 'instant',
			notes TEXT DEFAULT '',
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW(),
			expires_at TIMESTAMPTZ DEFAULT (NOW() + INTERVAL '30 days'),
			pin_hash VARCHAR(255) DEFAULT '',
			failed_pin_attempts INT DEFAULT 0,
			locked_until TIMESTAMPTZ,
			source VARCHAR(50) DEFAULT 'portal',
			trial_ends_at TIMESTAMPTZ,
			payment_status VARCHAR(20) DEFAULT 'unpaid'
		)`,
		`ALTER TABLE whatsapp_subscribers ALTER COLUMN status TYPE VARCHAR(50)`,
		`ALTER TABLE whatsapp_subscribers ADD COLUMN IF NOT EXISTS pin_hash VARCHAR(255) DEFAULT ''`,
		`ALTER TABLE whatsapp_subscribers ADD COLUMN IF NOT EXISTS failed_pin_attempts INT DEFAULT 0`,
		`ALTER TABLE whatsapp_subscribers ADD COLUMN IF NOT EXISTS locked_until TIMESTAMPTZ`,
		`ALTER TABLE whatsapp_subscribers ADD COLUMN IF NOT EXISTS source VARCHAR(50) DEFAULT 'portal'`,
		`ALTER TABLE whatsapp_subscribers ADD COLUMN IF NOT EXISTS trial_ends_at TIMESTAMPTZ`,
		`ALTER TABLE whatsapp_subscribers ADD COLUMN IF NOT EXISTS payment_status VARCHAR(20) DEFAULT 'unpaid'`,
		`CREATE INDEX IF NOT EXISTS idx_wa_subscribers_phone ON whatsapp_subscribers(phone_number)`,
		`CREATE INDEX IF NOT EXISTS idx_wa_subscribers_status ON whatsapp_subscribers(status)`,
		`CREATE INDEX IF NOT EXISTS idx_wa_subscribers_pin_hash ON whatsapp_subscribers(pin_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_wa_subscribers_source ON whatsapp_subscribers(source)`,
		`CREATE INDEX IF NOT EXISTS idx_wa_subscribers_trial_ends ON whatsapp_subscribers(trial_ends_at)`,

		// 2. WhatsApp Number History
		`CREATE TABLE IF NOT EXISTS whatsapp_number_history (
			id SERIAL PRIMARY KEY,
			subscriber_id INT REFERENCES whatsapp_subscribers(id) ON DELETE CASCADE,
			old_phone VARCHAR(30) NOT NULL,
			new_phone VARCHAR(30) NOT NULL,
			changed_at TIMESTAMPTZ DEFAULT NOW(),
			ip_address VARCHAR(45) DEFAULT ''
		)`,

		// 3. WhatsApp OTPs Table
		`CREATE TABLE IF NOT EXISTS whatsapp_otps (
			phone_number VARCHAR(30) PRIMARY KEY,
			otp_code VARCHAR(6) NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			attempts INT DEFAULT 0,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,

		// 4. WhatsApp Alert Logs
		`CREATE TABLE IF NOT EXISTS whatsapp_alert_logs (
			id SERIAL PRIMARY KEY,
			subscriber_id INT REFERENCES whatsapp_subscribers(id) ON DELETE SET NULL,
			phone_number VARCHAR(30) NOT NULL,
			job_id BIGINT REFERENCES jobs(id) ON DELETE SET NULL,
			job_title VARCHAR(255) NOT NULL,
			company VARCHAR(255) DEFAULT '',
			location VARCHAR(255) DEFAULT '',
			salary_text VARCHAR(100) DEFAULT '',
			status VARCHAR(20) DEFAULT 'sent',
			error_message TEXT DEFAULT '',
			sent_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_wa_alert_logs_sub_id ON whatsapp_alert_logs(subscriber_id)`,
		`CREATE INDEX IF NOT EXISTS idx_wa_alert_logs_sent_at ON whatsapp_alert_logs(sent_at DESC)`,

		// 5. WhatsApp Gateway Settings
		`CREATE TABLE IF NOT EXISTS whatsapp_gateway_settings (
			id INT PRIMARY KEY DEFAULT 1,
			is_connected BOOLEAN DEFAULT FALSE,
			connected_phone VARCHAR(30) DEFAULT '',
			connected_name VARCHAR(100) DEFAULT '',
			last_connected_at TIMESTAMPTZ,
			daily_messages_sent INT DEFAULT 0,
			last_counter_reset DATE DEFAULT CURRENT_DATE,
			updated_at TIMESTAMPTZ DEFAULT NOW(),
			support_connected BOOLEAN NOT NULL DEFAULT FALSE,
			support_phone VARCHAR(50) NOT NULL DEFAULT '',
			support_name VARCHAR(100) NOT NULL DEFAULT '',
			support_last_connected_at TIMESTAMPTZ,
			routing_mode VARCHAR(30) NOT NULL DEFAULT 'dual',
			alerts_device VARCHAR(20) NOT NULL DEFAULT 'alerts',
			support_device VARCHAR(20) NOT NULL DEFAULT 'support',
			otp_device VARCHAR(20) NOT NULL DEFAULT 'otp',
			billing_device VARCHAR(20) NOT NULL DEFAULT 'billing'
		)`,
		`INSERT INTO whatsapp_gateway_settings (id, is_connected) VALUES (1, FALSE) ON CONFLICT (id) DO NOTHING`,
		`ALTER TABLE whatsapp_gateway_settings ADD COLUMN IF NOT EXISTS otp_connected BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE whatsapp_gateway_settings ADD COLUMN IF NOT EXISTS otp_phone VARCHAR(50) NOT NULL DEFAULT ''`,
		`ALTER TABLE whatsapp_gateway_settings ADD COLUMN IF NOT EXISTS otp_name VARCHAR(100) NOT NULL DEFAULT ''`,
		`ALTER TABLE whatsapp_gateway_settings ADD COLUMN IF NOT EXISTS otp_last_connected_at TIMESTAMPTZ`,
		`ALTER TABLE whatsapp_gateway_settings ADD COLUMN IF NOT EXISTS billing_connected BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE whatsapp_gateway_settings ADD COLUMN IF NOT EXISTS billing_phone VARCHAR(50) NOT NULL DEFAULT ''`,
		`ALTER TABLE whatsapp_gateway_settings ADD COLUMN IF NOT EXISTS billing_name VARCHAR(100) NOT NULL DEFAULT ''`,
		`ALTER TABLE whatsapp_gateway_settings ADD COLUMN IF NOT EXISTS billing_last_connected_at TIMESTAMPTZ`,
		`ALTER TABLE whatsapp_gateway_settings ADD COLUMN IF NOT EXISTS support_connected BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE whatsapp_gateway_settings ADD COLUMN IF NOT EXISTS support_phone VARCHAR(50) NOT NULL DEFAULT ''`,
		`ALTER TABLE whatsapp_gateway_settings ADD COLUMN IF NOT EXISTS support_name VARCHAR(100) NOT NULL DEFAULT ''`,
		`ALTER TABLE whatsapp_gateway_settings ADD COLUMN IF NOT EXISTS support_last_connected_at TIMESTAMPTZ`,
		`ALTER TABLE whatsapp_gateway_settings ADD COLUMN IF NOT EXISTS routing_mode VARCHAR(30) NOT NULL DEFAULT 'dual'`,
		`ALTER TABLE whatsapp_gateway_settings ADD COLUMN IF NOT EXISTS alerts_device VARCHAR(20) NOT NULL DEFAULT 'alerts'`,
		`ALTER TABLE whatsapp_gateway_settings ADD COLUMN IF NOT EXISTS support_device VARCHAR(20) NOT NULL DEFAULT 'support'`,
		`ALTER TABLE whatsapp_gateway_settings ADD COLUMN IF NOT EXISTS otp_device VARCHAR(20) NOT NULL DEFAULT 'otp'`,
		`ALTER TABLE whatsapp_gateway_settings ADD COLUMN IF NOT EXISTS billing_device VARCHAR(20) NOT NULL DEFAULT 'billing'`,
		`UPDATE whatsapp_gateway_settings
		 SET connected_phone = COALESCE(connected_phone, ''),
		     connected_name = COALESCE(connected_name, ''),
		     otp_phone = COALESCE(otp_phone, ''),
		     otp_name = COALESCE(otp_name, ''),
		     billing_phone = COALESCE(billing_phone, ''),
		     billing_name = COALESCE(billing_name, ''),
		     support_phone = COALESCE(support_phone, ''),
		     support_name = COALESCE(support_name, ''),
		     routing_mode = COALESCE(routing_mode, 'dual'),
		     otp_device = COALESCE(otp_device, 'otp'),
		     alerts_device = COALESCE(alerts_device, 'alerts'),
		     billing_device = COALESCE(billing_device, 'billing'),
		     support_device = COALESCE(support_device, 'support')
		 WHERE id = 1`,

		// 6. WhatsApp Subscriber Logins
		`CREATE TABLE IF NOT EXISTS whatsapp_subscriber_logins (
			id SERIAL PRIMARY KEY,
			subscriber_id INT REFERENCES whatsapp_subscribers(id) ON DELETE CASCADE,
			phone_number VARCHAR(30) NOT NULL,
			ip_address VARCHAR(45) DEFAULT '',
			user_agent TEXT DEFAULT '',
			action VARCHAR(50) NOT NULL DEFAULT 'login_pin',
			status VARCHAR(20) NOT NULL DEFAULT 'success',
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_wa_sub_logins_sub_id ON whatsapp_subscriber_logins(subscriber_id, created_at DESC)`,

		// 7. WhatsApp Analytics Events
		`CREATE TABLE IF NOT EXISTS whatsapp_analytics_events (
			id SERIAL PRIMARY KEY,
			visitor_id VARCHAR(100) NOT NULL,
			source VARCHAR(100) NOT NULL DEFAULT 'floating_bar',
			aria_label VARCHAR(255) DEFAULT '',
			trigger_placement VARCHAR(150) DEFAULT '',
			page_url TEXT DEFAULT '',
			referrer TEXT DEFAULT '',
			ip_address VARCHAR(60) DEFAULT '',
			location VARCHAR(100) DEFAULT 'United Kingdom',
			user_agent TEXT DEFAULT '',
			device_type VARCHAR(30) DEFAULT 'desktop',
			browser VARCHAR(50) DEFAULT '',
			os VARCHAR(50) DEFAULT '',
			phone_number VARCHAR(30) DEFAULT '',
			step VARCHAR(30) NOT NULL DEFAULT 'clicked',
			otp_sent BOOLEAN DEFAULT FALSE,
			otp_verified BOOLEAN DEFAULT FALSE,
			subscriber_id INT REFERENCES whatsapp_subscribers(id) ON DELETE SET NULL,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_wa_analytics_created_at ON whatsapp_analytics_events(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_wa_analytics_visitor_id ON whatsapp_analytics_events(visitor_id)`,
		`CREATE INDEX IF NOT EXISTS idx_wa_analytics_source ON whatsapp_analytics_events(source)`,
		`CREATE INDEX IF NOT EXISTS idx_wa_analytics_step ON whatsapp_analytics_events(step)`,
		`CREATE INDEX IF NOT EXISTS idx_wa_analytics_phone ON whatsapp_analytics_events(phone_number)`,

		// 8. WhatsApp Crypto Wallets
		`CREATE TABLE IF NOT EXISTS whatsapp_crypto_wallets (
			id SERIAL PRIMARY KEY,
			coin VARCHAR(20) NOT NULL,
			network VARCHAR(50) NOT NULL,
			symbol VARCHAR(20) NOT NULL,
			address VARCHAR(255) NOT NULL,
			instructions TEXT DEFAULT '',
			is_active BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`INSERT INTO whatsapp_crypto_wallets (coin, network, symbol, address, instructions, is_active)
		 SELECT 'USDT', 'TRC-20 (Tron)', '₮', 'TYDzsYUEpvnYmQK4W5g72N26F52fG9EQ11', 'Send USDT strictly via the TRC20 network. Other networks will result in permanent loss of funds.', TRUE
		 WHERE NOT EXISTS (SELECT 1 FROM whatsapp_crypto_wallets WHERE coin = 'USDT' AND network LIKE '%TRC-20%')`,
		`INSERT INTO whatsapp_crypto_wallets (coin, network, symbol, address, instructions, is_active)
		 SELECT 'USDT', 'BEP-20 (BNB Smart Chain)', '₮', '0x89205A3A3b2A556108d852A35C4da43de584444', 'Send USDT strictly via the BEP20 (BNB Smart Chain) network.', TRUE
		 WHERE NOT EXISTS (SELECT 1 FROM whatsapp_crypto_wallets WHERE coin = 'USDT' AND network LIKE '%BEP-20%')`,
		`INSERT INTO whatsapp_crypto_wallets (coin, network, symbol, address, instructions, is_active)
		 SELECT 'BTC', 'Bitcoin Native', '₿', 'bc1qar0srrr7xfkvy5l643lydnw9re59gtzzwf5mdq', 'Send BTC strictly via the native Bitcoin network. Requires 1-3 network confirmations.', TRUE
		 WHERE NOT EXISTS (SELECT 1 FROM whatsapp_crypto_wallets WHERE coin = 'BTC')`,
		`INSERT INTO whatsapp_crypto_wallets (coin, network, symbol, address, instructions, is_active)
		 SELECT 'ETH', 'ERC-20 (Ethereum)', 'Ξ', '0x89205A3A3b2A556108d852A35C4da43de584444', 'Send ETH strictly via Ethereum ERC-20 mainnet.', TRUE
		 WHERE NOT EXISTS (SELECT 1 FROM whatsapp_crypto_wallets WHERE coin = 'ETH')`,

		// 9. WhatsApp Crypto Payments
		`CREATE TABLE IF NOT EXISTS whatsapp_crypto_payments (
			id SERIAL PRIMARY KEY,
			subscriber_id INT REFERENCES whatsapp_subscribers(id) ON DELETE CASCADE,
			phone_number VARCHAR(30) NOT NULL,
			full_name VARCHAR(150) DEFAULT '',
			plan_type VARCHAR(30) NOT NULL,
			amount_gbp NUMERIC(10,2) NOT NULL,
			coin VARCHAR(20) NOT NULL,
			network VARCHAR(50) NOT NULL,
			deposit_address VARCHAR(255) NOT NULL,
			tx_hash VARCHAR(255) NOT NULL,
			sender_address VARCHAR(255) DEFAULT '',
			screenshot_url VARCHAR(500) NOT NULL,
			status VARCHAR(30) DEFAULT 'pending',
			admin_notes TEXT DEFAULT '',
			created_at TIMESTAMPTZ DEFAULT NOW(),
			verified_at TIMESTAMPTZ
		)`,
		`CREATE INDEX IF NOT EXISTS idx_wa_crypto_payments_sub ON whatsapp_crypto_payments(subscriber_id)`,
		`CREATE INDEX IF NOT EXISTS idx_wa_crypto_payments_status ON whatsapp_crypto_payments(status)`,
		`CREATE INDEX IF NOT EXISTS idx_wa_crypto_payments_tx ON whatsapp_crypto_payments(tx_hash)`,

		// 10. WhatsApp Payment Settings
		`CREATE TABLE IF NOT EXISTS whatsapp_payment_settings (
			id INT PRIMARY KEY DEFAULT 1,
			paypal_handle VARCHAR(100) NOT NULL DEFAULT 'jobyuk',
			paypal_instructions TEXT NOT NULL DEFAULT 'Pay securely via your PayPal account balance, linked UK bank account, or debit/credit card.',
			paypal_active BOOLEAN NOT NULL DEFAULT TRUE,
			card_active BOOLEAN NOT NULL DEFAULT TRUE,
			crypto_active BOOLEAN NOT NULL DEFAULT TRUE,
			support_active BOOLEAN NOT NULL DEFAULT TRUE,
			support_number VARCHAR(50) NOT NULL DEFAULT '+923025015693',
			support_instructions TEXT NOT NULL DEFAULT 'Connect directly with our billing support concierge to receive bespoke UK bank transfer details, Wise invoice, or alternative payment rails.',
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`ALTER TABLE whatsapp_payment_settings 
			ADD COLUMN IF NOT EXISTS support_active BOOLEAN NOT NULL DEFAULT TRUE,
			ADD COLUMN IF NOT EXISTS support_number VARCHAR(50) NOT NULL DEFAULT '+923025015693',
			ADD COLUMN IF NOT EXISTS support_instructions TEXT NOT NULL DEFAULT 'Connect directly with our billing support concierge to receive bespoke UK bank transfer details, Wise invoice, or alternative payment rails.'`,
		`INSERT INTO whatsapp_payment_settings (id, paypal_handle, paypal_instructions, paypal_active, card_active, crypto_active, support_active, support_number, support_instructions)
		 VALUES (1, 'jobyuk', 'Pay securely via your PayPal account balance, linked UK bank account, or debit/credit card.', TRUE, TRUE, TRUE, TRUE, '+923025015693', 'Connect directly with our billing support concierge to receive bespoke UK bank transfer details, Wise invoice, or alternative payment rails.')
		 ON CONFLICT (id) DO NOTHING`,

		// 11. WhatsApp Transactions
		`CREATE TABLE IF NOT EXISTS whatsapp_transactions (
			id SERIAL PRIMARY KEY,
			subscriber_id INT REFERENCES whatsapp_subscribers(id) ON DELETE SET NULL,
			phone_number VARCHAR(30) NOT NULL,
			full_name VARCHAR(150) DEFAULT '',
			plan_type VARCHAR(30) NOT NULL,
			amount_gbp NUMERIC(10,2) NOT NULL,
			method VARCHAR(30) NOT NULL,
			tx_ref VARCHAR(100) NOT NULL,
			status VARCHAR(30) NOT NULL DEFAULT 'completed',
			notes TEXT DEFAULT '',
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_wa_transactions_sub ON whatsapp_transactions(subscriber_id)`,
		`CREATE INDEX IF NOT EXISTS idx_wa_transactions_created ON whatsapp_transactions(created_at DESC)`,

		// 12. Fix any registered candidates with unpaid status to active 3-Day Free Trial
		`UPDATE whatsapp_subscribers
		 SET plan_type = 'trial',
		     payment_status = 'trial',
		     status = 'active',
		     trial_ends_at = NOW() + INTERVAL '72 hours',
		     expires_at = NOW() + INTERVAL '72 hours'
		 WHERE payment_status = 'unpaid'
		   AND (plan_type = 'monthly' OR plan_type = 'trial' OR plan_type IS NULL)
		   AND (trial_ends_at IS NULL OR trial_ends_at <= NOW())`,

		// 13. Unlock any temporarily locked subscribers and reset attempts
		`UPDATE whatsapp_subscribers
		 SET failed_pin_attempts = 0, locked_until = NULL
		 WHERE failed_pin_attempts > 0 OR locked_until IS NOT NULL`,

		// 14. Ensure registered candidate PINs are synced if one has PIN and the other was reset
		`UPDATE whatsapp_subscribers
		 SET pin_hash = (SELECT pin_hash FROM whatsapp_subscribers WHERE phone_number = '+923003005100' AND pin_hash != '' LIMIT 1)
		 WHERE phone_number = '+923363220343'
		   AND EXISTS (SELECT 1 FROM whatsapp_subscribers WHERE phone_number = '+923003005100' AND pin_hash != '')`,

		// 15. WhatsApp Channel Posts Table
		`CREATE TABLE IF NOT EXISTS whatsapp_channel_posts (
			id SERIAL PRIMARY KEY,
			job_id BIGINT REFERENCES jobs(id) ON DELETE CASCADE,
			channel_invite VARCHAR(100) NOT NULL DEFAULT '0029Vb9iXUMAu3aQOlanBy2m',
			channel_jid VARCHAR(150) DEFAULT '',
			job_title VARCHAR(255) NOT NULL,
			company VARCHAR(255) DEFAULT '',
			location VARCHAR(255) DEFAULT '',
			salary_text VARCHAR(100) DEFAULT '',
			status VARCHAR(50) DEFAULT 'posted',
			error_message TEXT DEFAULT '',
			posted_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_wa_channel_posts_job_id ON whatsapp_channel_posts(job_id)`,
		`CREATE INDEX IF NOT EXISTS idx_wa_channel_posts_posted_at ON whatsapp_channel_posts(posted_at DESC)`,
	}

	for _, q := range queries {
		if _, err := r.db.Exec(ctx, q); err != nil {
			return fmt.Errorf("whatsapp automigrate query failed: %w (query: %s)", err, strings.TrimSpace(q))
		}
	}
	return nil
}

// NormalizePhoneNumber strips spaces, dashes and ensures standard international E.164 format.
func NormalizePhoneNumber(phone string) string {
	cleaned := strings.TrimSpace(phone)
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	cleaned = strings.ReplaceAll(cleaned, "-", "")
	cleaned = strings.ReplaceAll(cleaned, "(", "")
	cleaned = strings.ReplaceAll(cleaned, ")", "")
	cleaned = strings.ReplaceAll(cleaned, ".", "")

	// Handle 00 international prefix (e.g. 0044 -> +44, 0092 -> +92)
	if strings.HasPrefix(cleaned, "00") {
		cleaned = "+" + cleaned[2:]
	}

	// Fix accidental +0 prefix (e.g. +03363220343 -> +923363220343, +07123456789 -> +447123456789)
	if strings.HasPrefix(cleaned, "+0") {
		if strings.HasPrefix(cleaned, "+03") && len(cleaned) == 12 {
			cleaned = "+92" + cleaned[2:]
		} else if strings.HasPrefix(cleaned, "+07") && len(cleaned) == 12 {
			cleaned = "+44" + cleaned[2:]
		} else {
			cleaned = "+" + strings.TrimLeft(cleaned[1:], "0")
		}
	}

	// Fix accidental +443... (Pakistan mobile 3xxxxxxxxx submitted with default UK +44 dial code)
	if strings.HasPrefix(cleaned, "+443") && len(cleaned) == 13 {
		cleaned = "+92" + cleaned[3:]
	}

	// UK domestic 07xxxxxxxxx (11 digits) -> +447xxxxxxxxx
	if strings.HasPrefix(cleaned, "07") && len(cleaned) == 11 {
		cleaned = "+44" + cleaned[1:]
	} else if strings.HasPrefix(cleaned, "03") && len(cleaned) == 11 { // Pakistan domestic 03xxxxxxxxx -> +923xxxxxxxxx
		cleaned = "+92" + cleaned[1:]
	} else if strings.HasPrefix(cleaned, "3") && len(cleaned) == 10 { // Pakistan domestic without leading zero: 3xxxxxxxxx -> +923xxxxxxxxx
		cleaned = "+92" + cleaned
	} else if strings.HasPrefix(cleaned, "7") && len(cleaned) == 10 { // UK domestic without leading zero: 7xxxxxxxxx -> +447xxxxxxxxx
		cleaned = "+44" + cleaned
	} else if strings.HasPrefix(cleaned, "44") && !strings.HasPrefix(cleaned, "+") && len(cleaned) >= 11 {
		cleaned = "+" + cleaned
	} else if strings.HasPrefix(cleaned, "92") && !strings.HasPrefix(cleaned, "+") && len(cleaned) >= 11 {
		cleaned = "+" + cleaned
	} else if !strings.HasPrefix(cleaned, "+") && len(cleaned) >= 7 {
		cleaned = "+" + cleaned
	}
	return cleaned
}

// IsValidE164 checks if a phone number adheres strictly to international E.164 format:
// + followed by a non-zero country code digit, and 7 to 14 more digits (8-15 total digits).
func IsValidE164(phone string) bool {
	if !strings.HasPrefix(phone, "+") {
		return false
	}
	digits := phone[1:]
	if len(digits) < 7 || len(digits) > 15 {
		return false
	}
	// Country code cannot begin with 0
	if digits[0] == '0' {
		return false
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// CreateSubscriber creates a new WhatsApp subscriber.
func (r *WhatsAppRepository) CreateSubscriber(ctx context.Context, sub *models.WhatsAppSubscriber) error {
	sub.PhoneNumber = NormalizePhoneNumber(sub.PhoneNumber)
	sub.FullName = strings.TrimSpace(sub.FullName)
	sub.Email = strings.ToLower(strings.TrimSpace(sub.Email))

	if sub.Source == "" {
		sub.Source = "portal"
	}

	if sub.MaxNumberChanges <= 0 {
		switch sub.PlanType {
		case "monthly":
			sub.MaxNumberChanges = 1
			if sub.PlanPriceGBP <= 0 {
				sub.PlanPriceGBP = 4.99
			}
		case "6months":
			sub.MaxNumberChanges = 3
			if sub.PlanPriceGBP <= 0 {
				sub.PlanPriceGBP = 24.99
			}
		case "yearly":
			sub.MaxNumberChanges = 5
			if sub.PlanPriceGBP <= 0 {
				sub.PlanPriceGBP = 39.99
			}
		case "admin_pass":
			sub.MaxNumberChanges = 10
			sub.PlanPriceGBP = 0.0
		case "trial":
			sub.MaxNumberChanges = 0
			sub.PlanPriceGBP = 0.0
		default:
			sub.MaxNumberChanges = 1
		}
	}

	if sub.Status == "" {
		sub.Status = "active"
	}
	if sub.AlertMode == "" {
		sub.AlertMode = "instant"
	}
	if sub.PaymentStatus == "" {
		if sub.PlanType == "trial" {
			sub.PaymentStatus = "trial"
		} else if sub.PlanType == "admin_pass" {
			sub.PaymentStatus = "free_pass"
		} else {
			sub.PaymentStatus = "unpaid"
		}
	}
	if sub.PlanType == "trial" && sub.TrialEndsAt == nil {
		t := time.Now().Add(72 * time.Hour)
		sub.TrialEndsAt = &t
		sub.ExpiresAt = t
	}

	if sub.ExpiresAt.IsZero() {
		switch sub.PlanType {
		case "trial":
			sub.ExpiresAt = time.Now().Add(72 * time.Hour)
		case "6months":
			sub.ExpiresAt = time.Now().AddDate(0, 6, 0)
		case "yearly":
			sub.ExpiresAt = time.Now().AddDate(1, 0, 0)
		case "admin_pass":
			sub.ExpiresAt = time.Now().AddDate(10, 0, 0)
		default:
			sub.ExpiresAt = time.Now().AddDate(0, 1, 0)
		}
	}

	query := `
		INSERT INTO whatsapp_subscribers (
			phone_number, full_name, email, plan_type, plan_price_gbp,
			status, max_number_changes, number_changes_used,
			categories, locations, min_salary, alert_mode, notes, expires_at, pin_hash,
			source, trial_ends_at, payment_status
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18
		) RETURNING id, created_at, updated_at`

	return r.db.QueryRow(ctx, query,
		sub.PhoneNumber, sub.FullName, sub.Email, sub.PlanType, sub.PlanPriceGBP,
		sub.Status, sub.MaxNumberChanges, sub.NumberChangesUsed,
		sub.Categories, sub.Locations, sub.MinSalary, sub.AlertMode, sub.Notes, sub.ExpiresAt, sub.PINHash,
		sub.Source, sub.TrialEndsAt, sub.PaymentStatus,
	).Scan(&sub.ID, &sub.CreatedAt, &sub.UpdatedAt)
}

// UpsertPendingTrialCandidate pre-saves a candidate in whatsapp_subscribers with pending_verification status BEFORE OTP is sent.
func (r *WhatsAppRepository) UpsertPendingTrialCandidate(ctx context.Context, sub *models.WhatsAppSubscriber) (*models.WhatsAppSubscriber, error) {
	sub.PhoneNumber = NormalizePhoneNumber(sub.PhoneNumber)
	sub.FullName = strings.TrimSpace(sub.FullName)
	if sub.FullName == "" {
		suffix := sub.PhoneNumber
		if len(sub.PhoneNumber) > 4 {
			suffix = sub.PhoneNumber[len(sub.PhoneNumber)-4:]
		}
		sub.FullName = fmt.Sprintf("Candidate %s", suffix)
	}

	if sub.Source == "" {
		sub.Source = "homepage_trial"
	}
	if sub.PlanType == "" {
		sub.PlanType = "trial"
	}
	sub.Status = "pending_verification"
	sub.PaymentStatus = "trial"
	if sub.AlertMode == "" {
		sub.AlertMode = "instant"
	}
	if sub.ExpiresAt.IsZero() {
		sub.ExpiresAt = time.Now().Add(72 * time.Hour)
	}

	query := `
		INSERT INTO whatsapp_subscribers (
			phone_number, full_name, email, plan_type, plan_price_gbp,
			status, max_number_changes, number_changes_used,
			categories, locations, min_salary, alert_mode, notes, expires_at, pin_hash,
			source, trial_ends_at, payment_status
		) VALUES (
			$1, $2, '', 'trial', 0.0,
			'pending_verification', 0, 0,
			$3, $4, 0, $5, 'Registered via WhatsApp Free Trial modal - awaiting OTP verification', $6, '',
			$7, NULL, 'trial'
		)
		ON CONFLICT (phone_number) DO UPDATE SET
			full_name = CASE WHEN whatsapp_subscribers.status = 'pending_verification' AND EXCLUDED.full_name != '' THEN EXCLUDED.full_name ELSE whatsapp_subscribers.full_name END,
			categories = CASE WHEN whatsapp_subscribers.status = 'pending_verification' THEN EXCLUDED.categories ELSE whatsapp_subscribers.categories END,
			locations = CASE WHEN whatsapp_subscribers.status = 'pending_verification' THEN EXCLUDED.locations ELSE whatsapp_subscribers.locations END,
			updated_at = NOW()
		RETURNING id, phone_number, full_name, email, plan_type, plan_price_gbp,
		          status, max_number_changes, number_changes_used, last_number_changed_at,
		          categories, locations, min_salary, alert_mode, notes,
		          created_at, updated_at, expires_at,
		          COALESCE(source, 'homepage_trial'), trial_ends_at, COALESCE(payment_status, 'trial')`

	out := &models.WhatsAppSubscriber{}
	err := r.db.QueryRow(ctx, query,
		sub.PhoneNumber, sub.FullName, sub.Categories, sub.Locations, sub.AlertMode, sub.ExpiresAt, sub.Source,
	).Scan(
		&out.ID, &out.PhoneNumber, &out.FullName, &out.Email, &out.PlanType, &out.PlanPriceGBP,
		&out.Status, &out.MaxNumberChanges, &out.NumberChangesUsed, &out.LastNumberChangedAt,
		&out.Categories, &out.Locations, &out.MinSalary, &out.AlertMode, &out.Notes,
		&out.CreatedAt, &out.UpdatedAt, &out.ExpiresAt,
		&out.Source, &out.TrialEndsAt, &out.PaymentStatus,
	)
	if err != nil {
		return nil, err
	}
	r.enrichSubscriberComputedFields(out)
	return out, nil
}

// ActivatePendingTrialSubscriber promotes a pending trial candidate to active once OTP is verified.
func (r *WhatsAppRepository) ActivatePendingTrialSubscriber(ctx context.Context, id int, fullName, pinHash string, trialEnd time.Time) error {
	query := `
		UPDATE whatsapp_subscribers
		SET status = 'active',
		    full_name = CASE WHEN $2 != '' THEN $2 ELSE full_name END,
		    pin_hash = $3,
		    trial_ends_at = $4,
		    expires_at = $4,
		    payment_status = 'trial',
		    updated_at = NOW()
		WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id, fullName, pinHash, trialEnd)
	return err
}

// HashPIN generates a cryptographically secure bcrypt hash for a 6-digit access PIN.
func HashPIN(pin string) (string, error) {
	cleaned := strings.TrimSpace(pin)
	if len(cleaned) != 6 {
		return "", fmt.Errorf("access code must be exactly 6 digits")
	}
	bytes, err := bcrypt.GenerateFromPassword([]byte(cleaned), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// SetSubscriberPIN updates the 6-digit access code for a subscriber and clears lockouts.
func (r *WhatsAppRepository) SetSubscriberPIN(ctx context.Context, id int, pin string) error {
	hash, err := HashPIN(pin)
	if err != nil {
		return err
	}

	query := `
		UPDATE whatsapp_subscribers
		SET pin_hash = $2, failed_pin_attempts = 0, locked_until = NULL, updated_at = NOW()
		WHERE id = $1`
	_, err = r.db.Exec(ctx, query, id, hash)
	return err
}

// VerifySubscriberPIN checks phone + 6-digit PIN and enforces anti-brute-force lockout.
func (r *WhatsAppRepository) VerifySubscriberPIN(ctx context.Context, phone, pin string) (*models.WhatsAppSubscriber, error) {
	norm := NormalizePhoneNumber(phone)
	if norm == "" {
		return nil, fmt.Errorf("please enter a valid phone number")
	}

	cleanPIN := strings.TrimSpace(pin)
	if len(cleanPIN) != 6 {
		return nil, fmt.Errorf("access code must be exactly 6 digits")
	}

	sub, err := r.GetSubscriberByPhone(ctx, norm)
	if err != nil || sub == nil {
		return nil, fmt.Errorf("no subscription found for this phone number")
	}

	// Check if account is temporarily locked
	if sub.LockedUntil != nil && sub.LockedUntil.After(time.Now()) {
		remaining := time.Until(*sub.LockedUntil).Round(time.Minute)
		if remaining < time.Minute {
			remaining = time.Minute
		}
		return nil, fmt.Errorf("account temporarily locked due to failed attempts. Try again in %s or reset your code", remaining)
	}

	// If subscriber doesn't have a PIN set yet
	if sub.PINHash == "" {
		return nil, fmt.Errorf("access PIN not yet generated. Please verify via WhatsApp OTP to receive your PIN")
	}

	// Verify bcrypt hash
	if err := bcrypt.CompareHashAndPassword([]byte(sub.PINHash), []byte(cleanPIN)); err != nil {
		// Increment failed attempts
		newAttempts := sub.FailedPINAttempts + 1
		if newAttempts >= 5 {
			_, _ = r.db.Exec(ctx, `UPDATE whatsapp_subscribers SET failed_pin_attempts = $2, locked_until = NOW() + INTERVAL '15 minutes' WHERE id = $1`, sub.ID, newAttempts)
			return nil, fmt.Errorf("incorrect access code. Account locked for 15 minutes after 5 failed attempts")
		}

		_, _ = r.db.Exec(ctx, `UPDATE whatsapp_subscribers SET failed_pin_attempts = $2 WHERE id = $1`, sub.ID, newAttempts)
		return nil, fmt.Errorf("incorrect 6-digit access code (%d attempts remaining)", 5-newAttempts)
	}

	// Successful login: reset failed attempts
	if sub.FailedPINAttempts > 0 || sub.LockedUntil != nil {
		_, _ = r.db.Exec(ctx, `UPDATE whatsapp_subscribers SET failed_pin_attempts = 0, locked_until = NULL WHERE id = $1`, sub.ID)
		sub.FailedPINAttempts = 0
		sub.LockedUntil = nil
	}

	return sub, nil
}

// GetSubscriberByID fetches a subscriber by ID with enriched helper fields.
func (r *WhatsAppRepository) GetSubscriberByID(ctx context.Context, id int) (*models.WhatsAppSubscriber, error) {
	query := `
		SELECT id, phone_number, full_name, email, plan_type, plan_price_gbp,
		       status, max_number_changes, number_changes_used, last_number_changed_at,
		       categories, locations, min_salary, alert_mode, notes,
		       created_at, updated_at, expires_at, pin_hash, failed_pin_attempts, locked_until,
		       COALESCE(source, 'portal'), trial_ends_at, COALESCE(payment_status, 'unpaid')
		FROM whatsapp_subscribers
		WHERE id = $1`

	sub := &models.WhatsAppSubscriber{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&sub.ID, &sub.PhoneNumber, &sub.FullName, &sub.Email, &sub.PlanType, &sub.PlanPriceGBP,
		&sub.Status, &sub.MaxNumberChanges, &sub.NumberChangesUsed, &sub.LastNumberChangedAt,
		&sub.Categories, &sub.Locations, &sub.MinSalary, &sub.AlertMode, &sub.Notes,
		&sub.CreatedAt, &sub.UpdatedAt, &sub.ExpiresAt, &sub.PINHash, &sub.FailedPINAttempts, &sub.LockedUntil,
		&sub.Source, &sub.TrialEndsAt, &sub.PaymentStatus,
	)
	if err != nil {
		return nil, err
	}

	r.enrichSubscriberComputedFields(sub)
	return sub, nil
}

// GetSubscriberByPhone fetches a subscriber by phone number with multi-format matching.
func (r *WhatsAppRepository) GetSubscriberByPhone(ctx context.Context, phone string) (*models.WhatsAppSubscriber, error) {
	norm := NormalizePhoneNumber(phone)
	rawDigits := ""
	for _, ch := range phone {
		if ch >= '0' && ch <= '9' {
			rawDigits += string(ch)
		}
	}
	last10 := rawDigits
	if len(last10) > 10 {
		last10 = last10[len(last10)-10:]
	}

	query := `
		SELECT id, phone_number, full_name, email, plan_type, plan_price_gbp,
		       status, max_number_changes, number_changes_used, last_number_changed_at,
		       categories, locations, min_salary, alert_mode, notes,
		       created_at, updated_at, expires_at, pin_hash, failed_pin_attempts, locked_until,
		       COALESCE(source, 'portal'), trial_ends_at, COALESCE(payment_status, 'unpaid')
		FROM whatsapp_subscribers
		WHERE phone_number = $1
		   OR phone_number = $2
		   OR (LENGTH($3) >= 9 AND RIGHT(REGEXP_REPLACE(phone_number, '\D', '', 'g'), 10) = $3)
		ORDER BY (phone_number = $1) DESC, (phone_number = $2) DESC, created_at DESC
		LIMIT 1`

	sub := &models.WhatsAppSubscriber{}
	err := r.db.QueryRow(ctx, query, norm, phone, last10).Scan(
		&sub.ID, &sub.PhoneNumber, &sub.FullName, &sub.Email, &sub.PlanType, &sub.PlanPriceGBP,
		&sub.Status, &sub.MaxNumberChanges, &sub.NumberChangesUsed, &sub.LastNumberChangedAt,
		&sub.Categories, &sub.Locations, &sub.MinSalary, &sub.AlertMode, &sub.Notes,
		&sub.CreatedAt, &sub.UpdatedAt, &sub.ExpiresAt, &sub.PINHash, &sub.FailedPINAttempts, &sub.LockedUntil,
		&sub.Source, &sub.TrialEndsAt, &sub.PaymentStatus,
	)
	if err != nil {
		return nil, err
	}

	r.enrichSubscriberComputedFields(sub)
	return sub, nil
}

// enrichSubscriberComputedFields calculates quota, cooldown, days remaining, and trial/paywall status.
func (r *WhatsAppRepository) enrichSubscriberComputedFields(sub *models.WhatsAppSubscriber) {
	sub.RemainingChanges = sub.MaxNumberChanges - sub.NumberChangesUsed
	if sub.RemainingChanges < 0 {
		sub.RemainingChanges = 0
	}

	sub.IsTrial = (sub.PlanType == "trial" || sub.Source == "homepage_trial" || sub.Source == "register" || sub.PaymentStatus == "trial")
	if sub.TrialEndsAt != nil {
		if time.Now().Before(*sub.TrialEndsAt) {
			sub.IsTrialActive = true
			sub.TrialHoursRemaining = int(time.Until(*sub.TrialEndsAt).Hours())
			if sub.TrialHoursRemaining <= 0 {
				sub.TrialHoursRemaining = 1
			}
			if sub.Status == "expired" {
				sub.Status = "active"
			}
		} else {
			sub.IsTrialActive = false
			sub.TrialHoursRemaining = 0
			if sub.PaymentStatus != "paid" && sub.PlanType != "admin_pass" {
				sub.Status = "expired"
			}
		}
	}

	if sub.ExpiresAt.After(time.Now()) {
		sub.DaysRemaining = int(time.Until(sub.ExpiresAt).Hours() / 24)
	} else if !sub.IsTrialActive {
		sub.DaysRemaining = 0
		if sub.Status == "active" {
			sub.Status = "expired"
		}
	}

	// Paywall is active if subscriber is unpaid or trial expired (unless admin pass)
	if sub.PlanType != "admin_pass" {
		if (sub.IsTrial && !sub.IsTrialActive) || (!sub.IsTrial && sub.PaymentStatus == "unpaid") || sub.Status == "expired" {
			sub.IsPaywalled = true
		}
	}

	// 14-day cooldown rule between changes
	const cooldownDuration = 14 * 24 * time.Hour
	if sub.LastNumberChangedAt != nil {
		cooldownExpiry := sub.LastNumberChangedAt.Add(cooldownDuration)
		if time.Now().Before(cooldownExpiry) {
			sub.CooldownActive = true
			remaining := time.Until(cooldownExpiry)
			days := int(remaining.Hours() / 24)
			hours := int(remaining.Hours()) % 24
			if days > 0 {
				sub.CooldownEndsIn = fmt.Sprintf("%d days, %d hours", days, hours)
			} else {
				sub.CooldownEndsIn = fmt.Sprintf("%d hours", hours)
			}
		}
	}
}

// UpgradeSubscription updates the subscriber's plan, extends expiry, marks payment_status = 'paid', and activates account.
func (r *WhatsAppRepository) UpgradeSubscription(ctx context.Context, id int, planType string) error {
	var days int
	var price float64
	var maxChanges int

	switch planType {
	case "monthly":
		days = 30
		price = 4.99
		maxChanges = 1
	case "6months":
		days = 180
		price = 24.99
		maxChanges = 3
	case "yearly":
		days = 365
		price = 39.99
		maxChanges = 5
	default:
		planType = "monthly"
		days = 30
		price = 4.99
		maxChanges = 1
	}

	query := `
		UPDATE whatsapp_subscribers
		SET plan_type = $1,
		    plan_price_gbp = $2,
		    status = 'active',
		    payment_status = 'paid',
		    trial_ends_at = NULL,
		    max_number_changes = GREATEST(max_number_changes, $3),
		    expires_at = GREATEST(expires_at, NOW()) + ($4 || ' days')::interval,
		    updated_at = NOW()
		WHERE id = $5`

	_, err := r.db.Exec(ctx, query, planType, price, maxChanges, fmt.Sprintf("%d", days), id)
	return err
}

// ListSubscribers lists subscribers with search and filter.
func (r *WhatsAppRepository) ListSubscribers(ctx context.Context, search, statusFilter, planFilter string, limit, offset int) ([]*models.WhatsAppSubscriber, int, error) {
	if limit <= 0 {
		limit = 25
	}
	if offset < 0 {
		offset = 0
	}

	whereClauses := []string{"1=1"}
	args := []interface{}{}
	argIdx := 1

	if search != "" {
		searchTerm := "%" + strings.TrimSpace(search) + "%"
		whereClauses = append(whereClauses, fmt.Sprintf("(phone_number ILIKE $%d OR full_name ILIKE $%d OR email ILIKE $%d)", argIdx, argIdx, argIdx))
		args = append(args, searchTerm)
		argIdx++
	}

	if statusFilter != "" && statusFilter != "all" {
		whereClauses = append(whereClauses, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, statusFilter)
		argIdx++
	}

	if planFilter != "" && planFilter != "all" {
		if planFilter == "trial" {
			whereClauses = append(whereClauses, "(plan_type = 'trial' OR source = 'homepage_trial' OR trial_ends_at IS NOT NULL)")
		} else {
			whereClauses = append(whereClauses, fmt.Sprintf("plan_type = $%d", argIdx))
			args = append(args, planFilter)
			argIdx++
		}
	}

	whereSQL := strings.Join(whereClauses, " AND ")

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM whatsapp_subscribers WHERE %s", whereSQL)
	var total int
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`
		SELECT id, phone_number, full_name, email, plan_type, plan_price_gbp,
		       status, max_number_changes, number_changes_used, last_number_changed_at,
		       categories, locations, min_salary, alert_mode, notes,
		       created_at, updated_at, expires_at,
		       COALESCE(source, 'portal'), trial_ends_at, COALESCE(payment_status, 'unpaid')
		FROM whatsapp_subscribers
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, whereSQL, argIdx, argIdx+1)

	args = append(args, limit, offset)
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var subscribers []*models.WhatsAppSubscriber
	for rows.Next() {
		sub := &models.WhatsAppSubscriber{}
		err := rows.Scan(
			&sub.ID, &sub.PhoneNumber, &sub.FullName, &sub.Email, &sub.PlanType, &sub.PlanPriceGBP,
			&sub.Status, &sub.MaxNumberChanges, &sub.NumberChangesUsed, &sub.LastNumberChangedAt,
			&sub.Categories, &sub.Locations, &sub.MinSalary, &sub.AlertMode, &sub.Notes,
			&sub.CreatedAt, &sub.UpdatedAt, &sub.ExpiresAt,
			&sub.Source, &sub.TrialEndsAt, &sub.PaymentStatus,
		)
		if err != nil {
			return nil, 0, err
		}
		r.enrichSubscriberComputedFields(sub)
		subscribers = append(subscribers, sub)
	}

	return subscribers, total, nil
}

// CanChangePhoneNumber evaluates whether a subscriber is eligible to change their number.
func (r *WhatsAppRepository) CanChangePhoneNumber(sub *models.WhatsAppSubscriber) (bool, string) {
	if sub.IsTrial || sub.PaymentStatus != "paid" || sub.PlanType == "trial" {
		return false, "Phone number updates are exclusively reserved for paid VIP subscribers. Upgrade your membership below to unlock number updates."
	}
	if sub.Status != "active" {
		return false, "Your subscription is not active."
	}
	if time.Now().After(sub.ExpiresAt) {
		return false, "Your subscription has expired. Please renew your plan."
	}
	if sub.NumberChangesUsed >= sub.MaxNumberChanges {
		return false, fmt.Sprintf("You have reached the maximum allowed number changes (%d of %d) for your plan.", sub.NumberChangesUsed, sub.MaxNumberChanges)
	}
	if sub.CooldownActive {
		return false, fmt.Sprintf("A 14-day security cooldown is currently active. You can update your number again in %s.", sub.CooldownEndsIn)
	}
	return true, ""
}

// ChangePhoneNumber executes atomic phone number change and records history.
func (r *WhatsAppRepository) ChangePhoneNumber(ctx context.Context, subscriberID int, newPhone, ip string) error {
	sub, err := r.GetSubscriberByID(ctx, subscriberID)
	if err != nil {
		return err
	}

	canChange, reason := r.CanChangePhoneNumber(sub)
	if !canChange {
		return fmt.Errorf("%s", reason)
	}

	normNewPhone := NormalizePhoneNumber(newPhone)
	if normNewPhone == sub.PhoneNumber {
		return fmt.Errorf("new phone number cannot be the same as the current number")
	}

	// Check if new number is already registered
	var exists bool
	_ = r.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM whatsapp_subscribers WHERE phone_number = $1 AND id != $2)", normNewPhone, subscriberID).Scan(&exists)
	if exists {
		return fmt.Errorf("phone number %s is already in use by another subscriber", normNewPhone)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Update subscriber
	updateQuery := `
		UPDATE whatsapp_subscribers
		SET phone_number = $1,
		    number_changes_used = number_changes_used + 1,
		    last_number_changed_at = NOW(),
		    updated_at = NOW()
		WHERE id = $2`
	if _, err := tx.Exec(ctx, updateQuery, normNewPhone, subscriberID); err != nil {
		return err
	}

	// Record history
	historyQuery := `
		INSERT INTO whatsapp_number_history (subscriber_id, old_phone, new_phone, ip_address)
		VALUES ($1, $2, $3, $4)`
	if _, err := tx.Exec(ctx, historyQuery, subscriberID, sub.PhoneNumber, normNewPhone, ip); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// UpdatePreferences updates the job alert preferences for a subscriber.
func (r *WhatsAppRepository) UpdatePreferences(ctx context.Context, id int, categories, locations []string, minSalary int, alertMode string) error {
	query := `
		UPDATE whatsapp_subscribers
		SET categories = $1,
		    locations = $2,
		    min_salary = $3,
		    alert_mode = $4,
		    updated_at = NOW()
		WHERE id = $5`
	_, err := r.db.Exec(ctx, query, categories, locations, minSalary, alertMode, id)
	return err
}

// UpdateSubscriberByAdmin updates full subscriber details from admin portal.
func (r *WhatsAppRepository) UpdateSubscriberByAdmin(ctx context.Context, sub *models.WhatsAppSubscriber) error {
	sub.PhoneNumber = NormalizePhoneNumber(sub.PhoneNumber)
	query := `
		UPDATE whatsapp_subscribers
		SET phone_number = $1,
		    full_name = $2,
		    email = $3,
		    plan_type = $4,
		    status = $5,
		    max_number_changes = $6,
		    categories = $7,
		    locations = $8,
		    min_salary = $9,
		    alert_mode = $10,
		    notes = $11,
		    expires_at = $12,
		    updated_at = NOW()
		WHERE id = $13`
	_, err := r.db.Exec(ctx, query,
		sub.PhoneNumber, sub.FullName, sub.Email, sub.PlanType,
		sub.Status, sub.MaxNumberChanges, sub.Categories, sub.Locations,
		sub.MinSalary, sub.AlertMode, sub.Notes, sub.ExpiresAt, sub.ID,
	)
	return err
}

// ToggleStatus pauses or resumes a subscription.
func (r *WhatsAppRepository) ToggleStatus(ctx context.Context, id int, newStatus string) error {
	query := `UPDATE whatsapp_subscribers SET status = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.Exec(ctx, query, newStatus, id)
	return err
}

// DeleteSubscriber removes a subscriber.
func (r *WhatsAppRepository) DeleteSubscriber(ctx context.Context, id int) error {
	query := `DELETE FROM whatsapp_subscribers WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	return err
}

// SaveOTP stores an OTP code for a phone number with a 5-minute expiry.
func (r *WhatsAppRepository) SaveOTP(ctx context.Context, phone, code string) error {
	norm := NormalizePhoneNumber(phone)
	query := `
		INSERT INTO whatsapp_otps (phone_number, otp_code, expires_at, attempts, created_at)
		VALUES ($1, $2, NOW() + INTERVAL '5 minutes', 0, NOW())
		ON CONFLICT (phone_number) DO UPDATE
		SET otp_code = $2,
		    expires_at = NOW() + INTERVAL '5 minutes',
		    attempts = 0,
		    created_at = NOW()`
	_, err := r.db.Exec(ctx, query, norm, code)
	return err
}

// VerifyOTP checks the entered code.
func (r *WhatsAppRepository) VerifyOTP(ctx context.Context, phone, code string) (bool, error) {
	norm := NormalizePhoneNumber(phone)
	query := `
		SELECT otp_code, expires_at, attempts
		FROM whatsapp_otps
		WHERE phone_number = $1`

	var dbCode string
	var expiresAt time.Time
	var attempts int
	err := r.db.QueryRow(ctx, query, norm).Scan(&dbCode, &expiresAt, &attempts)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, fmt.Errorf("no verification code found for this number")
		}
		return false, err
	}

	if time.Now().After(expiresAt) {
		return false, fmt.Errorf("verification code has expired. Please request a new code")
	}

	if attempts >= 5 {
		return false, fmt.Errorf("too many failed attempts. Please request a new code")
	}

	if strings.TrimSpace(code) != strings.TrimSpace(dbCode) {
		_, _ = r.db.Exec(ctx, "UPDATE whatsapp_otps SET attempts = attempts + 1 WHERE phone_number = $1", norm)
		return false, fmt.Errorf("incorrect verification code")
	}

	// Clean up verified OTP
	_, _ = r.db.Exec(ctx, "DELETE FROM whatsapp_otps WHERE phone_number = $1", norm)
	return true, nil
}

// LogAlert records a sent job alert.
func (r *WhatsAppRepository) LogAlert(ctx context.Context, log *models.WhatsAppAlertLog) error {
	log.PhoneNumber = NormalizePhoneNumber(log.PhoneNumber)
	query := `
		INSERT INTO whatsapp_alert_logs (
			subscriber_id, phone_number, job_id, job_title, company,
			location, salary_text, status, error_message, sent_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		RETURNING id, sent_at`
	return r.db.QueryRow(ctx, query,
		log.SubscriberID, log.PhoneNumber, log.JobID, log.JobTitle,
		log.Company, log.Location, log.SalaryText, log.Status, log.ErrorMessage,
	).Scan(&log.ID, &log.SentAt)
}

// ListAlertLogs returns alert logs with pagination.
func (r *WhatsAppRepository) ListAlertLogs(ctx context.Context, subscriberID *int, limit, offset int) ([]*models.WhatsAppAlertLog, int, error) {
	if limit <= 0 {
		limit = 25
	}
	if offset < 0 {
		offset = 0
	}

	whereClause := "1=1"
	args := []interface{}{}
	if subscriberID != nil {
		whereClause = "l.subscriber_id = $1"
		args = append(args, *subscriberID)
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM whatsapp_alert_logs l WHERE %s", whereClause)
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`
		SELECT l.id, l.subscriber_id, l.phone_number, l.job_id, l.job_title,
		       l.company, l.location, l.salary_text, l.status, l.error_message, l.sent_at,
		       COALESCE(s.full_name, 'Direct') as subscriber_name
		FROM whatsapp_alert_logs l
		LEFT JOIN whatsapp_subscribers s ON s.id = l.subscriber_id
		WHERE %s
		ORDER BY l.sent_at DESC
		LIMIT $%d OFFSET $%d`, whereClause, len(args)+1, len(args)+2)

	args = append(args, limit, offset)
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []*models.WhatsAppAlertLog
	for rows.Next() {
		item := &models.WhatsAppAlertLog{}
		err := rows.Scan(
			&item.ID, &item.SubscriberID, &item.PhoneNumber, &item.JobID, &item.JobTitle,
			&item.Company, &item.Location, &item.SalaryText, &item.Status, &item.ErrorMessage,
			&item.SentAt, &item.SubscriberName,
		)
		if err != nil {
			return nil, 0, err
		}
		logs = append(logs, item)
	}
	return logs, total, nil
}

// GetLastAlertSentTime returns the timestamp of the most recent alert sent to the subscriber.
func (r *WhatsAppRepository) GetLastAlertSentTime(ctx context.Context, subscriberID int) (*time.Time, error) {
	query := `SELECT sent_at FROM whatsapp_alert_logs WHERE subscriber_id = $1 AND status = 'sent' ORDER BY sent_at DESC LIMIT 1`
	var t time.Time
	err := r.db.QueryRow(ctx, query, subscriberID).Scan(&t)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// SavedGatewayRouting defines the disk persistence format for gateway routing settings.
type SavedGatewayRouting struct {
	RoutingMode   string `json:"routing_mode"`
	OTPDevice     string `json:"otp_device"`
	AlertsDevice  string `json:"alerts_device"`
	BillingDevice string `json:"billing_device"`
	SupportDevice string `json:"support_device"`
	UpdatedAt     string `json:"updated_at"`
}

var gatewayRoutingFilePaths = []string{
	"data/gateway_routing.json",
	"/var/www/jobyuk/data/gateway_routing.json",
}

func (r *WhatsAppRepository) loadRoutingFromFile() *SavedGatewayRouting {
	r.routingMu.RLock()
	if r.cachedRouting != nil && r.cachedRouting.OTPDevice != "" {
		cached := *r.cachedRouting
		r.routingMu.RUnlock()
		return &cached
	}
	r.routingMu.RUnlock()

	for _, path := range gatewayRoutingFilePaths {
		data, err := os.ReadFile(path)
		if err == nil && len(data) > 0 {
			var sr SavedGatewayRouting
			if err := json.Unmarshal(data, &sr); err == nil && sr.OTPDevice != "" {
				r.routingMu.Lock()
				r.cachedRouting = &sr
				r.routingMu.Unlock()
				return &sr
			}
		}
	}
	return nil
}

func (r *WhatsAppRepository) saveRoutingToFile(routingMode, otpDevice, alertsDevice, billingDevice, supportDevice string) error {
	sr := SavedGatewayRouting{
		RoutingMode:   routingMode,
		OTPDevice:     otpDevice,
		AlertsDevice:  alertsDevice,
		BillingDevice: billingDevice,
		SupportDevice: supportDevice,
		UpdatedAt:     time.Now().Format(time.RFC3339),
	}

	// 1. Immediately cache in memory (thread-safe, immune to file permissions and DB locks)
	r.routingMu.Lock()
	r.cachedRouting = &sr
	r.routingMu.Unlock()

	// 2. Persist to disk files
	data, err := json.MarshalIndent(sr, "", "  ")
	if err != nil {
		return err
	}

	var lastErr error
	for _, path := range gatewayRoutingFilePaths {
		dir := filepath.Dir(path)
		_ = os.MkdirAll(dir, 0755)
		if err := os.WriteFile(path, data, 0644); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// GetGatewaySettings gets the current connection state and routing configuration of all 4 dedicated WhatsApp devices.
func (r *WhatsAppRepository) GetGatewaySettings(ctx context.Context) (*models.WhatsAppGatewaySettings, error) {
	settings := &models.WhatsAppGatewaySettings{
		ID:            1,
		RoutingMode:   "quad",
		OTPDevice:     "otp",
		AlertsDevice:  "alerts",
		BillingDevice: "billing",
		SupportDevice: "support",
	}

	// 1. Query PostgreSQL first
	query := `
		SELECT id,
		       COALESCE(is_connected, FALSE),
		       COALESCE(connected_phone, ''),
		       COALESCE(connected_name, ''),
		       last_connected_at,
		       COALESCE(daily_messages_sent, 0),
		       COALESCE(last_counter_reset, CURRENT_DATE),
		       COALESCE(otp_connected, FALSE),
		       COALESCE(otp_phone, ''),
		       COALESCE(otp_name, ''),
		       otp_last_connected_at,
		       COALESCE(billing_connected, FALSE),
		       COALESCE(billing_phone, ''),
		       COALESCE(billing_name, ''),
		       billing_last_connected_at,
		       COALESCE(support_connected, FALSE),
		       COALESCE(support_phone, ''),
		       COALESCE(support_name, ''),
		       support_last_connected_at,
		       COALESCE(routing_mode, ''),
		       COALESCE(otp_device, ''),
		       COALESCE(alerts_device, ''),
		       COALESCE(billing_device, ''),
		       COALESCE(support_device, ''),
		       COALESCE(updated_at, NOW())
		FROM whatsapp_gateway_settings
		ORDER BY id ASC
		LIMIT 1`

	var (
		dbRoutingMode, dbOTPDev, dbAlertsDev, dbBillingDev, dbSupportDev string
	)
	err := r.db.QueryRow(ctx, query).Scan(
		&settings.ID, &settings.IsConnected, &settings.ConnectedPhone, &settings.ConnectedName,
		&settings.LastConnectedAt, &settings.DailyMessagesSent, &settings.LastCounterReset,
		&settings.OTPConnected, &settings.OTPPhone, &settings.OTPName, &settings.OTPLastConnected,
		&settings.BillingConnected, &settings.BillingPhone, &settings.BillingName, &settings.BillingLastConnected,
		&settings.SupportConnected, &settings.SupportPhone, &settings.SupportName, &settings.SupportLastConnected,
		&dbRoutingMode, &dbOTPDev, &dbAlertsDev, &dbBillingDev, &dbSupportDev, &settings.UpdatedAt,
	)

	if err == nil {
		if dbRoutingMode != "" {
			settings.RoutingMode = dbRoutingMode
		}
		if dbOTPDev != "" {
			settings.OTPDevice = dbOTPDev
		}
		if dbAlertsDev != "" {
			settings.AlertsDevice = dbAlertsDev
		}
		if dbBillingDev != "" {
			settings.BillingDevice = dbBillingDev
		}
		if dbSupportDev != "" {
			settings.SupportDevice = dbSupportDev
		}
	} else {
		slog.Warn("GetGatewaySettings scan error, using resilient fallback settings", "err", err)
	}

	// 2. Check disk / memory cache
	// If the user has saved custom routing (in memory or file), it takes precedence over initial DB column defaults!
	savedRouting := r.loadRoutingFromFile()
	if savedRouting != nil {
		if savedRouting.RoutingMode != "" {
			settings.RoutingMode = savedRouting.RoutingMode
		}
		if savedRouting.OTPDevice != "" {
			settings.OTPDevice = savedRouting.OTPDevice
		}
		if savedRouting.AlertsDevice != "" {
			settings.AlertsDevice = savedRouting.AlertsDevice
		}
		if savedRouting.BillingDevice != "" {
			settings.BillingDevice = savedRouting.BillingDevice
		}
		if savedRouting.SupportDevice != "" {
			settings.SupportDevice = savedRouting.SupportDevice
		}

		// If DB had different routing or was empty, sync file values into DB asynchronously
		if dbRoutingMode != settings.RoutingMode || dbOTPDev != settings.OTPDevice || dbAlertsDev != settings.AlertsDevice || dbBillingDev != settings.BillingDevice || dbSupportDev != settings.SupportDevice {
			go func() {
				syncCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = r.syncRoutingToDB(syncCtx, settings.RoutingMode, settings.OTPDevice, settings.AlertsDevice, settings.BillingDevice, settings.SupportDevice)
			}()
		}
	} else if dbRoutingMode != "" && dbOTPDev != "" {
		// Cache DB values into memory/file
		_ = r.saveRoutingToFile(settings.RoutingMode, settings.OTPDevice, settings.AlertsDevice, settings.BillingDevice, settings.SupportDevice)
	}

	return settings, nil
}

// UpdateGatewayConnection updates whether a specific WhatsApp device (otp, alerts, billing, support) is connected or disconnected.
func (r *WhatsAppRepository) UpdateGatewayConnection(ctx context.Context, device string, isConnected bool, phone, name string) error {
	var query string
	switch device {
	case "otp":
		query = `
			UPDATE whatsapp_gateway_settings
			SET otp_connected = $1,
			    otp_phone = $2,
			    otp_name = $3,
			    otp_last_connected_at = CASE WHEN $1 = TRUE THEN NOW() ELSE otp_last_connected_at END,
			    updated_at = NOW()
			WHERE id = 1 OR id = (SELECT id FROM whatsapp_gateway_settings ORDER BY id ASC LIMIT 1)`
	case "billing", "payment", "invoice":
		query = `
			UPDATE whatsapp_gateway_settings
			SET billing_connected = $1,
			    billing_phone = $2,
			    billing_name = $3,
			    billing_last_connected_at = CASE WHEN $1 = TRUE THEN NOW() ELSE billing_last_connected_at END,
			    updated_at = NOW()
			WHERE id = 1 OR id = (SELECT id FROM whatsapp_gateway_settings ORDER BY id ASC LIMIT 1)`
	case "support":
		query = `
			UPDATE whatsapp_gateway_settings
			SET support_connected = $1,
			    support_phone = $2,
			    support_name = $3,
			    support_last_connected_at = CASE WHEN $1 = TRUE THEN NOW() ELSE support_last_connected_at END,
			    updated_at = NOW()
			WHERE id = 1 OR id = (SELECT id FROM whatsapp_gateway_settings ORDER BY id ASC LIMIT 1)`
	default: // "alerts"
		query = `
			UPDATE whatsapp_gateway_settings
			SET is_connected = $1,
			    connected_phone = $2,
			    connected_name = $3,
			    last_connected_at = CASE WHEN $1 = TRUE THEN NOW() ELSE last_connected_at END,
			    updated_at = NOW()
			WHERE id = 1 OR id = (SELECT id FROM whatsapp_gateway_settings ORDER BY id ASC LIMIT 1)`
	}
	_, err := r.db.Exec(ctx, query, isConnected, phone, name)
	return err
}

// UpdateGatewayRouting updates the smart routing mode and assigned devices for all 4 channels.
func (r *WhatsAppRepository) UpdateGatewayRouting(ctx context.Context, routingMode, otpDevice, alertsDevice, billingDevice, supportDevice string) error {
	if routingMode == "" {
		routingMode = "quad"
	}
	if otpDevice == "" {
		otpDevice = "otp"
	}
	if alertsDevice == "" {
		alertsDevice = "alerts"
	}
	if billingDevice == "" {
		billingDevice = "billing"
	}
	if supportDevice == "" {
		supportDevice = "support"
	}

	// 1. Immediately cache in memory and write to disk files
	_ = r.saveRoutingToFile(routingMode, otpDevice, alertsDevice, billingDevice, supportDevice)

	// 2. Persist to PostgreSQL database
	return r.syncRoutingToDB(ctx, routingMode, otpDevice, alertsDevice, billingDevice, supportDevice)
}

func (r *WhatsAppRepository) syncRoutingToDB(ctx context.Context, routingMode, otpDevice, alertsDevice, billingDevice, supportDevice string) error {
	// First try updating existing row (regardless of ID)
	updateQuery := `
		UPDATE whatsapp_gateway_settings
		SET routing_mode = $1,
		    otp_device = $2,
		    alerts_device = $3,
		    billing_device = $4,
		    support_device = $5,
		    updated_at = NOW()
		WHERE id = 1 OR id = (SELECT id FROM whatsapp_gateway_settings ORDER BY id ASC LIMIT 1)`

	cmdTag, err := r.db.Exec(ctx, updateQuery, routingMode, otpDevice, alertsDevice, billingDevice, supportDevice)
	if err == nil && cmdTag.RowsAffected() > 0 {
		return nil
	}

	// If no rows were updated, insert row 1
	insertQuery := `
		INSERT INTO whatsapp_gateway_settings (id, routing_mode, otp_device, alerts_device, billing_device, support_device, updated_at)
		VALUES (1, $1, $2, $3, $4, $5, NOW())
		ON CONFLICT (id) DO UPDATE
		SET routing_mode = EXCLUDED.routing_mode,
		    otp_device = EXCLUDED.otp_device,
		    alerts_device = EXCLUDED.alerts_device,
		    billing_device = EXCLUDED.billing_device,
		    support_device = EXCLUDED.support_device,
		    updated_at = NOW()`
	_, err = r.db.Exec(ctx, insertQuery, routingMode, otpDevice, alertsDevice, billingDevice, supportDevice)
	return err
}

// IncrementDailyMessages increments the counter for anti-ban rate limiting.
func (r *WhatsAppRepository) IncrementDailyMessages(ctx context.Context) error {
	query := `
		UPDATE whatsapp_gateway_settings
		SET daily_messages_sent = CASE
		        WHEN last_counter_reset < CURRENT_DATE THEN 1
		        ELSE daily_messages_sent + 1
		    END,
		    last_counter_reset = CURRENT_DATE,
		    updated_at = NOW()
		WHERE id = 1`
	_, err := r.db.Exec(ctx, query)
	return err
}

// GetPortalStats computes stats for the WhatsApp portal.
func (r *WhatsAppRepository) GetPortalStats(ctx context.Context) (*models.WhatsAppPortalStats, error) {
	stats := &models.WhatsAppPortalStats{}

	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM whatsapp_subscribers`).Scan(&stats.TotalSubscribers)
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM whatsapp_subscribers WHERE status = 'active' AND expires_at > NOW()`).Scan(&stats.ActiveSubscribers)
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM whatsapp_subscribers WHERE status = 'paused'`).Scan(&stats.PausedSubscribers)
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM whatsapp_alert_logs WHERE sent_at::date = CURRENT_DATE`).Scan(&stats.AlertsSentToday)
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM whatsapp_alert_logs`).Scan(&stats.TotalAlertsSent)

	return stats, nil
}

// FindMatchingSubscribers finds active subscribers whose criteria match a newly posted job.
func (r *WhatsAppRepository) FindMatchingSubscribers(ctx context.Context, jobTitle, category, location string, salary int) ([]*models.WhatsAppSubscriber, error) {
	query := `
		SELECT id, phone_number, full_name, email, plan_type, plan_price_gbp,
		       status, max_number_changes, number_changes_used, last_number_changed_at,
		       categories, locations, min_salary, alert_mode, notes,
		       created_at, updated_at, expires_at
		FROM whatsapp_subscribers
		WHERE status = 'active'
		  AND expires_at > NOW()
		  AND (min_salary = 0 OR min_salary <= $1)`

	rows, err := r.db.Query(ctx, query, salary)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []*models.WhatsAppSubscriber
	jobTitleLower := strings.ToLower(jobTitle)
	categoryLower := strings.ToLower(category)
	locationLower := strings.ToLower(location)

	for rows.Next() {
		sub := &models.WhatsAppSubscriber{}
		err := rows.Scan(
			&sub.ID, &sub.PhoneNumber, &sub.FullName, &sub.Email, &sub.PlanType, &sub.PlanPriceGBP,
			&sub.Status, &sub.MaxNumberChanges, &sub.NumberChangesUsed, &sub.LastNumberChangedAt,
			&sub.Categories, &sub.Locations, &sub.MinSalary, &sub.AlertMode, &sub.Notes,
			&sub.CreatedAt, &sub.UpdatedAt, &sub.ExpiresAt,
		)
		if err != nil {
			return nil, err
		}

		// Filter by category or title keywords
		categoryMatched := len(sub.Categories) == 0
		if !categoryMatched {
			for _, cat := range sub.Categories {
				catL := strings.ToLower(strings.TrimSpace(cat))
				if catL == "" {
					continue
				}
				if strings.Contains(jobTitleLower, catL) || strings.Contains(categoryLower, catL) {
					categoryMatched = true
					break
				}
			}
		}

		// Filter by location
		locationMatched := len(sub.Locations) == 0
		if !locationMatched {
			for _, loc := range sub.Locations {
				locL := strings.ToLower(strings.TrimSpace(loc))
				if locL == "" {
					continue
				}
				if strings.Contains(locationLower, locL) || strings.Contains(jobTitleLower, locL) {
					locationMatched = true
					break
				}
			}
		}

		if categoryMatched && locationMatched {
			matches = append(matches, sub)
		}
	}

	return matches, nil
}

// HasSubscriberReceivedJob checks if this subscriber was already sent this specific job alert.
func (r *WhatsAppRepository) HasSubscriberReceivedJob(ctx context.Context, subscriberID int, jobID int64) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM whatsapp_alert_logs WHERE subscriber_id = $1 AND job_id = $2 AND status = 'sent')`
	err := r.db.QueryRow(ctx, query, subscriberID, jobID).Scan(&exists)
	return exists, err
}

// GetAlertsSentCountToday returns the number of successfully delivered alerts for this subscriber today.
func (r *WhatsAppRepository) GetAlertsSentCountToday(ctx context.Context, subscriberID int) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM whatsapp_alert_logs WHERE subscriber_id = $1 AND sent_at >= CURRENT_DATE AND status = 'sent'`
	err := r.db.QueryRow(ctx, query, subscriberID).Scan(&count)
	return count, err
}

// ListActiveAlertSubscribers returns all active subscribers whose trial or subscription is valid.
func (r *WhatsAppRepository) ListActiveAlertSubscribers(ctx context.Context) ([]*models.WhatsAppSubscriber, error) {
	query := `
		SELECT id, phone_number, full_name, email, plan_type, plan_price_gbp,
		       status, max_number_changes, number_changes_used, last_number_changed_at,
		       categories, locations, min_salary, alert_mode, notes,
		       created_at, updated_at, expires_at, pin_hash, failed_pin_attempts, locked_until,
		       COALESCE(source, 'portal'), trial_ends_at, COALESCE(payment_status, 'unpaid')
		FROM whatsapp_subscribers
		WHERE status = 'active'
		  AND expires_at > NOW()
		ORDER BY CASE WHEN plan_type = 'trial' OR source = 'homepage_trial' THEN 2 ELSE 1 END, id DESC`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.WhatsAppSubscriber
	for rows.Next() {
		sub := &models.WhatsAppSubscriber{}
		err := rows.Scan(
			&sub.ID, &sub.PhoneNumber, &sub.FullName, &sub.Email, &sub.PlanType, &sub.PlanPriceGBP,
			&sub.Status, &sub.MaxNumberChanges, &sub.NumberChangesUsed, &sub.LastNumberChangedAt,
			&sub.Categories, &sub.Locations, &sub.MinSalary, &sub.AlertMode, &sub.Notes,
			&sub.CreatedAt, &sub.UpdatedAt, &sub.ExpiresAt, &sub.PINHash, &sub.FailedPINAttempts, &sub.LockedUntil,
			&sub.Source, &sub.TrialEndsAt, &sub.PaymentStatus,
		)
		if err != nil {
			return nil, err
		}
		r.enrichSubscriberComputedFields(sub)
		list = append(list, sub)
	}
	return list, nil
}

// FindBestJobForSubscriber finds the single best matching, unsent, active job for this subscriber.
// Priority:
// 1. Matches Category/Role Keywords + Specific Location.
// 2. Fallback: Matches Category/Role Keywords across any UK location.
// 3. Fallback: Matches Location across all categories.
// 4. Fallback: Most recent active UK job.
func (r *WhatsAppRepository) FindBestJobForSubscriber(ctx context.Context, sub *models.WhatsAppSubscriber) (*models.Job, error) {
	baseQuery := `
		SELECT j.id, j.reed_job_id, j.slug, j.title, j.description,
		       j.employer_id, j.location_id, j.location_name,
		       j.min_salary, j.max_salary, j.yearly_min, j.yearly_max,
		       j.currency, j.salary_type, j.contract_type, j.job_type,
		       j.is_graduate, j.is_remote, j.external_url, j.reed_url,
		       j.date_posted, j.expiration_date, j.status,
		       j.created_at, j.updated_at,
		       COALESCE(c.name, '') as employer_name,
		       COALESCE(c.logo_url, '') as company_logo_url,
		       COALESCE(c.slug, '') as company_slug
		FROM jobs j
		LEFT JOIN companies c ON j.employer_id = c.id
		WHERE j.status = 'active'
		  AND NOT EXISTS (
		      SELECT 1 FROM whatsapp_alert_logs l
		      WHERE l.subscriber_id = $1 AND l.job_id = j.id AND l.status = 'sent'
		  )`

	// Clean category keyword
	catKeyword := ""
	if len(sub.Categories) > 0 {
		for _, c := range sub.Categories {
			clean := strings.TrimSpace(c)
			if clean != "" && !strings.EqualFold(clean, "All Categories") && !strings.EqualFold(clean, "all") {
				catKeyword = clean
				break
			}
		}
	}

	// Clean location keyword
	locKeyword := ""
	if len(sub.Locations) > 0 {
		for _, l := range sub.Locations {
			clean := strings.TrimSpace(l)
			if clean != "" && !strings.EqualFold(clean, "All UK Locations") && !strings.EqualFold(clean, "all") && !strings.EqualFold(clean, "any") {
				locKeyword = clean
				break
			}
		}
	}

	// Helper to scan a single job row
	scanJob := func(row pgx.Row) (*models.Job, error) {
		job := &models.Job{}
		err := row.Scan(
			&job.ID, &job.ReedJobID, &job.Slug, &job.Title, &job.Description,
			&job.EmployerID, &job.LocationID, &job.LocationName,
			&job.MinSalary, &job.MaxSalary, &job.YearlyMin, &job.YearlyMax,
			&job.Currency, &job.SalaryType, &job.ContractType, &job.JobType,
			&job.IsGraduate, &job.IsRemote, &job.ExternalURL, &job.ReedURL,
			&job.DatePosted, &job.ExpirationDate, &job.Status,
			&job.CreatedAt, &job.UpdatedAt, &job.EmployerName, &job.CompanyLogoURLValue, &job.CompanySlug,
		)
		if err != nil {
			return nil, err
		}
		return job, nil
	}

	// Attempt 1: Strict match (Role + Location)
	if catKeyword != "" && locKeyword != "" {
		q1 := baseQuery + `
			AND (j.title ILIKE $2 OR j.description ILIKE $2)
			AND (j.location_name ILIKE $3 OR j.title ILIKE $3)
			ORDER BY j.date_posted DESC LIMIT 1`
		row := r.db.QueryRow(ctx, q1, sub.ID, "%"+catKeyword+"%", "%"+locKeyword+"%")
		if job, err := scanJob(row); err == nil && job != nil {
			return job, nil
		}
	}

	// Attempt 2: Match Role/Category across UK (if location didn't match or was wild)
	if catKeyword != "" {
		q2 := baseQuery + `
			AND (j.title ILIKE $2 OR j.description ILIKE $2)
			ORDER BY j.date_posted DESC LIMIT 1`
		row := r.db.QueryRow(ctx, q2, sub.ID, "%"+catKeyword+"%")
		if job, err := scanJob(row); err == nil && job != nil {
			return job, nil
		}
	}

	// Attempt 3: Match Location across roles (if location was specified)
	if locKeyword != "" {
		q3 := baseQuery + `
			AND (j.location_name ILIKE $2 OR j.title ILIKE $2)
			ORDER BY j.date_posted DESC LIMIT 1`
		row := r.db.QueryRow(ctx, q3, sub.ID, "%"+locKeyword+"%")
		if job, err := scanJob(row); err == nil && job != nil {
			return job, nil
		}
	}

	// Attempt 4: Fallback to most recent unsent active UK job
	q4 := baseQuery + ` ORDER BY j.date_posted DESC LIMIT 1`
	row := r.db.QueryRow(ctx, q4, sub.ID)
	return scanJob(row)
}

// LogSubscriberLogin records a login/security attempt for audit and subscriber awareness.
func (r *WhatsAppRepository) LogSubscriberLogin(ctx context.Context, subID int, phone, ip, userAgent, action, status string) error {
	query := `
		INSERT INTO whatsapp_subscriber_logins (subscriber_id, phone_number, ip_address, user_agent, action, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.Exec(ctx, query, subID, phone, ip, userAgent, action, status)
	return err
}

// GetSubscriberLoginLogs retrieves the most recent login logs for a subscriber.
func (r *WhatsAppRepository) GetSubscriberLoginLogs(ctx context.Context, subID int, limit int) ([]models.SubscriberLoginLog, error) {
	if limit <= 0 {
		limit = 10
	}
	query := `
		SELECT id, subscriber_id, phone_number, ip_address, user_agent, action, status, created_at
		FROM whatsapp_subscriber_logins
		WHERE subscriber_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`
	rows, err := r.db.Query(ctx, query, subID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.SubscriberLoginLog
	for rows.Next() {
		var l models.SubscriberLoginLog
		if err := rows.Scan(&l.ID, &l.SubscriberID, &l.PhoneNumber, &l.IPAddress, &l.UserAgent, &l.Action, &l.Status, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, nil
}

// GetActiveCryptoWallets returns all active deposit wallets configured by admin.
func (r *WhatsAppRepository) GetActiveCryptoWallets(ctx context.Context) ([]models.WhatsAppCryptoWallet, error) {
	query := `
		SELECT id, coin, network, symbol, address, instructions, is_active, created_at, updated_at
		FROM whatsapp_crypto_wallets
		WHERE is_active = TRUE
		ORDER BY id ASC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var wallets []models.WhatsAppCryptoWallet
	for rows.Next() {
		var w models.WhatsAppCryptoWallet
		if err := rows.Scan(&w.ID, &w.Coin, &w.Network, &w.Symbol, &w.Address, &w.Instructions, &w.IsActive, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		wallets = append(wallets, w)
	}
	return wallets, nil
}

// GetAllCryptoWallets returns all deposit wallets for admin settings.
func (r *WhatsAppRepository) GetAllCryptoWallets(ctx context.Context) ([]models.WhatsAppCryptoWallet, error) {
	query := `
		SELECT id, coin, network, symbol, address, instructions, is_active, created_at, updated_at
		FROM whatsapp_crypto_wallets
		ORDER BY id ASC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var wallets []models.WhatsAppCryptoWallet
	for rows.Next() {
		var w models.WhatsAppCryptoWallet
		if err := rows.Scan(&w.ID, &w.Coin, &w.Network, &w.Symbol, &w.Address, &w.Instructions, &w.IsActive, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		wallets = append(wallets, w)
	}
	return wallets, nil
}

// UpdateCryptoWallet updates an admin deposit wallet address or instructions.
func (r *WhatsAppRepository) UpdateCryptoWallet(ctx context.Context, id int, address, instructions string, isActive bool) error {
	query := `
		UPDATE whatsapp_crypto_wallets
		SET address = $1, instructions = $2, is_active = $3, updated_at = NOW()
		WHERE id = $4
	`
	_, err := r.db.Exec(ctx, query, strings.TrimSpace(address), strings.TrimSpace(instructions), isActive, id)
	return err
}

// CreateCryptoPayment records a new submitted payment proof.
func (r *WhatsAppRepository) CreateCryptoPayment(ctx context.Context, p *models.WhatsAppCryptoPayment) error {
	query := `
		INSERT INTO whatsapp_crypto_payments (
			subscriber_id, phone_number, full_name, plan_type, amount_gbp, 
			coin, network, deposit_address, tx_hash, sender_address, 
			screenshot_url, status, admin_notes
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'pending', $12)
		RETURNING id, created_at
	`
	return r.db.QueryRow(ctx, query,
		p.SubscriberID, p.PhoneNumber, p.FullName, p.PlanType, p.AmountGBP,
		p.Coin, p.Network, p.DepositAddress, p.TxHash, p.SenderAddress,
		p.ScreenshotURL, p.AdminNotes,
	).Scan(&p.ID, &p.CreatedAt)
}

// ListCryptoPayments lists crypto payments with pagination and status filter.
func (r *WhatsAppRepository) ListCryptoPayments(ctx context.Context, status string, limit, offset int) ([]models.WhatsAppCryptoPayment, int, error) {
	if limit <= 0 {
		limit = 20
	}
	whereClause := ""
	var args []interface{}
	argIdx := 1

	if status != "" && status != "all" {
		whereClause = fmt.Sprintf("WHERE status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM whatsapp_crypto_payments %s", whereClause)
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`
		SELECT id, subscriber_id, phone_number, full_name, plan_type, amount_gbp, 
		       coin, network, deposit_address, tx_hash, sender_address, 
		       screenshot_url, status, admin_notes, created_at, verified_at
		FROM whatsapp_crypto_payments
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var payments []models.WhatsAppCryptoPayment
	for rows.Next() {
		var p models.WhatsAppCryptoPayment
		if err := rows.Scan(
			&p.ID, &p.SubscriberID, &p.PhoneNumber, &p.FullName, &p.PlanType, &p.AmountGBP,
			&p.Coin, &p.Network, &p.DepositAddress, &p.TxHash, &p.SenderAddress,
			&p.ScreenshotURL, &p.Status, &p.AdminNotes, &p.CreatedAt, &p.VerifiedAt,
		); err != nil {
			return nil, 0, err
		}
		payments = append(payments, p)
	}
	return payments, total, nil
}

// GetCryptoPaymentByID retrieves a single crypto payment.
func (r *WhatsAppRepository) GetCryptoPaymentByID(ctx context.Context, id int) (*models.WhatsAppCryptoPayment, error) {
	query := `
		SELECT id, subscriber_id, phone_number, full_name, plan_type, amount_gbp, 
		       coin, network, deposit_address, tx_hash, sender_address, 
		       screenshot_url, status, admin_notes, created_at, verified_at
		FROM whatsapp_crypto_payments
		WHERE id = $1
	`
	var p models.WhatsAppCryptoPayment
	err := r.db.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.SubscriberID, &p.PhoneNumber, &p.FullName, &p.PlanType, &p.AmountGBP,
		&p.Coin, &p.Network, &p.DepositAddress, &p.TxHash, &p.SenderAddress,
		&p.ScreenshotURL, &p.Status, &p.AdminNotes, &p.CreatedAt, &p.VerifiedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// VerifyCryptoPayment marks a payment verified and unlocks the subscriber's plan.
func (r *WhatsAppRepository) VerifyCryptoPayment(ctx context.Context, id int, adminNotes string) (*models.WhatsAppCryptoPayment, error) {
	p, err := r.GetCryptoPaymentByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Upgrade subscriber plan
	if err := r.UpgradeSubscription(ctx, p.SubscriberID, p.PlanType); err != nil {
		return nil, fmt.Errorf("failed to activate subscription: %w", err)
	}

	now := time.Now()
	query := `
		UPDATE whatsapp_crypto_payments
		SET status = 'verified', admin_notes = $1, verified_at = $2
		WHERE id = $3
	`
	if _, err := r.db.Exec(ctx, query, adminNotes, now, id); err != nil {
		return nil, err
	}

	p.Status = "verified"
	p.AdminNotes = adminNotes
	p.VerifiedAt = &now

	_ = r.LogTransaction(ctx, &p.SubscriberID, p.PhoneNumber, p.FullName, p.PlanType, p.AmountGBP, "crypto", p.TxHash, "verified", adminNotes)
	return p, nil
}

// RejectCryptoPayment marks a payment rejected.
func (r *WhatsAppRepository) RejectCryptoPayment(ctx context.Context, id int, adminNotes string) (*models.WhatsAppCryptoPayment, error) {
	p, err := r.GetCryptoPaymentByID(ctx, id)
	if err != nil {
		return nil, err
	}

	query := `
		UPDATE whatsapp_crypto_payments
		SET status = 'rejected', admin_notes = $1
		WHERE id = $2
	`
	if _, err := r.db.Exec(ctx, query, adminNotes, id); err != nil {
		return nil, err
	}

	p.Status = "rejected"
	p.AdminNotes = adminNotes
	return p, nil
}

// GetPaymentSettings retrieves the active payment gateway configuration.
func (r *WhatsAppRepository) GetPaymentSettings(ctx context.Context) (*models.WhatsAppPaymentSettings, error) {
	query := `
		SELECT id, paypal_handle, paypal_instructions, paypal_active, card_active, crypto_active,
		       support_active, support_number, support_instructions, updated_at
		FROM whatsapp_payment_settings
		WHERE id = 1
	`
	var s models.WhatsAppPaymentSettings
	err := r.db.QueryRow(ctx, query).Scan(
		&s.ID, &s.PayPalHandle, &s.PayPalInstructions, &s.PayPalActive, &s.CardActive, &s.CryptoActive,
		&s.SupportActive, &s.SupportNumber, &s.SupportInstructions, &s.UpdatedAt,
	)
	if err != nil {
		// Fallback defaults if table was just created
		return &models.WhatsAppPaymentSettings{
			ID:                  1,
			PayPalHandle:        "jobyuk",
			PayPalInstructions:  "Pay securely via your PayPal account balance, linked UK bank account, or debit/credit card.",
			PayPalActive:        true,
			CardActive:          true,
			CryptoActive:        true,
			SupportActive:       true,
			SupportNumber:       "+923025015693",
			SupportInstructions: "Connect directly with our billing support concierge to receive bespoke UK bank transfer details, Wise invoice, or alternative payment rails.",
			UpdatedAt:           time.Now(),
		}, nil
	}
	return &s, nil
}

// UpdatePaymentSettings updates the payment gateway configuration (PayPal handle, active states, etc.).
func (r *WhatsAppRepository) UpdatePaymentSettings(ctx context.Context, paypalHandle, paypalInstructions string, paypalActive, cardActive, cryptoActive, supportActive bool, supportNumber, supportInstructions string) error {
	query := `
		INSERT INTO whatsapp_payment_settings (id, paypal_handle, paypal_instructions, paypal_active, card_active, crypto_active, support_active, support_number, support_instructions, updated_at)
		VALUES (1, $1, $2, $3, $4, $5, $6, $7, $8, NOW())
		ON CONFLICT (id) DO UPDATE
		SET paypal_handle = EXCLUDED.paypal_handle,
		    paypal_instructions = EXCLUDED.paypal_instructions,
		    paypal_active = EXCLUDED.paypal_active,
		    card_active = EXCLUDED.card_active,
		    crypto_active = EXCLUDED.crypto_active,
		    support_active = EXCLUDED.support_active,
		    support_number = EXCLUDED.support_number,
		    support_instructions = EXCLUDED.support_instructions,
		    updated_at = NOW()
	`
	_, err := r.db.Exec(ctx, query, paypalHandle, paypalInstructions, paypalActive, cardActive, cryptoActive, supportActive, supportNumber, supportInstructions)
	return err
}

// LogTransaction records a completed or pending transaction across all payment methods.
func (r *WhatsAppRepository) LogTransaction(ctx context.Context, subID *int, phone, fullName, planType string, amountGBP float64, method, txRef, status, notes string) error {
	query := `
		INSERT INTO whatsapp_transactions (subscriber_id, phone_number, full_name, plan_type, amount_gbp, method, tx_ref, status, notes, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
	`
	_, err := r.db.Exec(ctx, query, subID, phone, fullName, planType, amountGBP, method, txRef, status, notes)
	return err
}

// ListTransactions retrieves recent transactions for the admin audit log.
func (r *WhatsAppRepository) ListTransactions(ctx context.Context, limit, offset int) ([]models.WhatsAppTransaction, int, error) {
	var total int
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM whatsapp_transactions`).Scan(&total)

	query := `
		SELECT id, subscriber_id, phone_number, full_name, plan_type, amount_gbp, method, tx_ref, status, notes, created_at
		FROM whatsapp_transactions
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []models.WhatsAppTransaction
	for rows.Next() {
		var t models.WhatsAppTransaction
		if err := rows.Scan(
			&t.ID, &t.SubscriberID, &t.PhoneNumber, &t.FullName, &t.PlanType,
			&t.AmountGBP, &t.Method, &t.TxRef, &t.Status, &t.Notes, &t.CreatedAt,
		); err != nil {
			continue
		}
		list = append(list, t)
	}
	return list, total, nil
}

// -------------------------------------------------------------------------
// WhatsApp Trial Analytics & Conversion Funnel Engine
// -------------------------------------------------------------------------

func buildAnalyticsWhereClause(filter models.WhatsAppAnalyticsFilter) (string, []interface{}) {
	var conditions []string
	var args []interface{}
	idx := 1

	now := time.Now()

	switch strings.ToLower(filter.Period) {
	case "today":
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", idx))
		args = append(args, startOfDay)
		idx++
	case "yesterday":
		yesterday := now.AddDate(0, 0, -1)
		startOfYesterday := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, now.Location())
		endOfYesterday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d AND created_at < $%d", idx, idx+1))
		args = append(args, startOfYesterday, endOfYesterday)
		idx += 2
	case "week":
		startOfWeek := now.AddDate(0, 0, -7)
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", idx))
		args = append(args, startOfWeek)
		idx++
	case "month":
		startOfMonth := now.AddDate(0, 0, -30)
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", idx))
		args = append(args, startOfMonth)
		idx++
	case "custom":
		if filter.FromDate != nil {
			conditions = append(conditions, fmt.Sprintf("created_at >= $%d", idx))
			args = append(args, *filter.FromDate)
			idx++
		}
		if filter.ToDate != nil {
			conditions = append(conditions, fmt.Sprintf("created_at <= $%d", idx))
			args = append(args, *filter.ToDate)
			idx++
		}
	}

	// Always exclude internal admin, client portal, and management dashboards
	conditions = append(conditions, "(page_url IS NULL OR (page_url NOT LIKE '%/wa-portal%' AND page_url NOT LIKE '%/my-alerts%' AND page_url NOT LIKE '%/wa-client%' AND page_url NOT LIKE '%/admin%' AND page_url NOT LIKE '%/pin-%'))")

	if len(conditions) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

// LogAnalyticsClick records an initial trial CTA click.
func (r *WhatsAppRepository) LogAnalyticsClick(ctx context.Context, ev *models.WhatsAppAnalyticsEvent) (int, error) {
	query := `
		INSERT INTO whatsapp_analytics_events (
			visitor_id, source, aria_label, trigger_placement, page_url, referrer,
			ip_address, location, user_agent, device_type, browser, os,
			phone_number, step, otp_sent, otp_verified, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, 'clicked', false, false, NOW(), NOW())
		RETURNING id
	`
	var newID int
	err := r.db.QueryRow(ctx, query,
		ev.VisitorID, ev.Source, ev.AriaLabel, ev.TriggerPlacement, ev.PageURL, ev.Referrer,
		ev.IPAddress, ev.Location, ev.UserAgent, ev.DeviceType, ev.Browser, ev.OS,
		ev.PhoneNumber,
	).Scan(&newID)
	return newID, err
}

// UpdateAnalyticsPhoneAndOTP marks that the visitor entered a phone and OTP was dispatched.
func (r *WhatsAppRepository) UpdateAnalyticsPhoneAndOTP(ctx context.Context, trackingID int, visitorID, phone string) error {
	var query string
	var err error
	if trackingID > 0 {
		query = `
			UPDATE whatsapp_analytics_events
			SET phone_number = $1, step = 'otp_sent', otp_sent = true, updated_at = NOW()
			WHERE id = $2
		`
		_, err = r.db.Exec(ctx, query, phone, trackingID)
	} else if visitorID != "" {
		query = `
			UPDATE whatsapp_analytics_events
			SET phone_number = $1, step = 'otp_sent', otp_sent = true, updated_at = NOW()
			WHERE id = (
				SELECT id FROM whatsapp_analytics_events
				WHERE visitor_id = $2
				ORDER BY created_at DESC
				LIMIT 1
			)
		`
		_, err = r.db.Exec(ctx, query, phone, visitorID)
	}
	return err
}

// UpdateAnalyticsVerified marks that the visitor successfully verified OTP and got saved.
func (r *WhatsAppRepository) UpdateAnalyticsVerified(ctx context.Context, trackingID int, visitorID, phone string, subscriberID int) error {
	var query string
	var err error
	if trackingID > 0 {
		query = `
			UPDATE whatsapp_analytics_events
			SET step = 'verified', otp_verified = true, subscriber_id = $1, updated_at = NOW()
			WHERE id = $2
		`
		_, err = r.db.Exec(ctx, query, subscriberID, trackingID)
	} else if visitorID != "" {
		query = `
			UPDATE whatsapp_analytics_events
			SET step = 'verified', otp_verified = true, subscriber_id = $1, updated_at = NOW()
			WHERE id = (
				SELECT id FROM whatsapp_analytics_events
				WHERE visitor_id = $2
				ORDER BY created_at DESC
				LIMIT 1
			)
		`
		_, err = r.db.Exec(ctx, query, subscriberID, visitorID)
	} else if phone != "" {
		query = `
			UPDATE whatsapp_analytics_events
			SET step = 'verified', otp_verified = true, subscriber_id = $1, updated_at = NOW()
			WHERE id = (
				SELECT id FROM whatsapp_analytics_events
				WHERE phone_number = $2
				ORDER BY created_at DESC
				LIMIT 1
			)
		`
		_, err = r.db.Exec(ctx, query, subscriberID, phone)
	}
	return err
}

// GetAnalyticsSummary returns high level KPI metrics and funnel conversion rates.
func (r *WhatsAppRepository) GetAnalyticsSummary(ctx context.Context, filter models.WhatsAppAnalyticsFilter) (models.WhatsAppAnalyticsSummary, error) {
	whereClause, args := buildAnalyticsWhereClause(filter)

	query := fmt.Sprintf(`
		SELECT
			COALESCE(COUNT(*), 0) AS total_clicks,
			COALESCE(COUNT(DISTINCT visitor_id), 0) AS unique_visitors,
			COALESCE(COUNT(*) FILTER (WHERE otp_sent = true), 0) AS otp_sent_count,
			COALESCE(COUNT(*) FILTER (WHERE otp_verified = true), 0) AS verified_count
		FROM whatsapp_analytics_events
		%s
	`, whereClause)

	var s models.WhatsAppAnalyticsSummary
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&s.TotalClicks,
		&s.UniqueVisitors,
		&s.OTPSentCount,
		&s.VerifiedCount,
	)
	if err != nil {
		return s, err
	}

	if s.TotalClicks > 0 {
		s.ClickToOTPConvRate = float64(s.OTPSentCount) / float64(s.TotalClicks) * 100.0
		s.ClickToVerifiedRate = float64(s.VerifiedCount) / float64(s.TotalClicks) * 100.0
	}
	if s.OTPSentCount > 0 {
		s.OTPToVerifiedRate = float64(s.VerifiedCount) / float64(s.OTPSentCount) * 100.0
	}

	return s, nil
}

// GetAnalyticsSources returns breakdown by trigger source and aria-label.
func (r *WhatsAppRepository) GetAnalyticsSources(ctx context.Context, filter models.WhatsAppAnalyticsFilter) ([]models.WhatsAppSourceStat, error) {
	whereClause, args := buildAnalyticsWhereClause(filter)

	query := fmt.Sprintf(`
		SELECT
			source,
			COALESCE(aria_label, '') AS aria_label,
			COALESCE(trigger_placement, '') AS trigger_placement,
			COUNT(*) AS clicks,
			COUNT(DISTINCT visitor_id) AS unique_visitors,
			COUNT(*) FILTER (WHERE otp_sent = true) AS otp_sent,
			COUNT(*) FILTER (WHERE otp_verified = true) AS verified
		FROM whatsapp_analytics_events
		%s
		GROUP BY source, aria_label, trigger_placement
		ORDER BY clicks DESC
	`, whereClause)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []models.WhatsAppSourceStat
	for rows.Next() {
		var item models.WhatsAppSourceStat
		if err := rows.Scan(
			&item.Source, &item.AriaLabel, &item.TriggerPlacement,
			&item.Clicks, &item.UniqueVisitors, &item.OTPSent, &item.Verified,
		); err != nil {
			continue
		}
		if item.Clicks > 0 {
			item.ConvRate = float64(item.Verified) / float64(item.Clicks) * 100.0
		}
		stats = append(stats, item)
	}
	return stats, nil
}

// GetAnalyticsDeviceStats returns distribution of devices.
func (r *WhatsAppRepository) GetAnalyticsDeviceStats(ctx context.Context, filter models.WhatsAppAnalyticsFilter) ([]models.WhatsAppDeviceStat, error) {
	whereClause, args := buildAnalyticsWhereClause(filter)

	query := fmt.Sprintf(`
		SELECT
			COALESCE(NULLIF(device_type, ''), 'desktop') AS device_type,
			COUNT(*) AS cnt
		FROM whatsapp_analytics_events
		%s
		GROUP BY device_type
		ORDER BY cnt DESC
	`, whereClause)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.WhatsAppDeviceStat
	total := 0
	for rows.Next() {
		var s models.WhatsAppDeviceStat
		if err := rows.Scan(&s.DeviceType, &s.Count); err == nil {
			total += s.Count
			list = append(list, s)
		}
	}
	if total > 0 {
		for i := range list {
			list[i].Percentage = float64(list[i].Count) / float64(total) * 100.0
		}
	}
	return list, nil
}

// ListAnalyticsEvents returns paginated activity log events.
func (r *WhatsAppRepository) ListAnalyticsEvents(ctx context.Context, filter models.WhatsAppAnalyticsFilter, limit, offset int) ([]models.WhatsAppAnalyticsEvent, int, error) {
	whereClause, args := buildAnalyticsWhereClause(filter)

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM whatsapp_analytics_events %s`, whereClause)
	var total int
	_ = r.db.QueryRow(ctx, countQuery, args...).Scan(&total)

	queryArgs := append(args, limit, offset)
	query := fmt.Sprintf(`
		SELECT
			id, visitor_id, source, aria_label, trigger_placement, page_url, referrer,
			ip_address, location, user_agent, device_type, browser, os,
			phone_number, step, otp_sent, otp_verified, subscriber_id,
			created_at, updated_at
		FROM whatsapp_analytics_events
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, len(args)+1, len(args)+2)

	rows, err := r.db.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var events []models.WhatsAppAnalyticsEvent
	for rows.Next() {
		var ev models.WhatsAppAnalyticsEvent
		if err := rows.Scan(
			&ev.ID, &ev.VisitorID, &ev.Source, &ev.AriaLabel, &ev.TriggerPlacement, &ev.PageURL, &ev.Referrer,
			&ev.IPAddress, &ev.Location, &ev.UserAgent, &ev.DeviceType, &ev.Browser, &ev.OS,
			&ev.PhoneNumber, &ev.Step, &ev.OTPSent, &ev.OTPVerified, &ev.SubscriberID,
			&ev.CreatedAt, &ev.UpdatedAt,
		); err != nil {
			continue
		}
		events = append(events, ev)
	}
	return events, total, nil
}

// ResetAnalytics permanently truncates or clears all analytics logs.
func (r *WhatsAppRepository) ResetAnalytics(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `TRUNCATE TABLE whatsapp_analytics_events RESTART IDENTITY;`)
	return err
}

// ============================================================================
// WHATSAPP CHANNEL (NEWSLETTER) BROADCAST REPOSITORY METHODS
// ============================================================================

// GetChannelPostsTodayCount returns the number of jobs posted to the WhatsApp channel today (UTC).
func (r *WhatsAppRepository) GetChannelPostsTodayCount(ctx context.Context) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM whatsapp_channel_posts WHERE status = 'posted' AND posted_at >= CURRENT_DATE;`
	err := r.db.QueryRow(ctx, query).Scan(&count)
	return count, err
}

// IsJobPostedToChannel checks whether a specific job has already been posted to the WhatsApp Channel.
func (r *WhatsAppRepository) IsJobPostedToChannel(ctx context.Context, jobID int64) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM whatsapp_channel_posts WHERE job_id = $1 AND status = 'posted');`
	err := r.db.QueryRow(ctx, query, jobID).Scan(&exists)
	return exists, err
}

// LogChannelPost records a newly dispatched or failed post to the channel table.
func (r *WhatsAppRepository) LogChannelPost(ctx context.Context, post *models.WhatsAppChannelPost) error {
	query := `
		INSERT INTO whatsapp_channel_posts (
			job_id, channel_invite, channel_jid, job_title, company,
			location, salary_text, status, error_message, posted_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		ON CONFLICT (job_id) DO UPDATE SET
			status = EXCLUDED.status,
			error_message = EXCLUDED.error_message,
			posted_at = NOW();
	`
	_, err := r.db.Exec(ctx, query,
		post.JobID,
		post.ChannelInvite,
		post.ChannelJID,
		post.JobTitle,
		post.Company,
		post.Location,
		post.SalaryText,
		post.Status,
		post.ErrorMessage,
	)
	return err
}

// ListChannelPosts returns the most recent posts broadcast to the channel.
func (r *WhatsAppRepository) ListChannelPosts(ctx context.Context, limit int) ([]models.WhatsAppChannelPost, error) {
	if limit <= 0 {
		limit = 20
	}
	query := `
		SELECT p.id, p.job_id, p.channel_invite, p.channel_jid, p.job_title,
		       p.company, p.location, p.salary_text, p.status, p.error_message,
		       p.posted_at, COALESCE(j.slug, '') AS job_slug
		FROM whatsapp_channel_posts p
		LEFT JOIN jobs j ON p.job_id = j.id
		ORDER BY p.posted_at DESC
		LIMIT $1;
	`
	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []models.WhatsAppChannelPost
	for rows.Next() {
		var p models.WhatsAppChannelPost
		if err := rows.Scan(
			&p.ID, &p.JobID, &p.ChannelInvite, &p.ChannelJID, &p.JobTitle,
			&p.Company, &p.Location, &p.SalaryText, &p.Status, &p.ErrorMessage,
			&p.PostedAt, &p.JobSlug,
		); err != nil {
			continue
		}
		posts = append(posts, p)
	}
	return posts, nil
}

// FindBestHighCTRJobForChannel selects the top active, unposted, high-attractiveness UK job for channel broadcasting.
// Prioritizes transparent salary and popular roles to maximize click-through rate (CTR).
func (r *WhatsAppRepository) FindBestHighCTRJobForChannel(ctx context.Context) (*models.Job, error) {
	query := `
		SELECT j.id, j.reed_job_id, j.source, j.external_slug, j.slug, j.title, j.description,
		       j.employer_id, j.location_id, j.location_name,
		       j.min_salary, j.max_salary, j.yearly_min, j.yearly_max,
		       j.currency, j.salary_type, j.contract_type, j.job_type,
		       j.is_graduate, j.is_remote, j.external_url, j.reed_url,
		       j.date_posted, j.expiration_date, j.status,
		       j.created_at, j.updated_at,
		       COALESCE(c.name, '') as employer_name,
		       COALESCE(c.logo_url, '') as company_logo_url,
		       COALESCE(c.slug, '') as company_slug
		FROM jobs j
		LEFT JOIN companies c ON j.employer_id = c.id
		WHERE j.status = 'active'
		  AND (j.expiration_date IS NULL OR j.expiration_date > NOW())
		  AND j.id NOT IN (SELECT job_id FROM whatsapp_channel_posts WHERE status = 'posted')
		ORDER BY
		  -- 1. Transparent high salary listings have 3.4x higher CTR
		  CASE WHEN (COALESCE(j.yearly_max, 0) > 0 OR COALESCE(j.max_salary, 0) > 0 OR COALESCE(j.min_salary, 0) > 0) THEN 1 ELSE 0 END DESC,
		  -- 2. Remote / Hybrid opportunities have higher engagement
		  CASE WHEN j.is_remote = TRUE THEN 1 ELSE 0 END DESC,
		  -- 3. Freshness
		  j.date_posted DESC,
		  j.created_at DESC
		LIMIT 1;
	`
	job := &models.Job{}
	err := r.db.QueryRow(ctx, query).Scan(
		&job.ID, &job.ReedJobID, &job.Source, &job.ExternalSlug, &job.Slug, &job.Title, &job.Description,
		&job.EmployerID, &job.LocationID, &job.LocationName,
		&job.MinSalary, &job.MaxSalary, &job.YearlyMin, &job.YearlyMax,
		&job.Currency, &job.SalaryType, &job.ContractType, &job.JobType,
		&job.IsGraduate, &job.IsRemote, &job.ExternalURL, &job.ReedURL,
		&job.DatePosted, &job.ExpirationDate, &job.Status,
		&job.CreatedAt, &job.UpdatedAt,
		&job.EmployerName, &job.CompanyLogoURLValue, &job.CompanySlug,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return job, nil
}

// GetChannelStats returns aggregated stats for the WhatsApp Channel.
func (r *WhatsAppRepository) GetChannelStats(ctx context.Context) (*models.WhatsAppChannelStats, error) {
	stats := &models.WhatsAppChannelStats{
		ChannelInvite:    "0029Vb9iXUMAu3aQOlanBy2m",
		ChannelURL:       "https://whatsapp.com/channel/0029Vb9iXUMAu3aQOlanBy2m",
		ChannelInviteURL: "https://whatsapp.com/channel/0029Vb9iXUMAu3aQOlanBy2m",
		DailyMax:         20,
	}

	todayCount, err := r.GetChannelPostsTodayCount(ctx)
	if err == nil {
		stats.TodayCount = todayCount
	}

	var total int
	var lastPosted *time.Time
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*), MAX(posted_at) FROM whatsapp_channel_posts WHERE status = 'posted';`).Scan(&total, &lastPosted)
	stats.TotalPosts = total
	stats.LastPostedAt = lastPosted

	return stats, nil
}


