package handler

import (
	"net/http"

	"github.com/TonnPcst/martgo/internal/httpx"
	"github.com/TonnPcst/martgo/internal/service"
)

type SupplierHandler struct{ svc *service.SupplierService }

type supplierRequest struct {
	Name        string `json:"name" validate:"required,max=200"`
	ContactName string `json:"contact_name" validate:"max=100"`
	Email       string `json:"email" validate:"omitempty,email,max=254"`
	Phone       string `json:"phone" validate:"max=50"`
	Address     string `json:"address" validate:"max=500"`
}

func (req supplierRequest) toInput() service.SupplierInput {
	return service.SupplierInput{
		Name: req.Name, ContactName: req.ContactName, Email: req.Email, Phone: req.Phone, Address: req.Address,
	}
}

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
