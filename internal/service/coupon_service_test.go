package service

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
	"github.com/fairyhunter13/scalable-coupon-system/pkg/database"
)

// mockCouponRepository is a mock implementation of CouponRepositoryInterface.
type mockCouponRepository struct {
	insertFn             func(ctx context.Context, coupon *model.Coupon) error
	getByNameFn          func(ctx context.Context, name string) (*model.Coupon, error)
	getCouponForUpdateFn func(ctx context.Context, tx database.TxQuerier, name string) (*model.Coupon, error)
	decrementStockFn     func(ctx context.Context, tx database.TxQuerier, name string) error
}

func (m *mockCouponRepository) Insert(ctx context.Context, coupon *model.Coupon) error {
	if m.insertFn != nil {
		return m.insertFn(ctx, coupon)
	}
	return nil
}

func (m *mockCouponRepository) GetByName(ctx context.Context, name string) (*model.Coupon, error) {
	if m.getByNameFn != nil {
		return m.getByNameFn(ctx, name)
	}
	return nil, nil
}

func (m *mockCouponRepository) GetCouponForUpdate(ctx context.Context, tx database.TxQuerier, name string) (*model.Coupon, error) {
	if m.getCouponForUpdateFn != nil {
		return m.getCouponForUpdateFn(ctx, tx, name)
	}
	return nil, nil
}

func (m *mockCouponRepository) DecrementStock(ctx context.Context, tx database.TxQuerier, name string) error {
	if m.decrementStockFn != nil {
		return m.decrementStockFn(ctx, tx, name)
	}
	return nil
}

// mockClaimRepository is a mock implementation of ClaimRepositoryInterface.
type mockClaimRepository struct {
	getUsersByCouponFn func(ctx context.Context, couponName string) ([]string, error)
	insertFn           func(ctx context.Context, tx database.TxQuerier, userID, couponName string) error
}

func (m *mockClaimRepository) GetUsersByCoupon(ctx context.Context, couponName string) ([]string, error) {
	if m.getUsersByCouponFn != nil {
		return m.getUsersByCouponFn(ctx, couponName)
	}
	return []string{}, nil
}

func (m *mockClaimRepository) Insert(ctx context.Context, tx database.TxQuerier, userID, couponName string) error {
	if m.insertFn != nil {
		return m.insertFn(ctx, tx, userID, couponName)
	}
	return nil
}

func intPtr(i int) *int {
	return &i
}

func TestCouponService_Create_Success(t *testing.T) {
	var capturedCoupon *model.Coupon
	mockCouponRepo := &mockCouponRepository{
		insertFn: func(ctx context.Context, coupon *model.Coupon) error {
			capturedCoupon = coupon
			return nil
		},
	}
	mockClaimRepo := &mockClaimRepository{}

	svc := NewCouponService(nil, mockCouponRepo, mockClaimRepo)
	req := &model.CreateCouponRequest{
		Name:   "PROMO_SUPER",
		Amount: intPtr(100),
	}

	err := svc.Create(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "PROMO_SUPER", capturedCoupon.Name)
	assert.Equal(t, 100, capturedCoupon.Amount)
	assert.Equal(t, 100, capturedCoupon.RemainingAmount, "RemainingAmount should equal Amount on creation")
}

func TestCouponService_Create_DuplicateCoupon(t *testing.T) {
	mockCouponRepo := &mockCouponRepository{
		insertFn: func(ctx context.Context, coupon *model.Coupon) error {
			return ErrCouponExists
		},
	}
	mockClaimRepo := &mockClaimRepository{}

	svc := NewCouponService(nil, mockCouponRepo, mockClaimRepo)
	req := &model.CreateCouponRequest{
		Name:   "PROMO_SUPER",
		Amount: intPtr(100),
	}

	err := svc.Create(context.Background(), req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrCouponExists), "error should be ErrCouponExists")
}

func TestCouponService_Create_RepositoryError(t *testing.T) {
	repoErr := errors.New("database connection failed")
	mockCouponRepo := &mockCouponRepository{
		insertFn: func(ctx context.Context, coupon *model.Coupon) error {
			return repoErr
		},
	}
	mockClaimRepo := &mockClaimRepository{}

	svc := NewCouponService(nil, mockCouponRepo, mockClaimRepo)
	req := &model.CreateCouponRequest{
		Name:   "PROMO_SUPER",
		Amount: intPtr(50),
	}

	err := svc.Create(context.Background(), req)

	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrCouponExists), "error should not be ErrCouponExists")
}

