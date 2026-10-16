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

type RefundService struct{ store store.Store }

// NewRefundService creates the refund service.
//
// ENG: All data access goes through the given store.
// LAO: ສ້າງ service ສຳລັບຈັດການການຄືນເງິນ.
//
// @param s - data store
// @returns the service
func NewRefundService(s store.Store) *RefundService { return &RefundService{store: s} }

type RefundItemInput struct {
	ProductID int64
	Quantity  int32
	Restock   bool
}

type CreateRefundInput struct {
	SaleID     int64
	RefundedBy int64
	Reason     string
	Items      []RefundItemInput
}

type RefundDetail struct {
	db.Refund
	Items []db.RefundItem `json:"items"`
}

type refundLine struct {
	item    db.SaleItem
	qty     int32
	restock bool
}

// Create refunds some or all of a sale.
//
// ENG: In one transaction: locks the sale (so two refunds of the same sale run one
// after the other), checks each line was on the sale and has not been refunded
// already, works out the amount to give back, saves the refund and its items, and
// for restocked lines adds the stock back and writes a "refund" stock movement.
// The sale-level discount is shared out in proportion: partial refunds never add up
// to more than the sale total, and refunding everything returns exactly the total.
// LAO: ຄືນເງິນບາງສ່ວນ ຫຼື ທັງໝົດຂອງການຂາຍ — ກວດຈຳນວນທີ່ຄືນໄດ້, ຄິດໄລ່ຍອດເງິນ, ຄືນສະຕັອກຖ້າ restock.
//
// @param ctx - request context
// @param in - sale, who refunds, reason and items (empty = refund everything left)
// @returns the refund with its items, a validation error, NotFound, or Conflict when
// more is refunded than is left on the sale
func (s *RefundService) Create(ctx context.Context, in CreateRefundInput) (RefundDetail, error) {
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return RefundDetail{}, apperr.Validation("a refund needs a reason")
	}
	seen := make(map[int64]bool, len(in.Items))
	for _, it := range in.Items {
		if it.ProductID <= 0 {
			return RefundDetail{}, apperr.Validation("invalid product id %d", it.ProductID)
		}
		if it.Quantity <= 0 {
			return RefundDetail{}, apperr.Validation("quantity for product %d must be greater than 0", it.ProductID)
		}
		if seen[it.ProductID] {
			return RefundDetail{}, apperr.Validation("product %d appears more than once", it.ProductID)
		}
		seen[it.ProductID] = true
	}

	var out RefundDetail
	err := s.store.ExecTx(ctx, func(q db.Querier) error {
		sale, err := q.GetSaleForUpdate(ctx, in.SaleID)
		if err != nil {
			return dbError(err, "sale")
		}
		saleItems, err := q.ListSaleItems(ctx, sale.ID)
		if err != nil {
			return err
		}
		done, err := q.ListRefundedQuantities(ctx, sale.ID)
		if err != nil {
			return err
		}
		refunded := make(map[int64]int32, len(done))
		for _, d := range done {
			refunded[d.SaleItemID] = d.Quantity
		}

		lines, err := refundLines(sale.ID, saleItems, refunded, in.Items)
		if err != nil {
			return err
		}

		var prevGross, gross int64
		for _, si := range saleItems {
			prevGross += si.UnitPrice * int64(refunded[si.ID])
		}
		for _, l := range lines {
			gross += l.item.UnitPrice * int64(l.qty)
		}
		prevAmount, err := q.SumRefundedAmount(ctx, sale.ID)
		if err != nil {
			return err
		}

		refund, err := q.CreateRefund(ctx, db.CreateRefundParams{
			SaleID:     sale.ID,
			RefundedBy: in.RefundedBy,
			Amount:     refundAmount(sale.Subtotal, sale.Total, prevGross, gross, prevAmount),
			Reason:     reason,
		})
		if err != nil {
			return err
		}

		items := make([]db.RefundItem, 0, len(lines))
		for _, l := range lines {
			item, err := q.CreateRefundItem(ctx, db.CreateRefundItemParams{
				RefundID:   refund.ID,
				SaleItemID: l.item.ID,
				ProductID:  l.item.ProductID,
				Quantity:   l.qty,
				UnitPrice:  l.item.UnitPrice,
				LineTotal:  l.item.UnitPrice * int64(l.qty),
				Restock:    l.restock,
			})
			if err != nil {
				return err
			}
			if l.restock {
				if _, err := q.AddProductStock(ctx, db.AddProductStockParams{ID: l.item.ProductID, Delta: l.qty}); err != nil {
					return err
				}
				if err := q.CreateStockMovement(ctx, db.CreateStockMovementParams{
					ProductID:      l.item.ProductID,
					QuantityChange: l.qty,
					Reason:         domain.StockReasonRefund,
					ReferenceID:    &refund.ID,
					Note:           reason,
					CreatedBy:      in.RefundedBy,
				}); err != nil {
					return err
				}
			}
			items = append(items, item)
		}

		out = RefundDetail{Refund: refund, Items: items}
		return nil
	})
	return out, err
}

