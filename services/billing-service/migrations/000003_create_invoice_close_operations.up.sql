CREATE TABLE invoice_close_operations (
    id BIGSERIAL PRIMARY KEY,
    invoice_id BIGINT NOT NULL,
    idempotency_key VARCHAR NOT NULL,
    status VARCHAR NOT NULL,
    result JSONB NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ NULL,
    CONSTRAINT invoice_close_operations_invoice_fk
        FOREIGN KEY (invoice_id) REFERENCES invoices (id),
    CONSTRAINT invoice_close_operations_invoice_unique UNIQUE (invoice_id),
    CONSTRAINT invoice_close_operations_idempotency_key_unique UNIQUE (idempotency_key),
    CONSTRAINT invoice_close_operations_status_valid CHECK (status IN ('PROCESSING', 'COMPLETED'))
);
