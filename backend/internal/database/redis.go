package database

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewRedisClient creates a Redis client from a URL string (e.g., "redis://localhost:6379/0").
// The client is tested with a ping before being returned.
func NewRedisClient(ctx context.Context, redisURL string) (*redis.Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("redis: failed to parse URL: %w", err)
	}

	// ─── Connection tuning ───────────────────────────────────
	opts.PoolSize = 10
	opts.MinIdleConns = 2
	opts.ConnMaxIdleTime = 5 * time.Minute
	opts.ConnMaxLifetime = 30 * time.Minute

	client := redis.NewClient(opts)

	// Verify connectivity
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis: failed to ping: %w", err)
	}

	return client, nil
}
