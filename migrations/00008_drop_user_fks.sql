-- Users now live in the auth service's database, so these columns keep the
-- user id without a foreign key.

-- +goose Up
ALTER TABLE sales           DROP CONSTRAINT sales_cashier_id_fkey;
ALTER TABLE purchases       DROP CONSTRAINT purchases_created_by_fkey;
ALTER TABLE stock_movements DROP CONSTRAINT stock_movements_created_by_fkey;

-- +goose Down
ALTER TABLE sales           ADD CONSTRAINT sales_cashier_id_fkey           FOREIGN KEY (cashier_id) REFERENCES users (id);
ALTER TABLE purchases       ADD CONSTRAINT purchases_created_by_fkey       FOREIGN KEY (created_by) REFERENCES users (id);
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_created_by_fkey FOREIGN KEY (created_by) REFERENCES users (id);
