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

// Create records a sale at the checkout. POST /api/v1/sales
//
// ENG: Body: payment_method (cash, card, ewallet), discount, paid_amount, note and
// 1-200 items with product_id and quantity. The logged-in user is saved as the
// cashier. Prices come from the database, not from the client.
// An optional Idempotency-Key header makes retries safe: a repeat of the same
// request returns the original sale with 200 and "Idempotent-Replayed: true".
// LAO: ບັນທຶກການຂາຍ — ລາຄາເອົາຈາກ DB, ຕັດສະຕັອກ, ຄິດໄລ່ເງິນທອນ. ສົ່ງ Idempotency-Key ເພື່ອ retry ໄດ້ປອດໄພ.
//
// @param w - response writer
// @param r - request with the sale JSON body and optional Idempotency-Key header
// @returns writes 201 with the sale and its items, 200 for a replayed retry; 409 if
// stock is not enough or the key was used for a different sale
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
		CashierID:      actorFrom(r).UserID,
		PaymentMethod:  req.PaymentMethod,
		Discount:       req.Discount,
		PaidAmount:     req.PaidAmount,
		Note:           req.Note,
		Items:          items,
		IdempotencyKey: r.Header.Get("Idempotency-Key"),
	})
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	if sale.Replayed {
		w.Header().Set("Idempotent-Replayed", "true")
		httpx.JSON(w, http.StatusOK, sale)
		return
	}
	httpx.JSON(w, http.StatusCreated, sale)
}

// Get returns one sale with its items. GET /api/v1/sales/{id}
//
// ENG: Cashiers can only see their own sales.
// LAO: ດຶງຂໍ້ມູນການຂາຍ — cashier ເບິ່ງໄດ້ສະເພາະການຂາຍຂອງຕົນເອງ.
//
// @param w - response writer
// @param r - request with the {id} path parameter
// @returns writes 200 with the sale; 403 for another cashier's sale, 404 if missing
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

// List returns a page of sales. GET /api/v1/sales
//
// ENG: Optional cashier_id filter. For cashiers the filter is always forced to
// their own ID.
// LAO: ດຶງລາຍການການຂາຍແບບແບ່ງໜ້າ — cashier ເຫັນສະເພາະຂອງຕົນເອງ.
//
// @param w - response writer
// @param r - request with optional cashier_id and pagination query
// @returns writes 200 with a paginated list
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
