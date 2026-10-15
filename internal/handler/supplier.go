package handler

import (
	"net/http"

	"github.com/TonnPcst/martgo-backend/internal/httpx"
	"github.com/TonnPcst/martgo-backend/internal/service"
)

type SupplierHandler struct{ svc *service.SupplierService }

type supplierRequest struct {
	Name        string `json:"name" validate:"required,max=200"`
	ContactName string `json:"contact_name" validate:"max=100"`
	Email       string `json:"email" validate:"omitempty,email,max=254"`
	Phone       string `json:"phone" validate:"max=50"`
	Address     string `json:"address" validate:"max=500"`
}

// toInput converts the request body into the service input.
//
// ENG: Plain field copy; trimming is done in the service.
// LAO: ແປງ request body ເປັນ input ຂອງ service.
//
// @returns the supplier input
func (req supplierRequest) toInput() service.SupplierInput {
	return service.SupplierInput{
		Name: req.Name, ContactName: req.ContactName, Email: req.Email, Phone: req.Phone, Address: req.Address,
	}
}

// Create creates a supplier. POST /api/v1/suppliers (admin, manager)
//
// ENG: Decodes and validates the body (name required, email optional but must be valid).
// LAO: ສ້າງຜູ້ສະໜອງໃໝ່.
//
// @param w - response writer
// @param r - request with the supplier JSON body
// @returns writes 201 with the supplier; 400 on bad input
func (h *SupplierHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req supplierRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	s, err := h.svc.Create(r.Context(), req.toInput())
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, s)
}

// Get returns one supplier. GET /api/v1/suppliers/{id} (admin, manager)
//
// ENG: Reads the ID from the path and loads the supplier.
// LAO: ດຶງຂໍ້ມູນຜູ້ສະໜອງຕາມ ID.
//
// @param w - response writer
// @param r - request with the {id} path parameter
// @returns writes 200 with the supplier; 404 if it does not exist
func (h *SupplierHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.IDParam(r, "id")
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	s, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, s)
}

// List returns a page of suppliers. GET /api/v1/suppliers (admin, manager)
//
// ENG: Supports the limit and offset query parameters.
// LAO: ດຶງລາຍການຜູ້ສະໜອງແບບແບ່ງໜ້າ.
//
// @param w - response writer
// @param r - request with optional limit/offset query
// @returns writes 200 with a paginated list
func (h *SupplierHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := httpx.ParsePagination(r)
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	items, total, err := h.svc.List(r.Context(), limit, offset)
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	httpx.WritePage(w, items, limit, offset, total)
}

// Update replaces a supplier's fields. PUT /api/v1/suppliers/{id} (admin, manager)
//
// ENG: Takes the same body as Create.
// LAO: ແກ້ໄຂຂໍ້ມູນຜູ້ສະໜອງ.
//
// @param w - response writer
// @param r - request with the {id} path parameter and the supplier JSON body
// @returns writes 200 with the updated supplier; 404 if it does not exist
func (h *SupplierHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.IDParam(r, "id")
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	var req supplierRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	s, err := h.svc.Update(r.Context(), id, req.toInput())
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, s)
}

// Delete deletes a supplier. DELETE /api/v1/suppliers/{id} (admin, manager)
//
// ENG: Fails with 409 if the supplier already has purchases.
// LAO: ລຶບຜູ້ສະໜອງ — ຖ້າມີປະຫວັດການຊື້ແລ້ວຈະລຶບບໍ່ໄດ້.
//
// @param w - response writer
// @param r - request with the {id} path parameter
// @returns writes 204; 404 if missing, 409 if it has purchases
func (h *SupplierHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.IDParam(r, "id")
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
