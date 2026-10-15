package handler

import (
	"net/http"

	"github.com/TonnPcst/martgo-backend/internal/httpx"
	"github.com/TonnPcst/martgo-backend/internal/service"
)

type CategoryHandler struct{ svc *service.CategoryService }

type categoryRequest struct {
	Name        string `json:"name" validate:"required,max=100"`
	Description string `json:"description" validate:"max=500"`
}

// toInput converts the request body into the service input.
//
// ENG: Plain field copy; trimming is done in the service.
// LAO: ແປງ request body ເປັນ input ຂອງ service.
//
// @returns the category input
func (req categoryRequest) toInput() service.CategoryInput {
	return service.CategoryInput{Name: req.Name, Description: req.Description}
}

// Create creates a category. POST /api/v1/categories (admin, manager)
//
// ENG: Decodes and validates the body (name required, max 100 chars) and creates the category.
// LAO: ສ້າງໝວດໝູ່ສິນຄ້າໃໝ່.
//
// @param w - response writer
// @param r - request with the category JSON body
// @returns writes 201 with the category; 400 on bad input, 409 if the name exists
func (h *CategoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req categoryRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	c, err := h.svc.Create(r.Context(), req.toInput())
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, c)
}

// Get returns one category. GET /api/v1/categories/{id}
//
// ENG: Reads the ID from the path and loads the category.
// LAO: ດຶງຂໍ້ມູນໝວດໝູ່ຕາມ ID.
//
// @param w - response writer
// @param r - request with the {id} path parameter
// @returns writes 200 with the category; 404 if it does not exist
func (h *CategoryHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.IDParam(r, "id")
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	c, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, c)
}

// List returns a page of categories. GET /api/v1/categories
//
// ENG: Supports the limit and offset query parameters.
// LAO: ດຶງລາຍການໝວດໝູ່ແບບແບ່ງໜ້າ.
//
// @param w - response writer
// @param r - request with optional limit/offset query
// @returns writes 200 with a paginated list
func (h *CategoryHandler) List(w http.ResponseWriter, r *http.Request) {
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

// Update replaces a category's fields. PUT /api/v1/categories/{id} (admin, manager)
//
// ENG: Takes the same body as Create.
// LAO: ແກ້ໄຂຂໍ້ມູນໝວດໝູ່.
//
// @param w - response writer
// @param r - request with the {id} path parameter and the category JSON body
// @returns writes 200 with the updated category; 404 if missing, 409 if the name exists
func (h *CategoryHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.IDParam(r, "id")
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	var req categoryRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	c, err := h.svc.Update(r.Context(), id, req.toInput())
	if err != nil {
		httpx.HandleError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, c)
}

// Delete deletes a category. DELETE /api/v1/categories/{id} (admin, manager)
//
// ENG: Fails with 409 if products still use the category.
// LAO: ລຶບໝວດໝູ່ — ຖ້າຍັງມີສິນຄ້າໃຊ້ຢູ່ຈະລຶບບໍ່ໄດ້.
//
// @param w - response writer
// @param r - request with the {id} path parameter
// @returns writes 204; 404 if missing, 409 if still in use
func (h *CategoryHandler) Delete(w http.ResponseWriter, r *http.Request) {
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
