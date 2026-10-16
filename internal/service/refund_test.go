package service

import (
	"context"
	"testing"

	"github.com/TonnPcst/martgo-backend/internal/apperr"
	"github.com/TonnPcst/martgo-backend/internal/db"
	"github.com/TonnPcst/martgo-backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRefundAmount(t *testing.T) {
	tests := []struct {
		name                                          string
		subtotal, total, prevGross, gross, prevAmount int64
		want                                          int64
	}{
		{name: "no discount, partial", subtotal: 10000, total: 10000, gross: 3000, want: 3000},
		{name: "discount shared in proportion", subtotal: 10000, total: 9000, gross: 3000, want: 2700},
		{name: "rounds down on a partial refund", subtotal: 3, total: 2, gross: 1, want: 0},
		{name: "second partial catches up", subtotal: 3, total: 2, prevGross: 1, gross: 1, want: 1},
		{name: "last refund returns exactly the rest", subtotal: 3, total: 2, prevGross: 2, gross: 1, prevAmount: 1, want: 1},
		{name: "full refund at once", subtotal: 13500, total: 12500, gross: 13500, want: 12500},
		{name: "free sale", subtotal: 0, total: 0, gross: 0, want: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, refundAmount(tc.subtotal, tc.total, tc.prevGross, tc.gross, tc.prevAmount))
		})
	}
}

func TestRefundAmountNeverExceedsTotal(t *testing.T) {
	const unit, qty, discount = 333, 7, 101
	subtotal := int64(unit * qty)
	total := subtotal - discount
	var given, gross int64
	for range qty {
		amt := refundAmount(subtotal, total, gross, unit, given)
		require.GreaterOrEqual(t, amt, int64(0))
		given += amt
		gross += unit
		require.LessOrEqual(t, given, total)
	}
	assert.Equal(t, total, given)
}

func TestRefundLines(t *testing.T) {
	milk := db.SaleItem{ID: 11, ProductID: 1, ProductName: "Milk 1L", Quantity: 3, UnitPrice: 4500}
	bread := db.SaleItem{ID: 12, ProductID: 2, ProductName: "Bread", Quantity: 1, UnitPrice: 3000}
	items := []db.SaleItem{bread, milk}

	t.Run("void takes what is left, restocked, sorted by product", func(t *testing.T) {
		lines, err := refundLines(100, items, map[int64]int32{11: 1, 12: 1}, nil)
		require.NoError(t, err)
		require.Len(t, lines, 1)
		assert.Equal(t, int64(1), lines[0].item.ProductID)
		assert.Equal(t, int32(2), lines[0].qty)
		assert.True(t, lines[0].restock)
	})

	t.Run("void of a fully refuned sale", func(t *testing.T) {
		_, err := refundLines(100, items, map[int64]int32{11: 3, 12: 1}, nil)
		assert.Equal(t, apperr.KindConflict, apperr.KindOf(err))
	})

	t.Run("more than is left", func(t *testing.T) {
		_, err := refundLines(100, items, map[int64]int32{11: 2}, []RefundItemInput{{ProductID: 1, Quantity: 2}})
		assert.Equal(t, apperr.KindConflict, apperr.KindOf(err))
	})

	t.Run("product not on the sale", func(t *testing.T) {
		_, err := refundLines(100, items, nil, []RefundItemInput{{ProductID: 9, Quantity: 1}})
		assert.Equal(t, apperr.KindValidation, apperr.KindOf(err))
	})
}

func TestRefundService_CreateWriteOffDoesNotRestock(t *testing.T) {
	sale := db.Sale{ID: 100, CashierID: 7, Subtotal: 9000, Total: 9000}
	milk := db.SaleItem{ID: 11, SaleID: 100, ProductID: 1, ProductName: "Milk 1L", Quantity: 2, UnitPrice: 4500}

	q := newMockQuerier(t)
	q.On("GetSaleForUpdate", mock.Anything, int64(100)).Return(sale, nil).Once()
	q.On("ListSaleItems", mock.Anything, int64(100)).Return([]db.SaleItem{milk}, nil).Once()
	q.On("ListRefundedQuantities", mock.Anything, int64(100)).Return([]db.ListRefundedQuantitiesRow{}, nil).Once()
	q.On("SumRefundedAmount", mock.Anything, int64(100)).Return(int64(0), nil).Once()
	q.On("CreateRefund", mock.Anything, db.CreateRefundParams{SaleID: 100, RefundedBy: 2, Amount: 4500, Reason: "broken"}).
		Return(db.Refund{ID: 5, SaleID: 100, Amount: 4500}, nil).Once()
	q.On("CreateRefundItem", mock.Anything, db.CreateRefundItemParams{
		RefundID: 5, SaleItemID: 11, ProductID: 1, Quantity: 1, UnitPrice: 4500, LineTotal: 4500, Restock: false,
	}).Return(db.RefundItem{ID: 1, RefundID: 5}, nil).Once()

	got, err := NewRefundService(mockStore{q}).Create(context.Background(), CreateRefundInput{
		SaleID: 100, RefundedBy: 2, Reason: " broken ",
		Items: []RefundItemInput{{ProductID: 1, Quantity: 1, Restock: false}},
	})

	require.NoError(t, err)
	assert.Equal(t, int64(4500), got.Amount)
	q.AssertExpectations(t)
	q.AssertNotCalled(t, "AddProductStock", mock.Anything, mock.Anything)
	q.AssertNotCalled(t, "CreateStockMovement", mock.Anything, mock.Anything)
}

func TestRefundService_ListBySaleHidesOtherCashiers(t *testing.T) {
	q := newMockQuerier(t)
	q.On("GetSale", mock.Anything, int64(100)).Return(db.Sale{ID: 100, CashierID: 7}, nil)

	_, err := NewRefundService(mockStore{q}).ListBySale(context.Background(), Actor{UserID: 8, Role: domain.RoleCashier}, 100)

	assert.Equal(t, apperr.KindForbidden, apperr.KindOf(err))
}
