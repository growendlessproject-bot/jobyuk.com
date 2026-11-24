package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"

	"jobyuk/internal/config"
	"jobyuk/internal/middleware"
	"jobyuk/internal/models"
	"jobyuk/internal/repository"
	"jobyuk/internal/services"
	"jobyuk/internal/services/linkbot"
	"jobyuk/internal/services/reed"
	"jobyuk/internal/settings"
)

// AdminHandlers holds admin HTTP handler dependencies.
type AdminHandlers struct {
	renderer *Renderer
	repos    *repository.Repositories
	sync     *reed.SyncService
	sitemap  *services.SitemapService
	cache    *services.CacheService
	taxonomy *services.TaxonomyService
	indexing *services.GoogleIndexingService
	cfg      *config.Config
	linkBot  *linkbot.Bot
	autoIndexer *services.AutoIndexerService
	indexNow    *services.IndexNowService
}

// NewAdminHandlers creates admin handlers.
func NewAdminHandlers(
	renderer *Renderer,
	repos *repository.Repositories,
	sync *reed.SyncService,
	sitemap *services.SitemapService,
	cache *services.CacheService,
	taxonomy *services.TaxonomyService,
	indexing *services.GoogleIndexingService,
	cfg *config.Config,
) *AdminHandlers {
	return &AdminHandlers{
		renderer: renderer,
		repos:    repos,
		sync:     sync,
		sitemap:  sitemap,
		cache:    cache,
		taxonomy: taxonomy,
		indexing: indexing,
		cfg:      cfg,
	}
}

