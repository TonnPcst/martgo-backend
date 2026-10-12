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

type SalesService struct{ store store.Store }

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
}

type SaleDetail struct {
	db.Sale
	Items []db.SaleItem `json:"items"`
}

func (s *SalesService) Create(ctx context.Context, in CreateSaleInput) (SaleDetail, error) {
	if !domain.IsValidPaymentMethod(in.PaymentMethod) {
		return SaleDetail{}, apperr.Validation("invalid payment method %q", in.PaymentMethod)
	}
	if in.Discount < 0 || in.PaidAmount < 0 {
		return SaleDetail{}, apperr.Validation("discount and paid amount must not be negative")
	}
	lines, err := mergeSaleItems(in.Items)
	if err != nil {
		return SaleDetail{}, err
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
			CashierID:     in.CashierID,
			PaymentMethod: in.PaymentMethod,
			Subtotal:      subtotal,
			Discount:      in.Discount,
			Total:         total,
			PaidAmount:    paid,
			ChangeAmount:  paid - total,
			Note:          strings.TrimSpace(in.Note),
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
	return out, err
}

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
