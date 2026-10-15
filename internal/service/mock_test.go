package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/TonnPcst/martgo-backend/internal/db"
)

type mockQuerier struct {
	mock.Mock
	db.Querier
}

func newMockQuerier(t *testing.T) *mockQuerier {
	q := &mockQuerier{}
	q.Test(t)
	return q
}

type mockStore struct{ *mockQuerier }

func (s mockStore) ExecTx(_ context.Context, fn func(db.Querier) error) error {
	return fn(s.mockQuerier)
}

func ptr[T any](v T) *T { return &v }

func (q *mockQuerier) GetProductsForUpdate(ctx context.Context, ids []int64) ([]db.Product, error) {
	args := q.Called(ctx, ids)
	return args.Get(0).([]db.Product), args.Error(1)
}

func (q *mockQuerier) GetProductForUpdate(ctx context.Context, id int64) (db.Product, error) {
	args := q.Called(ctx, id)
	return args.Get(0).(db.Product), args.Error(1)
}

func (q *mockQuerier) AddProductStock(ctx context.Context, arg db.AddProductStockParams) (db.Product, error) {
	args := q.Called(ctx, arg)
	return args.Get(0).(db.Product), args.Error(1)
}

func (q *mockQuerier) ReceiveProductStock(ctx context.Context, arg db.ReceiveProductStockParams) (db.Product, error) {
	args := q.Called(ctx, arg)
	return args.Get(0).(db.Product), args.Error(1)
}

func (q *mockQuerier) CreateStockMovement(ctx context.Context, arg db.CreateStockMovementParams) error {
	return q.Called(ctx, arg).Error(0)
}

func (q *mockQuerier) CreateSale(ctx context.Context, arg db.CreateSaleParams) (db.Sale, error) {
	args := q.Called(ctx, arg)
	return args.Get(0).(db.Sale), args.Error(1)
}

func (q *mockQuerier) CreateSaleItem(ctx context.Context, arg db.CreateSaleItemParams) (db.SaleItem, error) {
	args := q.Called(ctx, arg)
	return args.Get(0).(db.SaleItem), args.Error(1)
}

func (q *mockQuerier) GetSaleByIdempotencyKey(ctx context.Context, arg db.GetSaleByIdempotencyKeyParams) (db.Sale, error) {
	args := q.Called(ctx, arg)
	return args.Get(0).(db.Sale), args.Error(1)
}

func (q *mockQuerier) GetSale(ctx context.Context, id int64) (db.Sale, error) {
	args := q.Called(ctx, id)
	return args.Get(0).(db.Sale), args.Error(1)
}

func (q *mockQuerier) ListSaleItems(ctx context.Context, saleID int64) ([]db.SaleItem, error) {
	args := q.Called(ctx, saleID)
	return args.Get(0).([]db.SaleItem), args.Error(1)
}

func (q *mockQuerier) GetSupplier(ctx context.Context, id int64) (db.Supplier, error) {
	args := q.Called(ctx, id)
	return args.Get(0).(db.Supplier), args.Error(1)
}

func (q *mockQuerier) CreatePurchase(ctx context.Context, arg db.CreatePurchaseParams) (db.Purchase, error) {
	args := q.Called(ctx, arg)
	return args.Get(0).(db.Purchase), args.Error(1)
}

func (q *mockQuerier) CreatePurchaseItem(ctx context.Context, arg db.CreatePurchaseItemParams) (db.PurchaseItem, error) {
	args := q.Called(ctx, arg)
	return args.Get(0).(db.PurchaseItem), args.Error(1)
}