func TestCouponService_Create_NilRequest(t *testing.T) {
	mockCouponRepo := &mockCouponRepository{}
	mockClaimRepo := &mockClaimRepository{}
	svc := NewCouponService(nil, mockCouponRepo, mockClaimRepo)

	err := svc.Create(context.Background(), nil)

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidRequest), "should return ErrInvalidRequest for nil request")
}

func TestCouponService_Create_NilAmount(t *testing.T) {
	mockCouponRepo := &mockCouponRepository{}
	mockClaimRepo := &mockClaimRepository{}
	svc := NewCouponService(nil, mockCouponRepo, mockClaimRepo)

	req := &model.CreateCouponRequest{
		Name:   "PROMO_SUPER",
		Amount: nil, // Nil amount
	}

	err := svc.Create(context.Background(), req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidRequest), "should return ErrInvalidRequest for nil amount")
}

func TestCouponService_GetByName_WithClaims(t *testing.T) {
	mockCouponRepo := &mockCouponRepository{
		getByNameFn: func(ctx context.Context, name string) (*model.Coupon, error) {
			return &model.Coupon{
				Name:            "PROMO_SUPER",
				Amount:          100,
				RemainingAmount: 95,
				CreatedAt:       time.Now(),
			}, nil
		},
	}
	mockClaimRepo := &mockClaimRepository{
		getUsersByCouponFn: func(ctx context.Context, couponName string) ([]string, error) {
			return []string{"user_001", "user_002", "user_003", "user_004", "user_005"}, nil
		},
	}

	svc := NewCouponService(nil, mockCouponRepo, mockClaimRepo)
	resp, err := svc.GetByName(context.Background(), "PROMO_SUPER")

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "PROMO_SUPER", resp.Name)
	assert.Equal(t, 100, resp.Amount)
	assert.Equal(t, 95, resp.RemainingAmount)
	assert.Equal(t, []string{"user_001", "user_002", "user_003", "user_004", "user_005"}, resp.ClaimedBy)
}

func TestCouponService_GetByName_EmptyClaims(t *testing.T) {
	mockCouponRepo := &mockCouponRepository{
		getByNameFn: func(ctx context.Context, name string) (*model.Coupon, error) {
			return &model.Coupon{
				Name:            "NEW_PROMO",
				Amount:          100,
				RemainingAmount: 100,
				CreatedAt:       time.Now(),
			}, nil
		},
	}
	mockClaimRepo := &mockClaimRepository{
		getUsersByCouponFn: func(ctx context.Context, couponName string) ([]string, error) {
			return []string{}, nil // Empty slice, not nil
		},
	}

	svc := NewCouponService(nil, mockCouponRepo, mockClaimRepo)
	resp, err := svc.GetByName(context.Background(), "NEW_PROMO")

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "NEW_PROMO", resp.Name)
	assert.Equal(t, 100, resp.Amount)
	assert.Equal(t, 100, resp.RemainingAmount)
	assert.NotNil(t, resp.ClaimedBy, "ClaimedBy should be empty slice, not nil")
	assert.Len(t, resp.ClaimedBy, 0)
}

func TestCouponService_GetByName_NotFound(t *testing.T) {
	mockCouponRepo := &mockCouponRepository{
		getByNameFn: func(ctx context.Context, name string) (*model.Coupon, error) {
			return nil, nil // Not found
		},
	}
	mockClaimRepo := &mockClaimRepository{}

	svc := NewCouponService(nil, mockCouponRepo, mockClaimRepo)
	resp, err := svc.GetByName(context.Background(), "NONEXISTENT")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrCouponNotFound), "error should be ErrCouponNotFound")
	assert.Nil(t, resp)
}

func TestCouponService_GetByName_CouponRepoError(t *testing.T) {
	dbErr := errors.New("database connection failed")
	mockCouponRepo := &mockCouponRepository{
		getByNameFn: func(ctx context.Context, name string) (*model.Coupon, error) {
			return nil, dbErr
		},
	}
	mockClaimRepo := &mockClaimRepository{}

	svc := NewCouponService(nil, mockCouponRepo, mockClaimRepo)
	resp, err := svc.GetByName(context.Background(), "PROMO_SUPER")

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.False(t, errors.Is(err, ErrCouponNotFound), "error should not be ErrCouponNotFound")
}

