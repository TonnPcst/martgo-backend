# martgo

A starter supermarket management API in Go: products, categories, suppliers,
purchases (stock in), sales (stock out), stock audit trail, users with roles.

**Stack:** chi · PostgreSQL · pgx/pgxpool · sqlc · goose · JWT · bcrypt · validator · slog

## Architecture

```
HTTP request → handler (parse/validate, JSON out)
             → service (business rules, transactions)
             → store / sqlc (queries only)
```

- Money is stored as `BIGINT` in the smallest currency unit (4500 = 45.00). Never floats.
- Sales and purchases run in one transaction and lock product rows (`FOR UPDATE`).
- Every stock change writes a row to `stock_movements`.
- Errors always look like `{"error": "message"}`.
- List endpoints accept `?limit=` (default 20, max 100) and `?offset=`, and return
  `{"data": [...], "pagination": {"limit", "offset", "total"}}`.

## Roles

| Endpoint group                               | admin | manager | cashier |
| -------------------------------------------- | :---: | :-----: | :-----: |
| `POST /auth/login`, `GET /auth/me`           |   ✓   |    ✓    |    ✓    |
| `/users`                                     |   ✓   |         |         |
| `GET /categories`, `GET /products…`          |   ✓   |    ✓    |    ✓    |
| write categories/products, stock adjustments |   ✓   |    ✓    |         |
| `/suppliers`, `/purchases`                   |   ✓   |    ✓    |         |
| `/sales` (cashiers see only their own)       |   ✓   |    ✓    |    ✓    |

## Setup

Requirements: Go 1.25+, Docker. (sqlc/air/goose run through `go run`, so no global installs are needed.)

```bash
cp .env.example .env          # optional: the Makefile falls back to .env.example
docker compose up -d          # Postgres on localhost:5433
make sqlc                     # generate internal/db from the SQL queries
make migrate                  # create tables + seed the admin user
make run                      # API on http://localhost:8080
```

Other targets: `make dev` (hot reload), `make test`, `make migrate-status`,
`make migrate-down`, `make migrate-create name=add_x`, `make help`.

## API docs

With the server running, open **http://localhost:8080/docs** for interactive
Swagger UI. Log in with `POST /auth/login`, click **Authorize**, paste the
`access_token`, and try any endpoint from the browser.

The OpenAPI 3.1 spec lives in [api/openapi.yaml](api/openapi.yaml) (served at
`/docs/openapi.yaml`). It is written by hand, so update it when you add or
change an endpoint. Check it with `make lint-api`.

## Example requests

```bash
API=http://localhost:8080/api/v1

# Log in as the seeded admin
TOKEN=$(curl -s -X POST $API/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"admin@martgo.local","password":"admin12345"}' | jq -r .access_token)
AUTH="Authorization: Bearer $TOKEN"
JSON='Content-Type: application/json'

curl -s $API/auth/me -H "$AUTH"

# Category, product (price 45.00 → 4500), supplier
curl -s -X POST $API/categories -H "$AUTH" -H "$JSON" \
  -d '{"name":"Dairy","description":"Milk, cheese, yogurt"}'

curl -s -X POST $API/products -H "$AUTH" -H "$JSON" \
  -d '{"sku":"MILK-1L","barcode":"8850001000011","name":"Fresh Milk 1L","category_id":1,"unit":"bottle","price":4500,"reorder_level":5}'

curl -s -X POST $API/suppliers -H "$AUTH" -H "$JSON" \
  -d '{"name":"Fresh Farm Co.","contact_name":"Somchai","phone":"+66 2 123 4567"}'

# Receive 24 bottles at 32.00 each → stock 24, cost 3200
curl -s -X POST $API/purchases -H "$AUTH" -H "$JSON" \
  -d '{"supplier_id":1,"invoice_no":"INV-1001","items":[{"product_id":1,"quantity":24,"unit_cost":3200}]}'

# Sell 2 bottles, pay 100.00 cash → total 9000, change_amount 1000
curl -s -X POST $API/sales -H "$AUTH" -H "$JSON" \
  -d '{"payment_method":"cash","paid_amount":10000,"items":[{"product_id":1,"quantity":2}]}'

# Lists with pagination & filters
curl -s "$API/products?search=milk&limit=10&offset=0" -H "$AUTH"
curl -s "$API/sales?limit=5" -H "$AUTH"
curl -s "$API/products/low-stock" -H "$AUTH"
curl -s "$API/products/barcode/8850001000011" -H "$AUTH"

# Manual stock correction + audit trail
curl -s -X POST $API/products/1/stock-adjustments -H "$AUTH" -H "$JSON" \
  -d '{"quantity_change":-1,"note":"damaged bottle"}'
curl -s $API/products/1/stock-movements -H "$AUTH"

# Create a cashier
curl -s -X POST $API/users -H "$AUTH" -H "$JSON" \
  -d '{"email":"cashier1@martgo.local","password":"cashier123","full_name":"Cashier One","role":"cashier"}'

# Error format example
curl -s -X POST $API/sales -H "$AUTH" -H "$JSON" \
  -d '{"payment_method":"cash","paid_amount":0,"items":[{"product_id":1,"quantity":999}]}'
# {"error":"insufficient stock for \"Fresh Milk 1L\": requested 999, available 21"}
```

## Tests

```bash
make test   # table-driven unit tests for the sale and purchase services (testify mocks)
```
