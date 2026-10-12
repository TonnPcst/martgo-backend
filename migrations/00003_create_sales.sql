-- +goose Up
CREATE TABLE sales (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    cashier_id     BIGINT      NOT NULL REFERENCES users (id),
    payment_method TEXT        NOT NULL CHECK (payment_method IN ('cash', 'card', 'ewallet')),
    subtotal       BIGINT      NOT NULL CHECK (subtotal >= 0),
    discount       BIGINT      NOT NULL DEFAULT 0 CHECK (discount >= 0),
    total          BIGINT      NOT NULL CHECK (total >= 0),
    paid_amount    BIGINT      NOT NULL CHECK (paid_amount >= 0),
    change_amount  BIGINT      NOT NULL CHECK (change_amount >= 0),
    note           TEXT        NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT sales_total_matches CHECK (total = subtotal - discount),
    CONSTRAINT sales_change_matches CHECK (change_amount = paid_amount - total)
);

CREATE INDEX sales_cashier_id_idx ON sales (cashier_id);
CREATE INDEX sales_created_at_idx ON sales (created_at DESC);

CREATE TABLE sale_items (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    sale_id      BIGINT  NOT NULL REFERENCES sales (id) ON DELETE CASCADE,
    product_id   BIGINT  NOT NULL REFERENCES products (id),
    product_name TEXT    NOT NULL, -- snapshot at time of sale
    quantity     INTEGER NOT NULL CHECK (quantity > 0),
    unit_price   BIGINT  NOT NULL CHECK (unit_price >= 0),
    line_total   BIGINT  NOT NULL CHECK (line_total >= 0),
    UNIQUE (sale_id, product_id)
);

CREATE INDEX sale_items_product_id_idx ON sale_items (product_id);

-- +goose Down
DROP TABLE IF EXISTS sale_items;
DROP TABLE IF EXISTS sales;
