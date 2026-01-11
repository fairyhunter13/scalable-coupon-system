package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/scalable-coupon-system/internal/model"
	"github.com/fairyhunter13/scalable-coupon-system/internal/service"
)

// mockRow implements pgx.Row for testing GetByName.
type mockRow struct {
	scanFn func(dest ...any) error
}

func (m *mockRow) Scan(dest ...any) error {
	if m.scanFn != nil {
		return m.scanFn(dest...)
	}
	return nil
}

// mockPool implements PoolInterface for testing.
type mockPool struct {
	execFn     func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
}

func (m *mockPool) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	if m.execFn != nil {
		return m.execFn(ctx, sql, arguments...)
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func (m *mockPool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.queryRowFn != nil {
		return m.queryRowFn(ctx, sql, args...)
	}
	return &mockRow{}
}

func TestCouponRepository_Insert_Success(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any

	mock := &mockPool{
		execFn: func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			capturedArgs = arguments
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		},
	}

	repo := NewCouponRepositoryWithPool(mock)
	coupon := &model.Coupon{
		Name:   "PROMO_SUPER",
		Amount: 100,
	}

	err := repo.Insert(context.Background(), coupon)

	require.NoError(t, err)
	assert.Contains(t, capturedSQL, "INSERT INTO coupons")
	assert.Contains(t, capturedSQL, "$1, $2, $3")
	assert.Equal(t, "PROMO_SUPER", capturedArgs[0])
	assert.Equal(t, 100, capturedArgs[1])
	assert.Equal(t, 100, capturedArgs[2]) // remaining_amount = amount
}

func TestCouponRepository_Insert_DuplicateCoupon(t *testing.T) {
	mock := &mockPool{
		execFn: func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
			// Simulate PostgreSQL unique violation error (code 23505)
			pgErr := &pgconn.PgError{
				Code:    "23505",
				Message: "duplicate key value violates unique constraint",
			}
			return pgconn.CommandTag{}, pgErr
		},
	}

	repo := NewCouponRepositoryWithPool(mock)
	coupon := &model.Coupon{
		Name:   "PROMO_SUPER",
		Amount: 100,
	}

	err := repo.Insert(context.Background(), coupon)

	require.Error(t, err)
	assert.True(t, errors.Is(err, service.ErrCouponExists), "should return ErrCouponExists for duplicate")
}

func TestCouponRepository_Insert_DatabaseError(t *testing.T) {
	dbErr := errors.New("connection refused")
	mock := &mockPool{
		execFn: func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, dbErr
		},
	}

	repo := NewCouponRepositoryWithPool(mock)
	coupon := &model.Coupon{
		Name:   "PROMO_SUPER",
		Amount: 100,
	}

	err := repo.Insert(context.Background(), coupon)

	require.Error(t, err)
	assert.False(t, errors.Is(err, service.ErrCouponExists), "should not return ErrCouponExists for generic error")
	assert.Contains(t, err.Error(), "insert coupon")
	assert.True(t, errors.Is(err, dbErr), "should wrap original error")
}

func TestCouponRepository_Insert_OtherPgError(t *testing.T) {
	mock := &mockPool{
		execFn: func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
			// Simulate a different PostgreSQL error (not 23505)
			pgErr := &pgconn.PgError{
				Code:    "23502", // not_null_violation
				Message: "null value in column violates not-null constraint",
			}
			return pgconn.CommandTag{}, pgErr
		},
	}

	repo := NewCouponRepositoryWithPool(mock)
	coupon := &model.Coupon{
		Name:   "PROMO_SUPER",
		Amount: 100,
	}

	err := repo.Insert(context.Background(), coupon)

	require.Error(t, err)
	assert.False(t, errors.Is(err, service.ErrCouponExists), "should not return ErrCouponExists for non-23505 error")
	assert.Contains(t, err.Error(), "insert coupon")
}

func TestCouponRepository_Insert_VerifiesParameterizedQuery(t *testing.T) {
	var capturedSQL string
	mock := &mockPool{
		execFn: func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		},
	}

	repo := NewCouponRepositoryWithPool(mock)

	// Test with SQL injection attempt in coupon name
	coupon := &model.Coupon{
		Name:   "'; DROP TABLE coupons;--",
		Amount: 1,
	}

	err := repo.Insert(context.Background(), coupon)

	require.NoError(t, err)
	// Verify that the SQL uses parameterized placeholders, not string interpolation
	assert.Contains(t, capturedSQL, "$1")
	assert.Contains(t, capturedSQL, "$2")
	assert.Contains(t, capturedSQL, "$3")
	assert.NotContains(t, capturedSQL, "DROP TABLE", "SQL injection should not appear in query")
}

