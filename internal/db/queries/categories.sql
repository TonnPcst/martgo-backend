-- name: CreateCategory :one
INSERT INTO categories (name, description)
VALUES ($1, $2)
RETURNING *;

-- name: GetCategory :one
SELECT * FROM categories WHERE id = $1;

-- name: ListCategories :many
SELECT * FROM categories
ORDER BY name
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: CountCategories :one
SELECT count(*) FROM categories;

-- name: UpdateCategory :one
UPDATE categories SET name = $2, description = $3
WHERE id = $1
RETURNING *;

-- name: DeleteCategory :execrows
DELETE FROM categories WHERE id = $1;
