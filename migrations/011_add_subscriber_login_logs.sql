-- Migration 011: Dedicated Subscriber Login Security & Activity Logs
CREATE TABLE IF NOT EXISTS whatsapp_subscriber_logins (
    id SERIAL PRIMARY KEY,
    subscriber_id INT REFERENCES whatsapp_subscribers(id) ON DELETE CASCADE,
    phone_number VARCHAR(30) NOT NULL,
    ip_address VARCHAR(45) DEFAULT '',
    user_agent TEXT DEFAULT '',
    action VARCHAR(50) NOT NULL DEFAULT 'login_pin', -- 'login_pin', 'register_verify', 'pin_reset', 'number_change'
    status VARCHAR(20) NOT NULL DEFAULT 'success', -- 'success', 'failed_pin', 'locked'
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_wa_sub_logins_sub_id ON whatsapp_subscriber_logins(subscriber_id, created_at DESC);
