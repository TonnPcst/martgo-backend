# martgo

A starter supermarket management API in Go: products, categories, suppliers,
purchases (stock in), sales (stock out), stock audit trail.

Login and users live in a separate service, **martgo-auth**
(`../auth-supermarket-go-backend-project`). This service (mart-api) only verifies
the tokens it issues.

**Stack:** chi · PostgreSQL · pgx/pgxpool · sqlc · goose · JWT (EdDSA verify) · Redis · validator · slog

## Services

```
client → gateway :8000 ─┬─ /api/v1/auth/*, /api/v1/users ─→ martgo-auth :8081 → auth Postgres :5434
                        └─ everything else ───────────────→ mart-api   :8080 → Postgres :5433
                 martgo-auth writes session:user:{id} ─→ Redis :6380 ←─ mart-api reads it
```

- mart-api verifies JWTs with the auth service's **public** key (`keys/jwt_public.pem`).
- On every request it reads `session:user:{id}` from Redis to check that the
  token is not revoked and the user is still active, and to get their current role.
  If Redis is unreachable it returns 500 (fails closed).
- `sales.cashier_id`, `purchases.created_by` and `stock_movements.created_by`
  store auth-service user IDs without a foreign key.
- The token and Redis contract is documented in martgo-auth's README. Change
  both services together.

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

Requirements: Go 1.27+, Docker. (sqlc/air/goose run through `go run`, so no global installs are needed.)

```bash
cp .env.example .env          # optional: the Makefile falls back to .env.example
mkdir -p keys && cp ../auth-supermarket-go-backend-project/keys/jwt_public.pem keys/
docker compose up -d          # Postgres :5433, Redis :6380, gateway :8000
make sqlc                     # generate internal/db from the SQL queries
make migrate                  # create tables
make run                      # API on http://localhost:8080
```

Then set up and run martgo-auth (see its README). Call everything through the
gateway at http://localhost:8000.

Other targets: `make dev` (hot reload), `make test`, `make migrate-status`,
`make migrate-down`, `make migrate-create name=add_x`, `make docker-build`, `make help`.

Before pushing, the same checks CI runs: `make test`, `make test-integration`
(needs the `martgo_test` database, see `internal/service/integration_test.go`),
`make sqlc-check` (generated code matches the SQL) and `make lint-api`.

### Running everything in containers

Instead of `make run` in both repos, you can run both APIs as containers:

```bash
(cd ../auth-supermarket-go-backend-project && docker compose up -d)   # auth Postgres
make up-app     # builds both images, runs both migrations, starts both APIs; gateway on :8000
make down-app   # removes the API containers and points the gateway back at the host
```

`docker-compose.app.yml` adds `mart-api` (:8080), `auth-api` (:8081) and one-shot
migration containers on top of `docker-compose.yml`, and swaps the gateway to
`deploy/nginx.app.conf`. Containers read `.env.example` and then `.env` from each
repo; database and Redis URLs are rewritten to the compose network. The auth
database is reached through `host.docker.internal:5434` (override with
`AUTH_DATABASE_URL`). Stop any `make run` first, since the ports are the same.

The image is distroless and runs as non-root. It contains `/app/api` and
`/app/migrate`; keys and config are mounted or passed in at runtime, never baked in.

`GET /healthz` returns `503` when Postgres **or** Redis is unreachable.

## API docs

Each service serves its own Swagger UI:

- **martgo-auth:** http://localhost:8081/docs: log in with `POST /auth/login`
  and copy the `access_token`.
- **mart-api:** http://localhost:8080/docs (or http://localhost:8000/docs through
  the gateway): click **Authorize**, paste the token, and try any endpoint.

This service's OpenAPI 3.1 spec lives in [api/openapi.yaml](api/openapi.yaml) (served at
`/docs/openapi.yaml`). It is written by hand, so update it when you add or
change an endpoint. Check it with `make lint-api`.

## Example requests

```bash
API=http://localhost:8000/api/v1

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
make test   # sale/purchase service tests (testify mocks) and router/auth middleware tests
```
