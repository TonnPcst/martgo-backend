//go:build integration

// Integration tests run the services against a real PostgreSQL. They reset the
// schema on every test, so TEST_DATABASE_URL must point at a throwaway database:
//
//	make up
//	docker exec martgo-postgres createdb -U martgo martgo_test
//	make test-integration
package service

import (
	"database/sql"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TonnPcst/martgo-backend/internal/apperr"
	"github.com/TonnPcst/martgo-backend/internal/db"
	"github.com/TonnPcst/martgo-backend/internal/domain"
	"github.com/TonnPcst/martgo-backend/internal/store"
	"github.com/TonnPcst/martgo-backend/migrations"
)

func newTestStore(t *testing.T) *store.PostgresStore {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	sqlDB, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	defer sqlDB.Close()
	_, err = sqlDB.ExecContext(t.Context(), "DROP SCHEMA public CASCADE; CREATE SCHEMA public")
	require.NoError(t, err)
	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	require.NoError(t, goose.SetDialect("postgres"))
	require.NoError(t, goose.Up(sqlDB, "."))

	pool, err := pgxpool.New(t.Context(), dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	// Open connections up front. Otherwise the first goroutine in a concurrency test
	// reuses the only open connection and finishes before the others have connected,
	// so the calls never actually overlap.
	conns := make([]*pgxpool.Conn, 0, 5)
	for range cap(conns) {
		c, err := pool.Acquire(t.Context())
		require.NoError(t, err)
		conns = append(conns, c)
	}
	for _, c := range conns {
		c.Release()
	}
	return store.New(pool)
}

func seedProduct(t *testing.T, st *store.PostgresStore, sku string, stock int32) db.Product {
	t.Helper()
	p, err := st.CreateProduct(t.Context(), db.CreateProductParams{
		Sku: sku, Name: sku, Unit: "pcs", Price: 4500,
	})
	require.NoError(t, err)
	p, err = st.AddProductStock(t.Context(), db.AddProductStockParams{ID: p.ID, Delta: stock})
	require.NoError(t, err)
	return p
}

// runConcurrently starts n calls of fn at the same instant and returns their errors.
func runConcurrently(n int, fn func(i int) error) []error {
	errs := make([]error, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = fn(i)
		}()
	}
	close(start)
	wg.Wait()
	return errs
}

func cardSale(cashier int64, items ...SaleItemInput) CreateSaleInput {
	return CreateSaleInput{CashierID: cashier, PaymentMethod: domain.PaymentCard, Items: items}
}

func TestIntegration_ConcurrentSalesDoNotOversell(t *testing.T) {
	st := newTestStore(t)
	milk := seedProduct(t, st, "MILK-1L", 1)
	svc := NewSaleService(st)

	errs := runConcurrently(5, func(i int) error {
		_, err := svc.Create(t.Context(), cardSale(int64(i+1), SaleItemInput{ProductID: milk.ID, Quantity: 1}))
		return err
	})

	succeeded := 0
	for _, err := range errs {
		if err == nil {
			succeeded++
			continue
		}
		assert.Equal(t, apperr.KindConflict, apperr.KindOf(err), "unexpected error: %v", err)
	}
	assert.Equal(t, 1, succeeded, "exactly one cashier should get the last bottle")

	got, err := st.GetProduct(t.Context(), milk.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(0), got.StockQuantity)
}

func TestIntegration_SaleWritesStockMovement(t *testing.T) {
	st := newTestStore(t)
	milk := seedProduct(t, st, "MILK-1L", 10)

	sale, err := NewSaleService(st).Create(t.Context(), cardSale(7, SaleItemInput{ProductID: milk.ID, Quantity: 3}))
	require.NoError(t, err)

	got, err := st.GetProduct(t.Context(), milk.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(7), got.StockQuantity)

	moves, err := st.ListStockMovementsByProduct(t.Context(), db.ListStockMovementsByProductParams{
		ProductID: milk.ID, Limit: 10,
	})
	require.NoError(t, err)
	require.Len(t, moves, 1)
	assert.Equal(t, int32(-3), moves[0].QuantityChange)
	assert.Equal(t, domain.StockReasonSale, moves[0].Reason)
	assert.Equal(t, &sale.ID, moves[0].ReferenceID)
	assert.Equal(t, int64(7), moves[0].CreatedBy)
}

func TestIntegration_FailedSaleRollsBack(t *testing.T) {
	st := newTestStore(t)
	milk := seedProduct(t, st, "MILK-1L", 10)
	bread := seedProduct(t, st, "BREAD", 1)

	_, err := NewSaleService(st).Create(t.Context(), cardSale(7,
		SaleItemInput{ProductID: milk.ID, Quantity: 2},
		SaleItemInput{ProductID: bread.ID, Quantity: 5},
	))
	require.Error(t, err)
	assert.Equal(t, apperr.KindConflict, apperr.KindOf(err))

	got, err := st.GetProduct(t.Context(), milk.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(10), got.StockQuantity, "milk must not be deducted when the sale fails")

	total, err := st.CountSales(t.Context(), nil)
	require.NoError(t, err)
	assert.Zero(t, total)
}

