package service

import (
	"context"
	"strings"

	"github.com/TonnPcst/martgo/internal/apperr"
	"github.com/TonnPcst/martgo/internal/db"
	"github.com/TonnPcst/martgo/internal/store"
)

type SupplierService struct{ store store.Store }

func NewSupplierService(s store.Store) *SupplierService { return &SupplierService{store: s} }

type SupplierInput struct {
	Name        string
	ContactName string
	Email       string
	Phone       string
	Address     string
}

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

func (s *SupplierService) Get(ctx context.Context, id int64) (db.Supplier, error) {
	sup, err := s.store.GetSupplier(ctx, id)
	return sup, dbError(err, "supplier")
}

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
