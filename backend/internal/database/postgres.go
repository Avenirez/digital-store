package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPostgresPool creates a connection pool to PostgreSQL with production-tuned
// settings. The pool is tested with a ping before being returned.
func NewPostgresPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("postgres: failed to parse database URL: %w", err)
	}

	// ─── Pool tuning for low-memory footprint ────────────────
	config.MaxConns = 20                        // Max concurrent connections
	config.MinConns = 2                         // Keep 2 warm connections
	config.MaxConnLifetime = 30 * time.Minute   // Recycle connections periodically
	config.MaxConnIdleTime = 5 * time.Minute    // Close idle connections quickly
	config.HealthCheckPeriod = 30 * time.Second // Background health checks

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("postgres: failed to create pool: %w", err)
	}

	// Verify connectivity
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: failed to ping database: %w", err)
	}

	return pool, nil
}
