-- Migration 014: Add dual WhatsApp gateway and smart routing settings
ALTER TABLE whatsapp_gateway_settings
    ADD COLUMN IF NOT EXISTS support_connected BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS support_phone VARCHAR(50) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS support_name VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS support_last_connected_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS routing_mode VARCHAR(30) NOT NULL DEFAULT 'dual',
    ADD COLUMN IF NOT EXISTS alerts_device VARCHAR(20) NOT NULL DEFAULT 'alerts',
    ADD COLUMN IF NOT EXISTS support_device VARCHAR(20) NOT NULL DEFAULT 'support';
