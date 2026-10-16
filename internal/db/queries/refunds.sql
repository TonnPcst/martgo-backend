-- name: GetSaleForUpdate :one
-- Serializes refunds of the same sale.
SELECT * FROM sales WHERE id = $1 FOR UPDATE;

-- name: ListRefundedQuantities :many
-- How much of each sale line has already been refunded.
SELECT ri.sale_item_id, sum(ri.quantity)::int AS quantity
FROM refund_items ri
JOIN refunds r ON r.id = ri.refund_id
WHERE r.sale_id = $1
GROUP BY ri.sale_item_id;

-- name: SumRefundedAmount :one
SELECT coalesce(sum(amount), 0)::bigint FROM refunds WHERE sale_id = $1;

-- name: CreateRefund :one
INSERT INTO refunds (sale_id, refunded_by, amount, reason)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: CreateRefundItem :one
INSERT INTO refund_items (refund_id, sale_item_id, product_id, quantity, unit_price, line_total, restock)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListRefundsBySale :many
SELECT * FROM refunds WHERE sale_id = $1 ORDER BY id;

-- name: ListRefundItemsBySale :many
SELECT ri.* FROM refund_items ri
JOIN refunds r ON r.id = ri.refund_id
WHERE r.sale_id = $1
ORDER BY ri.refund_id, ri.id;
