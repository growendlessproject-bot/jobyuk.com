-- Migration 016: Expand status column in whatsapp_subscribers to support pending_verification
ALTER TABLE whatsapp_subscribers ALTER COLUMN status TYPE VARCHAR(50);
