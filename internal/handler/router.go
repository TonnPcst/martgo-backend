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

func NewRouter(logger *slog.Logger, tokens *auth.Verifier, sessions mw.SessionLookup, db Pinger, svc Services, opts RouterOptions) http.Handler {
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

	r.Get("/healthz", health(db))
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

func health(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			httpx.Error(w, http.StatusServiceUnavailable, "database unavailable")
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func actorFrom(r *http.Request) service.Actor {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		return service.Actor{}
	}
	return service.Actor{UserID: claims.UserID, Role: claims.Role}
}
