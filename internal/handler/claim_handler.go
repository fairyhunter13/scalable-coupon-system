package handler

import (
	"context"
	"errors"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"

	"github.com/fairyhunter13/scalable-coupon-system/internal/model"
	"github.com/fairyhunter13/scalable-coupon-system/internal/service"
)

// ClaimServiceInterface defines the interface for claim business logic.
type ClaimServiceInterface interface {
	ClaimCoupon(ctx context.Context, userID, couponName string) error
}

// ClaimHandler handles HTTP requests for claim operations.
type ClaimHandler struct {
	service   ClaimServiceInterface
	validator *validator.Validate
}

// NewClaimHandler creates a new ClaimHandler with the given service and validator.
func NewClaimHandler(svc ClaimServiceInterface, v *validator.Validate) *ClaimHandler {
	return &ClaimHandler{service: svc, validator: v}
}

// formatClaimValidationError converts validator errors to AC-required messages for claims.
func formatClaimValidationError(err error) string {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		for _, fe := range ve {
			field := fe.Field()
			tag := fe.Tag()

			switch field {
			case "UserID":
				if tag == "required" {
					return "invalid request: user_id is required"
				}
				if tag == "max" {
					return "invalid request: user_id exceeds maximum length of 255"
				}
				return "invalid request: user_id is invalid"
			case "CouponName":
				if tag == "required" {
					return "invalid request: coupon_name is required"
				}
				if tag == "max" {
					return "invalid request: coupon_name exceeds maximum length of 255"
				}
				return "invalid request: coupon_name is invalid"
			default:
				if tag == "required" {
					return "invalid request: " + field + " is required"
				}
				if tag == "max" {
					return "invalid request: " + field + " exceeds maximum length"
				}
				return "invalid request: " + field + " is invalid"
			}
		}
	}
	return "invalid request"
}

// ClaimCoupon handles POST /api/coupons/claim requests to claim a coupon.
func (h *ClaimHandler) ClaimCoupon(c *fiber.Ctx) error {
	var req model.ClaimCouponRequest

	// Parse JSON body
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	// Validate request
	if err := h.validator.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": formatClaimValidationError(err)})
	}

	// Claim coupon via service
	if err := h.service.ClaimCoupon(c.Context(), req.UserID, req.CouponName); err != nil {
		if errors.Is(err, service.ErrCouponNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "coupon not found"})
		}
		if errors.Is(err, service.ErrAlreadyClaimed) {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "coupon already claimed by user"})
		}
		if errors.Is(err, service.ErrNoStock) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "coupon out of stock"})
		}
		log.Error().
			Err(err).
			Str("request_id", c.GetRespHeader("X-Request-ID")).
			Str("method", c.Method()).
			Str("path", c.Path()).
			Str("user_id", req.UserID).
			Str("coupon_name", req.CouponName).
			Msg("failed to claim coupon")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
	}

	log.Info().
		Str("request_id", c.GetRespHeader("X-Request-ID")).
		Str("method", c.Method()).
		Str("path", c.Path()).
		Str("user_id", req.UserID).
		Str("coupon_name", req.CouponName).
		Msg("coupon claimed successfully")

	return c.Status(fiber.StatusOK).Send(nil)
}
