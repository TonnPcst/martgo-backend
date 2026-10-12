-- +goose Up
CREATE TABLE categories (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name        TEXT        NOT NULL UNIQUE,
    description TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER categories_set_updated_at
    BEFORE UPDATE ON categories
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE suppliers (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name         TEXT        NOT NULL,
    contact_name TEXT        NOT NULL DEFAULT '',
    email        TEXT        NOT NULL DEFAULT '',
    phone        TEXT        NOT NULL DEFAULT '',
    address      TEXT        NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER suppliers_set_updated_at
    BEFORE UPDATE ON suppliers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Money columns (price, cost) are BIGINT amounts in the smallest currency unit
-- (e.g. cents / satang). Never use floats for money.
CREATE TABLE products (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    sku            TEXT        NOT NULL UNIQUE,
    barcode        TEXT        UNIQUE,
    name           TEXT        NOT NULL,
    description    TEXT        NOT NULL DEFAULT '',
    category_id    BIGINT      REFERENCES categories (id) ON DELETE SET NULL,
    unit           TEXT        NOT NULL DEFAULT 'pcs',
    price          BIGINT      NOT NULL CHECK (price >= 0),
    cost           BIGINT      NOT NULL DEFAULT 0 CHECK (cost >= 0),
    stock_quantity INTEGER     NOT NULL DEFAULT 0 CHECK (stock_quantity >= 0),
    reorder_level  INTEGER     NOT NULL DEFAULT 0 CHECK (reorder_level >= 0),
    is_active      BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX products_category_id_idx ON products (category_id);
CREATE INDEX products_name_idx ON products (name);

CREATE TRIGGER products_set_updated_at
    BEFORE UPDATE ON products
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS products;
DROP TABLE IF EXISTS suppliers;
DROP TABLE IF EXISTS categories;
