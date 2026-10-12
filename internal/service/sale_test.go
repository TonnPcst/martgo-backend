package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/TonnPcst/martgo/internal/apperr"
	"github.com/TonnPcst/martgo/internal/db"
	"github.com/TonnPcst/martgo/internal/domain"
)

const testSaleID int64 = 100

func TestSaleService_Create(t *testing.T) {
	milk := db.Product{ID: 1, Name: "Milk 1L", Price: 4500, StockQuantity: 10, IsActive: true}
	bread := db.Product{ID: 2, Name: "Bread", Price: 3000, StockQuantity: 2, IsActive: true}
	soda := db.Product{ID: 3, Name: "Old Soda", Price: 1500, StockQuantity: 50, IsActive: false}

	tests := []struct {
		name          string
		input         CreateSaleInput
		products      []db.Product // rows returned by GetProductsForUpdate
		createSaleErr error
		wantSale      *db.CreateSaleParams
		wantItems     []db.CreateSaleItemParams
		wantErr       bool
		wantKind      apperr.Kind
	}{
		{
			name: "cash sale computes totals and change",
			input: CreateSaleInput{CashierID: 7, PaymentMethod: domain.PaymentCash, PaidAmount: 20000,
				Items: []SaleItemInput{{ProductID: 2, Quantity: 1}, {ProductID: 1, Quantity: 2}}},
			products: []db.Product{milk, bread},
			wantSale: &db.CreateSaleParams{CashierID: 7, PaymentMethod: domain.PaymentCash,
				Subtotal: 12000, Total: 12000, PaidAmount: 20000, ChangeAmount: 8000},
			wantItems: []db.CreateSaleItemParams{
				{SaleID: testSaleID, ProductID: 1, ProductName: "Milk 1L", Quantity: 2, UnitPrice: 4500, LineTotal: 9000},
				{SaleID: testSaleID, ProductID: 2, ProductName: "Bread", Quantity: 1, UnitPrice: 3000, LineTotal: 3000},
			},
		},
		{
			name: "duplicate lines are merged and card is charged the exact total",
			input: CreateSaleInput{CashierID: 7, PaymentMethod: domain.PaymentCard,
				Items: []SaleItemInput{{ProductID: 1, Quantity: 1}, {ProductID: 1, Quantity: 2}}},
			products: []db.Product{milk},
			wantSale: &db.CreateSaleParams{CashierID: 7, PaymentMethod: domain.PaymentCard,
				Subtotal: 13500, Total: 13500, PaidAmount: 13500},
			wantItems: []db.CreateSaleItemParams{
				{SaleID: testSaleID, ProductID: 1, ProductName: "Milk 1L", Quantity: 3, UnitPrice: 4500, LineTotal: 13500},
			},
		},
		{
			name: "discount reduces the total",
			input: CreateSaleInput{CashierID: 7, PaymentMethod: domain.PaymentCash, Discount: 1000, PaidAmount: 8000,
				Items: []SaleItemInput{{ProductID: 1, Quantity: 2}}},
			products: []db.Product{milk},
			wantSale: &db.CreateSaleParams{CashierID: 7, PaymentMethod: domain.PaymentCash,
				Subtotal: 9000, Discount: 1000, Total: 8000, PaidAmount: 8000},
			wantItems: []db.CreateSaleItemParams{
				{SaleID: testSaleID, ProductID: 1, ProductName: "Milk 1L", Quantity: 2, UnitPrice: 4500, LineTotal: 9000},
			},
		},
		{
			name:     "no items",
			input:    CreateSaleInput{CashierID: 7, PaymentMethod: domain.PaymentCash},
			wantErr:  true,
			wantKind: apperr.KindValidation,
		},
		{
			name: "non-positive quantity",
			input: CreateSaleInput{CashierID: 7, PaymentMethod: domain.PaymentCash,
				Items: []SaleItemInput{{ProductID: 1, Quantity: 0}}},
			wantErr:  true,
			wantKind: apperr.KindValidation,
		},
		{
			name: "unsupported payment method",
			input: CreateSaleInput{CashierID: 7, PaymentMethod: "cheque",
				Items: []SaleItemInput{{ProductID: 1, Quantity: 1}}},
			wantErr:  true,
			wantKind: apperr.KindValidation,
		},
		{
			name: "unknown product",
			input: CreateSaleInput{CashierID: 7, PaymentMethod: domain.PaymentCash, PaidAmount: 10000,
				Items: []SaleItemInput{{ProductID: 99, Quantity: 1}}},
			products: []db.Product{},
			wantErr:  true,
			wantKind: apperr.KindNotFound,
		},
		{
			name: "inactive product",
			input: CreateSaleInput{CashierID: 7, PaymentMethod: domain.PaymentCash, PaidAmount: 10000,
				Items: []SaleItemInput{{ProductID: 3, Quantity: 1}}},
			products: []db.Product{soda},
			wantErr:  true,
			wantKind: apperr.KindValidation,
		},
		{
			name: "insufficient stock",
			input: CreateSaleInput{CashierID: 7, PaymentMethod: domain.PaymentCash, PaidAmount: 10000,
				Items: []SaleItemInput{{ProductID: 2, Quantity: 3}}},
			products: []db.Product{bread},
			wantErr:  true,
			wantKind: apperr.KindConflict,
		},
		{
			name: "discount exceeds subtotal",
			input: CreateSaleInput{CashierID: 7, PaymentMethod: domain.PaymentCash, Discount: 5000, PaidAmount: 0,
				Items: []SaleItemInput{{ProductID: 2, Quantity: 1}}},
			products: []db.Product{bread},
			wantErr:  true,
			wantKind: apperr.KindValidation,
		},
		{
			name: "cash underpaid",
			input: CreateSaleInput{CashierID: 7, PaymentMethod: domain.PaymentCash, PaidAmount: 4000,
				Items: []SaleItemInput{{ProductID: 1, Quantity: 1}}},
			products: []db.Product{milk},
			wantErr:  true,
			wantKind: apperr.KindValidation,
		},
		{
			name: "card amount must equal total",
			input: CreateSaleInput{CashierID: 7, PaymentMethod: domain.PaymentCard, PaidAmount: 5000,
				Items: []SaleItemInput{{ProductID: 1, Quantity: 1}}},
			products: []db.Product{milk},
			wantErr:  true,
			wantKind: apperr.KindValidation,
		},
		{
			name: "database failure is returned as internal error",
			input: CreateSaleInput{CashierID: 7, PaymentMethod: domain.PaymentCash, PaidAmount: 4500,
				Items: []SaleItemInput{{ProductID: 1, Quantity: 1}}},
			products: []db.Product{milk},
			wantSale: &db.CreateSaleParams{CashierID: 7, PaymentMethod: domain.PaymentCash,
				Subtotal: 4500, Total: 4500, PaidAmount: 4500},
			createSaleErr: errors.New("connection reset"),
			wantErr:       true,
			wantKind:      apperr.KindInternal,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := newMockQuerier(t)
			q.On("GetProductsForUpdate", mock.Anything, mock.Anything).Return(tc.products, nil).Maybe()
			if tc.wantSale != nil {
				q.On("CreateSale", mock.Anything, *tc.wantSale).Return(saleFrom(*tc.wantSale), tc.createSaleErr).Once()
			}
			for _, item := range tc.wantItems {
				expectSaleItem(q, item, tc.input.CashierID)
			}

			got, err := NewSaleService(mockStore{q}).Create(context.Background(), tc.input)

			if tc.wantErr {
				require.Error(t, err)
				assert.Equal(t, tc.wantKind, apperr.KindOf(err), "unexpected error: %v", err)
				if tc.createSaleErr == nil {
					q.AssertNotCalled(t, "CreateSale", mock.Anything, mock.Anything)
				}
				q.AssertNotCalled(t, "AddProductStock", mock.Anything, mock.Anything)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, testSaleID, got.ID)
			assert.Equal(t, tc.wantSale.Total, got.Total)
			assert.Equal(t, tc.wantSale.ChangeAmount, got.ChangeAmount)
			assert.Len(t, got.Items, len(tc.wantItems))
			q.AssertExpectations(t)
		})
	}
}

