-- +goose Up
CREATE TABLE purchases (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    supplier_id BIGINT      NOT NULL REFERENCES suppliers (id) ON DELETE RESTRICT,
    created_by  BIGINT      NOT NULL REFERENCES users (id),
    invoice_no  TEXT        NOT NULL DEFAULT '',
    total       BIGINT      NOT NULL CHECK (total >= 0),
    note        TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX purchases_supplier_id_idx ON purchases (supplier_id);
CREATE INDEX purchases_created_at_idx ON purchases (created_at DESC);

CREATE TABLE purchase_items (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    purchase_id BIGINT  NOT NULL REFERENCES purchases (id) ON DELETE CASCADE,
    product_id  BIGINT  NOT NULL REFERENCES products (id),
    quantity    INTEGER NOT NULL CHECK (quantity > 0),
    unit_cost   BIGINT  NOT NULL CHECK (unit_cost >= 0),
    line_total  BIGINT  NOT NULL CHECK (line_total >= 0),
    UNIQUE (purchase_id, product_id)
);

CREATE INDEX purchase_items_product_id_idx ON purchase_items (product_id);

-- +goose Down
DROP TABLE IF EXISTS purchase_items;
DROP TABLE IF EXISTS purchases;
