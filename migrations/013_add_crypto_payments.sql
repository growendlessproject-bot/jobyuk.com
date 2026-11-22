-- Migration 013: Add Crypto Wallets configuration and Payment Proofs table

CREATE TABLE IF NOT EXISTS whatsapp_crypto_wallets (
    id SERIAL PRIMARY KEY,
    coin VARCHAR(20) NOT NULL,
    network VARCHAR(50) NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    address VARCHAR(255) NOT NULL,
    instructions TEXT DEFAULT '',
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS whatsapp_crypto_payments (
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
    status VARCHAR(30) DEFAULT 'pending', -- 'pending', 'verified', 'rejected'
    admin_notes TEXT DEFAULT '',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    verified_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_wa_crypto_payments_sub ON whatsapp_crypto_payments(subscriber_id);
CREATE INDEX IF NOT EXISTS idx_wa_crypto_payments_status ON whatsapp_crypto_payments(status);
CREATE INDEX IF NOT EXISTS idx_wa_crypto_payments_tx ON whatsapp_crypto_payments(tx_hash);

-- Seed default crypto wallets if table is empty
INSERT INTO whatsapp_crypto_wallets (coin, network, symbol, address, instructions, is_active)
SELECT 'USDT', 'TRC-20 (Tron)', '₮', 'TYDzsYUEpvnYmQK4W5g72N26F52fG9EQ11', 'Send USDT strictly via the TRC20 network. Other networks will result in permanent loss of funds.', TRUE
WHERE NOT EXISTS (SELECT 1 FROM whatsapp_crypto_wallets WHERE coin = 'USDT' AND network LIKE '%TRC-20%');

INSERT INTO whatsapp_crypto_wallets (coin, network, symbol, address, instructions, is_active)
SELECT 'USDT', 'BEP-20 (BNB Smart Chain)', '₮', '0x89205A3A3b2A556108d852A35C4da43de584444', 'Send USDT strictly via the BEP20 (BNB Smart Chain) network.', TRUE
WHERE NOT EXISTS (SELECT 1 FROM whatsapp_crypto_wallets WHERE coin = 'USDT' AND network LIKE '%BEP-20%');

INSERT INTO whatsapp_crypto_wallets (coin, network, symbol, address, instructions, is_active)
SELECT 'BTC', 'Bitcoin Native', '₿', 'bc1qar0srrr7xfkvy5l643lydnw9re59gtzzwf5mdq', 'Send BTC strictly via the native Bitcoin network. Requires 1-3 network confirmations.', TRUE
WHERE NOT EXISTS (SELECT 1 FROM whatsapp_crypto_wallets WHERE coin = 'BTC');

INSERT INTO whatsapp_crypto_wallets (coin, network, symbol, address, instructions, is_active)
SELECT 'ETH', 'ERC-20 (Ethereum)', 'Ξ', '0x89205A3A3b2A556108d852A35C4da43de584444', 'Send ETH strictly via Ethereum ERC-20 mainnet.', TRUE
WHERE NOT EXISTS (SELECT 1 FROM whatsapp_crypto_wallets WHERE coin = 'ETH');
