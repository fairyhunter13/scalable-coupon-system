package service

import "errors"

var (
	// ErrCouponExists is returned when attempting to create a coupon that already exists
	ErrCouponExists = errors.New("coupon already exists")

	// ErrCouponNotFound is returned when a coupon cannot be found
	ErrCouponNotFound = errors.New("coupon not found")

	// ErrInvalidRequest is returned when request data is invalid or incomplete
	ErrInvalidRequest = errors.New("invalid request")

	// ErrAlreadyClaimed is returned when a user attempts to claim a coupon they already claimed
	ErrAlreadyClaimed = errors.New("coupon already claimed by user")

	// ErrNoStock is returned when a coupon has no remaining stock
	ErrNoStock = errors.New("coupon out of stock")
)
