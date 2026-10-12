package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/TonnPcst/martgo-backend/internal/httpx"
	"github.com/TonnPcst/martgo-backend/internal/service"
)

type ProductHandler struct{ svc *service.ProductService }

type productRequest struct {
	Sku          string  `json:"sku" validate:"required,max=64"`
	Barcode      *string `json:"barcode" validate:"omitempty,max=64"`
	Name         string  `json:"name" validate:"required,max=200"`
	Description  string  `json:"description" validate:"max=2000"`
	CategoryID   *int64  `json:"category_id" validate:"omitempty,gt=0"`
	Unit         string  `json:"unit" validate:"max=20"`
	Price        int64   `json:"price" validate:"gt=0"`
	Cost         int64   `json:"cost" validate:"gte=0"`
	ReorderLevel int32   `json:"reorder_level" validate:"gte=0"`
	IsActive     *bool   `json:"is_active"`
}

func (req productRequest) toInput() service.ProductInput {
	in := service.ProductInput{
		Sku: req.Sku, Barcode: req.Barcode, Name: req.Name, Description: req.Description,
		CategoryID: req.CategoryID, Unit: req.Unit, Price: req.Price, Cost: req.Cost,
		ReorderLevel: req.ReorderLevel, IsActive: true,
	}
	if req.IsActive != nil {
		in.IsActive = *req.IsActive
	}
	return in
}

type stockAdjustmentRequest struct {
	QuantityChange int32  `json:"quantity_change" validate:"required"`
	Note           string `json:"note" validate:"required,max=500"`
}

func (h *ProductHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req productRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	p, err := h.svc.Create(r.Context(), req.toInput())
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, p)
}

func (h *ProductHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.IDParam(r, "id")
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	p, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, p)
}

func (h *ProductHandler) GetByBarcode(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.GetByBarcode(r.Context(), chi.URLParam(r, "barcode"))
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, p)
}

func (h *ProductHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := httpx.ParsePagination(r)
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	categoryID, err := httpx.QueryInt64(r, "category_id")
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	isActive, err := httpx.QueryBool(r, "is_active")
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	filter := service.ProductFilter{
		Search: httpx.QueryString(r, "search"), CategoryID: categoryID, IsActive: isActive,
	}
	items, total, err := h.svc.List(r.Context(), filter, limit, offset)
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	httpx.WritePage(w, items, limit, offset, total)
}

func (h *ProductHandler) ListLowStock(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := httpx.ParsePagination(r)
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	items, total, err := h.svc.ListLowStock(r.Context(), limit, offset)
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	httpx.WritePage(w, items, limit, offset, total)
}

func (h *ProductHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.IDParam(r, "id")
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	var req productRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	p, err := h.svc.Update(r.Context(), id, req.toInput())
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, p)
}

func (h *ProductHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.IDParam(r, "id")
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	if err := h.svc.Deactivate(r.Context(), id); err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProductHandler) AdjustStock(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.IDParam(r, "id")
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	var req stockAdjustmentRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	p, err := h.svc.AdjustStock(r.Context(), actorFrom(r), id, req.QuantityChange, req.Note)
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, p)
}

func (h *ProductHandler) ListStockMovements(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.IDParam(r, "id")
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	limit, offset, err := httpx.ParsePagination(r)
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	items, total, err := h.svc.ListStockMovements(r.Context(), id, limit, offset)
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	httpx.WritePage(w, items, limit, offset, total)
}