func TestIntegration_ConcurrentRetriesSellOnce(t *testing.T) {
	st := newTestStore(t)
	milk := seedProduct(t, st, "MILK-1L", 10)
	svc := NewSaleService(st)
	in := cardSale(7, SaleItemInput{ProductID: milk.ID, Quantity: 3})
	in.IdempotencyKey = "till-1-0042"

	// A till that times out and retries: five copies of the same request at once.
	ids := make([]int64, 5)
	replayed := make([]bool, 5)
	errs := runConcurrently(5, func(i int) error {
		sale, err := svc.Create(t.Context(), in)
		ids[i], replayed[i] = sale.ID, sale.Replayed
		return err
	})

	fresh := 0
	for i, err := range errs {
		require.NoError(t, err)
		assert.Equal(t, ids[0], ids[i], "every retry must get the same sale")
		if !replayed[i] {
			fresh++
		}
	}
	assert.Equal(t, 1, fresh, "exactly one request creates the sale")

	got, err := st.GetProduct(t.Context(), milk.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(7), got.StockQuantity, "stock is deducted once")
	total, err := st.CountSales(t.Context(), nil)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)

	// The same key for a different basket is refused, not silently replayed.
	other := cardSale(7, SaleItemInput{ProductID: milk.ID, Quantity: 1})
	other.IdempotencyKey = in.IdempotencyKey
	_, err = svc.Create(t.Context(), other)
	assert.Equal(t, apperr.KindConflict, apperr.KindOf(err))

	// Keys are per cashier: another cashier may use the same one.
	_, err = svc.Create(t.Context(), func() CreateSaleInput { c := in; c.CashierID = 8; return c }())
	require.NoError(t, err)
}

func TestIntegration_AdjustStockNeverGoesNegative(t *testing.T) {
	st := newTestStore(t)
	milk := seedProduct(t, st, "MILK-1L", 3)
	svc := NewProductService(st)
	actor := Actor{UserID: 1, Role: domain.RoleManager}

	// Five managers each try to write off 2 of the 3 bottles at once: only one can win.
	errs := runConcurrently(5, func(int) error {
		_, err := svc.AdjustStock(t.Context(), actor, milk.ID, -2, "damaged")
		return err
	})

	succeeded := 0
	for _, err := range errs {
		if err == nil {
			succeeded++
			continue
		}
		// Without the row lock the CHECK constraint still stops negative stock,
		// but the caller gets a 500 instead of a clean conflict.
		assert.Equal(t, apperr.KindConflict, apperr.KindOf(err), "unexpected error: %v", err)
	}
	assert.Equal(t, 1, succeeded)

	got, err := st.GetProduct(t.Context(), milk.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(1), got.StockQuantity)
}

func TestIntegration_RefundsRestockAndAddUpToTotal(t *testing.T) {
	st := newTestStore(t)
	milk := seedProduct(t, st, "MILK-1L", 10)
	// 3 × 4500 = 13500, minus 1000 discount = 12500.
	sale, err := NewSaleService(st).Create(t.Context(), CreateSaleInput{
		CashierID: 7, PaymentMethod: domain.PaymentCash, Discount: 1000, PaidAmount: 20000,
		Items: []SaleItemInput{{ProductID: milk.ID, Quantity: 3}},
	})
	require.NoError(t, err)
	svc := NewRefundService(st)

	r1, err := svc.Create(t.Context(), CreateRefundInput{SaleID: sale.ID, RefundedBy: 2, Reason: "changed mind",
		Items: []RefundItemInput{{ProductID: milk.ID, Quantity: 1, Restock: true}}})
	require.NoError(t, err)
	assert.Equal(t, int64(4166), r1.Amount, "4500 × 12500/13500, rounded down")

	r2, err := svc.Create(t.Context(), CreateRefundInput{SaleID: sale.ID, RefundedBy: 2, Reason: "bottles broken",
		Items: []RefundItemInput{{ProductID: milk.ID, Quantity: 2, Restock: false}}})
	require.NoError(t, err)
	assert.Equal(t, sale.Total, r1.Amount+r2.Amount, "refunds add up to exactly the sale total")

	got, err := st.GetProduct(t.Context(), milk.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(8), got.StockQuantity, "10 − 3 sold + 1 restocked; written-off bottles stay out")

	_, err = svc.Create(t.Context(), CreateRefundInput{SaleID: sale.ID, RefundedBy: 2, Reason: "again"})
	assert.Equal(t, apperr.KindConflict, apperr.KindOf(err))

	refunds, err := svc.ListBySale(t.Context(), Actor{UserID: 7, Role: domain.RoleCashier}, sale.ID)
	require.NoError(t, err)
	require.Len(t, refunds, 2)
	assert.Len(t, refunds[1].Items, 1)
}

func TestIntegration_ConcurrentVoidsRefundOnce(t *testing.T) {
	st := newTestStore(t)
	milk := seedProduct(t, st, "MILK-1L", 10)
	sale, err := NewSaleService(st).Create(t.Context(), cardSale(7, SaleItemInput{ProductID: milk.ID, Quantity: 2}))
	require.NoError(t, err)
	svc := NewRefundService(st)

	errs := runConcurrently(5, func(int) error {
		_, err := svc.Create(t.Context(), CreateRefundInput{SaleID: sale.ID, RefundedBy: 2, Reason: "void"})
		return err
	})

	succeeded := 0
	for _, err := range errs {
		if err == nil {
			succeeded++
			continue
		}
		assert.Equal(t, apperr.KindConflict, apperr.KindOf(err), "unexpected error: %v", err)
	}
	assert.Equal(t, 1, succeeded, "the sale lock lets only one void through")

	got, err := st.GetProduct(t.Context(), milk.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(10), got.StockQuantity)
}
