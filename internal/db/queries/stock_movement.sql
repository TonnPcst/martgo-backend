-- name: CreateStockMovement :exec
INSERT INTO stock_movements (product_id, quantity_change, reason, reference_id, note, created_by)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListStockMovementsByProduct :many
SELECT * FROM stock_movements
WHERE product_id = sqlc.arg('product_id')
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: CountStockMovementsByProduct :one
SELECT count(*) FROM stock_movements WHERE product_id = $1;