// LoginPage renders the admin login form.
func (h *AdminHandlers) LoginPage(w http.ResponseWriter, r *http.Request) {
	errText := r.URL.Query().Get("error")
	errorBanner := ""
	if errText != "" {
		errorBanner = `<div class="login-alert">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" class="login-alert__icon"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="8" x2="12" y2="12"></line><line x1="12" y1="16" x2="12.01" y2="16"></line></svg>
            <span>` + html.EscapeString(errText) + `</span>
        </div>`
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
    <title>Admin Security Gateway | JobyUK.com</title>
    <meta name="robots" content="noindex,nofollow">
    <link rel="icon" type="image/svg+xml" href="/static/images/logo.svg">
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Plus+Jakarta+Sans:wght@400;500;600;700;800&display=swap" rel="stylesheet">
    <style>
        *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: 'Plus Jakarta Sans', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: #090d16;
            color: #f1f5f9;
            min-height: 100vh;
            min-height: 100dvh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 24px 16px;
            position: relative;
            overflow-x: hidden;
        }

        /* Ambient Glow Blobs */
        .ambient-bg {
            position: fixed;
            inset: 0;
            pointer-events: none;
            z-index: 0;
            overflow: hidden;
        }
        .blob-1 {
            position: absolute;
            top: -10%;
            left: 50%;
            transform: translateX(-50%);
            width: 500px;
            height: 350px;
            background: radial-gradient(circle, rgba(10, 102, 194, 0.28) 0%, rgba(10, 102, 194, 0) 70%);
            filter: blur(50px);
        }
        .blob-2 {
            position: absolute;
            bottom: -15%;
            right: 15%;
            width: 400px;
            height: 300px;
            background: radial-gradient(circle, rgba(0, 212, 170, 0.16) 0%, rgba(0, 212, 170, 0) 70%);
            filter: blur(60px);
        }

        .login-wrapper {
            position: relative;
            z-index: 1;
            width: 100%;
            max-width: 420px;
            margin: 0 auto;
        }

        .login-card {
            background: rgba(15, 23, 42, 0.75);
            backdrop-filter: blur(20px) saturate(180%);
            -webkit-backdrop-filter: blur(20px) saturate(180%);
            border: 1px solid rgba(255, 255, 255, 0.1);
            border-radius: 24px;
            padding: 36px 32px 32px;
            box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5), 0 0 0 1px rgba(255, 255, 255, 0.05);
        }

        .login-brand {
            text-align: center;
            margin-bottom: 28px;
        }

        .login-brand__logo {
            display: inline-flex;
            align-items: center;
            justify-content: center;
            width: 56px;
            height: 56px;
            border-radius: 18px;
            background: linear-gradient(135deg, rgba(10, 102, 194, 0.25) 0%, rgba(0, 212, 170, 0.2) 100%);
            border: 1px solid rgba(10, 102, 194, 0.4);
            color: #38bdf8;
            margin-bottom: 16px;
            box-shadow: 0 8px 24px -4px rgba(10, 102, 194, 0.4);
        }

        .login-brand__title {
            font-size: 1.45rem;
            font-weight: 800;
            color: #ffffff;
            letter-spacing: -0.02em;
            margin-bottom: 6px;
        }

        .login-brand__subtitle {
            font-size: 0.85rem;
            color: #94a3b8;
            line-height: 1.4;
        }

        .login-alert {
            display: flex;
            align-items: center;
            gap: 10px;
            background: rgba(239, 68, 68, 0.12);
            border: 1px solid rgba(239, 68, 68, 0.35);
            color: #fca5a5;
            padding: 12px 14px;
            border-radius: 12px;
            font-size: 0.84rem;
            font-weight: 600;
            margin-bottom: 20px;
            line-height: 1.4;
        }
        .login-alert__icon {
            color: #ef4444;
            flex-shrink: 0;
        }

        .login-form {
            display: flex;
            flex-direction: column;
            gap: 18px;
        }

        .form-group {
            display: flex;
            flex-direction: column;
            gap: 6px;
        }

        .form-label {
            font-size: 0.8rem;
            font-weight: 700;
            color: #cbd5e1;
            letter-spacing: 0.02em;
        }

        .input-wrap {
            position: relative;
            display: flex;
            align-items: center;
        }

        .input-icon {
            position: absolute;
            left: 14px;
            color: #64748b;
            pointer-events: none;
            transition: color 0.2s;
        }

        .login-input {
            width: 100%;
            height: 48px;
            padding: 0 44px 0 42px;
            background: rgba(30, 41, 59, 0.6);
            border: 1.5px solid rgba(255, 255, 255, 0.1);
            border-radius: 12px;
            color: #ffffff;
            font-size: 0.92rem;
            font-family: inherit;
            outline: none;
            transition: all 0.2s ease;
        }

        .login-input::placeholder {
            color: #64748b;
        }

        .login-input:focus {
            background: rgba(30, 41, 59, 0.95);
            border-color: #0ea5e9;
            box-shadow: 0 0 0 3px rgba(14, 165, 233, 0.2);
        }

        .login-input:focus + .input-icon,
        .input-wrap:focus-within .input-icon {
            color: #38bdf8;
        }

        .btn-toggle-pwd {
            position: absolute;
            right: 12px;
            background: transparent;
            border: none;
            color: #64748b;
            cursor: pointer;
            padding: 6px;
            display: flex;
            align-items: center;
            justify-content: center;
            border-radius: 6px;
            transition: color 0.2s;
        }
        .btn-toggle-pwd:hover {
            color: #cbd5e1;
        }

        .login-btn {
            display: inline-flex;
            align-items: center;
            justify-content: center;
            gap: 8px;
            width: 100%;
            height: 48px;
            margin-top: 6px;
            background: linear-gradient(135deg, #0A66C2 0%, #0284c7 100%);
            color: #ffffff;
            border: none;
            border-radius: 12px;
            font-size: 0.95rem;
            font-weight: 700;
            font-family: inherit;
            cursor: pointer;
            box-shadow: 0 4px 16px rgba(10, 102, 194, 0.35);
            transition: all 0.2s ease;
        }

        .login-btn:hover {
            background: linear-gradient(135deg, #084e96 0%, #0369a1 100%);
            transform: translateY(-1px);
            box-shadow: 0 6px 20px rgba(10, 102, 194, 0.45);
        }

        .login-btn:active {
            transform: translateY(0);
        }

        .login-security-footer {
            margin-top: 24px;
            text-align: center;
        }

        .security-badge {
            display: inline-flex;
            align-items: center;
            gap: 6px;
            font-size: 0.74rem;
            font-weight: 600;
            color: #64748b;
            background: rgba(255, 255, 255, 0.04);
            border: 1px solid rgba(255, 255, 255, 0.06);
            padding: 5px 12px;
            border-radius: 99px;
            margin-bottom: 14px;
        }

        .security-dot {
            width: 6px;
            height: 6px;
            border-radius: 50%;
            background: #10b981;
            box-shadow: 0 0 6px #10b981;
        }

        .back-link {
            display: inline-flex;
            align-items: center;
            gap: 5px;
            color: #94a3b8;
            font-size: 0.8rem;
            font-weight: 600;
            text-decoration: none;
            transition: color 0.15s ease;
        }
        .back-link:hover {
            color: #38bdf8;
        }

        @media (max-width: 480px) {
            .login-card {
                padding: 28px 20px 24px;
                border-radius: 20px;
            }
            .login-brand__title {
                font-size: 1.3rem;
            }
        }
    </style>
</head>
<body>
    <div class="ambient-bg">
        <div class="blob-1"></div>
        <div class="blob-2"></div>
    </div>

    <div class="login-wrapper">
        <div class="login-card">
            <!-- Brand Header -->
            <div class="login-brand">
                <div class="login-brand__logo">
                    <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                        <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"></path>
                    </svg>
                </div>
                <h1 class="login-brand__title">Admin Security Gateway</h1>
                <p class="login-brand__subtitle">JobyUK.com Central Administration & Control Portal</p>
            </div>

            ` + errorBanner + `

            <form method="POST" action="/admin-lol/login" class="login-form">
                <div class="form-group">
                    <label for="username" class="form-label">Username</label>
                    <div class="input-wrap">
                        <svg class="input-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
                            <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"></path>
                            <circle cx="12" cy="7" r="4"></circle>
                        </svg>
                        <input type="text" id="username" name="username" placeholder="Enter admin username" class="login-input" required autofocus autocomplete="username">
                    </div>
                </div>

                <div class="form-group">
                    <label for="password" class="form-label">Password</label>
                    <div class="input-wrap">
                        <svg class="input-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
                            <rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect>
                            <path d="M7 11V7a5 5 0 0 1 10 0v4"></path>
                        </svg>
                        <input type="password" id="password" name="password" placeholder="Enter security key / password" class="login-input" required autocomplete="current-password">
                        <button type="button" class="btn-toggle-pwd" id="btn-toggle-pwd" aria-label="Show password">
                            <svg id="eye-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                                <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path>
                                <circle cx="12" cy="12" r="3"></circle>
                            </svg>
                        </button>
                    </div>
                </div>

                <button type="submit" class="login-btn">
                    <span>Authorize & Sign In</span>
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                        <line x1="5" y1="12" x2="19" y2="12"></line>
                        <polyline points="12 5 19 12 12 19"></polyline>
                    </svg>
                </button>
            </form>

            <div class="login-security-footer">
                <div class="security-badge">
                    <span class="security-dot"></span>
                    <span>256-Bit TLS Encrypted • Zero-Trust Access</span>
                </div>
                <div>
                    <a href="/" class="back-link">
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                            <line x1="19" y1="12" x2="5" y2="12"></line>
                            <polyline points="12 19 5 12 12 5"></polyline>
                        </svg>
                        <span>Return to Main Site</span>
                    </a>
                </div>
            </div>
        </div>
    </div>

    <script>
        const btnToggle = document.getElementById('btn-toggle-pwd');
        const pwdInput = document.getElementById('password');
        const eyeIcon = document.getElementById('eye-icon');
        if (btnToggle && pwdInput) {
            btnToggle.addEventListener('click', () => {
                const isPwd = pwdInput.type === 'password';
                pwdInput.type = isPwd ? 'text' : 'password';
                eyeIcon.innerHTML = isPwd 
                    ? '<path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"></path><line x1="1" y1="1" x2="23" y2="23"></line>'
                    : '<path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path><circle cx="12" cy="12" r="3"></circle>';
            });
        }
    </script>
</body>
</html>`))
}

// LoginSubmit handles admin login form submission.
func (h *AdminHandlers) LoginSubmit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username := r.FormValue("username")
	password := r.FormValue("password")

	user, err := h.repos.Admin.FindByUsername(ctx, username)
	if err != nil {
		http.Redirect(w, r, "/admin-lol/login?error=Invalid+username+or+password", http.StatusSeeOther)
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		http.Redirect(w, r, "/admin-lol/login?error=Invalid+username+or+password", http.StatusSeeOther)
		return
	}

	// Generate and set admin cookie
	token, err := middleware.GenerateAdminToken(user.Username, user.Role, h.cfg.JWTSecret)
	if err != nil {
		http.Redirect(w, r, "/admin-lol/login?error=Failed+to+generate+token", http.StatusSeeOther)
		return
	}

	middleware.SetAdminCookie(w, token, h.cfg.IsDev())

	// Update last login
	_ = h.repos.Admin.UpdateLastLogin(ctx, user.ID)

	http.Redirect(w, r, "/admin-lol/", http.StatusSeeOther)
}

// Logout clears the admin authentication JWT cookie and redirects to login page.
func (h *AdminHandlers) Logout(w http.ResponseWriter, r *http.Request) {
	middleware.ClearAdminCookie(w)
	http.Redirect(w, r, "/admin-lol/login", http.StatusSeeOther)
}


// Heartbeat responds to active client watchdog pings, sliding the 15-minute token.
func (h *AdminHandlers) Heartbeat(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","authenticated":true,"timestamp":%d}`, time.Now().Unix())
}

