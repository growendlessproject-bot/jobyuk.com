-- Migration 010: Add 6-Digit Access PIN Security & Brute-Force Lockout to WhatsApp Subscribers

ALTER TABLE whatsapp_subscribers 
    ADD COLUMN IF NOT EXISTS pin_hash VARCHAR(255) DEFAULT '',
    ADD COLUMN IF NOT EXISTS failed_pin_attempts INT DEFAULT 0,
    ADD COLUMN IF NOT EXISTS locked_until TIMESTAMP WITH TIME ZONE;

CREATE INDEX IF NOT EXISTS idx_wa_subscribers_pin_hash ON whatsapp_subscribers(pin_hash);
