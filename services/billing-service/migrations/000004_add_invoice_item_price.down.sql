ALTER TABLE invoice_items
    DROP CONSTRAINT invoice_items_unit_price_non_negative,
    DROP COLUMN unit_price_in_cents;
