package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"jobyuk/internal/models"
	"jobyuk/internal/repository"
)

// redirectCache holds an in-memory copy of redirect rules to avoid
// querying the database on every single HTTP request.
type redirectCache struct {
	mu        sync.RWMutex
	redirects map[string]models.Redirect
	repo      *repository.RedirectRepository
}

// newRedirectCache creates a cache and loads initial data.
func newRedirectCache(repo *repository.RedirectRepository) *redirectCache {
	rc := &redirectCache{
		redirects: make(map[string]models.Redirect),
		repo:      repo,
	}
	rc.refresh()
	return rc
}

// refresh reloads all redirect rules from the database.
func (rc *redirectCache) refresh() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	redirects, err := rc.repo.FindAll(ctx)
	if err != nil {
		slog.Error("Failed to refresh redirect cache", "error", err)
		return
	}

	newMap := make(map[string]models.Redirect, len(redirects))
	for _, rd := range redirects {
		newMap[rd.FromPath] = rd
	}

	rc.mu.Lock()
	rc.redirects = newMap
	rc.mu.Unlock()

	slog.Info("Redirect cache refreshed", "count", len(redirects))
}

// lookup checks the cache for a matching redirect.
func (rc *redirectCache) lookup(path string) (models.Redirect, bool) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	rd, ok := rc.redirects[path]
	return rd, ok
}

// startAutoRefresh periodically refreshes the cache.
func (rc *redirectCache) startAutoRefresh(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			rc.refresh()
		}
	}()
}

// Redirects checks the requested path against the cached redirect table.
// If a match is found, it performs the redirect with the stored status code.
// The cache is refreshed every 2 minutes to pick up admin changes.
func Redirects(repo *repository.RedirectRepository) func(http.Handler) http.Handler {
	cache := newRedirectCache(repo)
	cache.startAutoRefresh(2 * time.Minute)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path

			// Try exact match, then with/without trailing slash
			redirect, ok := cache.lookup(path)
			if !ok && strings.HasSuffix(path, "/") {
				redirect, ok = cache.lookup(strings.TrimSuffix(path, "/"))
			} else if !ok && !strings.HasSuffix(path, "/") {
				redirect, ok = cache.lookup(path + "/")
			}

			if ok {
				statusCode := redirect.StatusCode
				if statusCode != http.StatusMovedPermanently && statusCode != http.StatusFound && statusCode != http.StatusGone {
					statusCode = http.StatusMovedPermanently
				}

				if statusCode == http.StatusGone {
					w.WriteHeader(http.StatusGone)
					return
				}

				http.Redirect(w, r, redirect.ToPath, statusCode)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
