package handler

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

// Pinger is an interface for health check ping operations.
type Pinger interface {
	Ping(ctx context.Context) error
}

// HealthHandler handles health check requests.
type HealthHandler struct {
	pool Pinger
}

// NewHealthHandler creates a new HealthHandler with the given database pool.
func NewHealthHandler(pool Pinger) *HealthHandler {
	return &HealthHandler{pool: pool}
}

// Check performs a health check by pinging the database.
// Returns 200 OK with {"status": "healthy"} when database is reachable.
// Returns 503 Service Unavailable with {"status": "unhealthy", "error": "..."} when database is unreachable.
func (h *HealthHandler) Check(c *fiber.Ctx) error {
	if err := h.pool.Ping(c.Context()); err != nil {
		log.Error().Err(err).Msg("health check failed: database unreachable")
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status": "unhealthy",
			"error":  "database connection failed",
		})
	}
	return c.JSON(fiber.Map{
		"status": "healthy",
	})
}