// Dashboard renders the admin dashboard.
func (h *AdminHandlers) Dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Query stats dynamically
	var activeCount, expiredCount, rolesCount, redirectsCount int
	if err := h.repos.DB().QueryRow(ctx, "SELECT COUNT(*) FROM jobs WHERE status = 'active'").Scan(&activeCount); err != nil {
		slog.Error("Failed to query active jobs count", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if err := h.repos.DB().QueryRow(ctx, "SELECT COUNT(*) FROM jobs WHERE status = 'expired'").Scan(&expiredCount); err != nil {
		slog.Error("Failed to query expired jobs count", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if err := h.repos.DB().QueryRow(ctx, "SELECT COUNT(*) FROM job_roles").Scan(&rolesCount); err != nil {
		slog.Error("Failed to query job roles count", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if err := h.repos.DB().QueryRow(ctx, "SELECT COUNT(*) FROM redirects").Scan(&redirectsCount); err != nil {
		slog.Error("Failed to query redirects count", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := PageData{
		Title: "Dashboard Overview",
		Data: map[string]interface{}{
			"Stats": map[string]interface{}{
				"ActiveJobs":     activeCount,
				"ExpiredJobs":    expiredCount,
				"TotalRoles":     rolesCount,
				"TotalRedirects": redirectsCount,
			},
		},
	}

	if err := h.renderer.RenderAdmin(w, r, "admin/dashboard", data); err != nil {
		slog.Error("Failed to render admin dashboard", "error", err)
	}
}

// JobsManage renders the admin jobs management page.
func (h *AdminHandlers) JobsManage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query().Get("q")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	jobs, total, err := h.repos.Job.AdminSearch(ctx, query, 30, (page-1)*30)
	if err != nil {
		slog.Error("Failed to fetch admin jobs", "error", err)
	}

	data := PageData{
		Title: "Manage Jobs",
		Data: map[string]interface{}{
			"Jobs":  jobs,
			"Total": total,
			"Page":  page,
			"Query": query,
		},
	}

	_ = h.renderer.RenderAdmin(w, r, "admin/jobs", data)
}

// JobExpire manually expires a job listing.
func (h *AdminHandlers) JobExpire(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "Invalid Job ID", http.StatusBadRequest)
		return
	}

	job, err := h.repos.Job.FindByID(r.Context(), id)
	if err == nil {
		if err := h.repos.Job.Expire(r.Context(), id); err != nil {
			slog.Error("Failed to expire job", "id", id, "error", err)
		} else {
			jobURL := fmt.Sprintf("%s/jobs/%s/", h.cfg.BaseURL, job.Slug)
			go func() {
				_ = h.indexing.PublishDelete(context.Background(), jobURL)
			}()
		}
	} else {
		slog.Error("Failed to find job before expiring", "id", id, "error", err)
	}

	http.Redirect(w, r, "/admin-lol/jobs", http.StatusSeeOther)
}

// JobReactivate manually reactivates an expired job listing.
func (h *AdminHandlers) JobReactivate(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "Invalid Job ID", http.StatusBadRequest)
		return
	}

	if err := h.repos.Job.Reactivate(r.Context(), id); err != nil {
		slog.Error("Failed to reactivate job", "id", id, "error", err)
	} else {
		job, err := h.repos.Job.FindByID(r.Context(), id)
		if err == nil {
			jobURL := fmt.Sprintf("%s/jobs/%s/", h.cfg.BaseURL, job.Slug)
			go func() {
				_ = h.indexing.PublishUpdate(context.Background(), jobURL)
			}()
		}
	}

	http.Redirect(w, r, "/admin-lol/jobs", http.StatusSeeOther)
}

// JobDelete manually soft-deletes a job listing.
func (h *AdminHandlers) JobDelete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "Invalid Job ID", http.StatusBadRequest)
		return
	}

	if err := h.repos.Job.SoftDelete(r.Context(), id); err != nil {
		slog.Error("Failed to soft-delete job", "id", id, "error", err)
	} else {
		job, err := h.repos.Job.FindByID(r.Context(), id)
		if err == nil {
			jobURL := fmt.Sprintf("%s/jobs/%s/", h.cfg.BaseURL, job.Slug)
			go func() {
				_ = h.indexing.PublishDelete(context.Background(), jobURL)
			}()
		}
	}
	_ = h.cache.ClearSearchCache(r.Context())
	http.Redirect(w, r, "/admin-lol/jobs", http.StatusSeeOther)
}

// TaxonomyManage renders the administrative taxonomy page.
func (h *AdminHandlers) TaxonomyManage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	roles, err := h.repos.Role.FindAll(ctx)
	if err != nil {
		slog.Error("Failed to load taxonomy roles", "error", err)
	}

	data := PageData{
		Title: "Taxonomy Settings",
		Data: map[string]interface{}{
			"Roles": roles,
		},
	}

	_ = h.renderer.RenderAdmin(w, r, "admin/taxonomy", data)
}

// RoleCreate inserts a new job role.
func (h *AdminHandlers) RoleCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := r.FormValue("name")
	slug := r.FormValue("slug")
	if slug == "" {
		slug = strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	}

	_, err := h.repos.DB().Exec(ctx, `
		INSERT INTO job_roles (slug, name, job_count)
		VALUES ($1, $2, 0)
	`, slug, name)
	if err != nil {
		slog.Error("Failed to create job role", "error", err)
	}

	http.Redirect(w, r, "/admin-lol/taxonomy", http.StatusSeeOther)
}

// RoleUpdate edits an existing job role.
func (h *AdminHandlers) RoleUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		http.Error(w, "Invalid Role ID", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	slug := r.FormValue("slug")
	if slug == "" {
		slug = strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	}

	_, err = h.repos.DB().Exec(ctx, `
		UPDATE job_roles
		SET name = $1, slug = $2
		WHERE id = $3
	`, name, slug, id)
	if err != nil {
		slog.Error("Failed to update job role", "id", id, "error", err)
	}

	http.Redirect(w, r, "/admin-lol/taxonomy", http.StatusSeeOther)
}

// ContentManage renders the CMS page listing all articles.
func (h *AdminHandlers) ContentManage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	articles, err := h.repos.Article.FindAll(ctx)
	if err != nil {
		slog.Error("Failed to load articles for content management", "error", err)
	}

	data := PageData{
		Title: "Article CMS",
		Data: map[string]interface{}{
			"Articles": articles,
		},
	}

	_ = h.renderer.RenderAdmin(w, r, "admin/content", data)
}

// ArticleNew renders the write new article form.
func (h *AdminHandlers) ArticleNew(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title: "Create Article",
		Data: map[string]interface{}{
			"Article": &models.Article{},
		},
	}
	_ = h.renderer.RenderAdmin(w, r, "admin/article_edit", data)
}

