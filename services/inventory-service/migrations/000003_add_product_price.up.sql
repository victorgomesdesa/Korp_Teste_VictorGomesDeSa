ALTER TABLE products
    ADD COLUMN price_in_cents BIGINT NOT NULL DEFAULT 0,
    ADD CONSTRAINT products_price_non_negative CHECK (price_in_cents >= 0);
