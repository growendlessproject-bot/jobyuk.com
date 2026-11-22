-- Migration 009: Dedicated WhatsApp Job Alerts & Subscriber Engine
CREATE TABLE IF NOT EXISTS whatsapp_subscribers (
    id SERIAL PRIMARY KEY,
    phone_number VARCHAR(30) UNIQUE NOT NULL,
    full_name VARCHAR(150) NOT NULL,
    email VARCHAR(255),
    plan_type VARCHAR(30) NOT NULL DEFAULT 'monthly', -- 'monthly', '6months', 'yearly', 'admin_pass'
    plan_price_gbp NUMERIC(8,2) DEFAULT 9.99,
    status VARCHAR(20) NOT NULL DEFAULT 'active', -- 'active', 'paused', 'expired', 'cancelled'
    max_number_changes INT NOT NULL DEFAULT 1,
    number_changes_used INT NOT NULL DEFAULT 0,
    last_number_changed_at TIMESTAMPTZ,
    categories TEXT[] DEFAULT '{}',
    locations TEXT[] DEFAULT '{}',
    min_salary INT DEFAULT 0,
    alert_mode VARCHAR(20) DEFAULT 'instant', -- 'instant', 'daily_digest'
    notes TEXT DEFAULT '',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ DEFAULT (NOW() + INTERVAL '30 days')
);

CREATE INDEX IF NOT EXISTS idx_wa_subscribers_phone ON whatsapp_subscribers(phone_number);
CREATE INDEX IF NOT EXISTS idx_wa_subscribers_status ON whatsapp_subscribers(status);

CREATE TABLE IF NOT EXISTS whatsapp_number_history (
    id SERIAL PRIMARY KEY,
    subscriber_id INT REFERENCES whatsapp_subscribers(id) ON DELETE CASCADE,
    old_phone VARCHAR(30) NOT NULL,
    new_phone VARCHAR(30) NOT NULL,
    changed_at TIMESTAMPTZ DEFAULT NOW(),
    ip_address VARCHAR(45) DEFAULT ''
);

CREATE TABLE IF NOT EXISTS whatsapp_otps (
    phone_number VARCHAR(30) PRIMARY KEY,
    otp_code VARCHAR(6) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    attempts INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS whatsapp_alert_logs (
    id SERIAL PRIMARY KEY,
    subscriber_id INT REFERENCES whatsapp_subscribers(id) ON DELETE SET NULL,
    phone_number VARCHAR(30) NOT NULL,
    job_id BIGINT REFERENCES jobs(id) ON DELETE SET NULL,
    job_title VARCHAR(255) NOT NULL,
    company VARCHAR(255) DEFAULT '',
    location VARCHAR(255) DEFAULT '',
    salary_text VARCHAR(100) DEFAULT '',
    status VARCHAR(20) DEFAULT 'sent', -- 'sent', 'delivered', 'failed'
    error_message TEXT DEFAULT '',
    sent_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_wa_alert_logs_sub_id ON whatsapp_alert_logs(subscriber_id);
CREATE INDEX IF NOT EXISTS idx_wa_alert_logs_sent_at ON whatsapp_alert_logs(sent_at DESC);

CREATE TABLE IF NOT EXISTS whatsapp_gateway_settings (
    id INT PRIMARY KEY DEFAULT 1,
    is_connected BOOLEAN DEFAULT FALSE,
    connected_phone VARCHAR(30) DEFAULT '',
    connected_name VARCHAR(100) DEFAULT '',
    last_connected_at TIMESTAMPTZ,
    daily_messages_sent INT DEFAULT 0,
    last_counter_reset DATE DEFAULT CURRENT_DATE,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Insert single row for gateway status if not present
INSERT INTO whatsapp_gateway_settings (id, is_connected)
VALUES (1, FALSE)
ON CONFLICT (id) DO NOTHING;
