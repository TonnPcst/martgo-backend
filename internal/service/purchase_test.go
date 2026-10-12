package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/TonnPcst/martgo-backend/internal/apperr"
	"github.com/TonnPcst/martgo-backend/internal/db"
	"github.com/TonnPcst/martgo-backend/internal/domain"
)

const testPurchaseID int64 = 500

func TestPurchaseService_Create(t *testing.T) {
	rice := db.Product{ID: 1, Name: "Rice 5kg", Cost: 15000, StockQuantity: 5, IsActive: true}
	oil := db.Product{ID: 2, Name: "Cooking Oil 1L", Cost: 5500, IsActive: true}

	tests := []struct {
		name         string
		input        CreatePurchaseInput
		supplierErr  error
		products     []db.Product // rows returned by GetProductsForUpdate
		wantPurchase *db.CreatePurchaseParams
		wantItems    []db.CreatePurchaseItemParams
		wantErr      bool
		wantKind     apperr.Kind
	}{
		{
			name: "receives stock and computes total",
			input: CreatePurchaseInput{SupplierID: 3, CreatedBy: 9, InvoiceNo: " INV-001 ",
				Items: []PurchaseItemInput{
					{ProductID: 2, Quantity: 5, UnitCost: 6000},
					{ProductID: 1, Quantity: 10, UnitCost: 14000},
				}},
			products:     []db.Product{rice, oil},
			wantPurchase: &db.CreatePurchaseParams{SupplierID: 3, CreatedBy: 9, InvoiceNo: "INV-001", Total: 170000},
			wantItems: []db.CreatePurchaseItemParams{
				{PurchaseID: testPurchaseID, ProductID: 1, Quantity: 10, UnitCost: 14000, LineTotal: 140000},
				{PurchaseID: testPurchaseID, ProductID: 2, Quantity: 5, UnitCost: 6000, LineTotal: 30000},
			},
		},
		{
			name: "free goods with zero cost are allowed",
			input: CreatePurchaseInput{SupplierID: 3, CreatedBy: 9,
				Items: []PurchaseItemInput{{ProductID: 1, Quantity: 2, UnitCost: 0}}},
			products:     []db.Product{rice},
			wantPurchase: &db.CreatePurchaseParams{SupplierID: 3, CreatedBy: 9, Total: 0},
			wantItems: []db.CreatePurchaseItemParams{
				{PurchaseID: testPurchaseID, ProductID: 1, Quantity: 2, UnitCost: 0, LineTotal: 0},
			},
		},
		{
			name:     "no items",
			input:    CreatePurchaseInput{SupplierID: 3, CreatedBy: 9},
			wantErr:  true,
			wantKind: apperr.KindValidation,
		},
		{
			name: "non-positive quantity",
			input: CreatePurchaseInput{SupplierID: 3, CreatedBy: 9,
				Items: []PurchaseItemInput{{ProductID: 1, Quantity: 0, UnitCost: 100}}},
			wantErr:  true,
			wantKind: apperr.KindValidation,
		},
		{
			name: "negative unit cost",
			input: CreatePurchaseInput{SupplierID: 3, CreatedBy: 9,
				Items: []PurchaseItemInput{{ProductID: 1, Quantity: 1, UnitCost: -1}}},
			wantErr:  true,
			wantKind: apperr.KindValidation,
		},
		{
			name: "duplicate product",
			input: CreatePurchaseInput{SupplierID: 3, CreatedBy: 9,
				Items: []PurchaseItemInput{{ProductID: 1, Quantity: 1, UnitCost: 100}, {ProductID: 1, Quantity: 2, UnitCost: 100}}},
			wantErr:  true,
			wantKind: apperr.KindValidation,
		},
		{
			name: "unknown supplier",
			input: CreatePurchaseInput{SupplierID: 3, CreatedBy: 9,
				Items: []PurchaseItemInput{{ProductID: 1, Quantity: 1, UnitCost: 100}}},
			supplierErr: pgx.ErrNoRows,
			wantErr:     true,
			wantKind:    apperr.KindNotFound,
		},
		{
			name: "unknown product",
			input: CreatePurchaseInput{SupplierID: 3, CreatedBy: 9,
				Items: []PurchaseItemInput{{ProductID: 1, Quantity: 1, UnitCost: 100}, {ProductID: 99, Quantity: 1, UnitCost: 100}}},
			products: []db.Product{rice},
			wantErr:  true,
			wantKind: apperr.KindNotFound,
		},
		{
			name: "database failure is returned as internal error",
			input: CreatePurchaseInput{SupplierID: 3, CreatedBy: 9,
				Items: []PurchaseItemInput{{ProductID: 1, Quantity: 1, UnitCost: 100}}},
			supplierErr: errors.New("connection reset"),
			wantErr:     true,
			wantKind:    apperr.KindInternal,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := newMockQuerier(t)
			q.On("GetSupplier", mock.Anything, tc.input.SupplierID).
				Return(db.Supplier{ID: tc.input.SupplierID}, tc.supplierErr).Maybe()
			q.On("GetProductsForUpdate", mock.Anything, mock.Anything).Return(tc.products, nil).Maybe()
			if tc.wantPurchase != nil {
				q.On("CreatePurchase", mock.Anything, *tc.wantPurchase).Return(purchaseFrom(*tc.wantPurchase), nil).Once()
			}
			for _, item := range tc.wantItems {
				expectPurchaseItem(q, item, tc.input.CreatedBy)
			}

			got, err := NewPurchaseService(mockStore{q}).Create(context.Background(), tc.input)

			if tc.wantErr {
				require.Error(t, err)
				assert.Equal(t, tc.wantKind, apperr.KindOf(err), "unexpected error: %v", err)
				q.AssertNotCalled(t, "CreatePurchase", mock.Anything, mock.Anything)
				q.AssertNotCalled(t, "ReceiveProductStock", mock.Anything, mock.Anything)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, testPurchaseID, got.ID)
			assert.Equal(t, tc.wantPurchase.Total, got.Total)
			assert.Len(t, got.Items, len(tc.wantItems))
			q.AssertExpectations(t)
		})
	}
}

func purchaseFrom(p db.CreatePurchaseParams) db.Purchase {
	return db.Purchase{
		ID: testPurchaseID, SupplierID: p.SupplierID, CreatedBy: p.CreatedBy,
		InvoiceNo: p.InvoiceNo, Total: p.Total, Note: p.Note,
	}
}

// expectPurchaseItem expects the item insert, stock receipt and audit row for one line.
func expectPurchaseItem(q *mockQuerier, p db.CreatePurchaseItemParams, userID int64) {
	q.On("CreatePurchaseItem", mock.Anything, p).Return(db.PurchaseItem{
		ID: p.ProductID, PurchaseID: p.PurchaseID, ProductID: p.ProductID,
		Quantity: p.Quantity, UnitCost: p.UnitCost, LineTotal: p.LineTotal,
	}, nil).Once()
	q.On("ReceiveProductStock", mock.Anything, db.ReceiveProductStockParams{
		ID: p.ProductID, Quantity: p.Quantity, UnitCost: p.UnitCost,
	}).Return(db.Product{}, nil).Once()
	q.On("CreateStockMovement", mock.Anything, db.CreateStockMovementParams{
		ProductID: p.ProductID, QuantityChange: p.Quantity, Reason: domain.StockReasonPurchase,
		ReferenceID: ptr(p.PurchaseID), CreatedBy: userID,
	}).Return(nil).Once()
}