func TestCouponRepository_GetByName_Success(t *testing.T) {
	expectedTime := time.Now()
	mock := &mockPool{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					// Fill in the dest pointers with test data
					*(dest[0].(*string)) = "PROMO_SUPER"
					*(dest[1].(*int)) = 100
					*(dest[2].(*int)) = 95
					*(dest[3].(*time.Time)) = expectedTime
					return nil
				},
			}
		},
	}

	repo := NewCouponRepositoryWithPool(mock)
	coupon, err := repo.GetByName(context.Background(), "PROMO_SUPER")

	require.NoError(t, err)
	require.NotNil(t, coupon)
	assert.Equal(t, "PROMO_SUPER", coupon.Name)
	assert.Equal(t, 100, coupon.Amount)
	assert.Equal(t, 95, coupon.RemainingAmount)
	assert.Equal(t, expectedTime, coupon.CreatedAt)
}

func TestCouponRepository_GetByName_NotFound(t *testing.T) {
	mock := &mockPool{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					return pgx.ErrNoRows
				},
			}
		},
	}

	repo := NewCouponRepositoryWithPool(mock)
	coupon, err := repo.GetByName(context.Background(), "NONEXISTENT")

	require.NoError(t, err)
	assert.Nil(t, coupon, "Should return nil for not found")
}

func TestCouponRepository_GetByName_DatabaseError(t *testing.T) {
	dbErr := errors.New("database connection failed")
	mock := &mockPool{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					return dbErr
				},
			}
		},
	}

	repo := NewCouponRepositoryWithPool(mock)
	coupon, err := repo.GetByName(context.Background(), "PROMO_SUPER")

	require.Error(t, err)
	assert.Nil(t, coupon)
	assert.Contains(t, err.Error(), "get coupon by name")
	assert.True(t, errors.Is(err, dbErr), "should wrap original error")
}

func TestCouponRepository_GetByName_VerifiesParameterizedQuery(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any
	mock := &mockPool{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			capturedSQL = sql
			capturedArgs = args
			return &mockRow{
				scanFn: func(dest ...any) error {
					return pgx.ErrNoRows
				},
			}
		},
	}

	repo := NewCouponRepositoryWithPool(mock)

	// Test with SQL injection attempt
	_, _ = repo.GetByName(context.Background(), "'; DROP TABLE coupons;--")

	// Verify parameterized query
	assert.Contains(t, capturedSQL, "$1")
	assert.NotContains(t, capturedSQL, "DROP TABLE", "SQL injection should not appear in query")
	assert.Equal(t, "'; DROP TABLE coupons;--", capturedArgs[0], "Name should be passed as parameter")
}

// mockCouponTxQuerier implements database.TxQuerier for testing transaction methods.
type mockCouponTxQuerier struct {
	execFn     func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	queryFn    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (m *mockCouponTxQuerier) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	if m.execFn != nil {
		return m.execFn(ctx, sql, arguments...)
	}
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (m *mockCouponTxQuerier) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.queryRowFn != nil {
		return m.queryRowFn(ctx, sql, args...)
	}
	return &mockRow{}
}

func (m *mockCouponTxQuerier) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.queryFn != nil {
		return m.queryFn(ctx, sql, args...)
	}
	return nil, nil
}

func TestCouponRepository_GetCouponForUpdate_Success(t *testing.T) {
	expectedTime := time.Now()
	mockTx := &mockCouponTxQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			// Verify FOR UPDATE is in query
			assert.Contains(t, sql, "FOR UPDATE", "Query must use FOR UPDATE for row locking")
			return &mockRow{
				scanFn: func(dest ...any) error {
					*(dest[0].(*string)) = "PROMO_SUPER"
					*(dest[1].(*int)) = 100
					*(dest[2].(*int)) = 5
					*(dest[3].(*time.Time)) = expectedTime
					return nil
				},
			}
		},
	}

	repo := NewCouponRepositoryWithPool(&mockPool{})
	coupon, err := repo.GetCouponForUpdate(context.Background(), mockTx, "PROMO_SUPER")

	require.NoError(t, err)
	require.NotNil(t, coupon)
	assert.Equal(t, "PROMO_SUPER", coupon.Name)
	assert.Equal(t, 100, coupon.Amount)
	assert.Equal(t, 5, coupon.RemainingAmount)
}

