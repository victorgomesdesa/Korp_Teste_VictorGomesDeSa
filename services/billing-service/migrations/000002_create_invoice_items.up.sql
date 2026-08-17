CREATE TABLE invoice_items (
    id BIGSERIAL PRIMARY KEY,
    invoice_id BIGINT NOT NULL,
    product_id BIGINT NOT NULL,
    product_code VARCHAR NOT NULL,
    product_description VARCHAR NOT NULL,
    quantity BIGINT NOT NULL,
    CONSTRAINT invoice_items_invoice_fk
        FOREIGN KEY (invoice_id) REFERENCES invoices (id),
    CONSTRAINT invoice_items_quantity_positive CHECK (quantity > 0)
);

CREATE INDEX invoice_items_invoice_id_idx ON invoice_items (invoice_id);
