package service

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"

	"github.com/TonnPcst/martgo-backend/internal/apperr"
	"github.com/TonnPcst/martgo-backend/internal/db"
	"github.com/TonnPcst/martgo-backend/internal/domain"
	"github.com/TonnPcst/martgo-backend/internal/store"
)

type SalesService struct{ store store.Store }

// NewSaleService creates the sales service.
//
// ENG: All data access goes through the given store.
// LAO: ສ້າງ service ສຳລັບຈັດການການຂາຍ.
//
// @param s - data store
// @returns the service
func NewSaleService(s store.Store) *SalesService { return &SalesService{store: s} }

type SaleItemInput struct {
	ProductID int64
	Quantity  int32
}

type CreateSaleInput struct {
	CashierID     int64
	PaymentMethod string
	Discount      int64
	PaidAmount    int64
	Note          string
	Items         []SaleItemInput
	// IdempotencyKey is optional. A retry with the same key returns the sale
	// already recorded instead of creating a second one.
	IdempotencyKey string
}

type SaleDetail struct {
	db.Sale
	Items []db.SaleItem `json:"items"`
	// Replayed is true when an earlier sale with the same idempotency key was
	// returned instead of creating a new one.
	Replayed bool `json:"-"`
}

const maxIdempotencyKeyLen = 255

// Create records a sale and takes the items out of stock.
//
// ENG: Validates the payment method and amounts, merges duplicate items, then in one
// transaction: locks the products (sorted by ID to avoid deadlocks), checks each is
// active and has enough stock, computes subtotal, discount, total and change, saves
// the sale and its items (with the product name and price at the time of sale),
// reduces stock and writes a "sale" stock movement for each item.
// With an idempotency key, a sale this cashier already recorded under that key is
// returned (Replayed) instead; the lookup runs after the product locks, so a retry
// racing the original waits for it. Reusing a key for a different request is a Conflict.
// LAO: ບັນທຶກການຂາຍ ໃນ transaction — ກວດສະຕັອກ, ຄິດໄລ່ຍອດ/ສ່ວນຫຼຸດ/ເງິນທອນ, ຕັດສະຕັອກ.
// ຖ້າສົ່ງ idempotency key ຊ້ຳ ຈະສົ່ງການຂາຍເກົ່າຄືນ ບໍ່ຂາຍຊ້ຳ.
//
// @param ctx - request context
// @param in - cashier, payment method, discount, paid amount, note, items and optional idempotency key
// @returns the sale with its items, a validation error, NotFound, or Conflict if stock is not enough
// or the idempotency key was used for a different sale
func (s *SalesService) Create(ctx context.Context, in CreateSaleInput) (SaleDetail, error) {
	if !domain.IsValidPaymentMethod(in.PaymentMethod) {
		return SaleDetail{}, apperr.Validation("invalid payment method %q", in.PaymentMethod)
	}
	if in.Discount < 0 || in.PaidAmount < 0 {
		return SaleDetail{}, apperr.Validation("discount and paid amount must not be negative")
	}
	if err := validateIdempotencyKey(in.IdempotencyKey); err != nil {
		return SaleDetail{}, err
	}
	lines, err := mergeSaleItems(in.Items)
	if err != nil {
		return SaleDetail{}, err
	}

	var key, hash *string
	if in.IdempotencyKey != "" {
		h := saleFingerprint(in, lines)
		key, hash = &in.IdempotencyKey, &h
	}

	var out SaleDetail
	err = s.store.ExecTx(ctx, func(q db.Querier) error {
		ids := make([]int64, len(lines))
		for i, l := range lines {
			ids[i] = l.ProductID
		}
		products, err := q.GetProductsForUpdate(ctx, ids)
		if err != nil {
			return err
		}

		if key != nil {
			prev, err := q.GetSaleByIdempotencyKey(ctx, db.GetSaleByIdempotencyKeyParams{CashierID: in.CashierID, IdempotencyKey: key})
			if err == nil {
				out, err = replaySale(ctx, q, prev, *hash)
				return err
			}
			if !store.IsNotFound(err) {
				return err
			}
		}

		byID := make(map[int64]db.Product, len(products))
		for _, p := range products {
			byID[p.ID] = p
		}

		var subtotal int64
		for _, l := range lines {
			p, ok := byID[l.ProductID]
			if !ok {
				return apperr.NotFound("product %d not found", l.ProductID)
			}
			if !p.IsActive {
				return apperr.Validation("product %q is not available for sale", p.Name)
			}
			if p.StockQuantity < l.Quantity {
				return apperr.Conflict("insufficient stock for %q: requested %d, available %d", p.Name, l.Quantity, p.StockQuantity)
			}
			subtotal += p.Price * int64(l.Quantity)
		}

		if in.Discount > subtotal {
			return apperr.Validation("discount %d exceeds subtotal %d", in.Discount, subtotal)
		}
		total := subtotal - in.Discount
		paid, err := settlePayment(in.PaymentMethod, total, in.PaidAmount)
		if err != nil {
			return err
		}

		sale, err := q.CreateSale(ctx, db.CreateSaleParams{
			CashierID:      in.CashierID,
			PaymentMethod:  in.PaymentMethod,
			Subtotal:       subtotal,
			Discount:       in.Discount,
			Total:          total,
			PaidAmount:     paid,
			ChangeAmount:   paid - total,
			Note:           strings.TrimSpace(in.Note),
			IdempotencyKey: key,
			RequestHash:    hash,
		})
		if err != nil {
			return err
		}

		items := make([]db.SaleItem, 0, len(lines))
		for _, l := range lines {
			p := byID[l.ProductID]
			item, err := q.CreateSaleItem(ctx, db.CreateSaleItemParams{
				SaleID:      sale.ID,
				ProductID:   p.ID,
				ProductName: p.Name,
				Quantity:    l.Quantity,
				UnitPrice:   p.Price,
				LineTotal:   p.Price * int64(l.Quantity),
			})
			if err != nil {
				return err
			}
			if _, err := q.AddProductStock(ctx, db.AddProductStockParams{ID: p.ID, Delta: -l.Quantity}); err != nil {
				return err
			}
			if err := q.CreateStockMovement(ctx, db.CreateStockMovementParams{
				ProductID:      p.ID,
				QuantityChange: -l.Quantity,
				Reason:         domain.StockReasonSale,
				ReferenceID:    &sale.ID,
				CreatedBy:      in.CashierID,
			}); err != nil {
				return err
			}
			items = append(items, item)
		}

		out = SaleDetail{Sale: sale, Items: items}
		return nil
	})
	if err != nil && key != nil && store.IsUniqueViolation(err) {
		// Another request with this key committed first (e.g. a retry for different
		// products, which the product locks do not serialize). Our transaction rolled back.
		prev, lookupErr := s.store.GetSaleByIdempotencyKey(ctx, db.GetSaleByIdempotencyKeyParams{CashierID: in.CashierID, IdempotencyKey: key})
		if lookupErr == nil {
			return replaySale(ctx, s.store, prev, *hash)
		}
	}
	return out, err
}

