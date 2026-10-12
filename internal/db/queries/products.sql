-- name: CreateProduct :one
INSERT INTO products (sku, barcode, name, description, category_id, unit, price, cost, reorder_level)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetProduct :one
SELECT * FROM products WHERE id = $1;

-- name: GetProductByBarcode :one
SELECT * FROM products WHERE barcode = sqlc.arg('barcode')::text;

-- name: ListProducts :many
SELECT * FROM products
WHERE (sqlc.narg('search')::text IS NULL
       OR name ILIKE '%' || sqlc.narg('search')::text || '%'
       OR sku ILIKE '%' || sqlc.narg('search')::text || '%'
       OR barcode = sqlc.narg('search')::text)
  AND (sqlc.narg('category_id')::bigint IS NULL OR category_id = sqlc.narg('category_id')::bigint)
  AND (sqlc.narg('is_active')::boolean IS NULL OR is_active = sqlc.narg('is_active')::boolean)
ORDER BY name, id
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: CountProducts :one
SELECT count(*) FROM products
WHERE (sqlc.narg('search')::text IS NULL
       OR name ILIKE '%' || sqlc.narg('search')::text || '%'
       OR sku ILIKE '%' || sqlc.narg('search')::text || '%'
       OR barcode = sqlc.narg('search')::text)
  AND (sqlc.narg('category_id')::bigint IS NULL OR category_id = sqlc.narg('category_id')::bigint)
  AND (sqlc.narg('is_active')::boolean IS NULL OR is_active = sqlc.narg('is_active')::boolean);

-- name: ListLowStockProducts :many
SELECT * FROM products
WHERE is_active AND stock_quantity <= reorder_level
ORDER BY stock_quantity, name
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: CountLowStockProducts :one
SELECT count(*) FROM products
WHERE is_active AND stock_quantity <= reorder_level;

-- name: UpdateProduct :one
UPDATE products SET
    sku = $2, barcode = $3, name = $4, description = $5, category_id = $6,
    unit = $7, price = $8, cost = $9, reorder_level = $10, is_active = $11
WHERE id = $1
RETURNING *;

-- name: DeactivateProduct :execrows
UPDATE products SET is_active = FALSE WHERE id = $1;

-- Locks rows in id order so concurrent sales/purchases cannot deadlock.
-- name: GetProductsForUpdate :many
SELECT * FROM products
WHERE id = ANY(sqlc.arg('ids')::bigint[])
ORDER BY id
FOR UPDATE;

-- name: GetProductForUpdate :one
SELECT * FROM products WHERE id = $1 FOR UPDATE;

-- name: AddProductStock :one
UPDATE products SET stock_quantity = stock_quantity + sqlc.arg('delta')::int
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: ReceiveProductStock :one
UPDATE products SET
    stock_quantity = stock_quantity + sqlc.arg('quantity')::int,
    cost = sqlc.arg('unit_cost')::bigint
WHERE id = sqlc.arg('id')
RETURNING *;