// ArticleCreate handles insertion of a new article.
func (h *AdminHandlers) ArticleCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	title := r.FormValue("title")
	category := r.FormValue("category")
	content := r.FormValue("content")
	slug := r.FormValue("slug")
	if slug == "" {
		slug = strings.ToLower(strings.ReplaceAll(title, " ", "-"))
	}
	seoTitle := r.FormValue("seo_title")
	seoDesc := r.FormValue("seo_description")
	published := r.FormValue("published") == "true"

	article := &models.Article{
		Title:          title,
		Category:       category,
		Content:        content,
		Slug:           slug,
		SEOTitle:       seoTitle,
		SEODescription: seoDesc,
		Published:      published,
	}

	if err := h.repos.Article.Create(ctx, article); err != nil {
		slog.Error("Failed to create article", "error", err)
	}

	http.Redirect(w, r, "/admin-lol/content", http.StatusSeeOther)
}

// ArticleEdit renders the article editing form.
func (h *AdminHandlers) ArticleEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		http.Error(w, "Invalid Article ID", http.StatusBadRequest)
		return
	}

	article, err := h.repos.Article.FindByID(ctx, id)
	if err != nil {
		h.Dashboard(w, r)
		return
	}

	data := PageData{
		Title: "Edit Article",
		Data: map[string]interface{}{
			"Article": article,
		},
	}
	_ = h.renderer.RenderAdmin(w, r, "admin/article_edit", data)
}

// ArticleUpdate updates an existing article.
func (h *AdminHandlers) ArticleUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		http.Error(w, "Invalid Article ID", http.StatusBadRequest)
		return
	}

	title := r.FormValue("title")
	category := r.FormValue("category")
	content := r.FormValue("content")
	slug := r.FormValue("slug")
	if slug == "" {
		slug = strings.ToLower(strings.ReplaceAll(title, " ", "-"))
	}
	seoTitle := r.FormValue("seo_title")
	seoDesc := r.FormValue("seo_description")
	published := r.FormValue("published") == "true"

	article, err := h.repos.Article.FindByID(ctx, id)
	if err == nil {
		article.Title = title
		article.Category = category
		article.Content = content
		article.Slug = slug
		article.SEOTitle = seoTitle
		article.SEODescription = seoDesc
		article.Published = published

		if err := h.repos.Article.Update(ctx, article); err != nil {
			slog.Error("Failed to update article", "id", id, "error", err)
		}
	}

	http.Redirect(w, r, "/admin-lol/content", http.StatusSeeOther)
}

// SEOManage renders redirects page.
func (h *AdminHandlers) SEOManage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	redirects, err := h.repos.Redirect.FindAll(ctx)
	if err != nil {
		slog.Error("Failed to load redirects", "error", err)
	}

	data := PageData{
		Title: "SEO Redirects",
		Data: map[string]interface{}{
			"Redirects": redirects,
		},
	}

	_ = h.renderer.RenderAdmin(w, r, "admin/seo", data)
}

// RedirectCreate inserts a new redirect rule.
func (h *AdminHandlers) RedirectCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	fromPath := r.FormValue("from_path")
	toPath := r.FormValue("to_path")
	statusCode, err := strconv.Atoi(r.FormValue("status_code"))
	if err != nil || (statusCode != 301 && statusCode != 302 && statusCode != 410) {
		statusCode = 301 // Default to 301 Moved Permanently
	}

	rd := &models.Redirect{
		FromPath:   fromPath,
		ToPath:     toPath,
		StatusCode: statusCode,
	}

	if err := h.repos.Redirect.Create(ctx, rd); err != nil {
		slog.Error("Failed to create redirect", "error", err)
	}

	http.Redirect(w, r, "/admin-lol/seo", http.StatusSeeOther)
}

// RedirectDelete deletes an existing redirect rule.
func (h *AdminHandlers) RedirectDelete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		http.Error(w, "Invalid Redirect ID", http.StatusBadRequest)
		return
	}

	if err := h.repos.Redirect.Delete(r.Context(), id); err != nil {
		slog.Error("Failed to delete redirect", "id", id, "error", err)
	}

	http.Redirect(w, r, "/admin-lol/seo", http.StatusSeeOther)
}