func TestCouponService_GetByName_ClaimRepoError(t *testing.T) {
	mockCouponRepo := &mockCouponRepository{
		getByNameFn: func(ctx context.Context, name string) (*model.Coupon, error) {
			return &model.Coupon{
				Name:            "PROMO_SUPER",
				Amount:          100,
				RemainingAmount: 95,
				CreatedAt:       time.Now(),
			}, nil
		},
	}
	dbErr := errors.New("database connection failed")
	mockClaimRepo := &mockClaimRepository{
		getUsersByCouponFn: func(ctx context.Context, couponName string) ([]string, error) {
			return nil, dbErr
		},
	}

	svc := NewCouponService(nil, mockCouponRepo, mockClaimRepo)
	resp, err := svc.GetByName(context.Background(), "PROMO_SUPER")

	require.Error(t, err)
	assert.Nil(t, resp)
}

// mockTx is a mock implementation of pgx.Tx for testing transactions.
type mockTx struct {
	commitFn   func(ctx context.Context) error
	rollbackFn func(ctx context.Context) error
}

func (m *mockTx) Begin(ctx context.Context) (pgx.Tx, error) {
	return nil, errors.New("nested transactions not supported")
}

func (m *mockTx) Commit(ctx context.Context) error {
	if m.commitFn != nil {
		return m.commitFn(ctx)
	}
	return nil
}

func (m *mockTx) Rollback(ctx context.Context) error {
	if m.rollbackFn != nil {
		return m.rollbackFn(ctx)
	}
	return nil
}

func (m *mockTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, nil
}

func (m *mockTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	return nil
}

func (m *mockTx) LargeObjects() pgx.LargeObjects {
	return pgx.LargeObjects{}
}

func (m *mockTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	return nil, nil
}

func (m *mockTx) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (m *mockTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}

func (m *mockTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return nil
}

func (m *mockTx) Conn() *pgx.Conn {
	return nil
}

// mockTxBeginner is a mock implementation of TxBeginner.
type mockTxBeginner struct {
	beginFn func(ctx context.Context) (pgx.Tx, error)
}

func (m *mockTxBeginner) Begin(ctx context.Context) (pgx.Tx, error) {
	if m.beginFn != nil {
		return m.beginFn(ctx)
	}
	return &mockTx{}, nil
}

func TestCouponService_ClaimCoupon_Success(t *testing.T) {
	tx := &mockTx{}
	mockPool := &mockTxBeginner{
		beginFn: func(ctx context.Context) (pgx.Tx, error) {
			return tx, nil
		},
	}
	mockCouponRepo := &mockCouponRepository{
		getCouponForUpdateFn: func(ctx context.Context, tx database.TxQuerier, name string) (*model.Coupon, error) {
			return &model.Coupon{
				Name:            "PROMO_SUPER",
				Amount:          100,
				RemainingAmount: 5,
				CreatedAt:       time.Now(),
			}, nil
		},
		decrementStockFn: func(ctx context.Context, tx database.TxQuerier, name string) error {
			return nil
		},
	}
	mockClaimRepo := &mockClaimRepository{
		insertFn: func(ctx context.Context, tx database.TxQuerier, userID, couponName string) error {
			return nil
		},
	}

	svc := NewCouponServiceWithTxBeginner(mockPool, mockCouponRepo, mockClaimRepo)
	err := svc.ClaimCoupon(context.Background(), "user_001", "PROMO_SUPER")

	require.NoError(t, err)
}

func TestCouponService_ClaimCoupon_DuplicateClaim(t *testing.T) {
	tx := &mockTx{}
	mockPool := &mockTxBeginner{
		beginFn: func(ctx context.Context) (pgx.Tx, error) {
			return tx, nil
		},
	}
	mockCouponRepo := &mockCouponRepository{
		getCouponForUpdateFn: func(ctx context.Context, tx database.TxQuerier, name string) (*model.Coupon, error) {
			return &model.Coupon{
				Name:            "PROMO_SUPER",
				Amount:          100,
				RemainingAmount: 5,
				CreatedAt:       time.Now(),
			}, nil
		},
	}
	mockClaimRepo := &mockClaimRepository{
		insertFn: func(ctx context.Context, tx database.TxQuerier, userID, couponName string) error {
			return ErrAlreadyClaimed
		},
	}

	svc := NewCouponServiceWithTxBeginner(mockPool, mockCouponRepo, mockClaimRepo)
	err := svc.ClaimCoupon(context.Background(), "user_001", "PROMO_SUPER")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAlreadyClaimed), "error should be ErrAlreadyClaimed")
}

