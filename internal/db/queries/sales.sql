-- name: CreateSale :one
INSERT INTO sales (cashier_id, payment_method, subtotal, discount, total, paid_amount, change_amount, note)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: CreateSaleItem :one
INSERT INTO sale_items (sale_id, product_id, product_name, quantity, unit_price, line_total)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetSale :one
SELECT * FROM sales WHERE id = $1;

-- name: ListSaleItems :many
SELECT * FROM sale_items WHERE sale_id = $1 ORDER BY id;

-- name: ListSales :many
SELECT * FROM sales
WHERE (sqlc.narg('cashier_id')::bigint IS NULL OR cashier_id = sqlc.narg('cashier_id')::bigint)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: CountSales :one
SELECT count(*) FROM sales
WHERE (sqlc.narg('cashier_id')::bigint IS NULL OR cashier_id = sqlc.narg('cashier_id')::bigint);
