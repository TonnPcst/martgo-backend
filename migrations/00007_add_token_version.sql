-- +goose Up
ALTER TABLE users ADD COLUMN token_version INTEGER NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE users DROP COLUMN token_version;