// ListBySale returns every refund of a sale, oldest first.
//
// ENG: Same visibility as the sale: cashiers may only see refunds of their own sales.
// LAO: ດຶງລາຍການຄືນເງິນຂອງການຂາຍ — cashier ເບິ່ງໄດ້ສະເພາະການຂາຍຂອງຕົນເອງ.
//
// @param ctx - request context
// @param actor - user asking
// @param saleID - sale ID
// @returns the refunds with their items, NotFound, or Forbidden
func (s *RefundService) ListBySale(ctx context.Context, actor Actor, saleID int64) ([]RefundDetail, error) {
	sale, err := s.store.GetSale(ctx, saleID)
	if err != nil {
		return nil, dbError(err, "sale")
	}
	if actor.Role == domain.RoleCashier && sale.CashierID != actor.UserID {
		return nil, apperr.Forbidden("cashiers can only view their own sales")
	}
	refunds, err := s.store.ListRefundsBySale(ctx, saleID)
	if err != nil {
		return nil, err
	}
	items, err := s.store.ListRefundItemsBySale(ctx, saleID)
	if err != nil {
		return nil, err
	}

	out := make([]RefundDetail, len(refunds))
	index := make(map[int64]int, len(refunds))
	for i, r := range refunds {
		out[i] = RefundDetail{Refund: r, Items: []db.RefundItem{}}
		index[r.ID] = i
	}
	for _, it := range items {
		i := index[it.RefundID]
		out[i].Items = append(out[i].Items, it)
	}
	return out, nil
}

// refundLines matches the requested items to the sale's lines.
//
// ENG: With no items requested, takes everything not yet refunded (restocked). Each
// requested product must be on the sale, and its quantity must not exceed what is
// left to refund. Lines are sorted by product ID so stock rows are always updated
// in the same order as sales and purchases lock them.
// LAO: ຈັບຄູ່ສິນຄ້າທີ່ຂໍຄືນກັບລາຍການຂາຍ ແລະ ກວດວ່າບໍ່ຄືນເກີນທີ່ເຫຼືອ.
//
// @param saleID - sale being refunded (for messages)
// @param saleItems - lines of the sale
// @param refunded - quantity already refunded per sale item ID
// @param req - requested items, or empty for everything left
// @returns the lines to refund, a validation error, or Conflict
func refundLines(saleID int64, saleItems []db.SaleItem, refunded map[int64]int32, req []RefundItemInput) ([]refundLine, error) {
	var lines []refundLine
	if len(req) == 0 {
		for _, si := range saleItems {
			if left := si.Quantity - refunded[si.ID]; left > 0 {
				lines = append(lines, refundLine{item: si, qty: left, restock: true})
			}
		}
		if len(lines) == 0 {
			return nil, apperr.Conflict("sale %d is already fully refunded", saleID)
		}
	} else {
		byProduct := make(map[int64]db.SaleItem, len(saleItems))
		for _, si := range saleItems {
			byProduct[si.ProductID] = si
		}
		for _, it := range req {
			si, ok := byProduct[it.ProductID]
			if !ok {
				return nil, apperr.Validation("product %d is not on sale %d", it.ProductID, saleID)
			}
			if left := si.Quantity - refunded[si.ID]; it.Quantity > left {
				return nil, apperr.Conflict("cannot refund %d of %q: only %d left to refund", it.Quantity, si.ProductName, left)
			}
			lines = append(lines, refundLine{item: si, qty: it.Quantity, restock: it.Restock})
		}
	}
	slices.SortFunc(lines, func(a, b refundLine) int { return cmp.Compare(a.item.ProductID, b.item.ProductID) })
	return lines, nil
}

// refundAmount works out how much money a refund gives back.
//
// ENG: Shares the sale's discount out in proportion to the goods refunded, measured
// cumulatively: after this refund, the sale has given back floor(refunded goods ×
// total / subtotal) in all, minus what earlier refunds already gave. Rounding never
// drifts, and once every item is refunded the sum is exactly the sale total.
// LAO: ຄິດໄລ່ເງິນທີ່ຕ້ອງຄືນ — ແບ່ງສ່ວນຫຼຸດຕາມສັດສ່ວນ, ຄືນທັງໝົດຈະໄດ້ເທົ່າກັບຍອດຂາຍພໍດີ.
//
// @param subtotal - sale subtotal (before discount)
// @param total - sale total (after discount)
// @param prevGross - goods value already refunded, at sale prices
// @param gross - goods value of this refund, at sale prices
// @param prevAmount - money already given back by earlier refunds
// @returns the amount for this refund
func refundAmount(subtotal, total, prevGross, gross, prevAmount int64) int64 {
	after := prevGross + gross
	if after >= subtotal {
		return total - prevAmount
	}
	return after*total/subtotal - prevAmount
}
