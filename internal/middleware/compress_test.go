package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"
)

func sampleHandler(htmlPayload string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Length", "1000") // Simulate old Content-Length
		_, _ = w.Write([]byte(htmlPayload))
	})
}

func TestCompress_Zstd(t *testing.T) {
	sampleHTML := `<!DOCTYPE html><html><head><script async src="https://pagead2.googlesyndication.com/pagead/js/adsbygoogle.js"></script></head><body><h1>Locksmiths in London</h1><div id="ad-slot"><ins class="adsbygoogle" style="display:block" data-ad-client="ca-pub-12345678" data-ad-slot="98765432"></ins></div></body></html>`

	handler := Compress()(sampleHandler(sampleHTML))

	req := httptest.NewRequest(http.MethodGet, "/services/locksmiths/london", nil)
	req.Header.Set("Accept-Encoding", "zstd, gzip, br")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if ce := res.Header.Get("Content-Encoding"); ce != "zstd" {
		t.Fatalf("expected Content-Encoding 'zstd', got %q", ce)
	}

	if eng := res.Header.Get("X-Compression-Engine"); !strings.Contains(eng, "zstd") {
		t.Fatalf("expected X-Compression-Engine to mention zstd, got %q", eng)
	}

	if vary := res.Header.Get("Vary"); !strings.Contains(vary, "Accept-Encoding") {
		t.Fatalf("expected Vary to include Accept-Encoding, got %q", vary)
	}

	// Decompress zstd payload and verify it matches original byte-for-byte
	zr, err := zstd.NewReader(res.Body)
	if err != nil {
		t.Fatalf("failed to create zstd reader: %v", err)
	}
	defer zr.Close()

	decompressed, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("failed to decompress zstd stream: %v", err)
	}

	if string(decompressed) != sampleHTML {
		t.Fatalf("decompressed HTML does not match original HTML. Got:\n%s", string(decompressed))
	}
}

func TestCompress_GzipFallback(t *testing.T) {
	sampleHTML := `<!DOCTYPE html><html><head><script>console.log('google ads initialized');</script></head><body>Jobs Directory JobyUK</body></html>`

	handler := Compress()(sampleHandler(sampleHTML))

	// Client only supports gzip (older browser or AdSense crawler)
	req := httptest.NewRequest(http.MethodGet, "/jobs", nil)
	req.Header.Set("Accept-Encoding", "gzip, deflate")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if ce := res.Header.Get("Content-Encoding"); ce != "gzip" {
		t.Fatalf("expected Content-Encoding 'gzip', got %q", ce)
	}

	if eng := res.Header.Get("X-Compression-Engine"); !strings.Contains(eng, "gzip") {
		t.Fatalf("expected X-Compression-Engine to mention gzip, got %q", eng)
	}

	// Decompress gzip payload and verify
	gr, err := gzip.NewReader(res.Body)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer gr.Close()

	decompressed, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("failed to decompress gzip stream: %v", err)
	}

	if string(decompressed) != sampleHTML {
		t.Fatalf("decompressed HTML does not match original HTML. Got:\n%s", string(decompressed))
	}
}

func TestCompress_UncompressedFallback(t *testing.T) {
	sampleHTML := `Plain uncompressed response`

	handler := Compress()(sampleHandler(sampleHTML))

	// Client has NO Accept-Encoding
	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if ce := res.Header.Get("Content-Encoding"); ce != "" {
		t.Fatalf("expected empty Content-Encoding, got %q", ce)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	if string(body) != sampleHTML {
		t.Fatalf("expected plain body, got %q", string(body))
	}
}

func TestCompress_WebSocketBypass(t *testing.T) {
	sampleMsg := `WebSocket handshake response`

	handler := Compress()(sampleHandler(sampleMsg))

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Accept-Encoding", "zstd, gzip")
	req.Header.Set("Upgrade", "websocket")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if ce := res.Header.Get("Content-Encoding"); ce != "" {
		t.Fatalf("expected no compression for WebSocket Upgrade, got %q", ce)
	}
}

func TestCompress_SSEBypass(t *testing.T) {
	sampleMsg := "data: ping\n\n"

	handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(sampleMsg))
	}))

	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	req.Header.Set("Accept-Encoding", "zstd, gzip")
	req.Header.Set("Accept", "text/event-stream")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if ce := res.Header.Get("Content-Encoding"); ce != "" {
		t.Fatalf("expected no compression for SSE event stream, got %q", ce)
	}
}

func BenchmarkCompress_Zstd(b *testing.B) {
	sampleHTML := strings.Repeat(`<div>Locksmith in Slough - 24/7 Verified Emergency Trades - Call 0800 123 4567</div>`, 50)
	handler := Compress()(sampleHandler(sampleHTML))
	req := httptest.NewRequest(http.MethodGet, "/services", nil)
	req.Header.Set("Accept-Encoding", "zstd, gzip")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}

func BenchmarkCompress_Gzip(b *testing.B) {
	sampleHTML := strings.Repeat(`<div>Locksmith in Slough - 24/7 Verified Emergency Trades - Call 0800 123 4567</div>`, 50)
	handler := Compress()(sampleHandler(sampleHTML))
	req := httptest.NewRequest(http.MethodGet, "/services", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}
