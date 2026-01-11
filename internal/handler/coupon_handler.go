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

// CouponServiceInterface defines the interface for coupon business logic.
type CouponServiceInterface interface {
	Create(ctx context.Context, req *model.CreateCouponRequest) error
	GetByName(ctx context.Context, name string) (*model.CouponResponse, error)
}

// CouponHandler handles HTTP requests for coupon operations.
type CouponHandler struct {
	service   CouponServiceInterface
	validator *validator.Validate
}

// NewCouponHandler creates a new CouponHandler with the given service and validator.
func NewCouponHandler(svc CouponServiceInterface, v *validator.Validate) *CouponHandler {
	return &CouponHandler{service: svc, validator: v}
}

// formatValidationError converts validator errors to AC-required messages.
// Provides defensive handling for unknown fields with descriptive fallback messages.
func formatValidationError(err error) string {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		for _, fe := range ve {
			field := fe.Field()
			tag := fe.Tag()

			switch field {
			case "Name":
				if tag == "required" {
					return "invalid request: name is required"
				}
				if tag == "notblank" {
					return "invalid request: name cannot be whitespace only"
				}
				if tag == "max" {
					return "invalid request: name exceeds maximum length of 255"
				}
				return "invalid request: name is invalid"
			case "Amount":
				if tag == "required" {
					return "invalid request: amount is required"
				}
				if tag == "gte" {
					return "invalid request: amount must be at least 1"
				}
				// Defensive: handle other amount validation tags
				return "invalid request: amount is invalid"
			default:
				// Defensive: handle unknown fields with descriptive message
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

// CreateCoupon handles POST /api/coupons requests to create a new coupon.
func (h *CouponHandler) CreateCoupon(c *fiber.Ctx) error {
	var req model.CreateCouponRequest

	// Parse JSON body
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	// Validate request
	if err := h.validator.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": formatValidationError(err)})
	}

	// Create coupon via service
	if err := h.service.Create(c.Context(), &req); err != nil {
		if errors.Is(err, service.ErrCouponExists) {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "coupon already exists"})
		}
		if errors.Is(err, service.ErrInvalidRequest) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
		}
		log.Error().Err(err).Str("coupon_name", req.Name).Msg("failed to create coupon")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
	}

	return c.Status(fiber.StatusCreated).Send(nil)
}

// GetCoupon handles GET /api/coupons/:name requests to retrieve coupon details.
func (h *CouponHandler) GetCoupon(c *fiber.Ctx) error {
	name := c.Params("name")
	if name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request: name is required",
		})
	}

	coupon, err := h.service.GetByName(c.Context(), name)
	if err != nil {
		if errors.Is(err, service.ErrCouponNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "coupon not found",
			})
		}
		log.Error().Err(err).Str("coupon_name", name).Msg("failed to get coupon")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "internal server error",
		})
	}

	log.Info().
		Str("coupon_name", coupon.Name).
		Int("remaining_amount", coupon.RemainingAmount).
		Int("claims_count", len(coupon.ClaimedBy)).
		Msg("coupon retrieved")

	return c.JSON(coupon)
}