func TestSaleService_Get(t *testing.T) {
	sale := db.Sale{ID: testSaleID, CashierID: 7, Total: 4500}

	tests := []struct {
		name     string
		actor    Actor
		getErr   error
		wantErr  bool
		wantKind apperr.Kind
	}{
		{name: "admin can view any sale", actor: Actor{UserID: 1, Role: domain.RoleAdmin}},
		{name: "cashier can view own sale", actor: Actor{UserID: 7, Role: domain.RoleCashier}},
		{
			name:     "cashier cannot view another cashier's sale",
			actor:    Actor{UserID: 8, Role: domain.RoleCashier},
			wantErr:  true,
			wantKind: apperr.KindForbidden,
		},
		{
			name:     "missing sale",
			actor:    Actor{UserID: 1, Role: domain.RoleAdmin},
			getErr:   pgx.ErrNoRows,
			wantErr:  true,
			wantKind: apperr.KindNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := newMockQuerier(t)
			q.On("GetSale", mock.Anything, testSaleID).Return(sale, tc.getErr)
			q.On("ListSaleItems", mock.Anything, testSaleID).
				Return([]db.SaleItem{{ID: 1, SaleID: testSaleID}}, nil).Maybe()

			got, err := NewSaleService(mockStore{q}).Get(context.Background(), tc.actor, testSaleID)

			if tc.wantErr {
				require.Error(t, err)
				assert.Equal(t, tc.wantKind, apperr.KindOf(err))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, testSaleID, got.ID)
			assert.Len(t, got.Items, 1)
		})
	}
}

func saleFrom(p db.CreateSaleParams) db.Sale {
	return db.Sale{
		ID: testSaleID, CashierID: p.CashierID, PaymentMethod: p.PaymentMethod,
		Subtotal: p.Subtotal, Discount: p.Discount, Total: p.Total,
		PaidAmount: p.PaidAmount, ChangeAmount: p.ChangeAmount, Note: p.Note,
	}
}

// expectSaleItem expects the item insert, stock decrement and audit row for one line.
func expectSaleItem(q *mockQuerier, p db.CreateSaleItemParams, cashierID int64) {
	q.On("CreateSaleItem", mock.Anything, p).Return(db.SaleItem{
		ID: p.ProductID, SaleID: p.SaleID, ProductID: p.ProductID, ProductName: p.ProductName,
		Quantity: p.Quantity, UnitPrice: p.UnitPrice, LineTotal: p.LineTotal,
	}, nil).Once()
	q.On("AddProductStock", mock.Anything, db.AddProductStockParams{ID: p.ProductID, Delta: -p.Quantity}).
		Return(db.Product{}, nil).Once()
	q.On("CreateStockMovement", mock.Anything, db.CreateStockMovementParams{
		ProductID: p.ProductID, QuantityChange: -p.Quantity, Reason: domain.StockReasonSale,
		ReferenceID: ptr(p.SaleID), CreatedBy: cashierID,
	}).Return(nil).Once()
}
