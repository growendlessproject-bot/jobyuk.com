package middleware

import (
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"
)

// SecurityHeaders adds comprehensive HTTP security headers.
func SecurityHeaders(isDev bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Relax CSP in dev for http images
			imgSrc := "img-src 'self' data: https:"
			if isDev {
				imgSrc = "img-src 'self' data: http: https:"
			}

			// Content Security Policy (Configured for Google AdSense, GAM & Prebid RTB programmatic advertising)
			w.Header().Set("Content-Security-Policy",
				"default-src 'self'; "+
					"script-src 'self' 'unsafe-inline' 'unsafe-eval' https: data: blob:; "+
					"style-src 'self' 'unsafe-inline' https:; "+
					imgSrc+"; "+
					"font-src 'self' https: data:; "+
					"media-src 'self' https: data: blob:; "+
					"connect-src 'self' https: wss: data: blob:; "+
					"frame-src 'self' https: data: blob:; "+
					"child-src 'self' https: blob:; "+
					"worker-src 'self' blob: https:; "+
					"frame-ancestors 'none'; "+
					"base-uri 'self'; "+
					"form-action 'self'")

			// HTTPS enforcement (only in production; HSTS on localhost breaks local HTTP dev servers)
			if !isDev {
				w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
			}

			// Prevent MIME type sniffing
			w.Header().Set("X-Content-Type-Options", "nosniff")

			// Prevent clickjacking
			w.Header().Set("X-Frame-Options", "DENY")

			// Control referrer information
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// Restrict browser features
			w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), interest-cohort=()")

			// Content Signals: Explicit consent declaration for AI search, grounding/RAG, and training
			w.Header().Set("Content-Signal", "search=yes, ai-input=yes, ai-train=yes")

			// Remove server identification headers
			w.Header().Del("X-Powered-By")
			w.Header().Del("Server")

			next.ServeHTTP(w, r)
		})
	}
}

// Recovery recovers from panics, logs the error, and invokes a custom error handler.
func Recovery(logger *slog.Logger, errorHandler func(http.ResponseWriter, *http.Request, interface{})) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("Panic recovered",
						"error", err,
						"path", r.URL.Path,
						"stack", string(debug.Stack()),
					)
					if errorHandler != nil {
						errorHandler(w, r, err)
					} else {
						http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					}
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// RequestLogger logs HTTP request details with timing.
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			ww := &statusWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(ww, r)

			duration := time.Since(start)

			// Skip logging for static files to reduce noise
			if strings.HasPrefix(r.URL.Path, "/static/") {
				return
			}

			logger.Info("HTTP request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.statusCode,
				"duration_ms", duration.Milliseconds(),
				"ip", r.RemoteAddr,
				"user_agent", r.UserAgent(),
			)
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// Flush implements http.Flusher for SSE/streaming support.
func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

var (
	// zstdEncPool caches Zstandard encoders configured for ultra-low latency & fastest compression.
	zstdEncPool = sync.Pool{
		New: func() interface{} {
			enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedFastest), zstd.WithWindowSize(1<<17))
			if err != nil {
				return nil
			}
			return enc
		},
	}

	// gzipEncPool caches Gzip writers tuned to BestSpeed for near-instant compression with SIMD speed.
	gzipEncPool = sync.Pool{
		New: func() interface{} {
			w, err := gzip.NewWriterLevel(nil, gzip.BestSpeed)
			if err != nil {
				return nil
			}
			return w
		},
	}
)

type compressResponseWriter struct {
	http.ResponseWriter
	writer io.Writer
}

func (w *compressResponseWriter) Write(b []byte) (int, error) {
	return w.writer.Write(b)
}

func (w *compressResponseWriter) WriteHeader(code int) {
	// Remove Content-Length since compression changes the payload size.
	// This prevents browser stream truncation or hanging ad assets.
	w.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(code)
}

