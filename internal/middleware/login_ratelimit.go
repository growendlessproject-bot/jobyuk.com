package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

// LoginRateLimit enforces strict rate limiting on login attempts.
// Allows maxAttempts per window per IP. Much stricter than global rate limit.
func LoginRateLimit(rdb *redis.Client, maxAttempts int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only rate-limit POST requests (actual login attempts)
			if r.Method != http.MethodPost {
				next.ServeHTTP(w, r)
				return
			}

			ip := getRateLimitIP(r)
			if ip == "" || ip == "127.0.0.1" || ip == "::1" || ip == "localhost" {
				next.ServeHTTP(w, r)
				return
			}
			key := fmt.Sprintf("login_rl:%s", ip)

			ctx, cancel := context.WithTimeout(r.Context(), 100*time.Millisecond)
			defer cancel()

			// Atomic pipeline: INCR + TTL in a single round trip
			pipe := rdb.Pipeline()
			incrCmd := pipe.Incr(ctx, key)
			ttlCmd := pipe.TTL(ctx, key)
			_, err := pipe.Exec(ctx)
			if err != nil {
				// If Redis is down, allow request (fail-open)
				next.ServeHTTP(w, r)
				return
			}

			count := incrCmd.Val()
			ttl := ttlCmd.Val()

			if count == 1 || ttl < 0 {
				rdb.Expire(ctx, key, window)
				ttl = window
			}

			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", maxAttempts))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", max(0, int64(maxAttempts)-count)))

			if count > int64(maxAttempts) {
				ttl, _ := rdb.TTL(ctx, key).Result()
				w.Header().Set("Retry-After", fmt.Sprintf("%.0f", ttl.Seconds()))
				http.Error(w, "Too many login attempts. Please try again later.", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
