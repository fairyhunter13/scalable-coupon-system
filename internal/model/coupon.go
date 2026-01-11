package model

import "time"

// Coupon represents a coupon in the system
type Coupon struct {
	Name            string    `json:"name"`
	Amount          int       `json:"amount"`
	RemainingAmount int       `json:"remaining_amount"`
	CreatedAt       time.Time `json:"-"` // Not exposed in API
}

// CouponResponse is the API response DTO for GET /api/coupons/:name
type CouponResponse struct {
	Name            string   `json:"name"`
	Amount          int      `json:"amount"`
	RemainingAmount int      `json:"remaining_amount"`
	ClaimedBy       []string `json:"claimed_by"`
}

// CreateCouponRequest is the DTO for creating a coupon
type CreateCouponRequest struct {
	Name   string `json:"name" validate:"required,notblank,max=255"`
	Amount *int   `json:"amount" validate:"required,gte=1"`
}

// ClaimCouponRequest is the DTO for claiming a coupon
type ClaimCouponRequest struct {
	UserID     string `json:"user_id" validate:"required,notblank,max=255"`
	CouponName string `json:"coupon_name" validate:"required,notblank,max=255"`
}
