-- Users live in the auth service (martgo-auth). Nothing in this database reads
-- this table any more; sales, purchases and stock_movements keep the user id
-- without a foreign key (00008). set_updated_at() stays: other tables use it.

-- +goose Up
DROP TABLE users;

-- +goose Down
CREATE TABLE users (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email         TEXT        NOT NULL UNIQUE,
    password_hash TEXT        NOT NULL,
    full_name     TEXT        NOT NULL,
    role          TEXT        NOT NULL CHECK (role IN ('admin', 'manager', 'cashier')),
    is_active     BOOLEAN     NOT NULL DEFAULT TRUE,
    token_version INTEGER     NOT NULL DEFAULT 1,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER users_set_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