func TestCouponService_ClaimCoupon_NoStock(t *testing.T) {
	tx := &mockTx{}
	mockPool := &mockTxBeginner{
		beginFn: func(ctx context.Context) (pgx.Tx, error) {
			return tx, nil
		},
	}
	mockCouponRepo := &mockCouponRepository{
		getCouponForUpdateFn: func(ctx context.Context, tx database.TxQuerier, name string) (*model.Coupon, error) {
			return &model.Coupon{
				Name:            "PROMO_SUPER",
				Amount:          100,
				RemainingAmount: 0, // No stock
				CreatedAt:       time.Now(),
			}, nil
		},
	}
	mockClaimRepo := &mockClaimRepository{}

	svc := NewCouponServiceWithTxBeginner(mockPool, mockCouponRepo, mockClaimRepo)
	err := svc.ClaimCoupon(context.Background(), "user_999", "PROMO_SUPER")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoStock), "error should be ErrNoStock")
}

func TestCouponService_ClaimCoupon_CouponNotFound(t *testing.T) {
	tx := &mockTx{}
	mockPool := &mockTxBeginner{
		beginFn: func(ctx context.Context) (pgx.Tx, error) {
			return tx, nil
		},
	}
	mockCouponRepo := &mockCouponRepository{
		getCouponForUpdateFn: func(ctx context.Context, tx database.TxQuerier, name string) (*model.Coupon, error) {
			return nil, ErrCouponNotFound
		},
	}
	mockClaimRepo := &mockClaimRepository{}

	svc := NewCouponServiceWithTxBeginner(mockPool, mockCouponRepo, mockClaimRepo)
	err := svc.ClaimCoupon(context.Background(), "user_001", "NONEXISTENT")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrCouponNotFound), "error should be ErrCouponNotFound")
}

func TestCouponService_ClaimCoupon_TransactionRollbackOnFailure(t *testing.T) {
	rollbackCalled := false
	tx := &mockTx{
		rollbackFn: func(ctx context.Context) error {
			rollbackCalled = true
			return nil
		},
	}
	mockPool := &mockTxBeginner{
		beginFn: func(ctx context.Context) (pgx.Tx, error) {
			return tx, nil
		},
	}
	mockCouponRepo := &mockCouponRepository{
		getCouponForUpdateFn: func(ctx context.Context, tx database.TxQuerier, name string) (*model.Coupon, error) {
			return nil, ErrCouponNotFound
		},
	}
	mockClaimRepo := &mockClaimRepository{}

	svc := NewCouponServiceWithTxBeginner(mockPool, mockCouponRepo, mockClaimRepo)
	err := svc.ClaimCoupon(context.Background(), "user_001", "NONEXISTENT")

	require.Error(t, err)
	assert.True(t, rollbackCalled, "rollback should be called on failure")
}

func TestCouponService_ClaimCoupon_BeginTxError(t *testing.T) {
	txErr := errors.New("database connection pool exhausted")
	mockPool := &mockTxBeginner{
		beginFn: func(ctx context.Context) (pgx.Tx, error) {
			return nil, txErr
		},
	}
	mockCouponRepo := &mockCouponRepository{}
	mockClaimRepo := &mockClaimRepository{}

	svc := NewCouponServiceWithTxBeginner(mockPool, mockCouponRepo, mockClaimRepo)
	err := svc.ClaimCoupon(context.Background(), "user_001", "PROMO_SUPER")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "begin tx", "error should mention transaction begin")
}