// AnalyticsPage renders site analytics metrics.
func (h *AdminHandlers) AnalyticsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Query page views count
	var totalViews, todayViews int
	if err := h.repos.DB().QueryRow(ctx, "SELECT COUNT(*) FROM page_views").Scan(&totalViews); err != nil {
		slog.Error("Failed to query total page views count", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if err := h.repos.DB().QueryRow(ctx, "SELECT COUNT(*) FROM page_views WHERE created_at >= NOW() - INTERVAL '1 day'").Scan(&todayViews); err != nil {
		slog.Error("Failed to query today's page views count", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Fetch top viewed paths
	rows, err := h.repos.DB().Query(ctx, `
		SELECT path, COUNT(*) as count 
		FROM page_views 
		GROUP BY path 
		ORDER BY count DESC 
		LIMIT 10
	`)
	
	type ViewStat struct {
		Path  string
		Count int
	}
	var stats []ViewStat
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var s ViewStat
			if err := rows.Scan(&s.Path, &s.Count); err == nil {
				stats = append(stats, s)
			}
		}
	}

	data := PageData{
		Title: "Site Analytics",
		Data: map[string]interface{}{
			"TotalViews": totalViews,
			"TodayViews": todayViews,
			"Stats":      stats,
		},
	}

	_ = h.renderer.RenderAdmin(w, r, "admin/dashboard", data)
}

// SettingsPage renders settings panel.
func (h *AdminHandlers) SettingsPage(w http.ResponseWriter, r *http.Request) {
	synced := r.URL.Query().Get("synced") == "true"
	cacheCleared := r.URL.Query().Get("cache_cleared") == "true"
	sitemapRebuilt := r.URL.Query().Get("sitemap_rebuilt") == "true"
	saved := r.URL.Query().Get("saved") == "true"
	indexingSaved := r.URL.Query().Get("indexing_saved") == "true"
	indexingTestSuccess := r.URL.Query().Get("indexing_test") == "true"
	autoIndexerStarted := r.URL.Query().Get("auto_indexer_started") == "true"
	indexNowSubmitted := r.URL.Query().Get("indexnow_submitted") == "true"
	indexingError := r.URL.Query().Get("indexing_error")

	adsSaved := r.URL.Query().Get("ads_saved") == "true"
	adsDeleted := r.URL.Query().Get("ads_deleted") == "true"
	adblockSaved := r.URL.Query().Get("adblock_saved") == "true"
	adblockStatus := r.URL.Query().Get("adblock_status")
	waSaved := r.URL.Query().Get("wa_saved") == "true"
	waStatus := r.URL.Query().Get("wa_status")

	indexingEnabled := false
	indexingEmail := ""
	if h.indexing != nil {
		indexingEnabled = h.indexing.IsEnabled()
		indexingEmail = h.indexing.GetClientEmail()
	}

	data := PageData{
		Title: "Settings & Dev Operations",
		Data: map[string]interface{}{
			"Synced":              synced,
			"CacheCleared":        cacheCleared,
			"SitemapRebuilt":      sitemapRebuilt,
			"Saved":               saved,
			"AdsSaved":            adsSaved,
			"AdsDeleted":          adsDeleted,
			"AdblockSaved":        adblockSaved,
			"AdblockStatus":       adblockStatus,
			"WASaved":             waSaved,
			"WAStatus":            waStatus,
			"IndexingSaved":       indexingSaved,
			"IndexingTestSuccess": indexingTestSuccess,
			"AutoIndexerStarted":  autoIndexerStarted,
			"IndexNowSubmitted":   indexNowSubmitted,
			"IndexNowKey":         h.cfg.IndexNowKey,
			"IndexingError":       indexingError,
			"IndexingEnabled":     indexingEnabled,
			"IndexingEmail":       indexingEmail,
			"Settings":            settings.GetManager().GetAll(),
		},
	}

	_ = h.renderer.RenderAdmin(w, r, "admin/settings", data)
}

func (h *AdminHandlers) UpdateGoogleIndexing(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10 MB max
		_ = r.ParseForm()
	}

	var rawJSON []byte

	// 1. Check if user uploaded a .json file
	file, _, err := r.FormFile("credentials_file")
	if err == nil && file != nil {
		defer file.Close()
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(file)
		rawJSON = buf.Bytes()
	}

	// 2. If no file uploaded, check textarea input
	if len(rawJSON) == 0 {
		text := strings.TrimSpace(r.FormValue("credentials_json"))
		if text != "" {
			rawJSON = []byte(text)
		}
	}

	if len(rawJSON) == 0 {
		http.Redirect(w, r, "/admin-lol/settings?indexing_error=Please+paste+or+upload+a+valid+Google+Service+Account+JSON", http.StatusSeeOther)
		return
	}

	if h.indexing == nil {
		http.Redirect(w, r, "/admin-lol/settings?indexing_error=Indexing+service+not+initialized", http.StatusSeeOther)
		return
	}

	if err := h.indexing.SaveAndReload(rawJSON); err != nil {
		http.Redirect(w, r, "/admin-lol/settings?indexing_error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin-lol/settings?indexing_saved=true", http.StatusSeeOther)
}

func (h *AdminHandlers) TestGoogleIndexing(w http.ResponseWriter, r *http.Request) {
	if h.indexing == nil || !h.indexing.IsEnabled() {
		http.Redirect(w, r, "/admin-lol/settings?indexing_error=Google+Indexing+API+is+not+configured+yet", http.StatusSeeOther)
		return
	}

	testURL := "https://jobyuk.com/"
	if err := h.indexing.PublishUpdate(r.Context(), testURL); err != nil {
		http.Redirect(w, r, "/admin-lol/settings?indexing_error="+url.QueryEscape("Google API call failed: "+err.Error()), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin-lol/settings?indexing_test=true", http.StatusSeeOther)
}

func (h *AdminHandlers) SetAutoIndexer(ai *services.AutoIndexerService) {
	h.autoIndexer = ai
}

func (h *AdminHandlers) TriggerAutoIndex(w http.ResponseWriter, r *http.Request) {
	if h.autoIndexer == nil || h.indexing == nil || !h.indexing.IsEnabled() {
		http.Redirect(w, r, "/admin-lol/settings?indexing_error=Google+Indexing+API+is+not+configured+yet", http.StatusSeeOther)
		return
	}

	go func() {
		_, _ = h.autoIndexer.RunBatch(context.Background(), 150)
	}()

	http.Redirect(w, r, "/admin-lol/settings?auto_indexer_started=true", http.StatusSeeOther)
}

func (h *AdminHandlers) SetIndexNow(in *services.IndexNowService) {
	h.indexNow = in
}

func (h *AdminHandlers) TriggerIndexNow(w http.ResponseWriter, r *http.Request) {
	if h.indexNow == nil {
		http.Redirect(w, r, "/admin-lol/settings?indexing_error=IndexNow+service+not+initialized", http.StatusSeeOther)
		return
	}

	go func() {
		_, _ = h.indexNow.SubmitTopJobs(context.Background(), h.repos.Job, 1000)
	}()

	http.Redirect(w, r, "/admin-lol/settings?indexnow_submitted=true", http.StatusSeeOther)
}

func (h *AdminHandlers) UpdateSiteSettings(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin-lol/settings?error=invalid_form", http.StatusSeeOther)
		return
	}

	current := settings.GetManager().GetAll()
	current.ContactEmail = strings.TrimSpace(r.FormValue("contact_email"))
	current.SocialTwitter = strings.TrimSpace(r.FormValue("social_twitter"))
	current.SocialLinkedIn = strings.TrimSpace(r.FormValue("social_linkedin"))
	current.SocialFacebook = strings.TrimSpace(r.FormValue("social_facebook"))
	current.SocialInstagram = strings.TrimSpace(r.FormValue("social_instagram"))
	current.SocialPinterest = strings.TrimSpace(r.FormValue("social_pinterest"))
	current.GoogleAnalyticsID = strings.TrimSpace(r.FormValue("google_analytics_id"))
	current.AdSensePublisherID = strings.TrimSpace(r.FormValue("adsense_publisher_id"))
	if adsVal, ok := r.Form["ads_txt"]; ok && len(adsVal) > 0 {
		txt := strings.ReplaceAll(adsVal[0], "\r\n", "\n")
		txt = strings.TrimSpace(txt)
		if txt != "" {
			txt += "\n"
		}
		current.AdsTxt = txt
	}
	current.DefaultMetaTitle = strings.TrimSpace(r.FormValue("default_meta_title"))
	current.DefaultMetaDesc = strings.TrimSpace(r.FormValue("default_meta_desc"))
	current.DefaultMetaKeywords = strings.TrimSpace(r.FormValue("default_meta_keywords"))
	current.DefaultOGImage = strings.TrimSpace(r.FormValue("default_og_image"))
	current.InsightsTitle = strings.TrimSpace(r.FormValue("insights_title"))
	current.InsightsSubtitle = strings.TrimSpace(r.FormValue("insights_subtitle"))
	current.HiringTitle = strings.TrimSpace(r.FormValue("hiring_title"))
	current.HiringCtaText = strings.TrimSpace(r.FormValue("hiring_cta_text"))
	current.HiringCtaLink = strings.TrimSpace(r.FormValue("hiring_cta_link"))
	current.ApplyCtaText = strings.TrimSpace(r.FormValue("apply_cta_text"))
	if _, ok := r.Form["anti_adblock_present"]; ok {
		val := r.FormValue("anti_adblock_enabled")
		current.AntiAdblockEnabled = (val == "true" || val == "on" || val == "1")
	}
	if _, ok := r.Form["whatsapp_alerts_present"]; ok {
		val := r.FormValue("whatsapp_alerts_enabled")
		current.WhatsAppAlertsEnabled = (val == "true" || val == "on" || val == "1")
	}

	_ = settings.GetManager().Update(current)
	http.Redirect(w, r, "/admin-lol/settings?saved=true", http.StatusSeeOther)
}

// UpdateAdsTxt updates or clears the ads.txt content.
func (h *AdminHandlers) UpdateAdsTxt(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin-lol/settings?error=invalid_form#ads-txt-card", http.StatusSeeOther)
		return
	}

	action := r.FormValue("action")
	if action == "delete" {
		_ = settings.UpdateAdsTxt("")
		http.Redirect(w, r, "/admin-lol/settings?ads_deleted=true#ads-txt-card", http.StatusSeeOther)
		return
	}

	content := r.FormValue("ads_txt")
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.TrimSpace(content)
	if content != "" {
		content += "\n"
	}

	_ = settings.UpdateAdsTxt(content)
	http.Redirect(w, r, "/admin-lol/settings?ads_saved=true#ads-txt-card", http.StatusSeeOther)
}

// ToggleAntiAdblock updates the live anti-adblock enforcement status.
func (h *AdminHandlers) ToggleAntiAdblock(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin-lol/settings?error=invalid_form#anti-adblock-card", http.StatusSeeOther)
		return
	}

	current := settings.GetManager().GetAll()
	state := strings.ToLower(strings.TrimSpace(r.FormValue("state")))
	if state == "enable" || state == "1" || state == "true" || state == "on" {
		current.AntiAdblockEnabled = true
	} else if state == "disable" || state == "0" || state == "false" || state == "off" {
		current.AntiAdblockEnabled = false
	} else {
		current.AntiAdblockEnabled = !current.AntiAdblockEnabled
	}

	_ = settings.GetManager().Update(current)

	if r.Header.Get("X-Requested-With") == "XMLHttpRequest" || strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":              true,
			"anti_adblock_enabled": current.AntiAdblockEnabled,
		})
		return
	}

	statusStr := "disabled"
	if current.AntiAdblockEnabled {
		statusStr = "enabled"
	}
	http.Redirect(w, r, "/admin-lol/settings?adblock_saved=true&adblock_status="+statusStr+"#anti-adblock-card", http.StatusSeeOther)
}

