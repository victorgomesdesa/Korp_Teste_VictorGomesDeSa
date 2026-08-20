ALTER TABLE invoice_items
    ADD COLUMN unit_price_in_cents BIGINT NOT NULL DEFAULT 0,
    ADD CONSTRAINT invoice_items_unit_price_non_negative CHECK (unit_price_in_cents >= 0);