func TestCouponService_ClaimCoupon_GetCouponForUpdateError(t *testing.T) {
	tx := &mockTx{}
	mockPool := &mockTxBeginner{
		beginFn: func(ctx context.Context) (pgx.Tx, error) {
			return tx, nil
		},
	}
	dbErr := errors.New("database query timeout")
	mockCouponRepo := &mockCouponRepository{
		getCouponForUpdateFn: func(ctx context.Context, tx database.TxQuerier, name string) (*model.Coupon, error) {
			return nil, dbErr // Non-ErrCouponNotFound error
		},
	}
	mockClaimRepo := &mockClaimRepository{}

	svc := NewCouponServiceWithTxBeginner(mockPool, mockCouponRepo, mockClaimRepo)
	err := svc.ClaimCoupon(context.Background(), "user_001", "PROMO_SUPER")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "get coupon for update", "error should mention get coupon for update")
	assert.False(t, errors.Is(err, ErrCouponNotFound), "error should not be ErrCouponNotFound")
}

func TestCouponService_ClaimCoupon_ClaimInsertError(t *testing.T) {
	tx := &mockTx{}
	mockPool := &mockTxBeginner{
		beginFn: func(ctx context.Context) (pgx.Tx, error) {
			return tx, nil
		},
	}
	mockCouponRepo := &mockCouponRepository{
		getCouponForUpdateFn: func(ctx context.Context, tx database.TxQuerier, name string) (*model.Coupon, error) {
			return &model.Coupon{
				Name:            "PROMO_SUPER",
				Amount:          100,
				RemainingAmount: 5,
			}, nil
		},
	}
	dbErr := errors.New("database insert timeout")
	mockClaimRepo := &mockClaimRepository{
		insertFn: func(ctx context.Context, tx database.TxQuerier, userID, couponName string) error {
			return dbErr // Non-ErrAlreadyClaimed error
		},
	}

	svc := NewCouponServiceWithTxBeginner(mockPool, mockCouponRepo, mockClaimRepo)
	err := svc.ClaimCoupon(context.Background(), "user_001", "PROMO_SUPER")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "insert claim", "error should mention insert claim")
	assert.False(t, errors.Is(err, ErrAlreadyClaimed), "error should not be ErrAlreadyClaimed")
}

func TestCouponService_ClaimCoupon_DecrementStockError(t *testing.T) {
	tx := &mockTx{}
	mockPool := &mockTxBeginner{
		beginFn: func(ctx context.Context) (pgx.Tx, error) {
			return tx, nil
		},
	}
	mockCouponRepo := &mockCouponRepository{
		getCouponForUpdateFn: func(ctx context.Context, tx database.TxQuerier, name string) (*model.Coupon, error) {
			return &model.Coupon{
				Name:            "PROMO_SUPER",
				Amount:          100,
				RemainingAmount: 5,
			}, nil
		},
		decrementStockFn: func(ctx context.Context, tx database.TxQuerier, name string) error {
			return errors.New("database update timeout")
		},
	}
	mockClaimRepo := &mockClaimRepository{
		insertFn: func(ctx context.Context, tx database.TxQuerier, userID, couponName string) error {
			return nil
		},
	}

	svc := NewCouponServiceWithTxBeginner(mockPool, mockCouponRepo, mockClaimRepo)
	err := svc.ClaimCoupon(context.Background(), "user_001", "PROMO_SUPER")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "decrement stock", "error should mention decrement stock")
}

func TestCouponService_ClaimCoupon_CommitError(t *testing.T) {
	commitErr := errors.New("database commit timeout")
	tx := &mockTx{
		commitFn: func(ctx context.Context) error {
			return commitErr
		},
	}
	mockPool := &mockTxBeginner{
		beginFn: func(ctx context.Context) (pgx.Tx, error) {
			return tx, nil
		},
	}
	mockCouponRepo := &mockCouponRepository{
		getCouponForUpdateFn: func(ctx context.Context, tx database.TxQuerier, name string) (*model.Coupon, error) {
			return &model.Coupon{
				Name:            "PROMO_SUPER",
				Amount:          100,
				RemainingAmount: 5,
			}, nil
		},
		decrementStockFn: func(ctx context.Context, tx database.TxQuerier, name string) error {
			return nil
		},
	}
	mockClaimRepo := &mockClaimRepository{
		insertFn: func(ctx context.Context, tx database.TxQuerier, userID, couponName string) error {
			return nil
		},
	}

	svc := NewCouponServiceWithTxBeginner(mockPool, mockCouponRepo, mockClaimRepo)
	err := svc.ClaimCoupon(context.Background(), "user_001", "PROMO_SUPER")

	require.Error(t, err)
	assert.True(t, errors.Is(err, commitErr), "error should wrap commit error")
}