// replaySale returns a sale already recorded under the same idempotency key.
//
// ENG: The stored request hash must match, otherwise the key was reused for a
// different sale and the caller gets a Conflict instead of someone else's receipt.
// LAO: ສົ່ງການຂາຍທີ່ບັນທຶກແລ້ວຄືນ — ຖ້າ request ບໍ່ກົງກັບຂອງເກົ່າ ຕອບ Conflict.
//
// @param ctx - request context
// @param q - queries to load the items with (inside or outside a transaction)
// @param prev - the sale found by idempotency key
// @param hash - fingerprint of the current request
// @returns the earlier sale marked Replayed, or Conflict if the requests differ
func replaySale(ctx context.Context, q db.Querier, prev db.Sale, hash string) (SaleDetail, error) {
	if prev.RequestHash == nil || *prev.RequestHash != hash {
		return SaleDetail{}, apperr.Conflict("idempotency key %q was already used for a different sale", *prev.IdempotencyKey)
	}
	items, err := q.ListSaleItems(ctx, prev.ID)
	if err != nil {
		return SaleDetail{}, err
	}
	return SaleDetail{Sale: prev, Items: items, Replayed: true}, nil
}

// validateIdempotencyKey checks an optional idempotency key.
//
// ENG: Empty means "no key". Otherwise 1-255 printable ASCII characters (a UUID fits).
// LAO: ກວດ idempotency key — ວ່າງໄດ້, ຖ້າມີຕ້ອງເປັນ ASCII ທີ່ພິມໄດ້ ບໍ່ເກີນ 255 ຕົວ.
//
// @param key - the key from the request
// @returns nil if valid, or a validation error
func validateIdempotencyKey(key string) error {
	if len(key) > maxIdempotencyKeyLen {
		return apperr.Validation("idempotency key must be at most %d characters", maxIdempotencyKeyLen)
	}
	for i := 0; i < len(key); i++ {
		if key[i] < 0x21 || key[i] > 0x7e {
			return apperr.Validation("idempotency key must contain only printable ASCII characters without spaces")
		}
	}
	return nil
}

