-- Refunds give money back for some or all of a sale. Items can go back on the
-- shelf (restock) or be written off (damaged); either way the sale itself is
-- never changed, so receipts and history stay as they were.

-- +goose Up
CREATE TABLE refunds (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    sale_id     BIGINT      NOT NULL REFERENCES sales (id),
    refunded_by BIGINT      NOT NULL, -- martgo-auth user id, no foreign key (see 00008)
    amount      BIGINT      NOT NULL CHECK (amount >= 0),
    reason      TEXT        NOT NULL CHECK (reason <> ''),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX refunds_sale_id_idx ON refunds (sale_id);
CREATE INDEX refunds_created_at_idx ON refunds (created_at DESC);

CREATE TABLE refund_items (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    refund_id    BIGINT  NOT NULL REFERENCES refunds (id) ON DELETE CASCADE,
    sale_item_id BIGINT  NOT NULL REFERENCES sale_items (id),
    product_id   BIGINT  NOT NULL REFERENCES products (id),
    quantity     INTEGER NOT NULL CHECK (quantity > 0),
    unit_price   BIGINT  NOT NULL CHECK (unit_price >= 0), -- price on the sale, before discount
    line_total   BIGINT  NOT NULL CHECK (line_total >= 0),
    restock      BOOLEAN NOT NULL,
    UNIQUE (refund_id, sale_item_id)
);

CREATE INDEX refund_items_sale_item_id_idx ON refund_items (sale_item_id);

ALTER TABLE stock_movements DROP CONSTRAINT stock_movements_reason_check;
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_reason_check
    CHECK (reason IN ('sale', 'purchase', 'adjustment', 'refund'));

-- +goose Down
DELETE FROM stock_movements WHERE reason = 'refund';
ALTER TABLE stock_movements DROP CONSTRAINT stock_movements_reason_check;
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_reason_check
    CHECK (reason IN ('sale', 'purchase', 'adjustment'));

DROP TABLE IF EXISTS refund_items;
DROP TABLE IF EXISTS refunds;
