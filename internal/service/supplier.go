package service

import (
	"context"
	"strings"

	"github.com/TonnPcst/martgo-backend/internal/apperr"
	"github.com/TonnPcst/martgo-backend/internal/db"
	"github.com/TonnPcst/martgo-backend/internal/store"
)

type SupplierService struct{ store store.Store }

// NewSupplierService creates the supplier service.
//
// ENG: All data access goes through the given store.
// LAO: ສ້າງ service ສຳລັບຈັດການຜູ້ສະໜອງ.
//
// @param s - data store
// @returns the service
func NewSupplierService(s store.Store) *SupplierService { return &SupplierService{store: s} }

type SupplierInput struct {
	Name        string
	ContactName string
	Email       string
	Phone       string
	Address     string
}

// Create creates a new supplier.
//
// ENG: Trims all fields and lower-cases the email before saving.
// LAO: ສ້າງຜູ້ສະໜອງໃໝ່ (ຕັດຍະຫວ່າງ, email ເປັນຕົວພິມນ້ອຍ).
//
// @param ctx - request context
// @param in - supplier fields
// @returns the created supplier, or an error
func (s *SupplierService) Create(ctx context.Context, in SupplierInput) (db.Supplier, error) {
	sup, err := s.store.CreateSupplier(ctx, db.CreateSupplierParams{
		Name:        strings.TrimSpace(in.Name),
		ContactName: strings.TrimSpace(in.ContactName),
		Email:       normalizeEmail(in.Email),
		Phone:       strings.TrimSpace(in.Phone),
		Address:     strings.TrimSpace(in.Address),
	})
	return sup, dbError(err, "supplier")
}

// Get loads a supplier by ID.
//
// ENG: Returns NotFound when there is no such supplier.
// LAO: ດຶງຜູ້ສະໜອງຕາມ ID.
//
// @param ctx - request context
// @param id - supplier ID
// @returns the supplier, or a NotFound error
func (s *SupplierService) Get(ctx context.Context, id int64) (db.Supplier, error) {
	sup, err := s.store.GetSupplier(ctx, id)
	return sup, dbError(err, "supplier")
}

// List returns one page of suppliers and the total count.
//
// ENG: Runs the page query and the count query separately.
// LAO: ດຶງລາຍການຜູ້ສະໜອງແບບແບ່ງໜ້າ ພ້ອມຈຳນວນທັງໝົດ.
//
// @param ctx - request context
// @param limit - page size
// @param offset - number of rows to skip
// @returns the page items and the total number of suppliers
func (s *SupplierService) List(ctx context.Context, limit, offset int32) ([]db.Supplier, int64, error) {
	items, err := s.store.ListSuppliers(ctx, db.ListSuppliersParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.store.CountSuppliers(ctx)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Update replaces a supplier's fields.
//
// ENG: Same clean-up as Create.
// LAO: ແກ້ໄຂຂໍ້ມູນຜູ້ສະໜອງ.
//
// @param ctx - request context
// @param id - supplier ID
// @param in - new supplier fields
// @returns the updated supplier, or a NotFound error
func (s *SupplierService) Update(ctx context.Context, id int64, in SupplierInput) (db.Supplier, error) {
	sup, err := s.store.UpdateSupplier(ctx, db.UpdateSupplierParams{
		ID:          id,
		Name:        strings.TrimSpace(in.Name),
		ContactName: strings.TrimSpace(in.ContactName),
		Email:       normalizeEmail(in.Email),
		Phone:       strings.TrimSpace(in.Phone),
		Address:     strings.TrimSpace(in.Address),
	})
	return sup, dbError(err, "supplier")
}

// Delete removes a supplier.
//
// ENG: Returns Conflict if the supplier has purchases, so purchase history is never lost.
// LAO: ລຶບຜູ້ສະໜອງ — ຖ້າມີປະຫວັດການຊື້ແລ້ວຈະລຶບບໍ່ໄດ້.
//
// @param ctx - request context
// @param id - supplier ID
// @returns nil, NotFound, or Conflict
func (s *SupplierService) Delete(ctx context.Context, id int64) error {
	n, err := s.store.DeleteSupplier(ctx, id)
	if store.IsForeignKeyViolation(err) {
		return apperr.Conflict("supplier has purchase and cannot be deleted")
	}
	if err != nil {
		return err
	}
	if n == 0 {
		return apperr.NotFound("supplier not found")
	}
	return nil
}
