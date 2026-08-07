-- ============================================================
-- Digital Account Store — Database Migration 001
-- Creates: products, orders, product_stocks, restock_subscriptions
-- ============================================================

BEGIN;

-- ─── Extensions ──────────────────────────────────
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ─── Products ────────────────────────────────────
CREATE TABLE IF NOT EXISTS products (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title           VARCHAR(255) NOT NULL,
    slug            VARCHAR(255) UNIQUE NOT NULL,
    description     TEXT,
    price_idr       NUMERIC(12, 2) NOT NULL,
    image_url       TEXT,
    is_active       BOOLEAN DEFAULT TRUE,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- ─── Orders ──────────────────────────────────────
CREATE TABLE IF NOT EXISTS orders (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_number    VARCHAR(64) UNIQUE NOT NULL,
    customer_email  VARCHAR(255) NOT NULL,
    product_id      UUID REFERENCES products(id),
    quantity        INTEGER NOT NULL DEFAULT 1,
    total_amount    NUMERIC(12, 2) NOT NULL,
    status          VARCHAR(32) NOT NULL DEFAULT 'PENDING',
    snap_token      VARCHAR(255),
    pin             VARCHAR(10) DEFAULT '123456',
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- ─── Product Stocks (Digital Credentials) ────────
CREATE TABLE IF NOT EXISTS product_stocks (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id          UUID REFERENCES products(id) ON DELETE CASCADE,
    email               VARCHAR(255) NOT NULL,
    password_encrypted  TEXT NOT NULL,
    additional_info     TEXT,
    status              VARCHAR(32) NOT NULL DEFAULT 'AVAILABLE',
    order_id            UUID REFERENCES orders(id),
    sold_at             TIMESTAMP WITH TIME ZONE,
    created_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- ─── Restock Subscriptions ──────────────────────
CREATE TABLE IF NOT EXISTS restock_subscriptions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id      UUID REFERENCES products(id) ON DELETE CASCADE,
    email           VARCHAR(255) NOT NULL,
    is_notified     BOOLEAN DEFAULT FALSE,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(product_id, email)
);

-- ─── Performance Indexes ─────────────────────────
-- Partial index: only AVAILABLE stocks for fast reservation queries
CREATE INDEX IF NOT EXISTS idx_stocks_avail
    ON product_stocks(product_id, status)
    WHERE status = 'AVAILABLE';

-- Partial index: only RESERVED stocks for expiry worker
CREATE INDEX IF NOT EXISTS idx_stocks_reserved
    ON product_stocks(product_id, status)
    WHERE status = 'RESERVED';

-- Composite index for guest order lookup
CREATE INDEX IF NOT EXISTS idx_orders_lookup
    ON orders(order_number, customer_email);

-- Index for pending order expiry worker
CREATE INDEX IF NOT EXISTS idx_orders_pending
    ON orders(status, created_at)
    WHERE status = 'PENDING';

-- Index for restock notification worker
CREATE INDEX IF NOT EXISTS idx_restock_pending
    ON restock_subscriptions(product_id, is_notified)
    WHERE is_notified = FALSE;

COMMIT;