// saleFingerprint hashes the parts of a sale request that decide what is sold.
//
// ENG: Uses the merged, sorted lines, so the same basket in a different order or
// with split lines gives the same hash. The cashier is not included: keys are
// already scoped to the cashier.
// LAO: ສ້າງ hash ຂອງ request ການຂາຍ ເພື່ອກວດວ່າ request ທີ່ສົ່ງຊ້ຳເປັນອັນດຽວກັນບໍ່.
//
// @param in - the sale request
// @param lines - merged and sorted sale lines
// @returns the hex SHA-256 of the canonical request
func saleFingerprint(in CreateSaleInput, lines []SaleItemInput) string {
	b, _ := json.Marshal(struct {
		PaymentMethod string          `json:"payment_method"`
		Discount      int64           `json:"discount"`
		PaidAmount    int64           `json:"paid_amount"`
		Note          string          `json:"note"`
		Items         []SaleItemInput `json:"items"`
	}{in.PaymentMethod, in.Discount, in.PaidAmount, strings.TrimSpace(in.Note), lines})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// Get loads a sale with its line items.
//
// ENG: Cashiers may only view sales they made themselves.
// LAO: ດຶງຂໍ້ມູນການຂາຍ — cashier ເບິ່ງໄດ້ສະເພາະຂອງຕົນເອງ.
//
// @param ctx - request context
// @param actor - user asking for the sale
// @param id - sale ID
// @returns the sale detail, NotFound, or Forbidden
func (s *SalesService) Get(ctx context.Context, actor Actor, id int64) (SaleDetail, error) {
	sale, err := s.store.GetSale(ctx, id)
	if err != nil {
		return SaleDetail{}, dbError(err, "sale")
	}
	if actor.Role == domain.RoleCashier && sale.CashierID != actor.UserID {
		return SaleDetail{}, apperr.Forbidden("cashiers can only view their own sales")
	}
	items, err := s.store.ListSaleItems(ctx, id)
	if err != nil {
		return SaleDetail{}, err
	}
	return SaleDetail{Sale: sale, Items: items}, nil
}

// List returns one page of sales and the total count.
//
// ENG: Optionally filtered by cashier. For a cashier the filter is always their own
// ID, whatever they asked for.
// LAO: ດຶງລາຍການການຂາຍແບບແບ່ງໜ້າ — cashier ເຫັນສະເພາະຂອງຕົນເອງ.
//
// @param ctx - request context
// @param actor - user asking for the list
// @param cashierID - cashier filter, nil for all (ignored for cashiers)
// @param limit - page size
// @param offset - number of rows to skip
// @returns the page items and the total number of matching sales
func (s *SalesService) List(ctx context.Context, actor Actor, cashierID *int64, limit, offset int32) ([]db.Sale, int64, error) {
	if actor.Role == domain.RoleCashier {
		cashierID = &actor.UserID
	}
	sales, err := s.store.ListSales(ctx, db.ListSalesParams{CashierID: cashierID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.store.CountSales(ctx, cashierID)
	if err != nil {
		return nil, 0, err
	}
	return sales, total, nil
}

// mergeSaleItems validates the sale lines and merges repeated products.
//
// ENG: Two lines for the same product are added together. The result is sorted by
// product ID so rows are always locked in the same order.
// LAO: ກວດ ແລະ ລວມລາຍການສິນຄ້າທີ່ຊ້ຳກັນ, ຮຽງຕາມ ID ສິນຄ້າ.
//
// @param items - sale lines from the request
// @returns one line per product, or a validation error
func mergeSaleItems(items []SaleItemInput) ([]SaleItemInput, error) {
	if len(items) == 0 {
		return nil, apperr.Validation("a sale must have at least one item")
	}
	qty := make(map[int64]int32, len(items))
	for _, it := range items {
		if it.ProductID <= 0 {
			return nil, apperr.Validation("invalid product id %d", it.ProductID)
		}
		if it.Quantity <= 0 {
			return nil, apperr.Validation("quantity for product %d must be greater than 0", it.ProductID)
		}
		qty[it.ProductID] += it.Quantity
	}
	lines := make([]SaleItemInput, 0, len(qty))
	for id, q := range qty {
		lines = append(lines, SaleItemInput{ProductID: id, Quantity: q})
	}
	slices.SortFunc(lines, func(a, b SaleItemInput) int { return cmp.Compare(a.ProductID, b.ProductID) })
	return lines, nil
}

// settlePayment checks the paid amount and returns what was actually paid.
//
// ENG: Cash must cover the total (the rest is change). Card and e-wallet must pay the
// exact total; a paid amount of 0 means "charge the total".
// LAO: ກວດຈຳນວນເງິນທີ່ຈ່າຍ — ເງິນສົດຕ້ອງພໍ (ມີເງິນທອນ), ບັດ/e-wallet ຕ້ອງເທົ່າກັບຍອດ.
//
// @param method - payment method
// @param total - amount due after discount
// @param paid - amount the customer gave
// @returns the paid amount to store, or a validation error
func settlePayment(method string, total, paid int64) (int64, error) {
	if method == domain.PaymentCash {
		if paid < total {
			return 0, apperr.Validation("paid amount %d is less than total %d", paid, total)
		}
		return paid, nil
	}
	if paid == 0 {
		return total, nil
	}
	if paid != total {
		return 0, apperr.Validation("%s payments must equal the total %d", method, total)
	}
	return paid, nil
}
