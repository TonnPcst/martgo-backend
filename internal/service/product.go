package service

import (
	"context"
	"strings"

	"github.com/TonnPcst/martgo/internal/apperr"
	"github.com/TonnPcst/martgo/internal/db"
	"github.com/TonnPcst/martgo/internal/domain"
	"github.com/TonnPcst/martgo/internal/store"
)

type ProductService struct{ store store.Store }

func NewProductService(s store.Store) *ProductService { return &ProductService{store: s} }

type ProductInput struct {
	Sku          string
	Barcode      *string
	Name         string
	Description  string
	CategoryID   *int64
	Unit         string
	Price        int64
	Cost         int64
	ReorderLevel int32
	IsActive     bool
}

type ProductFilter struct {
	Search     *string
	CategoryID *int64
	IsActive   *bool
}

func (in *ProductInput) normalize() error {
	in.Sku = strings.TrimSpace(in.Sku)
	in.Name = strings.TrimSpace(in.Name)
	in.Unit = strings.TrimSpace(in.Unit)
	if in.Unit == "" {
		in.Unit = "pcs"
	}
	if in.Barcode != nil {
		b := strings.TrimSpace(*in.Barcode)
		if b == "" {
			in.Barcode = nil
		} else {
			in.Barcode = &b
		}
	}
	switch {
	case in.Sku == "" || in.Name == "":
		return apperr.Validation("sku and name are required")
	case in.Price <= 0:
		return apperr.Validation("price must be greater than 0")
	case in.Cost < 0 || in.ReorderLevel < 0:
		return apperr.Validation("cost and reorder_level must not be negative")
	}
	return nil
}

func (s *ProductService) Create(ctx context.Context, in ProductInput) (db.Product, error) {
	if err := in.normalize(); err != nil {
		return db.Product{}, err
	}
	if err := s.checkCategory(ctx, in.CategoryID); err != nil {
		return db.Product{}, err
	}
	p, err := s.store.CreateProduct(ctx, db.CreateProductParams{
		Sku:          in.Sku,
		Barcode:      in.Barcode,
		Name:         in.Name,
		Description:  in.Description,
		CategoryID:   in.CategoryID,
		Unit:         in.Unit,
		Price:        in.Price,
		Cost:         in.Cost,
		ReorderLevel: in.ReorderLevel,
	})
	return p, productError(err)
}

func (s *ProductService) Update(ctx context.Context, id int64, in ProductInput) (db.Product, error) {
	if err := in.normalize(); err != nil {
		return db.Product{}, err
	}
	if err := s.checkCategory(ctx, in.CategoryID); err != nil {
		return db.Product{}, err
	}
	p, err := s.store.UpdateProduct(ctx, db.UpdateProductParams{
		ID:           id,
		Sku:          in.Sku,
		Barcode:      in.Barcode,
		Name:         in.Name,
		Description:  in.Description,
		CategoryID:   in.CategoryID,
		Unit:         in.Unit,
		Price:        in.Price,
		Cost:         in.Cost,
		ReorderLevel: in.ReorderLevel,
		IsActive:     in.IsActive,
	})
	return p, productError(err)
}

func (s *ProductService) Get(ctx context.Context, id int64) (db.Product, error) {
	p, err := s.store.GetProduct(ctx, id)
	return p, dbError(err, "product")
}

func (s *ProductService) GetByBarcode(ctx context.Context, barcode string) (db.Product, error) {
	p, err := s.store.GetProductByBarcode(ctx, strings.TrimSpace(barcode))
	return p, dbError(err, "product")
}

func (s *ProductService) List(ctx context.Context, f ProductFilter, limit, offset int32) ([]db.Product, int64, error) {
	items, err := s.store.ListProducts(ctx, db.ListProductsParams{
		Search:     f.Search,
		CategoryID: f.CategoryID,
		IsActive:   f.IsActive,
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.store.CountProducts(ctx, db.CountProductsParams{
		Search:     f.Search,
		CategoryID: f.CategoryID,
		IsActive:   f.IsActive,
	})
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *ProductService) ListLowStock(ctx context.Context, limit, offset int32) ([]db.Product, int64, error) {
	items, err := s.store.ListLowStockProducts(ctx, db.ListLowStockProductsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.store.CountLowStockProducts(ctx)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *ProductService) Deactivate(ctx context.Context, id int64) error {
	n, err := s.store.DeactivateProduct(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return apperr.NotFound("product not found")
	}
	return nil
}

func (s *ProductService) AdjustStock(ctx context.Context, actor Actor, id int64, delta int32, note string) (db.Product, error) {
	if delta == 0 {
		return db.Product{}, apperr.Validation("quantity_change must not be zero")
	}
	var out db.Product
	err := s.store.ExecTx(ctx, func(q db.Querier) error {
		p, err := q.GetProductForUpdate(ctx, id)
		if err != nil {
			return dbError(err, "product")
		}
		if int64(p.StockQuantity)+int64(delta) < 0 {
			return apperr.Conflict("adjustment would make stock negative: current stock is %d", p.StockQuantity)
		}
		out, err = q.AddProductStock(ctx, db.AddProductStockParams{
			ID:    id,
			Delta: delta,
		})
		if err != nil {
			return err
		}
		return q.CreateStockMovement(ctx, db.CreateStockMovementParams{
			ProductID:      id,
			QuantityChange: delta,
			Reason:         domain.StockReasonAdjustment,
			Note:           strings.TrimSpace(note),
			CreatedBy:      actor.UserID,
		})
	})
	return out, err
}

func (s *ProductService) ListStockMovements(ctx context.Context, productID int64, limit, offset int32) ([]db.StockMovement, int64, error) {
	if _, err := s.Get(ctx, productID); err != nil {
		return nil, 0, err
	}
	items, err := s.store.ListStockMovementsByProduct(ctx, db.ListStockMovementsByProductParams{
		ProductID: productID, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.store.CountStockMovementsByProduct(ctx, productID)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *ProductService) checkCategory(ctx context.Context, id *int64) error {
	if id == nil {
		return nil
	}
	if _, err := s.store.GetCategory(ctx, *id); err != nil {
		if store.IsNotFound(err) {
			return apperr.Validation("category %d does not exist", *id)
		}
		return err
	}
	return nil
}

func productError(err error) error {
	if store.IsUniqueViolation(err) {
		return apperr.Conflict("a product with this SKU or barcode already exists")
	}
	return dbError(err, "product")
}
