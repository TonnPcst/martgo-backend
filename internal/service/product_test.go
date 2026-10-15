package service

import (
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/TonnPcst/martgo-backend/internal/apperr"
	"github.com/TonnPcst/martgo-backend/internal/db"
	"github.com/TonnPcst/martgo-backend/internal/domain"
)

func TestProductInput_Normalize(t *testing.T) {
	valid := func() ProductInput {
		return ProductInput{Sku: " MILK-1L ", Name: " Milk 1L ", Price: 4500}
	}

	tests := []struct {
		name        string
		modify      func(in *ProductInput)
		wantErr     bool
		wantBarcode *string
		wantUnit    string
	}{
		{name: "valid input gets default unit", modify: func(in *ProductInput) {}, wantUnit: "pcs"},
		{name: "nil barcode stays nil", modify: func(in *ProductInput) { in.Barcode = nil }, wantUnit: "pcs"},
		{
			name:        "barcode is trimmed",
			modify:      func(in *ProductInput) { in.Barcode = ptr("  885123  ") },
			wantBarcode: ptr("885123"),
			wantUnit:    "pcs",
		},
		{name: "blank barcode becomes nil", modify: func(in *ProductInput) { in.Barcode = ptr("   ") }, wantUnit: "pcs"},
		{name: "custom unit is trimmed and kept", modify: func(in *ProductInput) { in.Unit = " kg " }, wantUnit: "kg"},
		{name: "missing sku", modify: func(in *ProductInput) { in.Sku = "" }, wantErr: true},
		{name: "name is only spaces", modify: func(in *ProductInput) { in.Name = "   " }, wantErr: true},
		{name: "zero price", modify: func(in *ProductInput) { in.Price = 0 }, wantErr: true},
		{name: "negative cost", modify: func(in *ProductInput) { in.Cost = -1 }, wantErr: true},
		{name: "negative reorder level", modify: func(in *ProductInput) { in.ReorderLevel = -1 }, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := valid()
			tc.modify(&in)

			err := in.normalize()

			if tc.wantErr {
				require.Error(t, err)
				assert.Equal(t, apperr.KindValidation, apperr.KindOf(err))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "MILK-1L", in.Sku)
			assert.Equal(t, "Milk 1L", in.Name)
			assert.Equal(t, tc.wantUnit, in.Unit)
			assert.Equal(t, tc.wantBarcode, in.Barcode)
		})
	}
}

func TestProductService_AdjustStock(t *testing.T) {
	milk := db.Product{ID: 1, Name: "Milk 1L", StockQuantity: 10}
	actor := Actor{UserID: 7, Role: domain.RoleManager}

	tests := []struct {
		name     string
		delta    int32
		getErr   error
		wantCall bool // AddProductStock and CreateStockMovement are expected
		wantErr  bool
		wantKind apperr.Kind
	}{
		{name: "add stock", delta: 5, wantCall: true},
		{name: "remove all stock exactly", delta: -10, wantCall: true},
		{name: "would go negative", delta: -11, wantErr: true, wantKind: apperr.KindConflict},
		{name: "zero delta", delta: 0, wantErr: true, wantKind: apperr.KindValidation},
		{name: "product not found", delta: 1, getErr: pgx.ErrNoRows, wantErr: true, wantKind: apperr.KindNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := newMockQuerier(t)
			svc := NewProductService(mockStore{q})

			q.On("GetProductForUpdate", mock.Anything, int64(1)).Return(milk, tc.getErr).Maybe()
			if tc.wantCall {
				updated := milk
				updated.StockQuantity += tc.delta
				q.On("AddProductStock", mock.Anything, db.AddProductStockParams{ID: 1, Delta: tc.delta}).
					Return(updated, nil).Once()
				q.On("CreateStockMovement", mock.Anything, db.CreateStockMovementParams{
					ProductID:      1,
					QuantityChange: tc.delta,
					Reason:         domain.StockReasonAdjustment,
					Note:           "recount",
					CreatedBy:      7,
				}).Return(nil).Once()
			}

			got, err := svc.AdjustStock(t.Context(), actor, 1, tc.delta, "  recount  ")

			if tc.wantErr {
				require.Error(t, err)
				assert.Equal(t, tc.wantKind, apperr.KindOf(err), "unexpected error: %v", err)
				q.AssertNotCalled(t, "AddProductStock", mock.Anything, mock.Anything)
				q.AssertNotCalled(t, "CreateStockMovement", mock.Anything, mock.Anything)
			} else {
				require.NoError(t, err)
				assert.Equal(t, 10+tc.delta, got.StockQuantity)
			}
			if tc.delta == 0 {
				q.AssertNotCalled(t, "GetProductForUpdate", mock.Anything, mock.Anything)
			}
			q.AssertExpectations(t)
		})
	}
}
