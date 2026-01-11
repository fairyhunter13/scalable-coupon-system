package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/fairyhunter13/scalable-coupon-system/internal/config"
	"github.com/fairyhunter13/scalable-coupon-system/internal/handler"
	"github.com/fairyhunter13/scalable-coupon-system/internal/repository"
	"github.com/fairyhunter13/scalable-coupon-system/internal/service"
	"github.com/fairyhunter13/scalable-coupon-system/pkg/database"
)

func main() {
	// Load configuration first
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load configuration")
	}

	// Initialize zerolog based on configuration
	initLogger(cfg)

	// Create context for startup
	ctx := context.Background()

	// Initialize database pool with retry
	pool, err := database.NewPool(ctx, cfg.DB.DSN(), 5)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}

	// Initialize Fiber with production-ready configuration
	app := fiber.New(fiber.Config{
		AppName:      "Scalable Coupon System",
		ReadTimeout:  30 * time.Second,  // Max time to read request
		WriteTimeout: 30 * time.Second,  // Max time to write response
		IdleTimeout:  120 * time.Second, // Max time for keep-alive connections
		BodyLimit:    1 * 1024 * 1024,   // 1MB body limit (explicit, prevents large payloads)
	})

	// Middleware
	app.Use(recover.New())
	app.Use(requestid.New()) // Adds X-Request-ID header to all requests
	app.Use(logger.New())

	// Initialize validator
	validate := validator.New()

	// Initialize coupon components (layered architecture)
	couponRepo := repository.NewCouponRepository(pool)
	claimRepo := repository.NewClaimRepository(pool)
	couponService := service.NewCouponService(pool, couponRepo, claimRepo)
	couponHandler := handler.NewCouponHandler(couponService, validate)
	claimHandler := handler.NewClaimHandler(couponService, validate)

	// Health handler
	healthHandler := handler.NewHealthHandler(pool)
	app.Get("/health", healthHandler.Check)

	// Coupon routes
	app.Post("/api/coupons", couponHandler.CreateCoupon)
	app.Get("/api/coupons/:name", couponHandler.GetCoupon)
	app.Post("/api/coupons/claim", claimHandler.ClaimCoupon)

	// Start server with graceful shutdown
	go func() {
		log.Info().Str("port", cfg.Server.Port).Msg("starting server")
		if err := app.Listen(":" + cfg.Server.Port); err != nil {
			log.Fatal().Err(err).Msg("failed to start server")
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	log.Info().Str("signal", sig.String()).Msg("received shutdown signal")
	log.Info().Int("timeout_seconds", cfg.Server.ShutdownTimeout).Msg("shutting down server...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(
		context.Background(),
		time.Duration(cfg.Server.ShutdownTimeout)*time.Second,
	)
	defer shutdownCancel()

	// Shutdown server (waits for in-flight requests)
	log.Info().Msg("waiting for in-flight requests to complete...")
	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("error during server shutdown")
	}

	// Close database pool AFTER server shutdown (even if shutdown timed out)
	log.Info().Msg("closing database connections...")
	pool.Close()
	log.Info().Msg("database connections closed")
	log.Info().Msg("server stopped")
}

// initLogger configures zerolog based on the application configuration.
func initLogger(cfg *config.Config) {
	// Set log level
	level, err := zerolog.ParseLevel(cfg.Log.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Configure output format
	if cfg.Log.Pretty {
		// Human-readable output for development
		log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).
			With().Timestamp().Logger()
	} else {
		// JSON output for production
		zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
		log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	}
}
