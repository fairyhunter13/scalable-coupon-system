package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fairyhunter13/scalable-coupon-system/internal/model"
	"github.com/fairyhunter13/scalable-coupon-system/pkg/database"
)

// CouponRepositoryInterface defines the interface for coupon data access.
type CouponRepositoryInterface interface {
	Insert(ctx context.Context, coupon *model.Coupon) error
	GetByName(ctx context.Context, name string) (*model.Coupon, error)
	GetCouponForUpdate(ctx context.Context, tx database.TxQuerier, name string) (*model.Coupon, error)
	DecrementStock(ctx context.Context, tx database.TxQuerier, name string) error
}

// ClaimRepositoryInterface defines the interface for claim data access.
type ClaimRepositoryInterface interface {
	GetUsersByCoupon(ctx context.Context, couponName string) ([]string, error)
	Insert(ctx context.Context, tx database.TxQuerier, userID, couponName string) error
}

// TxBeginner defines the interface for beginning transactions.
type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// CouponService provides business logic for coupon operations.
type CouponService struct {
	pool       TxBeginner
	couponRepo CouponRepositoryInterface
	claimRepo  ClaimRepositoryInterface
}

// NewCouponService creates a new CouponService with the given pool and repositories.
func NewCouponService(pool *pgxpool.Pool, couponRepo CouponRepositoryInterface, claimRepo ClaimRepositoryInterface) *CouponService {
	return &CouponService{
		pool:       pool,
		couponRepo: couponRepo,
		claimRepo:  claimRepo,
	}
}

// NewCouponServiceWithTxBeginner creates a CouponService with a custom TxBeginner.
// Primarily used for testing.
func NewCouponServiceWithTxBeginner(pool TxBeginner, couponRepo CouponRepositoryInterface, claimRepo ClaimRepositoryInterface) *CouponService {
	return &CouponService{
		pool:       pool,
		couponRepo: couponRepo,
		claimRepo:  claimRepo,
	}
}

// Create creates a new coupon from the request.
// Returns ErrCouponExists if a coupon with the same name already exists.
// Returns ErrInvalidRequest if request data is nil or incomplete.
func (s *CouponService) Create(ctx context.Context, req *model.CreateCouponRequest) error {
	// Defense-in-depth: check for nil pointer even though handler validates
	if req == nil || req.Amount == nil {
		return ErrInvalidRequest
	}

	coupon := &model.Coupon{
		Name:            req.Name,
		Amount:          *req.Amount,
		RemainingAmount: *req.Amount,
	}
	return s.couponRepo.Insert(ctx, coupon)
}

// GetByName retrieves a coupon by name with its claim list.
// Returns ErrCouponNotFound if the coupon doesn't exist.
func (s *CouponService) GetByName(ctx context.Context, name string) (*model.CouponResponse, error) {
	coupon, err := s.couponRepo.GetByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("get coupon: %w", err)
	}
	if coupon == nil {
		return nil, ErrCouponNotFound
	}

	claimedBy, err := s.claimRepo.GetUsersByCoupon(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("get claims: %w", err)
	}

	return &model.CouponResponse{
		Name:            coupon.Name,
		Amount:          coupon.Amount,
		RemainingAmount: coupon.RemainingAmount,
		ClaimedBy:       claimedBy,
	}, nil
}

// ClaimCoupon atomically claims a coupon for a user.
// Uses SELECT FOR UPDATE to lock the coupon row during the transaction.
// Returns:
//   - ErrCouponNotFound if the coupon doesn't exist
//   - ErrNoStock if the coupon has no remaining stock
//   - ErrAlreadyClaimed if the user has already claimed this coupon
func (s *CouponService) ClaimCoupon(ctx context.Context, userID, couponName string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // Safe: no-op if committed

	// 1. Lock the coupon row (SELECT FOR UPDATE)
	coupon, err := s.couponRepo.GetCouponForUpdate(ctx, tx, couponName)
	if err != nil {
		if errors.Is(err, ErrCouponNotFound) {
			return ErrCouponNotFound
		}
		return fmt.Errorf("get coupon for update: %w", err)
	}

	// 2. Check stock
	if coupon.RemainingAmount <= 0 {
		return ErrNoStock
	}

	// 3. Insert claim (UNIQUE constraint catches duplicates)
	err = s.claimRepo.Insert(ctx, tx, userID, couponName)
	if err != nil {
		if errors.Is(err, ErrAlreadyClaimed) {
			return ErrAlreadyClaimed
		}
		return fmt.Errorf("insert claim: %w", err)
	}

	// 4. Decrement stock
	err = s.couponRepo.DecrementStock(ctx, tx, couponName)
	if err != nil {
		return fmt.Errorf("decrement stock: %w", err)
	}

	return tx.Commit(ctx)
}