// ToggleWhatsAppAlerts updates the live WhatsApp alerts dispatcher status.
func (h *AdminHandlers) ToggleWhatsAppAlerts(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin-lol/settings?error=invalid_form#whatsapp-alerts-card", http.StatusSeeOther)
		return
	}

	current := settings.GetManager().GetAll()
	state := strings.ToLower(strings.TrimSpace(r.FormValue("state")))
	if state == "enable" || state == "1" || state == "true" || state == "on" {
		current.WhatsAppAlertsEnabled = true
	} else if state == "disable" || state == "0" || state == "false" || state == "off" {
		current.WhatsAppAlertsEnabled = false
	} else {
		current.WhatsAppAlertsEnabled = !current.WhatsAppAlertsEnabled
	}

	_ = settings.GetManager().Update(current)

	if r.Header.Get("X-Requested-With") == "XMLHttpRequest" || strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"enabled": current.WhatsAppAlertsEnabled,
			"status":  map[bool]string{true: "enabled", false: "disabled"}[current.WhatsAppAlertsEnabled],
		})
		return
	}

	statusStr := "disabled"
	if current.WhatsAppAlertsEnabled {
		statusStr = "enabled"
	}
	http.Redirect(w, r, fmt.Sprintf("/admin-lol/settings?wa_saved=true&wa_status=%s#whatsapp-alerts-card", statusStr), http.StatusSeeOther)
}

func (h *AdminHandlers) SetLinkBot(lb *linkbot.Bot) {
	h.linkBot = lb
}

func (h *AdminHandlers) TriggerLinkCheck(w http.ResponseWriter, r *http.Request) {
	if h.linkBot != nil {
		go func() {
			_, _, _ = h.linkBot.RunCycle(context.Background())
		}()
	}
	http.Redirect(w, r, "/admin-lol/settings?link_checked=true", http.StatusSeeOther)
}

func (h *AdminHandlers) TriggerSync(w http.ResponseWriter, r *http.Request) {
	h.sync.TriggerSync(r.Context())
	http.Redirect(w, r, "/admin-lol/settings?synced=true", http.StatusSeeOther)
}

func (h *AdminHandlers) ClearCache(w http.ResponseWriter, r *http.Request) {
	h.cache.ClearAll(r.Context())
	http.Redirect(w, r, "/admin-lol/settings?cache_cleared=true", http.StatusSeeOther)
}

func (h *AdminHandlers) RebuildSitemap(w http.ResponseWriter, r *http.Request) {
	_ = h.sitemap.RebuildAll(r.Context())
	http.Redirect(w, r, "/admin-lol/settings?sitemap_rebuilt=true", http.StatusSeeOther)
}

// JobNew renders the manual job creation form.
func (h *AdminHandlers) JobNew(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title: "Create New Job",
	}
	if err := h.renderer.RenderAdmin(w, r, "admin/job_new", data); err != nil {
		slog.Error("Failed to render job_new template", "error", err)
	}
}

