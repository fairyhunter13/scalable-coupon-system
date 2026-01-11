package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fairyhunter13/scalable-coupon-system/internal/service"
	"github.com/fairyhunter13/scalable-coupon-system/pkg/database"
)

// ClaimPoolInterface defines the database operations needed by ClaimRepository.
type ClaimPoolInterface interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// ClaimRepository provides data access for claims using pgx.
type ClaimRepository struct {
	pool ClaimPoolInterface
}

// NewClaimRepository creates a new ClaimRepository with the given pool.
func NewClaimRepository(pool *pgxpool.Pool) *ClaimRepository {
	return &ClaimRepository{pool: pool}
}

// NewClaimRepositoryWithPool creates a new ClaimRepository with a custom pool interface.
// This is primarily used for testing.
func NewClaimRepositoryWithPool(pool ClaimPoolInterface) *ClaimRepository {
	return &ClaimRepository{pool: pool}
}

// GetUsersByCoupon retrieves all user IDs who have claimed a specific coupon.
// On success, returns an empty slice (not nil) when no claims exist.
// On error, returns nil and the wrapped error.
func (r *ClaimRepository) GetUsersByCoupon(ctx context.Context, couponName string) ([]string, error) {
	query := `SELECT user_id FROM claims WHERE coupon_name = $1 ORDER BY created_at`

	rows, err := r.pool.Query(ctx, query, couponName)
	if err != nil {
		return nil, fmt.Errorf("get claims for coupon %s: %w", couponName, err)
	}
	defer rows.Close()

	var users []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("scan claim user_id: %w", err)
		}
		users = append(users, userID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claims rows: %w", err)
	}

	// Return empty slice, not nil
	if users == nil {
		users = []string{}
	}

	return users, nil
}

// Insert inserts a new claim record within a transaction.
// Returns service.ErrAlreadyClaimed if the user has already claimed this coupon.
func (r *ClaimRepository) Insert(ctx context.Context, tx database.TxQuerier, userID, couponName string) error {
	query := `INSERT INTO claims (user_id, coupon_name) VALUES ($1, $2)`

	_, err := tx.Exec(ctx, query, userID, couponName)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return service.ErrAlreadyClaimed
		}
		return fmt.Errorf("insert claim: %w", err)
	}
	return nil
}
