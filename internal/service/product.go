package service

import (
	"context"
	"strings"

	"github.com/TonnPcst/martgo-backend/internal/apperr"
	"github.com/TonnPcst/martgo-backend/internal/db"
	"github.com/TonnPcst/martgo-backend/internal/domain"
	"github.com/TonnPcst/martgo-backend/internal/store"
)

type ProductService struct{ store store.Store }

// NewProductService creates the product service.
//
// ENG: All data access goes through the given store.
// LAO: ສ້າງ service ສຳລັບຈັດການສິນຄ້າ.
//
// @param s - data store
// @returns the service
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

// normalize cleans up and validates a product input in place.
//
// ENG: Trims sku, name and unit, defaults unit to "pcs" and turns a blank barcode into
// nil. Then checks that sku and name are set, price is above 0, and cost and
// reorder level are not negative.
// LAO: ທຳຄວາມສະອາດ ແລະ ກວດສອບຂໍ້ມູນສິນຄ້າ — unit ຄ່າເລີ່ມຕົ້ນ "pcs", ລາຄາຕ້ອງ > 0.
//
// @returns nil, or a validation error
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

// Create creates a new product.
//
// ENG: Normalizes and validates the input, checks that the category exists, then
// inserts the product with zero stock.
// LAO: ສ້າງສິນຄ້າໃໝ່ — ກວດສອບຂໍ້ມູນ, ກວດໝວດໝູ່, ແລ້ວບັນທຶກລົງ DB.
//
// @param ctx - request context
// @param in - product fields
// @returns the created product, a validation error, or Conflict if the SKU or barcode exists
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

// Update replaces a product's fields.
//
// ENG: Same checks as Create. Stock quantity is not touched; use AdjustStock for that.
// LAO: ແກ້ໄຂຂໍ້ມູນສິນຄ້າ (ບໍ່ປ່ຽນຈຳນວນສະຕັອກ).
//
// @param ctx - request context
// @param id - product ID
// @param in - new product fields
// @returns the updated product, a validation error, NotFound, or Conflict
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

// Get loads a product by ID.
//
// ENG: Returns NotFound when there is no such product.
// LAO: ດຶງສິນຄ້າຕາມ ID.
//
// @param ctx - request context
// @param id - product ID
// @returns the product, or a NotFound error
func (s *ProductService) Get(ctx context.Context, id int64) (db.Product, error) {
	p, err := s.store.GetProduct(ctx, id)
	return p, dbError(err, "product")
}

// GetByBarcode loads a product by its barcode.
//
// ENG: The barcode is trimmed before the lookup.
// LAO: ຄົ້ນຫາສິນຄ້າດ້ວຍ barcode.
//
// @param ctx - request context
// @param barcode - scanned barcode
// @returns the product, or a NotFound error
func (s *ProductService) GetByBarcode(ctx context.Context, barcode string) (db.Product, error) {
	p, err := s.store.GetProductByBarcode(ctx, strings.TrimSpace(barcode))
	return p, dbError(err, "product")
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// likePattern builds a SQL LIKE pattern that matches search anywhere in the text.
//
// ENG: Escapes \, % and _ so they are matched literally, then wraps the result in %...%.
// LAO: ສ້າງ pattern ສຳລັບ SQL LIKE (escape ຕົວອັກສອນພິເສດ) ເພື່ອຄົ້ນຫາ.
//
// @param search - user search text, may be nil
// @returns the pattern, or nil when search is nil
func likePattern(search *string) *string {
	if search == nil {
		return nil
	}
	p := "%" + likeEscaper.Replace(*search) + "%"
	return &p
}

// List returns one page of products matching the filter, and the total count.
//
// ENG: All filters are optional and combined with AND: search text (partial name or
// SKU, or exact barcode), category and active status. Sorted by name.
// LAO: ດຶງລາຍການສິນຄ້າຕາມຕົວກອງ ແບບແບ່ງໜ້າ ພ້ອມຈຳນວນທັງໝົດ.
//
// @param ctx - request context
// @param f - search, category and active filters
// @param limit - page size
// @param offset - number of rows to skip
// @returns the page items and the total number of matching products
func (s *ProductService) List(ctx context.Context, f ProductFilter, limit, offset int32) ([]db.Product, int64, error) {
	pattern := likePattern(f.Search)
	items, err := s.store.ListProducts(ctx, db.ListProductsParams{
		Search:     f.Search,
		Pattern:    pattern,
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
		Pattern:    pattern,
		CategoryID: f.CategoryID,
		IsActive:   f.IsActive,
	})
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ListLowStock returns one page of active products at or below their reorder level.
//
// ENG: Used to decide what to buy from suppliers next.
// LAO: ດຶງລາຍການສິນຄ້າທີ່ສະຕັອກໃກ້ໝົດ ພ້ອມຈຳນວນທັງໝົດ.
//
// @param ctx - request context
// @param limit - page size
// @param offset - number of rows to skip
// @returns the page items and the total number of low-stock products
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

// Deactivate marks a product as inactive.
//
// ENG: Inactive products stay in the database but cannot be sold.
// LAO: ປິດການໃຊ້ງານສິນຄ້າ — ຍັງຢູ່ໃນ DB ແຕ່ຂາຍບໍ່ໄດ້.
//
// @param ctx - request context
// @param id - product ID
// @returns nil, or a NotFound error
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

// AdjustStock adds or removes stock by hand and records why.
//
// ENG: In one transaction: locks the product row, checks the stock will not go
// negative, updates the quantity and writes an "adjustment" stock movement with
// the note and the user who made it.
// LAO: ປັບສະຕັອກດ້ວຍມື ໃນ transaction — ລັອກແຖວ, ກວດບໍ່ໃຫ້ຕິດລົບ, ບັນທຶກການເຄື່ອນໄຫວ.
//
// @param ctx - request context
// @param actor - user making the change
// @param id - product ID
// @param delta - change in quantity, must not be zero
// @param note - reason for the adjustment
// @returns the updated product, a validation error, NotFound, or Conflict if stock would go negative
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

// ListStockMovements returns one page of a product's stock history.
//
// ENG: Checks the product exists first so a wrong ID gives 404 instead of an empty list.
// LAO: ດຶງປະຫວັດການເຄື່ອນໄຫວສະຕັອກຂອງສິນຄ້າ ແບບແບ່ງໜ້າ.
//
// @param ctx - request context
// @param productID - product ID
// @param limit - page size
// @param offset - number of rows to skip
// @returns the page items and the total count, or NotFound
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

// checkCategory makes sure the category exists when one is given.
//
// ENG: A nil ID means "no category" and is always accepted.
// LAO: ກວດວ່າໝວດໝູ່ທີ່ລະບຸມີຢູ່ແທ້ (ຖ້າບໍ່ລະບຸຖືວ່າຜ່ານ).
//
// @param ctx - request context
// @param id - category ID, may be nil
// @returns nil, or a validation error if the category does not exist
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

// productError converts a database error from a product write.
//
// ENG: Gives a clearer message for a duplicate SKU or barcode, otherwise falls back to dbError.
// LAO: ແປງຂໍ້ຜິດພາດຂອງ DB ສຳລັບສິນຄ້າ — SKU/barcode ຊ້ຳຈະບອກຊັດເຈນ.
//
// @param err - error from the store, may be nil
// @returns the converted error, or nil
func productError(err error) error {
	if store.IsUniqueViolation(err) {
		return apperr.Conflict("a product with this SKU or barcode already exists")
	}
	return dbError(err, "product")
}
