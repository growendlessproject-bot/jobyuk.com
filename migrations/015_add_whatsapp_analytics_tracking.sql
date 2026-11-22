-- Migration 015: WhatsApp Trial Analytics & Conversion Funnel Engine
CREATE TABLE IF NOT EXISTS whatsapp_analytics_events (
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
    device_type VARCHAR(30) DEFAULT 'desktop', -- 'desktop', 'mobile', 'tablet'
    browser VARCHAR(50) DEFAULT '',
    os VARCHAR(50) DEFAULT '',
    phone_number VARCHAR(30) DEFAULT '',
    step VARCHAR(30) NOT NULL DEFAULT 'clicked', -- 'clicked', 'otp_sent', 'verified'
    otp_sent BOOLEAN DEFAULT FALSE,
    otp_verified BOOLEAN DEFAULT FALSE,
    subscriber_id INT REFERENCES whatsapp_subscribers(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_wa_analytics_created_at ON whatsapp_analytics_events(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_wa_analytics_visitor_id ON whatsapp_analytics_events(visitor_id);
CREATE INDEX IF NOT EXISTS idx_wa_analytics_source ON whatsapp_analytics_events(source);
CREATE INDEX IF NOT EXISTS idx_wa_analytics_step ON whatsapp_analytics_events(step);
CREATE INDEX IF NOT EXISTS idx_wa_analytics_phone ON whatsapp_analytics_events(phone_number);
