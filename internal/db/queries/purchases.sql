-- name: CreatePurchase :one
INSERT INTO purchases (supplier_id, created_by, invoice_no, total, note)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: CreatePurchaseItem :one
INSERT INTO purchase_items (purchase_id, product_id, quantity, unit_cost, line_total)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetPurchase :one
SELECT * FROM purchases WHERE id = $1;

-- name: ListPurchaseItems :many
SELECT * FROM purchase_items WHERE purchase_id = $1 ORDER BY id;

-- name: ListPurchases :many
SELECT * FROM purchases
WHERE (sqlc.narg('supplier_id')::bigint IS NULL OR supplier_id = sqlc.narg('supplier_id')::bigint)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: CountPurchases :one
SELECT count(*) FROM purchases
WHERE (sqlc.narg('supplier_id')::bigint IS NULL OR supplier_id = sqlc.narg('supplier_id')::bigint);
