package handler

import (
	"net/http"

	"github.com/TonnPcst/martgo-backend/internal/httpx"
	"github.com/TonnPcst/martgo-backend/internal/service"
	"github.com/go-chi/chi/v5"
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

// toInput converts the request body into the service input.
//
// ENG: is_active defaults to true when it is not sent.
// LAO: ແປງ request body ເປັນ input ຂອງ service — is_active ຄ່າເລີ່ມຕົ້ນແມ່ນ true.
//
// @returns the product input
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

// Create creates a product. POST /api/v1/products (admin, manager)
//
// ENG: Decodes and validates the body (sku, name and price are required) and creates the product.
// LAO: ສ້າງສິນຄ້າໃໝ່.
//
// @param w - response writer
// @param r - request with the product JSON body
// @returns writes 201 with the product; 400 on bad input, 409 if the SKU or barcode exists
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

// Get returns one product. GET /api/v1/products/{id}
//
// ENG: Reads the ID from the path and loads the product.
// LAO: ດຶງຂໍ້ມູນສິນຄ້າຕາມ ID.
//
// @param w - response writer
// @param r - request with the {id} path parameter
// @returns writes 200 with the product; 404 if it does not exist
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

// GetByBarcode finds a product by its barcode. GET /api/v1/products/barcode/{barcode}
//
// ENG: Used by the POS when a cashier scans an item.
// LAO: ຄົ້ນຫາສິນຄ້າດ້ວຍ barcode (ໃຊ້ຕອນສະແກນສິນຄ້າ).
//
// @param w - response writer
// @param r - request with the {barcode} path parameter
// @returns writes 200 with the product; 404 if no product has that barcode
func (h *ProductHandler) GetByBarcode(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.GetByBarcode(r.Context(), chi.URLParam(r, "barcode"))
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, p)
}

// List returns a page of products. GET /api/v1/products
//
// ENG: Optional filters: search (partial name or SKU, or exact barcode), category_id
// and is_active, plus limit and offset. Sorted by name.
// LAO: ດຶງລາຍການສິນຄ້າແບບແບ່ງໜ້າ ພ້ອມຕົວກອງ search, category_id, is_active.
//
// @param w - response writer
// @param r - request with optional filter and pagination query
// @returns writes 200 with a paginated list; 400 on a bad query value
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

// ListLowStock returns products that need reordering. GET /api/v1/products/low-stock
//
// ENG: An active product is low on stock when its quantity is at or below its
// reorder level. Lowest stock comes first.
// LAO: ດຶງລາຍການສິນຄ້າທີ່ໃກ້ໝົດ (ສະຕັອກ <= ຈຸດສັ່ງຊື້ໃໝ່).
//
// @param w - response writer
// @param r - request with optional limit/offset query
// @returns writes 200 with a paginated list
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

// Update replaces a product's fields. PUT /api/v1/products/{id} (admin, manager)
//
// ENG: Takes the same body as Create. Stock is not changed here; use AdjustStock.
// LAO: ແກ້ໄຂຂໍ້ມູນສິນຄ້າ (ບໍ່ລວມຈຳນວນສະຕັອກ).
//
// @param w - response writer
// @param r - request with the {id} path parameter and the product JSON body
// @returns writes 200 with the updated product; 404 if missing, 409 if the SKU or barcode exists
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

// Deactivate soft-deletes a product. DELETE /api/v1/products/{id} (admin, manager)
//
// ENG: Sets is_active to false instead of deleting, so old sales keep their product.
// LAO: ປິດການໃຊ້ງານສິນຄ້າ (soft delete) ແທນການລຶບ.
//
// @param w - response writer
// @param r - request with the {id} path parameter
// @returns writes 204; 404 if the product does not exist
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

// AdjustStock manually changes a product's stock. POST /api/v1/products/{id}/stock-adjustments (admin, manager)
//
// ENG: Body: quantity_change (positive to add, negative to remove, not zero) and a
// required note explaining why. The change is recorded as a stock movement.
// LAO: ປັບຈຳນວນສະຕັອກດ້ວຍມື (ເພີ່ມ ຫຼື ຫຼຸດ) ພ້ອມໝາຍເຫດ.
//
// @param w - response writer
// @param r - request with the {id} path parameter and the adjustment JSON body
// @returns writes 200 with the updated product; 409 if the stock would go negative
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

// ListStockMovements returns a product's stock history. GET /api/v1/products/{id}/stock-movements (admin, manager)
//
// ENG: Lists sales, purchases and manual adjustments for the product, newest first.
// LAO: ດຶງປະຫວັດການເຄື່ອນໄຫວສະຕັອກຂອງສິນຄ້າ (ຂາຍ, ຊື້, ປັບ).
//
// @param w - response writer
// @param r - request with the {id} path parameter and optional limit/offset query
// @returns writes 200 with a paginated list; 404 if the product does not exist
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
