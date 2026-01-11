package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/scalable-coupon-system/internal/service"
)

// mockClaimRows implements pgx.Rows for testing.
type mockClaimRows struct {
	data      []string
	index     int
	errOnScan error
	errOnRows error
}

func (m *mockClaimRows) Close() {}

func (m *mockClaimRows) Err() error {
	return m.errOnRows
}

func (m *mockClaimRows) Next() bool {
	if m.index < len(m.data) {
		m.index++
		return true
	}
	return false
}

func (m *mockClaimRows) Scan(dest ...any) error {
	if m.errOnScan != nil {
		return m.errOnScan
	}
	if m.index > 0 && m.index <= len(m.data) {
		*(dest[0].(*string)) = m.data[m.index-1]
	}
	return nil
}

func (m *mockClaimRows) CommandTag() pgconn.CommandTag             { return pgconn.CommandTag{} }
func (m *mockClaimRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (m *mockClaimRows) RawValues() [][]byte                       { return nil }
func (m *mockClaimRows) Values() ([]any, error)                    { return nil, nil }
func (m *mockClaimRows) Conn() *pgx.Conn                           { return nil }

// mockClaimPool implements ClaimPoolInterface for testing.
type mockClaimPool struct {
	queryFn func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (m *mockClaimPool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.queryFn != nil {
		return m.queryFn(ctx, sql, args...)
	}
	return &mockClaimRows{}, nil
}

func TestClaimRepository_GetUsersByCoupon_Success(t *testing.T) {
	mock := &mockClaimPool{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockClaimRows{
				data: []string{"user_001", "user_002", "user_003"},
			}, nil
		},
	}

	repo := NewClaimRepositoryWithPool(mock)
	users, err := repo.GetUsersByCoupon(context.Background(), "PROMO_SUPER")

	require.NoError(t, err)
	assert.Equal(t, []string{"user_001", "user_002", "user_003"}, users)
}

func TestClaimRepository_GetUsersByCoupon_Empty(t *testing.T) {
	mock := &mockClaimPool{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockClaimRows{
				data: []string{}, // Empty
			}, nil
		},
	}

	repo := NewClaimRepositoryWithPool(mock)
	users, err := repo.GetUsersByCoupon(context.Background(), "NEW_PROMO")

	require.NoError(t, err)
	require.NotNil(t, users, "Should return empty slice, not nil")
	assert.Len(t, users, 0)
}

func TestClaimRepository_GetUsersByCoupon_QueryError(t *testing.T) {
	dbErr := errors.New("database connection failed")
	mock := &mockClaimPool{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, dbErr
		},
	}

	repo := NewClaimRepositoryWithPool(mock)
	users, err := repo.GetUsersByCoupon(context.Background(), "PROMO_SUPER")

	require.Error(t, err)
	assert.Nil(t, users)
	assert.Contains(t, err.Error(), "get claims for coupon")
	assert.True(t, errors.Is(err, dbErr), "should wrap original error")
}

func TestClaimRepository_GetUsersByCoupon_ScanError(t *testing.T) {
	scanErr := errors.New("scan error")
	mock := &mockClaimPool{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockClaimRows{
				data:      []string{"user_001"},
				errOnScan: scanErr,
			}, nil
		},
	}

	repo := NewClaimRepositoryWithPool(mock)
	users, err := repo.GetUsersByCoupon(context.Background(), "PROMO_SUPER")

	require.Error(t, err)
	assert.Nil(t, users)
	assert.Contains(t, err.Error(), "scan claim user_id")
}

func TestClaimRepository_GetUsersByCoupon_RowsError(t *testing.T) {
	rowsErr := errors.New("rows iteration error")
	mock := &mockClaimPool{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockClaimRows{
				data:      []string{},
				errOnRows: rowsErr,
			}, nil
		},
	}

	repo := NewClaimRepositoryWithPool(mock)
	users, err := repo.GetUsersByCoupon(context.Background(), "PROMO_SUPER")

	require.Error(t, err)
	assert.Nil(t, users)
	assert.Contains(t, err.Error(), "iterate claims rows")
}

func TestClaimRepository_GetUsersByCoupon_VerifiesParameterizedQuery(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any
	mock := &mockClaimPool{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			capturedSQL = sql
			capturedArgs = args
			return &mockClaimRows{data: []string{}}, nil
		},
	}

	repo := NewClaimRepositoryWithPool(mock)

	// Test with SQL injection attempt
	_, _ = repo.GetUsersByCoupon(context.Background(), "'; DROP TABLE claims;--")

	// Verify parameterized query
	assert.Contains(t, capturedSQL, "$1")
	assert.NotContains(t, capturedSQL, "DROP TABLE", "SQL injection should not appear in query")
	assert.Equal(t, "'; DROP TABLE claims;--", capturedArgs[0], "Name should be passed as parameter")
}