// Flush implements http.Flusher for SSE/streaming support.
func (w *compressResponseWriter) Flush() {
	if f, ok := w.writer.(interface{ Flush() error }); ok {
		_ = f.Flush()
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap supports Go 1.20+ ResponseWriter unwrapping
func (w *compressResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// Compress applies smart content negotiation:
// 1. Zstandard (zstd) - Fastest compression/decompression for modern browsers (Chrome 123+, Firefox 126+, Safari 18+).
// 2. Gzip (gzip) - Universal fallback with Klauspost SIMD optimization for all other browsers/crawlers (Googlebot, AdSense bot).
// 3. Uncompressed - Fallback if client doesn't accept compression or for WebSockets / streaming.
//
// Guaranteed safe for Google AdSense, scripts, analytics, and dynamic DOM rendering.
func Compress() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Always inform CDNs and browser caches to cache variants per Accept-Encoding
			w.Header().Add("Vary", "Accept-Encoding")

			// Skip compression for WebSocket upgrades, HEAD requests, or event streams
			if r.Method == http.MethodHead || r.Header.Get("Upgrade") != "" || strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
				next.ServeHTTP(w, r)
				return
			}

			accept := r.Header.Get("Accept-Encoding")

			// 1. Zstandard (Fastest CPU throughput + highest modern ratio)
			if strings.Contains(accept, "zstd") {
				if item := zstdEncPool.Get(); item != nil {
					enc := item.(*zstd.Encoder)
					enc.Reset(w)
					defer func() {
						_ = enc.Close()
						zstdEncPool.Put(enc)
					}()

					w.Header().Set("Content-Encoding", "zstd")
					w.Header().Set("X-Compression-Engine", "zstd-fastest (Meta Zstandard)")
					w.Header().Del("Content-Length")

					cw := &compressResponseWriter{ResponseWriter: w, writer: enc}
					next.ServeHTTP(cw, r)
					return
				}
			}

			// 2. Gzip Fallback (Universal 100% compatibility for Googlebot, AdSense crawler & older clients)
			if strings.Contains(accept, "gzip") {
				if item := gzipEncPool.Get(); item != nil {
					gz := item.(*gzip.Writer)
					gz.Reset(w)
					defer func() {
						_ = gz.Close()
						gzipEncPool.Put(gz)
					}()

					w.Header().Set("Content-Encoding", "gzip")
					w.Header().Set("X-Compression-Engine", "gzip-fastest (Klauspost SIMD)")
					w.Header().Del("Content-Length")

					cw := &compressResponseWriter{ResponseWriter: w, writer: gz}
					next.ServeHTTP(cw, r)
					return
				}
			}

			// 3. Uncompressed fallback
			next.ServeHTTP(w, r)
		})
	}
}

// CacheControl returns a middleware that sets Cache-Control header.
func CacheControl(value string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", value)
			next.ServeHTTP(w, r)
		})
	}
}

// EnforceTrailingSlash redirects requests without a trailing slash to the trailing slash version,
// except for static files, sitemaps, or files with extensions.
func EnforceTrailingSlash() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only redirect GET and HEAD requests to prevent breaking POST/PUT/DELETE body payloads
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				next.ServeHTTP(w, r)
				return
			}

			path := r.URL.Path

			// Don't redirect if it's the homepage, has a trailing slash, or has a file extension (like .css, .xml)
			if path == "/" || strings.HasSuffix(path, "/") || strings.Contains(filepath.Base(path), ".") {
				next.ServeHTTP(w, r)
				return
			}

			// Don't redirect admin routes, wa portal, pin portal, obsolete /admin path, or health endpoint without trailing slashes
			if strings.HasPrefix(path, "/admin-lol") || strings.HasPrefix(path, "/admin") || strings.HasPrefix(path, "/wa-portal") || strings.HasPrefix(path, "/pin-") || path == "/health" {
				next.ServeHTTP(w, r)
				return
			}

			// Redirect to the trailing slash version
			r.URL.Path = path + "/"
			http.Redirect(w, r, r.URL.String(), http.StatusMovedPermanently)
		})
	}
}
