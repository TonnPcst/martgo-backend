package handler

import (
	"net/http"

	"github.com/TonnPcst/martgo/internal/httpx"
	"github.com/TonnPcst/martgo/internal/service"
)

type PurchaseHandler struct{ svc *service.PurchaseService }

type purchaseItemRequest struct {
	ProductID int64 `json:"product_id" validate:"required,gt=0"`
	Quantity  int32 `json:"quantity" validate:"required,gt=0,lte=1000000"`
	UnitCost  int64 `json:"unit_cost" validate:"gte=0"`
}

type createPurchaseRequest struct {
	SupplierID int64                 `json:"supplier_id" validate:"required,gt=0"`
	InvoiceNo  string                `json:"invoice_no" validate:"max=100"`
	Note       string                `json:"note" validate:"max=500"`
	Items      []purchaseItemRequest `json:"items" validate:"required,min=1,max=500,dive"`
}

func (h *PurchaseHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createPurchaseRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	items := make([]service.PurchaseItemInput, len(req.Items))
	for i, it := range req.Items {
		items[i] = service.PurchaseItemInput{ProductID: it.ProductID, Quantity: it.Quantity, UnitCost: it.UnitCost}
	}
	purchase, err := h.svc.Create(r.Context(), service.CreatePurchaseInput{
		SupplierID: req.SupplierID,
		CreatedBy:  actorFrom(r).UserID,
		InvoiceNo:  req.InvoiceNo,
		Note:       req.Note,
		Items:      items,
	})
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, purchase)
}

func (h *PurchaseHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.IDParam(r, "id")
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	purchase, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, purchase)
}

func (h *PurchaseHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := httpx.ParsePagination(r)
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	supplierID, err := httpx.QueryInt64(r, "supplier_id")
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	items, total, err := h.svc.List(r.Context(), supplierID, limit, offset)
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	httpx.WritePage(w, items, limit, offset, total)
}
