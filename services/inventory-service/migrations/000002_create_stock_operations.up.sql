CREATE TABLE stock_operations (
    id BIGSERIAL PRIMARY KEY,
    invoice_id BIGINT NOT NULL,
    idempotency_key VARCHAR NOT NULL,
    fingerprint VARCHAR NOT NULL,
    result JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT stock_operations_idempotency_key_key UNIQUE (idempotency_key)
);
