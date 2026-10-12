-- +goose Up
-- Audit trail of every stock change (sales, purchases, manual adjustments).
CREATE TABLE stock_movements (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    product_id      BIGINT      NOT NULL REFERENCES products (id),
    quantity_change INTEGER     NOT NULL CHECK (quantity_change <> 0),
    reason          TEXT        NOT NULL CHECK (reason IN ('sale', 'purchase', 'adjustment')),
    reference_id    BIGINT,
    note            TEXT        NOT NULL DEFAULT '',
    created_by      BIGINT      NOT NULL REFERENCES users (id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX stock_movements_product_id_idx ON stock_movements (product_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS stock_movements;
