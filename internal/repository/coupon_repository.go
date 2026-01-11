package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fairyhunter13/scalable-coupon-system/internal/model"
	"github.com/fairyhunter13/scalable-coupon-system/internal/service"
	"github.com/fairyhunter13/scalable-coupon-system/pkg/database"
)

// PoolInterface defines the database operations needed by repositories.
// This allows for easier testing with mocks.
type PoolInterface interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// CouponRepository provides data access for coupons using pgx.
type CouponRepository struct {
	pool PoolInterface
}

// NewCouponRepository creates a new CouponRepository with the given pool.
func NewCouponRepository(pool *pgxpool.Pool) *CouponRepository {
	return &CouponRepository{pool: pool}
}

// NewCouponRepositoryWithPool creates a new CouponRepository with a custom pool interface.
// This is primarily used for testing.
func NewCouponRepositoryWithPool(pool PoolInterface) *CouponRepository {
	return &CouponRepository{pool: pool}
}

// Insert inserts a new coupon into the database.
// Returns service.ErrCouponExists if a coupon with the same name already exists.
func (r *CouponRepository) Insert(ctx context.Context, coupon *model.Coupon) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)`,
		coupon.Name, coupon.Amount, coupon.Amount) // remaining_amount = amount
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return service.ErrCouponExists
		}
		return fmt.Errorf("insert coupon: %w", err)
	}
	return nil
}

// GetByName retrieves a coupon by its name.
// Returns nil, nil if the coupon is not found (service layer handles this).
func (r *CouponRepository) GetByName(ctx context.Context, name string) (*model.Coupon, error) {
	query := `SELECT name, amount, remaining_amount, created_at FROM coupons WHERE name = $1`

	var coupon model.Coupon
	err := r.pool.QueryRow(ctx, query, name).Scan(
		&coupon.Name,
		&coupon.Amount,
		&coupon.RemainingAmount,
		&coupon.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Not found - let service handle
		}
		return nil, fmt.Errorf("get coupon by name %s: %w", name, err)
	}
	return &coupon, nil
}

// GetCouponForUpdate retrieves a coupon with a row lock (SELECT FOR UPDATE).
// This locks the row until the transaction completes.
// Returns service.ErrCouponNotFound if the coupon doesn't exist.
func (r *CouponRepository) GetCouponForUpdate(ctx context.Context, tx database.TxQuerier, name string) (*model.Coupon, error) {
	query := `SELECT name, amount, remaining_amount, created_at FROM coupons WHERE name = $1 FOR UPDATE`

	var coupon model.Coupon
	err := tx.QueryRow(ctx, query, name).Scan(
		&coupon.Name,
		&coupon.Amount,
		&coupon.RemainingAmount,
		&coupon.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, service.ErrCouponNotFound
		}
		return nil, fmt.Errorf("get coupon for update %s: %w", name, err)
	}
	return &coupon, nil
}

// DecrementStock decrements the remaining_amount of a coupon by 1.
// Must be called within a transaction after locking the row.
func (r *CouponRepository) DecrementStock(ctx context.Context, tx database.TxQuerier, name string) error {
	query := `UPDATE coupons SET remaining_amount = remaining_amount - 1 WHERE name = $1`

	_, err := tx.Exec(ctx, query, name)
	if err != nil {
		return fmt.Errorf("decrement stock for %s: %w", name, err)
	}
	return nil
}
