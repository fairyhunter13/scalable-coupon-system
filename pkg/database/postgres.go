package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// TxQuerier is implemented by both pgxpool.Pool and pgx.Tx.
// Repository methods that need transaction support should accept TxQuerier.
type TxQuerier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// NewPool creates a PostgreSQL connection pool with retry logic.
// Retries with exponential backoff: 1s, 2s, 4s, 8s, 16s (total ~31s before failure).
func NewPool(ctx context.Context, dsn string, maxRetries int) (*pgxpool.Pool, error) {
	var pool *pgxpool.Pool
	var err error

	// Ensure at least one attempt even if maxRetries is 0
	attempts := maxRetries
	if attempts < 1 {
		attempts = 1
	}

	for attempt := 0; attempt < attempts; attempt++ {
		pool, err = pgxpool.New(ctx, dsn)
		if err == nil {
			// Verify connection actually works
			if pingErr := pool.Ping(ctx); pingErr == nil {
				log.Info().Msg("database connection established")
				return pool, nil
			} else {
				pool.Close()
				err = fmt.Errorf("ping failed: %w", pingErr)
			}
		}

		backoff := time.Duration(1<<attempt) * time.Second
		log.Warn().
			Err(err).
			Int("attempt", attempt+1).
			Int("max_retries", maxRetries).
			Dur("next_retry_in", backoff).
			Msg("database connection failed, retrying")

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
	}

	return nil, fmt.Errorf("failed to connect after %d attempts: %w", attempts, err)
}
