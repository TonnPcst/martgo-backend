-- Lets a till retry POST /sales safely: a request sent again with the same
-- Idempotency-Key returns the sale already recorded instead of selling twice.
-- request_hash is a fingerprint of the request, so a key reused for a
-- different sale is rejected instead of silently returning the old one.

-- +goose Up
ALTER TABLE sales
    ADD COLUMN idempotency_key TEXT,
    ADD COLUMN request_hash    TEXT,
    ADD CONSTRAINT sales_idempotency_complete
        CHECK ((idempotency_key IS NULL) = (request_hash IS NULL));

-- Keys are scoped to the cashier, so two tills cannot collide by accident.
CREATE UNIQUE INDEX sales_cashier_idempotency_key_idx
    ON sales (cashier_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS sales_cashier_idempotency_key_idx;
ALTER TABLE sales
    DROP CONSTRAINT sales_idempotency_complete,
    DROP COLUMN request_hash,
    DROP COLUMN idempotency_key;