// JobCreate processes manual job creation form submission.
func (h *AdminHandlers) JobCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	title := strings.TrimSpace(r.FormValue("title"))
	companyName := strings.TrimSpace(r.FormValue("company_name"))
	locationName := strings.TrimSpace(r.FormValue("location_name"))
	description := strings.TrimSpace(r.FormValue("description"))
	externalURL := strings.TrimSpace(r.FormValue("external_url"))

	var minSalary, maxSalary *float64
	if minStr := r.FormValue("min_salary"); minStr != "" {
		if val, err := strconv.ParseFloat(minStr, 64); err == nil {
			minSalary = &val
		}
	}
	if maxStr := r.FormValue("max_salary"); maxStr != "" {
		if val, err := strconv.ParseFloat(maxStr, 64); err == nil {
			maxSalary = &val
		}
	}

	salaryType := r.FormValue("salary_type")
	contractType := r.FormValue("contract_type")
	jobType := r.FormValue("job_type")
	isRemote := r.FormValue("is_remote") == "on"
	isGraduate := r.FormValue("is_graduate") == "on"

	var expirationDate *time.Time
	if expStr := r.FormValue("expiration_date"); expStr != "" {
		if t, err := time.Parse("2006-01-02", expStr); err == nil {
			expirationDate = &t
		}
	}

	// Generate unique negative ID for reed_job_id
	reedJobID := -int64(time.Now().UnixNano() / 1000)

	// Generate slug
	slug := generateSlug(title)
	if locationName != "" {
		slug += "-" + generateSlug(locationName)
	}
	slug += fmt.Sprintf("-%d", -reedJobID)

	// Resolve or create company
	var employerID *int64
	if companyName != "" {
		companySlug := generateSlug(companyName)
		company, err := h.repos.Company.FindBySlug(ctx, companySlug)
		if err != nil {
			// Create company
			newCompany := &models.Company{
				Slug: companySlug,
				Name: companyName,
			}
			if err := h.repos.Company.Create(ctx, newCompany); err != nil {
				slog.Error("Failed to create company for manual job", "name", companyName, "error", err)
			} else {
				employerID = &newCompany.ID
			}
		} else {
			employerID = &company.ID
		}
	}

	// Resolve location ID if possible
	var locationID *int
	if locationName != "" {
		id := h.taxonomy.FindLocationID(ctx, locationName)
		if id > 0 {
			locationID = &id
		}
	}

	// Build job model
	job := &models.Job{
		ReedJobID:      &reedJobID,
		Source:         "manual",
		Slug:           slug,
		Title:          title,
		Description:    description,
		EmployerID:     employerID,
		LocationID:     locationID,
		LocationName:   locationName,
		MinSalary:      minSalary,
		MaxSalary:      maxSalary,
		Currency:       "GBP",
		SalaryType:     salaryType,
		ContractType:   contractType,
		JobType:        jobType,
		IsGraduate:     isGraduate,
		IsRemote:       isRemote,
		ExternalURL:    externalURL,
		DatePosted:     time.Now(),
		ExpirationDate: expirationDate,
		Status:         "active",
	}

	// Compute yearly salaries
	if job.MinSalary != nil {
		yearly := convertToYearly(*job.MinSalary, job.SalaryType)
		job.YearlyMin = &yearly
	}
	if job.MaxSalary != nil {
		yearly := convertToYearly(*job.MaxSalary, job.SalaryType)
		job.YearlyMax = &yearly
	}

	// Save to DB
	if err := h.repos.Job.Create(ctx, job); err != nil {
		slog.Error("Failed to save manual job", "error", err)
		http.Redirect(w, r, "/admin-lol/jobs?error=Failed+to+create+job", http.StatusSeeOther)
		return
	}

	// Map to taxonomy
	h.taxonomy.MapJobToTaxonomy(ctx, job.ID, job.Title, job.Description)

	// Notify Google Indexing API of new job
	go func(jobSlug string) {
		jobURL := fmt.Sprintf("%s/jobs/%s/", h.cfg.BaseURL, jobSlug)
		_ = h.indexing.PublishUpdate(context.Background(), jobURL)
	}(job.Slug)

	// Update taxonomy & company counts
	_ = h.repos.Company.UpdateJobCounts(ctx)
	_ = h.taxonomy.UpdateAllCounts(ctx)

	// Clear cache
	_ = h.cache.ClearPattern(ctx, "jobs:home:*")
	_ = h.cache.ClearPattern(ctx, "jobs:search:*")

	// Rebuild sitemaps
	_ = h.sitemap.RebuildAll(ctx)

	http.Redirect(w, r, "/admin-lol/jobs", http.StatusSeeOther)
}

func convertToYearly(amount float64, salaryType string) float64 {
	switch salaryType {
	case "per_hour":
		return amount * 37.5 * 52
	case "per_day":
		return amount * 260
	case "per_week":
		return amount * 52
	case "per_month":
		return amount * 12
	default:
		return amount
	}
}

var slugRegex = regexp.MustCompile(`[^a-z0-9]+`)

func generateSlug(s string) string {
	slug := strings.ToLower(strings.TrimSpace(s))
	slug = slugRegex.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 80 {
		slug = slug[:80]
		if i := strings.LastIndex(slug, "-"); i > 40 {
			slug = slug[:i]
		}
	}
	return slug
}

