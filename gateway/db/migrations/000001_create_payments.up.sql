CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TYPE payment_state AS ENUM (
    'PENDING',
    'AUTHORIZED',
    'CAPTURED',
    'VOIDED',
    'REFUNDED'
);

CREATE TABLE payments (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    order_id        TEXT NOT NULL,
    customer_id     TEXT NOT NULL,
    amount          BIGINT NOT NULL,
    currency        TEXT NOT NULL DEFAULT 'USD',
    state           payment_state NOT NULL DEFAULT 'PENDING',

    -- card details, supposed to vault these in prod not store raw
    card_number     TEXT NOT NULL,
    card_expiry     TEXT NOT NULL,
    card_cvv        TEXT NOT NULL,

    bank_auth_id    TEXT,
    bank_capture_id TEXT,
    bank_void_id    TEXT,
    bank_refund_id  TEXT,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    authorized_at   TIMESTAMPTZ,
    captured_at     TIMESTAMPTZ,
    voided_at       TIMESTAMPTZ,
    refunded_at     TIMESTAMPTZ
);

CREATE INDEX idx_payments_order_id    ON payments(order_id);
CREATE INDEX idx_payments_customer_id ON payments(customer_id);

CREATE TABLE idempotency_keys (
    key         TEXT NOT NULL,
    operation   TEXT NOT NULL,
    payment_id  TEXT REFERENCES payments(id),
    response    BYTEA NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (key, operation)
);
