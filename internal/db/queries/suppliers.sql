-- name: CreateSupplier :one
INSERT INTO suppliers (name, contact_name, email, phone, address)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetSupplier :one
SELECT * FROM suppliers WHERE id = $1;

-- name: ListSuppliers :many
SELECT * FROM suppliers
ORDER BY name
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: CountSuppliers :one
SELECT count(*) FROM suppliers;

-- name: UpdateSupplier :one
UPDATE suppliers SET
    name = $2, contact_name = $3, email = $4, phone = $5, address = $6
WHERE id = $1
RETURNING *;

-- name: DeleteSupplier :execrows
DELETE FROM suppliers WHERE id = $1;
