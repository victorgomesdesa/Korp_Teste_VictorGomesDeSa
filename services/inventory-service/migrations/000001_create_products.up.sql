CREATE TABLE products (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR NOT NULL,
    description VARCHAR NOT NULL,
    balance BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT products_code_key UNIQUE (code),
    CONSTRAINT products_balance_non_negative CHECK (balance >= 0)
);
