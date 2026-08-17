CREATE SEQUENCE invoice_number_seq AS BIGINT;

CREATE TABLE invoices (
    id BIGSERIAL PRIMARY KEY,
    number BIGINT NOT NULL DEFAULT nextval('invoice_number_seq'),
    status VARCHAR NOT NULL DEFAULT 'OPEN',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at TIMESTAMPTZ NULL,
    CONSTRAINT invoices_number_unique UNIQUE (number),
    CONSTRAINT invoices_status_valid CHECK (status IN ('OPEN', 'CLOSED'))
);
