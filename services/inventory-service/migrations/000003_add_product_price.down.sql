ALTER TABLE products
    DROP CONSTRAINT products_price_non_negative,
    DROP COLUMN price_in_cents;
