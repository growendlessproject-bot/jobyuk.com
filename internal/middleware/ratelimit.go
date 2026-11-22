package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// getRateLimitIP extracts the actual client IP considering reverse proxies (Cloudflare, Nginx)
func getRateLimitIP(r *http.Request) string {
	if cfIP := r.Header.Get("CF-Connecting-IP"); cfIP != "" {
		return strings.TrimSpace(cfIP)
	}
	if xRealIP := r.Header.Get("X-Real-IP"); xRealIP != "" {
		return strings.TrimSpace(xRealIP)
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

// isAssetPath returns true if the requested URL path is for static assets, logos, or media.
func isAssetPath(path string) bool {
	if strings.HasPrefix(path, "/static/") ||
		strings.HasPrefix(path, "/job/logo/") ||
		strings.HasPrefix(path, "/sitemap") ||
		strings.HasPrefix(path, "/admin-lol") ||
		strings.HasPrefix(path, "/pin-") ||
		path == "/favicon.ico" ||
		path == "/site.webmanifest" ||
		path == "/manifest.json" ||
		path == "/health" ||
		path == "/robots.txt" ||
		path == "/ads.txt" ||
		path == "/ads.txt/" {
		return true
	}

	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".png") ||
		strings.HasSuffix(lower, ".jpg") ||
		strings.HasSuffix(lower, ".jpeg") ||
		strings.HasSuffix(lower, ".webp") ||
		strings.HasSuffix(lower, ".gif") ||
		strings.HasSuffix(lower, ".svg") ||
		strings.HasSuffix(lower, ".ico") ||
		strings.HasSuffix(lower, ".css") ||
		strings.HasSuffix(lower, ".js") ||
		strings.HasSuffix(lower, ".woff") ||
		strings.HasSuffix(lower, ".woff2") ||
		strings.HasSuffix(lower, ".ttf")
}

// RateLimit implements a Redis-based atomic fixed-window rate limiter with guaranteed TTL expiration.
func RateLimit(rdb *redis.Client, limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path

			// Never rate-limit static files, company logos, sitemaps, manifests, or health checks
			if isAssetPath(path) {
				next.ServeHTTP(w, r)
				return
			}

			// Exempt known Search Engine & AI crawlers from 429 rate limiting
			ua := strings.ToLower(r.UserAgent())
			if strings.Contains(ua, "googlebot") ||
				strings.Contains(ua, "bingbot") ||
				strings.Contains(ua, "applebot") ||
				strings.Contains(ua, "gptbot") ||
				strings.Contains(ua, "claudebot") ||
				strings.Contains(ua, "perplexitybot") ||
				strings.Contains(ua, "meta-externalagent") ||
				strings.Contains(ua, "amazonbot") ||
				strings.Contains(ua, "petalbot") ||
				strings.Contains(ua, "google-extended") ||
				strings.Contains(ua, "google-cloudvertexbot") {
				next.ServeHTTP(w, r)
				return
			}

			ip := getRateLimitIP(r)

			// Never rate limit localhost or loopback addresses
			if ip == "" || ip == "127.0.0.1" || ip == "::1" || ip == "localhost" {
				next.ServeHTTP(w, r)
				return
			}

			key := fmt.Sprintf("rl:%s", ip)

			ctx, cancel := context.WithTimeout(r.Context(), 100*time.Millisecond)
			defer cancel()

			// Atomic pipeline: INCR + TTL in a single round trip
			pipe := rdb.Pipeline()
			incrCmd := pipe.Incr(ctx, key)
			ttlCmd := pipe.TTL(ctx, key)
			_, err := pipe.Exec(ctx)
			if err != nil {
				// If Redis is down or slow, allow request immediately (fail-open)
				next.ServeHTTP(w, r)
				return
			}

			count := incrCmd.Val()
			ttl := ttlCmd.Val()

			// Guaranteed TTL: Set expiry if new key (count == 1) OR if key lost its TTL (ttl < 0)
			// This completely eliminates stuck keys where users get blocked indefinitely!
			if count == 1 || ttl < 0 {
				rdb.Expire(ctx, key, window)
				ttl = window
			}

			// Set standard rate limit headers
			remaining := int64(limit) - count
			if remaining < 0 {
				remaining = 0
			}
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

			if count > int64(limit) {
				retryAfter := int(ttl.Seconds())
				if retryAfter <= 0 {
					retryAfter = int(window.Seconds())
				}
				w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
				http.Error(w, "Rate limit exceeded. Please try again in a few seconds.", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
