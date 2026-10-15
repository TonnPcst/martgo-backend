package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/TonnPcst/martgo-backend/internal/auth"
	"github.com/TonnPcst/martgo-backend/internal/domain"
	"github.com/TonnPcst/martgo-backend/internal/httpx"
	mw "github.com/TonnPcst/martgo-backend/internal/middleware"
	"github.com/TonnPcst/martgo-backend/internal/service"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

type Services struct {
	Categories *service.CategoryService
	Suppliers  *service.SupplierService
	Products   *service.ProductService
	Sales      *service.SalesService
	Purchases  *service.PurchaseService
}

type RouterOptions struct {
	TrustProxy         bool
	EnableDocs         bool
	CORSAllowedOrigins []string
}

type Pinger interface {
	Ping(ctx context.Context) error
}

// PingFunc adapts a plain function (e.g. a Redis ping) to Pinger.
type PingFunc func(ctx context.Context) error

func (f PingFunc) Ping(ctx context.Context) error { return f(ctx) }

// HealthCheck is one dependency /healthz must reach, such as "database" or "redis".
type HealthCheck struct {
	Name   string
	Pinger Pinger
}

// NewRouter builds the HTTP router with all middleware and routes.
//
// ENG: Global middleware: request ID, real IP (when behind a trusted proxy), logging,
// panic recovery, security headers and optional CORS. Every /api/v1 route needs a
// valid token. Anyone logged in can read categories and products and create sales;
// suppliers, purchases and product/category writes need admin or manager.
// /auth/* and /users are served by the auth service behind the gateway.
// LAO: ສ້າງ router ພ້ອມ middleware ແລະ ເສັ້ນທາງທັງໝົດ — ທຸກ /api/v1 ຕ້ອງມີ token, ບາງເສັ້ນທາງສະເພາະ admin/manager.
//
// @param logger - logger for request logs and panics
// @param tokens - verifies the JWT access tokens
// @param sessions - looks up session state in Redis for revocation
// @param checks - dependencies pinged by /healthz
// @param svc - business services used by the handlers
// @param opts - proxy, docs and CORS settings
// @returns the root HTTP handler
func NewRouter(logger *slog.Logger, tokens *auth.Verifier, sessions mw.SessionLookup, checks []HealthCheck, svc Services, opts RouterOptions) http.Handler {
	categoryH := &CategoryHandler{svc: svc.Categories}
	supplierH := &SupplierHandler{svc: svc.Suppliers}
	productH := &ProductHandler{svc: svc.Products}
	saleH := &SaleHandler{svc: svc.Sales}
	purchaseH := &PurchaseHandler{svc: svc.Purchases}

	managers := mw.RequireRole(domain.RoleAdmin, domain.RoleManager)

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	if opts.TrustProxy {
		r.Use(chimw.RealIP)
	}
	r.Use(mw.RequestLogger(logger), mw.Recoverer(logger), mw.SecureHeaders)
	if len(opts.CORSAllowedOrigins) > 0 {
		r.Use(mw.CORS(opts.CORSAllowedOrigins))
	}

	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		httpx.Error(w, http.StatusNotFound, "route not found")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
	})

	r.Get("/healthz", health(checks))
	if opts.EnableDocs {
		r.Get("/docs", swaggerUI)
		r.Get("/docs/openapi.yaml", openAPISpec)
	}

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(mw.APIHeaders)

		// /auth/* and /users are served by the auth service (martgo-auth) behind the gateway.
		r.Group(func(r chi.Router) {
			r.Use(mw.Authenticate(tokens, sessions))

			r.Route("/categories", func(r chi.Router) {
				r.Get("/", categoryH.List)
				r.Get("/{id}", categoryH.Get)
				r.With(managers).Post("/", categoryH.Create)
				r.With(managers).Put("/{id}", categoryH.Update)
				r.With(managers).Delete("/{id}", categoryH.Delete)
			})

			r.Route("/suppliers", func(r chi.Router) {
				r.Use(managers)
				r.Get("/", supplierH.List)
				r.Post("/", supplierH.Create)
				r.Get("/{id}", supplierH.Get)
				r.Put("/{id}", supplierH.Update)
				r.Delete("/{id}", supplierH.Delete)
			})

			r.Route("/products", func(r chi.Router) {
				r.Get("/", productH.List)
				r.Get("/low-stock", productH.ListLowStock)
				r.Get("/barcode/{barcode}", productH.GetByBarcode)
				r.Get("/{id}", productH.Get)
				r.Group(func(r chi.Router) {
					r.Use(managers)
					r.Post("/", productH.Create)
					r.Put("/{id}", productH.Update)
					r.Delete("/{id}", productH.Deactivate)
					r.Post("/{id}/stock-adjustments", productH.AdjustStock)
					r.Get("/{id}/stock-movements", productH.ListStockMovements)
				})
			})

			r.Route("/sales", func(r chi.Router) {
				r.Post("/", saleH.Create)
				r.Get("/", saleH.List)
				r.Get("/{id}", saleH.Get)
			})

			r.Route("/purchases", func(r chi.Router) {
				r.Use(managers)
				r.Post("/", purchaseH.Create)
				r.Get("/", purchaseH.List)
				r.Get("/{id}", purchaseH.Get)
			})
		})
	})

	return r
}

// health builds the health check handler. GET /healthz
//
// ENG: Pings every dependency (Postgres, Redis) within one 2-second timeout. Returns
// 200 {"status":"ok"} when all answer, or 503 naming the first one that does not, so
// load balancers take the instance out while it cannot serve requests.
// LAO: ສ້າງ handler ກວດສຸຂະພາບລະບົບ — ping DB ແລະ Redis ພາຍໃນ 2 ວິນາທີ, ຕອບ 200 ຫຼື 503.
//
// @param checks - dependencies to ping, in order
// @returns the HTTP handler
func health(checks []HealthCheck) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		for _, c := range checks {
			if err := c.Pinger.Ping(ctx); err != nil {
				httpx.Error(w, http.StatusServiceUnavailable, c.Name+" unavailable")
				return
			}
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// actorFrom returns the logged-in user from the request context.
//
// ENG: Reads the token claims that the Authenticate middleware stored. Returns an
// empty Actor when the request is not authenticated.
// LAO: ດຶງຂໍ້ມູນຜູ້ໃຊ້ທີ່ເຂົ້າສູ່ລະບົບ (ID, role) ຈາກ context ຂອງ request.
//
// @param r - incoming request
// @returns the actor (user ID and role)
func actorFrom(r *http.Request) service.Actor {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		return service.Actor{}
	}
	return service.Actor{UserID: claims.UserID, Role: claims.Role}
}
