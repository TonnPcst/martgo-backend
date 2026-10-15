package service

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"github.com/TonnPcst/martgo-backend/internal/apperr"
	"github.com/TonnPcst/martgo-backend/internal/db"
	"github.com/TonnPcst/martgo-backend/internal/domain"
	"github.com/TonnPcst/martgo-backend/internal/store"
)

type PurchaseService struct{ store store.Store }

// NewPurchaseService creates the purchase service.
//
// ENG: All data access goes through the given store.
// LAO: ສ້າງ service ສຳລັບຈັດການການຊື້ສິນຄ້າ.
//
// @param s - data store
// @returns the service
func NewPurchaseService(s store.Store) *PurchaseService { return &PurchaseService{store: s} }

type PurchaseItemInput struct {
	ProductID int64
	Quantity  int32
	UnitCost  int64
}

type CreatePurchaseInput struct {
	SupplierID int64
	CreatedBy  int64
	InvoiceNo  string
	Note       string
	Items      []PurchaseItemInput
}

type PurchaseDetail struct {
	db.Purchase
	Items []db.PurchaseItem `json:"items"`
}

// Create records a purchase from a supplier and adds the items to stock.
//
// ENG: Validates the items, then in one transaction: checks the supplier exists,
// locks the products (sorted by ID to avoid deadlocks), computes the total, saves
// the purchase and its items, increases stock (updating the cost) and writes a
// "purchase" stock movement for each item.
// LAO: ບັນທຶກການຊື້ ໃນ transaction — ກວດຜູ້ສະໜອງ, ລັອກສິນຄ້າ, ຄິດໄລ່ຍອດ, ເພີ່ມສະຕັອກ.
//
// @param ctx - request context
// @param in - supplier, creator, invoice number, note and items
// @returns the purchase with its items, a validation error, or NotFound for a missing supplier or product
func (s *PurchaseService) Create(ctx context.Context, in CreatePurchaseInput) (PurchaseDetail, error) {
	if err := validatePurchaseItems(in.Items); err != nil {
		return PurchaseDetail{}, err
	}
	items := slices.Clone(in.Items)
	slices.SortFunc(items, func(a, b PurchaseItemInput) int { return cmp.Compare(a.ProductID, b.ProductID) })

	var out PurchaseDetail
	err := s.store.ExecTx(ctx, func(q db.Querier) error {
		if _, err := q.GetSupplier(ctx, in.SupplierID); err != nil {
			if store.IsNotFound(err) {
				return apperr.NotFound("supplier %d not found", in.SupplierID)
			}
			return err
		}

		ids := make([]int64, len(items))
		for i, it := range items {
			ids[i] = it.ProductID
		}
		products, err := q.GetProductsForUpdate(ctx, ids)
		if err != nil {
			return err
		}
		found := make(map[int64]bool, len(products))
		for _, p := range products {
			found[p.ID] = true
		}
		var total int64
		for _, it := range items {
			if !found[it.ProductID] {
				return apperr.NotFound("product %d not found", it.ProductID)
			}
			total += it.UnitCost * int64(it.Quantity)
		}

		purchase, err := q.CreatePurchase(ctx, db.CreatePurchaseParams{
			SupplierID: in.SupplierID,
			CreatedBy:  in.CreatedBy,
			InvoiceNo:  strings.TrimSpace(in.InvoiceNo),
			Total:      total,
			Note:       strings.TrimSpace(in.Note),
		})
		if err != nil {
			return err
		}

		created := make([]db.PurchaseItem, 0, len(items))
		for _, it := range items {
			item, err := q.CreatePurchaseItem(ctx, db.CreatePurchaseItemParams{
				PurchaseID: purchase.ID,
				ProductID:  it.ProductID,
				Quantity:   it.Quantity,
				UnitCost:   it.UnitCost,
				LineTotal:  it.UnitCost * int64(it.Quantity),
			})
			if err != nil {
				return err
			}
			if _, err := q.ReceiveProductStock(ctx, db.ReceiveProductStockParams{
				ID: it.ProductID, Quantity: it.Quantity, UnitCost: it.UnitCost,
			}); err != nil {
				return err
			}
			if err := q.CreateStockMovement(ctx, db.CreateStockMovementParams{
				ProductID:      it.ProductID,
				QuantityChange: it.Quantity,
				Reason:         domain.StockReasonPurchase,
				ReferenceID:    &purchase.ID,
				CreatedBy:      in.CreatedBy,
			}); err != nil {
				return err
			}
			created = append(created, item)
		}

		out = PurchaseDetail{Purchase: purchase, Items: created}
		return nil
	})
	return out, err
}

// Get loads a purchase with its line items.
//
// ENG: Returns NotFound when there is no such purchase.
// LAO: ດຶງຂໍ້ມູນການຊື້ ພ້ອມລາຍການສິນຄ້າ.
//
// @param ctx - request context
// @param id - purchase ID
// @returns the purchase detail, or a NotFound error
func (s *PurchaseService) Get(ctx context.Context, id int64) (PurchaseDetail, error) {
	purchase, err := s.store.GetPurchase(ctx, id)
	if err != nil {
		return PurchaseDetail{}, dbError(err, "purchase")
	}
	items, err := s.store.ListPurchaseItems(ctx, id)
	if err != nil {
		return PurchaseDetail{}, err
	}
	return PurchaseDetail{Purchase: purchase, Items: items}, nil
}

// List returns one page of purchases and the total count.
//
// ENG: Optionally filtered by supplier.
// LAO: ດຶງລາຍການການຊື້ແບບແບ່ງໜ້າ — ກອງຕາມຜູ້ສະໜອງໄດ້.
//
// @param ctx - request context
// @param supplierID - supplier filter, nil for all
// @param limit - page size
// @param offset - number of rows to skip
// @returns the page items and the total number of matching purchases
func (s *PurchaseService) List(ctx context.Context, supplierID *int64, limit, offset int32) ([]db.Purchase, int64, error) {
	purchase, err := s.store.ListPurchases(ctx, db.ListPurchasesParams{
		SupplierID: supplierID,
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.store.CountPurchases(ctx, supplierID)
	if err != nil {
		return nil, 0, err
	}
	return purchase, total, nil
}

// validatePurchaseItems checks the purchase lines before anything is written.
//
// ENG: Needs at least one item. Each item must have a valid product ID, quantity
// above 0 and a unit cost that is not negative, and a product may appear only once.
// LAO: ກວດລາຍການສິນຄ້າໃນການຊື້ — ຕ້ອງມີຢ່າງໜ້ອຍ 1, ຈຳນວນ > 0, ສິນຄ້າຫ້າມຊ້ຳ.
//
// @param items - purchase lines
// @returns nil, or a validation error
func validatePurchaseItems(items []PurchaseItemInput) error {
	if len(items) == 0 {
		return apperr.Validation("a purchase must have at least one item")
	}
	seen := make(map[int64]bool, len(items))
	for _, it := range items {
		switch {
		case it.ProductID <= 0:
			return apperr.Validation("invalid product id %d", it.ProductID)
		case it.Quantity <= 0:
			return apperr.Validation("quantity for product %d must be greater than 0", it.ProductID)
		case it.UnitCost < 0:
			return apperr.Validation("unit cost for product %d must not be negative", it.ProductID)
		case seen[it.ProductID]:
			return apperr.Validation("product %d appears more than once", it.ProductID)
		}
		seen[it.ProductID] = true
	}
	return nil
}