// ReportsManage renders the admin reported jobs management page.
func (h *AdminHandlers) ReportsManage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	reports, err := h.repos.Job.FindAllReports(ctx)
	if err != nil {
		slog.Error("Failed to fetch job reports", "error", err)
		reports = []models.JobReport{}
	}

	blacklist, err := h.repos.Job.GetBlacklistedIPs(ctx)
	if err != nil {
		slog.Error("Failed to fetch blacklisted IPs", "error", err)
		blacklist = []models.BlacklistedIP{}
	}

	data := PageData{
		Title:     "Reported Jobs | Admin Dashboard",
		ActiveNav: "reports",
		Data: map[string]interface{}{
			"Reports":        reports,
			"BlacklistedIPs": blacklist,
		},
	}

	if err := h.renderer.RenderAdmin(w, r, "admin/reports", data); err != nil {
		slog.Error("Failed to render admin reports page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// ReportAction handles admin action on a report (accept/delete job, or dismiss/delete report).
func (h *AdminHandlers) ReportAction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	reportIDStr := chi.URLParam(r, "id")
	reportID, err := strconv.ParseInt(reportIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid Report ID", http.StatusBadRequest)
		return
	}

	action := r.FormValue("action") // accept, dismiss, or restore
	jobIDStr := r.FormValue("job_id")
	jobID, _ := strconv.ParseInt(jobIDStr, 10, 64)

	if action == "accept" && jobID > 0 {
		// Accept report = Soft-delete the job listing
		if err := h.repos.Job.SoftDelete(ctx, jobID); err != nil {
			slog.Error("Failed to soft-delete job listing from report", "error", err)
			http.Redirect(w, r, "/admin-lol/reports?error=Failed+to+delete+job", http.StatusSeeOther)
			return
		}

		// Clear search cache
		_ = h.cache.ClearSearchCache(ctx)
		slog.Info("Job soft-deleted via admin report accept", "job_id", jobID)
		http.Redirect(w, r, "/admin-lol/reports?success=Job+listing+removed+successfully", http.StatusSeeOther)
		return
	} else if action == "restore" && jobID > 0 {
		// Restore report = Reactivate the job listing
		if err := h.repos.Job.Reactivate(ctx, jobID); err != nil {
			slog.Error("Failed to restore job listing from report", "error", err)
			http.Redirect(w, r, "/admin-lol/reports?error=Failed+to+restore+job", http.StatusSeeOther)
			return
		}

		// Clear search cache
		_ = h.cache.ClearSearchCache(ctx)
		slog.Info("Job restored via admin report restore", "job_id", jobID)
		http.Redirect(w, r, "/admin-lol/reports?success=Job+listing+restored+successfully", http.StatusSeeOther)
		return
	} else if action == "dismiss" {
		// Dismiss report = delete only the report entry from db
		if err := h.repos.Job.DeleteReport(ctx, reportID); err != nil {
			slog.Error("Failed to delete job report", "error", err)
			http.Redirect(w, r, "/admin-lol/reports?error=Failed+to+dismiss+report", http.StatusSeeOther)
			return
		}
		slog.Info("Job report dismissed by admin", "report_id", reportID)
		http.Redirect(w, r, "/admin-lol/reports?success=Report+dismissed+successfully", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin-lol/reports", http.StatusSeeOther)
}

// BlacklistIP manually blocks an IP address.
func (h *AdminHandlers) BlacklistIP(w http.ResponseWriter, r *http.Request) {
	ip := strings.Trim(strings.TrimSpace(r.FormValue("ip_address")), "[]")
	reason := strings.TrimSpace(r.FormValue("reason"))
	if reason == "" {
		reason = "Manual block by Admin"
	}

	if ip != "" {
		if err := h.repos.Job.BlacklistIP(r.Context(), ip, reason); err != nil {
			slog.Error("Failed to blacklist IP", "ip", ip, "error", err)
			http.Redirect(w, r, "/admin-lol/reports?error=Failed+to+blacklist+IP", http.StatusSeeOther)
			return
		}
		slog.Info("IP address blacklisted by admin", "ip", ip, "reason", reason)
		http.Redirect(w, r, "/admin-lol/reports?success=IP+address+blocked+successfully", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin-lol/reports", http.StatusSeeOther)
}

// UnblockIP manually unblocks a blocked IP address.
func (h *AdminHandlers) UnblockIP(w http.ResponseWriter, r *http.Request) {
	ip := strings.Trim(strings.TrimSpace(r.FormValue("ip")), "[]")
	if ip != "" {
		if err := h.repos.Job.UnblacklistIP(r.Context(), ip); err != nil {
			slog.Error("Failed to unblacklist IP", "ip", ip, "error", err)
			http.Redirect(w, r, "/admin-lol/reports?error=Failed+to+unblock+IP", http.StatusSeeOther)
			return
		}
		slog.Info("IP address unblocked by admin", "ip", ip)
		http.Redirect(w, r, "/admin-lol/reports?success=IP+address+unblocked+successfully", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin-lol/reports", http.StatusSeeOther)
}

// BulkDismiss deletes multiple reports at once.
func (h *AdminHandlers) BulkDismiss(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	reportIDStrs := r.Form["report_ids"]
	var ids []int64
	for _, idStr := range reportIDStrs {
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			ids = append(ids, id)
		}
	}

	if len(ids) > 0 {
		if err := h.repos.Job.BulkDeleteReports(r.Context(), ids); err != nil {
			slog.Error("Failed to bulk dismiss reports", "error", err)
		}
	}

	http.Redirect(w, r, "/admin-lol/reports", http.StatusSeeOther)
}

// PinAdmin renders the Pinterest Pin Management Dashboard.
func (h *AdminHandlers) PinAdmin(w http.ResponseWriter, r *http.Request) {
	tab := r.URL.Query().Get("tab")
	if tab != "pins" {
		tab = "all"
	}
	pinnedOnly := (tab == "pins")

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit := 30
	offset := (page - 1) * limit

	jobs, total, err := h.repos.Job.FindPinterestJobs(r.Context(), pinnedOnly, query, limit, offset)
	if err != nil {
		slog.Error("Failed to find jobs for Pinterest admin", "error", err)
	}

	totalActive, totalPinned, err := h.repos.Job.GetPinterestStats(r.Context())
	if err != nil {
		slog.Error("Failed to get Pinterest stats", "error", err)
	}

	unpinnedCount := totalActive - totalPinned
	if unpinnedCount < 0 {
		unpinnedCount = 0
	}

	var pinRate float64
	if totalActive > 0 {
		pinRate = (float64(totalPinned) / float64(totalActive)) * 100
	}

	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	data := PageData{
		Title: "Pinterest Pins Manager - JobyUK.com Admin",
		Data: map[string]interface{}{
			"Jobs":          jobs,
			"Total":         total,
			"Page":          page,
			"TotalPages":    totalPages,
			"Query":         query,
			"Tab":           tab,
			"TotalActive":   totalActive,
			"TotalPinned":   totalPinned,
			"UnpinnedCount": unpinnedCount,
			"PinRate":       fmt.Sprintf("%.1f", pinRate),
			"BaseURL":       h.cfg.BaseURL,
		},
	}

	if err := h.renderer.RenderAdmin(w, r, "admin/pin_admin", data); err != nil {
		slog.Error("Failed to render pin admin template", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// PinUpdate handles updating or clearing a job's Pinterest Pin URL.
func (h *AdminHandlers) PinUpdate(w http.ResponseWriter, r *http.Request) {
	jobIDStr := chi.URLParam(r, "id")
	jobID, err := strconv.ParseInt(jobIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid job ID", http.StatusBadRequest)
		return
	}

	pinURL := strings.TrimSpace(r.FormValue("pin_url"))

	if err := h.repos.Job.UpdatePinterestPin(r.Context(), jobID, pinURL); err != nil {
		slog.Error("Failed to update Pinterest pin URL", "job_id", jobID, "error", err)
		if r.Header.Get("Accept") == "application/json" || r.Header.Get("X-Requested-With") == "XMLHttpRequest" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"success":false,"error":"%s"}`, err.Error())
			return
		}
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	if r.Header.Get("Accept") == "application/json" || r.Header.Get("X-Requested-With") == "XMLHttpRequest" {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"success":true,"job_id":%d,"pin_url":"%s"}`, jobID, html.EscapeString(pinURL))
		return
	}

	ref := r.Referer()
	if ref == "" {
		ref = "/pin-admin"
	}
	http.Redirect(w, r, ref, http.StatusSeeOther)
}

// InquiriesManage renders the admin contact inquiries list page.
func (h *AdminHandlers) InquiriesManage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status == "" {
		status = "all"
	}

	searchQuery := strings.TrimSpace(r.URL.Query().Get("q"))

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit := 30
	offset := (page - 1) * limit

	inquiries, total, err := h.repos.Contact.FindAll(ctx, status, searchQuery, limit, offset)
	if err != nil {
		slog.Error("Failed to fetch contact inquiries", "error", err)
		inquiries = []models.ContactInquiry{}
		total = 0
	}

	stats, err := h.repos.Contact.GetStats(ctx)
	if err != nil {
		slog.Error("Failed to fetch contact inquiry stats", "error", err)
	}

	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	data := PageData{
		Title:     "Contact Inquiries | Admin Dashboard",
		ActiveNav: "inquiries",
		Data: map[string]interface{}{
			"Inquiries":    inquiries,
			"Stats":        stats,
			"StatusFilter": status,
			"SearchQuery":  searchQuery,
			"Page":         page,
			"TotalPages":   totalPages,
			"TotalCount":   total,
		},
	}

	if err := h.renderer.RenderAdmin(w, r, "admin/inquiries", data); err != nil {
		slog.Error("Failed to render admin inquiries page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// InquiryUpdateStatus updates the status of an inquiry (e.g., read, resolved, unread).
func (h *AdminHandlers) InquiryUpdateStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid Inquiry ID", http.StatusBadRequest)
		return
	}

	status := strings.TrimSpace(r.FormValue("status"))
	if status == "" {
		status = "read"
	}

	if err := h.repos.Contact.UpdateStatus(ctx, id, status); err != nil {
		slog.Error("Failed to update inquiry status", "id", id, "status", status, "error", err)
	}

	if r.Header.Get("X-Requested-With") == "XMLHttpRequest" || strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"id":      id,
			"status":  status,
		})
		return
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

// InquiryDelete deletes a contact inquiry.
func (h *AdminHandlers) InquiryDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid Inquiry ID", http.StatusBadRequest)
		return
	}

	if err := h.repos.Contact.Delete(ctx, id); err != nil {
		slog.Error("Failed to delete inquiry", "id", id, "error", err)
	}

	if r.Header.Get("X-Requested-With") == "XMLHttpRequest" || strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"id":      id,
		})
		return
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}



