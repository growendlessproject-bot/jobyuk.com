package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
)

// FeedJobrapido serves the official Jobrapido-specification XML feed.
func (h *Handlers) FeedJobrapido(w http.ResponseWriter, r *http.Request) {
	data, err := h.feed.GetJobrapidoFeed(r.Context())
	if err != nil {
		slog.Error("Failed to generate Jobrapido feed", "error", err)
		http.Error(w, "Failed to generate feed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=1800")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// FeedJooble serves the official Jooble-specification XML feed.
func (h *Handlers) FeedJooble(w http.ResponseWriter, r *http.Request) {
	data, err := h.feed.GetJoobleFeed(r.Context())
	if err != nil {
		slog.Error("Failed to generate Jooble feed", "error", err)
		http.Error(w, "Failed to generate feed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=1800")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// FeedAdzuna serves the Adzuna-compliant XML feed.
func (h *Handlers) FeedAdzuna(w http.ResponseWriter, r *http.Request) {
	data, err := h.feed.GetIndeedFeed(r.Context(), "adzuna")
	if err != nil {
		slog.Error("Failed to generate Adzuna feed", "error", err)
		http.Error(w, "Failed to generate feed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=1800")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// FeedIndeed serves the universal Indeed-standard XML feed.
func (h *Handlers) FeedIndeed(w http.ResponseWriter, r *http.Request) {
	data, err := h.feed.GetIndeedFeed(r.Context(), "indeed")
	if err != nil {
		slog.Error("Failed to generate Indeed feed", "error", err)
		http.Error(w, "Failed to generate feed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=1800")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// FeedWhatJobs serves the WhatJobs-compliant XML feed.
func (h *Handlers) FeedWhatJobs(w http.ResponseWriter, r *http.Request) {
	data, err := h.feed.GetIndeedFeed(r.Context(), "whatjobs")
	if err != nil {
		slog.Error("Failed to generate WhatJobs feed", "error", err)
		http.Error(w, "Failed to generate feed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=1800")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// FeedCareerjet serves the Careerjet-compliant XML feed.
func (h *Handlers) FeedCareerjet(w http.ResponseWriter, r *http.Request) {
	data, err := h.feed.GetIndeedFeed(r.Context(), "careerjet")
	if err != nil {
		slog.Error("Failed to generate Careerjet feed", "error", err)
		http.Error(w, "Failed to generate feed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=1800")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// FeedsDirectory renders a sleek directory of all available XML feeds with copyable endpoints.
func (h *Handlers) FeedsDirectory(w http.ResponseWriter, r *http.Request) {
	baseURL := "https://jobyuk.com"
	if h.cfg.BaseURL != "" {
		baseURL = h.cfg.BaseURL
	}

	htmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Job Aggregator XML Feeds | JobyUK.com</title>
    <meta name="robots" content="noindex, follow">
    <style>
        :root { --primary: #059669; --dark: #0F172A; --bg: #F8FAF8; --border: #E2E8F0; }
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: var(--bg); color: var(--dark); padding: 40px 20px; line-height: 1.6; }
        .wrap { max-width: 840px; margin: 0 auto; background: #fff; border-radius: 16px; border: 1px solid var(--border); box-shadow: 0 4px 20px rgba(0,0,0,0.04); padding: 36px; }
        h1 { font-size: 1.85rem; font-weight: 800; margin: 0 0 8px; color: #064E3B; letter-spacing: -0.02em; }
        p.sub { color: #64748B; margin: 0 0 28px; font-size: 0.98rem; }
        .feed-grid { display: flex; flex-direction: column; gap: 18px; }
        .card { border: 1px solid var(--border); border-radius: 12px; padding: 20px; background: #fff; transition: border-color 0.2s; }
        .card:hover { border-color: var(--primary); }
        .card-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; }
        .badge { font-size: 0.72rem; font-weight: 700; background: #ECFDF5; color: #065F46; padding: 4px 10px; border-radius: 999px; text-transform: uppercase; }
        .title { font-weight: 700; font-size: 1.15rem; }
        .desc { font-size: 0.88rem; color: #64748B; margin-bottom: 12px; }
        .input-wrap { display: flex; gap: 8px; }
        input { flex: 1; padding: 10px 14px; border: 1px solid var(--border); border-radius: 8px; font-family: monospace; font-size: 0.9rem; background: #F8FAFC; color: #334155; }
        .btn { padding: 10px 16px; border-radius: 8px; font-weight: 600; font-size: 0.88rem; cursor: pointer; text-decoration: none; border: none; display: inline-flex; align-items: center; gap: 6px; }
        .btn-primary { background: #059669; color: #fff; }
        .btn-outline { background: transparent; border: 1px solid var(--border); color: #334155; }
        .btn-outline:hover { background: #F1F5F9; }
        .toast { position: fixed; bottom: 20px; right: 20px; background: #0F172A; color: #fff; padding: 12px 20px; border-radius: 8px; font-size: 0.9rem; opacity: 0; transition: opacity 0.3s; pointer-events: none; }
        .toast.show { opacity: 1; }
    </style>
</head>
<body>
    <div class="wrap">
        <h1>Job Aggregator XML Feeds</h1>
        <p class="sub">Standardized XML syndication feeds for automated job distribution across UK search engines and partner aggregators.</p>
        
        <div class="feed-grid">
            <!-- 1. Jobrapido -->
            <div class="card">
                <div class="card-head">
                    <span class="title">Jobrapido UK Feed</span>
                    <span class="badge">Jobrapido 2.0 Format</span>
                </div>
                <div class="desc">Official XML feed for uk.jobrapido.com publisher network with automated daily ingestion.</div>
                <div class="input-wrap">
                    <input type="text" readonly value="%s/feeds/jobrapido.xml" id="f-jobrapido">
                    <button class="btn btn-outline" onclick="copyFeed('f-jobrapido')">Copy URL</button>
                    <a class="btn btn-primary" href="/feeds/jobrapido.xml" target="_blank">View XML</a>
                </div>
            </div>

            <!-- 2. Jooble -->
            <div class="card">
                <div class="card-head">
                    <span class="title">Jooble UK Feed</span>
                    <span class="badge">Jooble XML Format</span>
                </div>
                <div class="desc">Standardized feed for Jooble UK partner program (uk.jooble.org).</div>
                <div class="input-wrap">
                    <input type="text" readonly value="%s/feeds/jooble.xml" id="f-jooble">
                    <button class="btn btn-outline" onclick="copyFeed('f-jooble')">Copy URL</button>
                    <a class="btn btn-primary" href="/feeds/jooble.xml" target="_blank">View XML</a>
                </div>
            </div>

            <!-- 3. Adzuna -->
            <div class="card">
                <div class="card-head">
                    <span class="title">Adzuna UK Feed</span>
                    <span class="badge">Universal Format</span>
                </div>
                <div class="desc">High-volume feed formatted for Adzuna UK (adzuna.co.uk) automated aggregator partner feed.</div>
                <div class="input-wrap">
                    <input type="text" readonly value="%s/feeds/adzuna.xml" id="f-adzuna">
                    <button class="btn btn-outline" onclick="copyFeed('f-adzuna')">Copy URL</button>
                    <a class="btn btn-primary" href="/feeds/adzuna.xml" target="_blank">View XML</a>
                </div>
            </div>

            <!-- 4. Universal / Indeed -->
            <div class="card">
                <div class="card-head">
                    <span class="title">Universal Aggregator Feed (Indeed / WhatJobs / Careerjet)</span>
                    <span class="badge">Universal XML</span>
                </div>
                <div class="desc">Industry-standard format accepted by WhatJobs, Careerjet, Trovit, Talent.com, and Indeed.</div>
                <div class="input-wrap">
                    <input type="text" readonly value="%s/feeds/indeed.xml" id="f-indeed">
                    <button class="btn btn-outline" onclick="copyFeed('f-indeed')">Copy URL</button>
                    <a class="btn btn-primary" href="/feeds/indeed.xml" target="_blank">View XML</a>
                </div>
            </div>
        </div>
    </div>

    <div id="toast" class="toast">Feed URL copied to clipboard!</div>

    <script>
        function copyFeed(id) {
            var input = document.getElementById(id);
            input.select();
            input.setSelectionRange(0, 99999);
            navigator.clipboard.writeText(input.value);
            var toast = document.getElementById('toast');
            toast.classList.add('show');
            setTimeout(function() { toast.classList.remove('show'); }, 2200);
        }
    </script>
</body>
</html>`, baseURL, baseURL, baseURL, baseURL)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(htmlContent))
}
