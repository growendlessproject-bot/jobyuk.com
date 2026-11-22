-- Migration 012: Add 3-Day Free Trial tracking, Source origin, and Payment status
ALTER TABLE whatsapp_subscribers 
    ADD COLUMN IF NOT EXISTS source VARCHAR(50) DEFAULT 'portal',
    ADD COLUMN IF NOT EXISTS trial_ends_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS payment_status VARCHAR(20) DEFAULT 'unpaid';

CREATE INDEX IF NOT EXISTS idx_wa_subscribers_source ON whatsapp_subscribers(source);
CREATE INDEX IF NOT EXISTS idx_wa_subscribers_trial_ends ON whatsapp_subscribers(trial_ends_at);