func TestCouponRepository_GetCouponForUpdate_NotFound(t *testing.T) {
	mockTx := &mockCouponTxQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					return pgx.ErrNoRows
				},
			}
		},
	}

	repo := NewCouponRepositoryWithPool(&mockPool{})
	coupon, err := repo.GetCouponForUpdate(context.Background(), mockTx, "NONEXISTENT")

	require.Error(t, err)
	assert.True(t, errors.Is(err, service.ErrCouponNotFound), "should return ErrCouponNotFound")
	assert.Nil(t, coupon)
}

func TestCouponRepository_GetCouponForUpdate_DatabaseError(t *testing.T) {
	dbErr := errors.New("database connection failed")
	mockTx := &mockCouponTxQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					return dbErr
				},
			}
		},
	}

	repo := NewCouponRepositoryWithPool(&mockPool{})
	coupon, err := repo.GetCouponForUpdate(context.Background(), mockTx, "PROMO_SUPER")

	require.Error(t, err)
	assert.Nil(t, coupon)
	assert.Contains(t, err.Error(), "get coupon for update")
	assert.True(t, errors.Is(err, dbErr), "should wrap original error")
}

func TestCouponRepository_GetCouponForUpdate_VerifiesParameterizedQuery(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any
	mockTx := &mockCouponTxQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			capturedSQL = sql
			capturedArgs = args
			return &mockRow{
				scanFn: func(dest ...any) error {
					return pgx.ErrNoRows
				},
			}
		},
	}

	repo := NewCouponRepositoryWithPool(&mockPool{})
	_, _ = repo.GetCouponForUpdate(context.Background(), mockTx, "'; DROP TABLE coupons;--")

	assert.Contains(t, capturedSQL, "$1")
	assert.Contains(t, capturedSQL, "FOR UPDATE")
	assert.NotContains(t, capturedSQL, "DROP TABLE", "SQL injection should not appear in query")
	assert.Equal(t, "'; DROP TABLE coupons;--", capturedArgs[0])
}

func TestCouponRepository_DecrementStock_Success(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any
	mockTx := &mockCouponTxQuerier{
		execFn: func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			capturedArgs = arguments
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}

	repo := NewCouponRepositoryWithPool(&mockPool{})
	err := repo.DecrementStock(context.Background(), mockTx, "PROMO_SUPER")

	require.NoError(t, err)
	assert.Contains(t, capturedSQL, "UPDATE coupons")
	assert.Contains(t, capturedSQL, "remaining_amount = remaining_amount - 1")
	assert.Equal(t, "PROMO_SUPER", capturedArgs[0])
}

func TestCouponRepository_DecrementStock_DatabaseError(t *testing.T) {
	dbErr := errors.New("database connection failed")
	mockTx := &mockCouponTxQuerier{
		execFn: func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, dbErr
		},
	}

	repo := NewCouponRepositoryWithPool(&mockPool{})
	err := repo.DecrementStock(context.Background(), mockTx, "PROMO_SUPER")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "decrement stock")
	assert.True(t, errors.Is(err, dbErr), "should wrap original error")
}

func TestCouponRepository_DecrementStock_VerifiesParameterizedQuery(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any
	mockTx := &mockCouponTxQuerier{
		execFn: func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			capturedArgs = arguments
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}

	repo := NewCouponRepositoryWithPool(&mockPool{})
	_ = repo.DecrementStock(context.Background(), mockTx, "'; DROP TABLE coupons;--")

	assert.Contains(t, capturedSQL, "$1")
	assert.NotContains(t, capturedSQL, "DROP TABLE", "SQL injection should not appear in query")
	assert.Equal(t, "'; DROP TABLE coupons;--", capturedArgs[0])
}

// TestNewCouponRepository_Production tests the production constructor.
// Note: This constructor is typically tested via integration tests with a real pgxpool.Pool.
// This test verifies the constructor exists and returns a non-nil repository.
func TestNewCouponRepository_Production(t *testing.T) {
	// NewCouponRepository requires a *pgxpool.Pool which implements PoolInterface.
	// Passing nil is valid for constructor testing - actual usage requires a real pool.
	repo := NewCouponRepository(nil)
	require.NotNil(t, repo, "NewCouponRepository should return a non-nil repository")
}