// mockTxQuerier implements database.TxQuerier for testing Insert method.
type mockTxQuerier struct {
	execFn     func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	queryFn    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (m *mockTxQuerier) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	if m.execFn != nil {
		return m.execFn(ctx, sql, arguments...)
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func (m *mockTxQuerier) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.queryRowFn != nil {
		return m.queryRowFn(ctx, sql, args...)
	}
	return nil
}

func (m *mockTxQuerier) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.queryFn != nil {
		return m.queryFn(ctx, sql, args...)
	}
	return nil, nil
}

func TestClaimRepository_Insert_Success(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any
	mockTx := &mockTxQuerier{
		execFn: func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			capturedArgs = arguments
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		},
	}

	repo := NewClaimRepositoryWithPool(&mockClaimPool{})
	err := repo.Insert(context.Background(), mockTx, "user_001", "PROMO_SUPER")

	require.NoError(t, err)
	assert.Contains(t, capturedSQL, "INSERT INTO claims")
	assert.Contains(t, capturedSQL, "$1, $2")
	assert.Equal(t, "user_001", capturedArgs[0])
	assert.Equal(t, "PROMO_SUPER", capturedArgs[1])
}

func TestClaimRepository_Insert_DuplicateClaim(t *testing.T) {
	mockTx := &mockTxQuerier{
		execFn: func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
			// Simulate PostgreSQL unique violation error (code 23505)
			pgErr := &pgconn.PgError{
				Code:    "23505",
				Message: "duplicate key value violates unique constraint",
			}
			return pgconn.CommandTag{}, pgErr
		},
	}

	repo := NewClaimRepositoryWithPool(&mockClaimPool{})
	err := repo.Insert(context.Background(), mockTx, "user_001", "PROMO_SUPER")

	require.Error(t, err)
	assert.True(t, errors.Is(err, service.ErrAlreadyClaimed), "should return ErrAlreadyClaimed for duplicate")
}

func TestClaimRepository_Insert_DatabaseError(t *testing.T) {
	dbErr := errors.New("database connection failed")
	mockTx := &mockTxQuerier{
		execFn: func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, dbErr
		},
	}

	repo := NewClaimRepositoryWithPool(&mockClaimPool{})
	err := repo.Insert(context.Background(), mockTx, "user_001", "PROMO_SUPER")

	require.Error(t, err)
	assert.False(t, errors.Is(err, service.ErrAlreadyClaimed), "should not return ErrAlreadyClaimed for generic error")
	assert.Contains(t, err.Error(), "insert claim")
	assert.True(t, errors.Is(err, dbErr), "should wrap original error")
}

func TestClaimRepository_Insert_OtherPgError(t *testing.T) {
	mockTx := &mockTxQuerier{
		execFn: func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
			// Simulate a different PostgreSQL error (not 23505)
			pgErr := &pgconn.PgError{
				Code:    "23503", // foreign_key_violation
				Message: "insert or update on table violates foreign key constraint",
			}
			return pgconn.CommandTag{}, pgErr
		},
	}

	repo := NewClaimRepositoryWithPool(&mockClaimPool{})
	err := repo.Insert(context.Background(), mockTx, "user_001", "NONEXISTENT")

	require.Error(t, err)
	assert.False(t, errors.Is(err, service.ErrAlreadyClaimed), "should not return ErrAlreadyClaimed for non-23505 error")
	assert.Contains(t, err.Error(), "insert claim")
}

func TestClaimRepository_Insert_VerifiesParameterizedQuery(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any
	mockTx := &mockTxQuerier{
		execFn: func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			capturedArgs = arguments
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		},
	}

	repo := NewClaimRepositoryWithPool(&mockClaimPool{})

	// Test with SQL injection attempt
	_ = repo.Insert(context.Background(), mockTx, "'; DROP TABLE claims;--", "PROMO_SUPER")

	// Verify parameterized query
	assert.Contains(t, capturedSQL, "$1")
	assert.Contains(t, capturedSQL, "$2")
	assert.NotContains(t, capturedSQL, "DROP TABLE", "SQL injection should not appear in query")
	assert.Equal(t, "'; DROP TABLE claims;--", capturedArgs[0], "User ID should be passed as parameter")
}

// TestNewClaimRepository_Production tests the production constructor.
// Note: This constructor is typically tested via integration tests with a real pgxpool.Pool.
// This test verifies the constructor exists and returns a non-nil repository.
func TestNewClaimRepository_Production(t *testing.T) {
	// NewClaimRepository requires a *pgxpool.Pool which implements ClaimPoolInterface.
	// Passing nil is valid for constructor testing - actual usage requires a real pool.
	repo := NewClaimRepository(nil)
	require.NotNil(t, repo, "NewClaimRepository should return a non-nil repository")
}
