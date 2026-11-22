package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"jobyuk/internal/services"
)

// pageViewEvent holds the data needed to record an analytics event.
type pageViewEvent struct {
	path      string
	referrer  string
	userAgent string
	ip        string
	country   string
}

// analyticsWorkerPool processes page view events with a fixed number of workers.
type analyticsWorkerPool struct {
	events chan pageViewEvent
	svc    *services.AnalyticsService
	salt   string
}

// newAnalyticsWorkerPool creates a pool with the given buffer size and worker count.
func newAnalyticsWorkerPool(svc *services.AnalyticsService, salt string, bufferSize, workers int) *analyticsWorkerPool {
	pool := &analyticsWorkerPool{
		events: make(chan pageViewEvent, bufferSize),
		svc:    svc,
		salt:   salt,
	}

	for i := 0; i < workers; i++ {
		go pool.worker()
	}

	return pool
}

func (p *analyticsWorkerPool) worker() {
	for ev := range p.events {
		err := p.svc.RecordPageView(context.Background(), ev.path, ev.referrer, ev.userAgent, ev.ip, ev.country, p.salt)
		if err != nil {
			slog.Error("Failed to record page view", "path", ev.path, "error", err)
		}
	}
}

func (p *analyticsWorkerPool) submit(ev pageViewEvent) {
	select {
	case p.events <- ev:
		// Submitted successfully
	default:
		// Buffer full, drop the event to avoid blocking
		slog.Warn("Analytics buffer full, dropping page view event", "path", ev.path)
	}
}

// Analytics returns a middleware that logs page views to the database
// using a bounded worker pool instead of unbounded goroutines.
func Analytics(analyticsSvc *services.AnalyticsService, salt string) func(http.Handler) http.Handler {
	// Create a pool with 1000-event buffer and 4 workers
	pool := newAnalyticsWorkerPool(analyticsSvc, salt, 1000, 4)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path

			// 1. Skip logging for static assets, health checks, images, and non-HTML assets
			if strings.HasPrefix(path, "/static/") ||
				strings.HasPrefix(path, "/job/logo/") ||
				strings.HasPrefix(path, "/.well-known/") ||
				strings.HasPrefix(path, "/api/") ||
				path == "/favicon.ico" ||
				path == "/health" ||
				path == "/health/" ||
				path == "/robots.txt" ||
				path == "/ads.txt" ||
				path == "/ads.txt/" ||
				path == "/site.webmanifest" ||
				strings.HasSuffix(path, ".xml") ||
				strings.HasSuffix(path, ".json") ||
				strings.HasSuffix(path, ".png") ||
				strings.HasSuffix(path, ".jpg") ||
				strings.HasSuffix(path, ".jpeg") ||
				strings.HasSuffix(path, ".gif") ||
				strings.HasSuffix(path, ".svg") ||
				strings.HasSuffix(path, ".ico") ||
				strings.HasSuffix(path, ".webp") ||
				strings.HasSuffix(path, ".css") ||
				strings.HasSuffix(path, ".js") ||
				strings.HasSuffix(path, ".map") {
				next.ServeHTTP(w, r)
				return
			}

			// 2. Strictly EXCLUDE Admin Portal & Client Portal routes from public analytics
			if strings.HasPrefix(path, "/wa-portal") ||
				strings.HasPrefix(path, "/my-alerts") ||
				strings.HasPrefix(path, "/wa-client") ||
				strings.HasPrefix(path, "/admin") ||
				strings.HasPrefix(path, "/pin-") {
				next.ServeHTTP(w, r)
				return
			}

			// 3. Strictly EXCLUDE internal sessions (Admins / Client Portal Users browsing public pages)
			if cookie, err := r.Cookie("wa_admin_session"); err == nil && cookie.Value != "" {
				next.ServeHTTP(w, r)
				return
			}
			if cookie, err := r.Cookie("admin_token"); err == nil && cookie.Value != "" {
				next.ServeHTTP(w, r)
				return
			}
			if cookie, err := r.Cookie("pin_session"); err == nil && cookie.Value != "" {
				next.ServeHTTP(w, r)
				return
			}
			if cookie, err := r.Cookie("wa_client_session"); err == nil && cookie.Value != "" {
				next.ServeHTTP(w, r)
				return
			}

			// Capture request details
			referrer := r.Referer()
			userAgent := r.UserAgent()
			ip := r.RemoteAddr

			// Strip port from RemoteAddr if present (e.g. 192.0.2.1:12345 -> 192.0.2.1)
			if idx := strings.LastIndex(ip, ":"); idx != -1 {
				// Handle IPv6 address with port e.g. [::1]:12345
				if !strings.HasPrefix(ip, "[") || strings.HasSuffix(ip[:idx], "]") {
					ip = ip[:idx]
					ip = strings.Trim(ip, "[]")
				}
			}

			// Country from Cloudflare header
			country := r.Header.Get("CF-IPCountry")

			// Submit to worker pool (non-blocking)
			pool.submit(pageViewEvent{
				path:      path,
				referrer:  referrer,
				userAgent: userAgent,
				ip:        ip,
				country:   country,
			})

			next.ServeHTTP(w, r)
		})
	}
}
