package handler

import (
	"net/http"

	"github.com/TonnPcst/martgo-backend/internal/httpx"
	"github.com/TonnPcst/martgo-backend/internal/service"
)

type SaleHandler struct{ svc *service.SalesService }

type saleItemRequest struct {
	ProductID int64 `json:"product_id" validate:"required,gt=0"`
	Quantity  int32 `json:"quantity" validate:"required,gt=0,lte=10000"`
}

type createSaleRequest struct {
	PaymentMethod string            `json:"payment_method" validate:"required,oneof=cash card ewallet"`
	Discount      int64             `json:"discount" validate:"gte=0"`
	PaidAmount    int64             `json:"paid_amount" validate:"gte=0"`
	Note          string            `json:"note" validate:"max=500"`
	Items         []saleItemRequest `json:"items" validate:"required,min=1,max=200,dive"`
}

func (h *SaleHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createSaleRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	items := make([]service.SaleItemInput, len(req.Items))
	for i, it := range req.Items {
		items[i] = service.SaleItemInput{ProductID: it.ProductID, Quantity: it.Quantity}
	}
	sale, err := h.svc.Create(r.Context(), service.CreateSaleInput{
		CashierID:     actorFrom(r).UserID,
		PaymentMethod: req.PaymentMethod,
		Discount:      req.Discount,
		PaidAmount:    req.PaidAmount,
		Note:          req.Note,
		Items:         items,
	})
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, sale)
}

func (h *SaleHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.IDParam(r, "id")
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	sale, err := h.svc.Get(r.Context(), actorFrom(r), id)
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, sale)
}

func (h *SaleHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := httpx.ParsePagination(r)
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	cashierID, err := httpx.QueryInt64(r, "cashier_id")
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	sales, total, err := h.svc.List(r.Context(), actorFrom(r), cashierID, limit, offset)
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	httpx.WritePage(w, sales, limit, offset, total)
}
