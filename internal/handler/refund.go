package handler

import (
	"net/http"

	"github.com/TonnPcst/martgo-backend/internal/httpx"
	"github.com/TonnPcst/martgo-backend/internal/service"
)

type RefundHandler struct{ svc *service.RefundService }

type refundItemRequest struct {
	ProductID int64 `json:"product_id" validate:"required,gt=0"`
	Quantity  int32 `json:"quantity" validate:"required,gt=0,lte=10000"`
	Restock   *bool `json:"restock"`
}

type createRefundRequest struct {
	Reason string              `json:"reason" validate:"required,max=500"`
	Items  []refundItemRequest `json:"items" validate:"max=200,dive"`
}

// Create refunds some or all of a sale. POST /api/v1/sales/{id}/refunds
//
// ENG: Body: reason (required) and optional items with product_id, quantity and
// restock (default true; false writes damaged items off). No items refunds
// everything not yet refunded. Admin and manager only.
// LAO: ຄືນເງິນການຂາຍ — ບໍ່ສົ່ງ items ແມ່ນຄືນທັງໝົດທີ່ເຫຼືອ (void).
//
// @param w - response writer
// @param r - request with the {id} path parameter and refund JSON body
// @returns writes 201 with the refund; 404 if the sale is missing, 409 if more is
// refunded than is left
func (h *RefundHandler) Create(w http.ResponseWriter, r *http.Request) {
	saleID, err := httpx.IDParam(r, "id")
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	var req createRefundRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	items := make([]service.RefundItemInput, len(req.Items))
	for i, it := range req.Items {
		items[i] = service.RefundItemInput{
			ProductID: it.ProductID,
			Quantity:  it.Quantity,
			Restock:   it.Restock == nil || *it.Restock,
		}
	}
	refund, err := h.svc.Create(r.Context(), service.CreateRefundInput{
		SaleID:     saleID,
		RefundedBy: actorFrom(r).UserID,
		Reason:     req.Reason,
		Items:      items,
	})
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, refund)
}

// List returns every refund of a sale. GET /api/v1/sales/{id}/refunds
//
// ENG: Cashiers can only see refunds of their own sales.
// LAO: ດຶງລາຍການຄືນເງິນຂອງການຂາຍ.
//
// @param w - response writer
// @param r - request with the {id} path parameter
// @returns writes 200 with {"data": [...]}; 403 for another cashier's sale, 404 if missing
func (h *RefundHandler) List(w http.ResponseWriter, r *http.Request) {
	saleID, err := httpx.IDParam(r, "id")
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	refunds, err := h.svc.ListBySale(r.Context(), actorFrom(r), saleID)
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": refunds})
}
