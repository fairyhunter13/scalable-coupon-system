package validator

import (
	"strings"

	"github.com/go-playground/validator/v10"
)

// New creates a new validator instance with custom validations registered.
// This ensures consistent validation across the application and tests.
func New() *validator.Validate {
	v := validator.New()

	// Register custom "notblank" validator - rejects whitespace-only strings
	// This is used for fields like coupon names that must have meaningful content
	_ = v.RegisterValidation("notblank", func(fl validator.FieldLevel) bool {
		str, ok := fl.Field().Interface().(string)
		if !ok {
			return true // Not a string, let other validators handle it
		}
		return strings.TrimSpace(str) != ""
	})

	return v
}
