package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter is a Redis-based sliding window rate limiter middleware.
type RateLimiter struct {
	client     *redis.Client
	maxReqs    int
	windowSecs int
	prefix     string
}

// NewRateLimiter creates a rate limiter that allows maxReqs requests per windowSecs seconds.
func NewRateLimiter(client *redis.Client, maxReqs, windowSecs int, prefix string) *RateLimiter {
	return &RateLimiter{
		client:     client,
		maxReqs:    maxReqs,
		windowSecs: windowSecs,
		prefix:     prefix,
	}
}

// Middleware returns an HTTP middleware that enforces the rate limit per client IP.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Extract client IP (Chi's RealIP middleware sets this)
		ip := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ip = strings.Split(forwarded, ",")[0]
		}
		if realIP := r.Header.Get("X-Real-Ip"); realIP != "" {
			ip = realIP
		}

		key := fmt.Sprintf("%s:%s", rl.prefix, ip)
		window := time.Duration(rl.windowSecs) * time.Second

		// Increment the counter for this IP
		count, err := rl.client.Incr(ctx, key).Result()
		if err != nil {
			// If Redis is down, allow the request (fail-open)
			next.ServeHTTP(w, r)
			return
		}

		// Set expiry on first request in the window
		if count == 1 {
			rl.client.Expire(ctx, key, window)
		}

		// Check if rate limit exceeded
		if count > int64(rl.maxReqs) {
			ttl, _ := rl.client.TTL(ctx, key).Result()
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(ttl.Seconds())))
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rl.maxReqs))
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprintf(w, `{"error":"rate limit exceeded","retry_after_seconds":%d}`, int(ttl.Seconds()))
			return
		}

		// Set rate limit headers
		remaining := rl.maxReqs - int(count)
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rl.maxReqs))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

		next.ServeHTTP(w, r)
	})
}
