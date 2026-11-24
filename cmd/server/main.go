package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"

	"jobyuk/internal/config"
	"jobyuk/internal/handlers"
	"jobyuk/internal/middleware"
	"jobyuk/internal/repository"
	"jobyuk/internal/services"
	"jobyuk/internal/services/freehire"
	"jobyuk/internal/services/linkbot"
	"jobyuk/internal/services/reed"
	"jobyuk/internal/services/whatsapp"
)

func main() {
	// Load .env file (ignore error in production)
	_ = godotenv.Load()

	// Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Initialize database connection
	db, err := repository.NewDB(cfg.DatabaseURL)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Initialize blacklisted_ips and pinterest tables if not exists
	_, _ = db.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS blacklisted_ips (
			ip_address TEXT PRIMARY KEY,
			reason TEXT,
			created_at TIMESTAMPTZ DEFAULT NOW()
		);
		CREATE TABLE IF NOT EXISTS job_reports (
			id BIGSERIAL PRIMARY KEY,
			job_id BIGINT,
			reason TEXT,
			details TEXT DEFAULT '',
			ip_address TEXT DEFAULT '',
			created_at TIMESTAMPTZ DEFAULT NOW()
		);
		ALTER TABLE job_reports ADD COLUMN IF NOT EXISTS reason TEXT DEFAULT '';
		ALTER TABLE job_reports ADD COLUMN IF NOT EXISTS details TEXT DEFAULT '';
		ALTER TABLE job_reports ADD COLUMN IF NOT EXISTS ip_address TEXT DEFAULT '';
		ALTER TABLE job_reports ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ DEFAULT NOW();
		CREATE INDEX IF NOT EXISTS idx_job_reports_job_id ON job_reports(job_id);
		CREATE INDEX IF NOT EXISTS idx_job_reports_ip ON job_reports(ip_address);
		ALTER TABLE jobs ADD COLUMN IF NOT EXISTS pinterest_pin_url TEXT DEFAULT '';
		CREATE INDEX IF NOT EXISTS idx_jobs_pinterest_pin ON jobs (pinterest_pin_url) WHERE pinterest_pin_url != '' AND pinterest_pin_url IS NOT NULL;

		CREATE TABLE IF NOT EXISTS pinterest_users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			email VARCHAR(150) UNIQUE NOT NULL,
			phone VARCHAR(50) DEFAULT '',
			access_code VARCHAR(10) UNIQUE NOT NULL,
			pinterest_handle VARCHAR(100) DEFAULT '',
			daily_limit INT DEFAULT 5,
			is_active BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		);
		ALTER TABLE pinterest_users ADD COLUMN IF NOT EXISTS pinterest_handle VARCHAR(100) DEFAULT '';
		ALTER TABLE pinterest_users ADD COLUMN IF NOT EXISTS deactivation_reason VARCHAR(255) DEFAULT '';

		CREATE TABLE IF NOT EXISTS pinterest_assignments (
			id BIGSERIAL PRIMARY KEY,
			user_id INT REFERENCES pinterest_users(id) ON DELETE CASCADE,
			job_id BIGINT UNIQUE REFERENCES jobs(id) ON DELETE CASCADE,
			pin_url TEXT DEFAULT '',
			status VARCHAR(20) DEFAULT 'assigned',
			assigned_at TIMESTAMPTZ DEFAULT NOW(),
			submitted_at TIMESTAMPTZ,
			pin_date DATE DEFAULT CURRENT_DATE,
			created_at TIMESTAMPTZ DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS pinterest_user_daily_quotas (
			id SERIAL PRIMARY KEY,
			user_id INT REFERENCES pinterest_users(id) ON DELETE CASCADE,
			quota_date DATE NOT NULL,
			daily_limit INT NOT NULL,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			UNIQUE(user_id, quota_date)
		);

		CREATE INDEX IF NOT EXISTS idx_pinterest_assignments_user ON pinterest_assignments(user_id, status, submitted_at);
		CREATE INDEX IF NOT EXISTS idx_pinterest_users_code ON pinterest_users(access_code);
		CREATE INDEX IF NOT EXISTS idx_pinterest_user_daily_quotas ON pinterest_user_daily_quotas(user_id, quota_date);
	`)
	if err != nil {
		slog.Warn("Notice on auxiliary tables startup script", "err", err)
	}

	// Automatically ensure admin user credentials from .env exist in database
	if cfg.AdminUsername != "" && cfg.AdminPassword != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
		if err == nil {
			_, _ = db.Exec(context.Background(), `
				INSERT INTO admin_users (username, password_hash, role)
				VALUES ($1, $2, 'admin')
				ON CONFLICT (username) DO UPDATE 
				SET password_hash = EXCLUDED.password_hash;
			`, cfg.AdminUsername, string(hash))
		}
	}

	// Auto-seed Career Hub articles and schedule drip feed (works across local and VPS automatically)
	if err := repository.AutoSeedArticles(context.Background(), db); err != nil {
		slog.Warn("Notice on AutoSeedArticles", "error", err)
	}

	// Auto-seed UK Job Roles and sync active job mappings (works across local and VPS)
	if err := repository.AutoSeedRoles(context.Background(), db); err != nil {
		slog.Warn("Notice on AutoSeedRoles", "error", err)
	}

	// Asynchronously update job role active counts in background
	go func() {
		time.Sleep(30 * time.Second)
		_, err := db.Exec(context.Background(), `
			UPDATE job_roles jr
			SET job_count = sub.cnt
			FROM (
				SELECT jr2.id, COUNT(DISTINCT j.id) as cnt
				FROM job_roles jr2
				LEFT JOIN job_roles_mapping jrm ON jr2.id = jrm.role_id
				LEFT JOIN jobs j ON (
					(j.id = jrm.job_id OR j.title ILIKE '%' || REPLACE(jr2.name, '-', ' ') || '%')
					AND j.status = 'active'
				)
				GROUP BY jr2.id
			) sub
			WHERE jr.id = sub.id
		`)
		if err != nil {
			slog.Warn("Failed to background sync role counts", "error", err)
		}
	}()

	// Initialize Redis connection
	rdb, err := services.NewRedisClient(cfg.RedisURL)
	if err != nil {
		slog.Error("Failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()

	// Initialize repositories
	repos := repository.NewRepositories(db)

	// Ensure WhatsApp database tables, columns, constraints and seed rows are migrated
	if err := repos.WhatsApp.AutoMigrate(context.Background()); err != nil {
		slog.Error("Failed to auto-migrate WhatsApp schema", "error", err)
	} else {
		slog.Info("WhatsApp schema auto-migrated successfully")
	}

	// Ensure Contact Inquiries database tables and indexes are migrated
	if err := repos.Contact.AutoMigrate(context.Background()); err != nil {
		slog.Error("Failed to auto-migrate Contact inquiries schema", "error", err)
	} else {
		slog.Info("Contact inquiries schema auto-migrated successfully")
	}

	// Initialize services
	cacheService := services.NewCacheService(rdb)
	reedClient := reed.NewClient(cfg.ReedAPIKey, cacheService)
	searchService := services.NewSearchService(repos.Job, cacheService)
	seoService := services.NewSEOService(cfg.BaseURL)
	sitemapService := services.NewSitemapService(repos, cfg.BaseURL)
	taxonomyService := services.NewTaxonomyService(repos, cacheService)
	googleIndexingService := services.NewGoogleIndexingService(cfg.GoogleCredentialsJSON)
	syncService := reed.NewSyncService(reedClient, repos, taxonomyService, googleIndexingService, cfg.BaseURL)
	freehireClient := freehire.NewClient()
	freehireSyncService := freehire.NewSyncService(freehireClient, repos, taxonomyService, googleIndexingService, cfg.BaseURL)
	analyticsService := services.NewAnalyticsService(repos.PageView)

	// Initialize template renderer
	renderer, err := handlers.NewRenderer("web/templates")
	if err != nil {
		slog.Error("Failed to initialize templates", "error", err)
		os.Exit(1)
	}

	// Initialize handlers
	h := handlers.NewHandlers(
		renderer,
		repos,
		searchService,
		seoService,
		sitemapService,
		taxonomyService,
		cacheService,
		analyticsService,
		cfg,
	)

	// Initialize Smart Job Link Health & Expiry Bot
	linkBot := linkbot.NewBot(db, repos.Job, sitemapService, googleIndexingService, cfg.BaseURL)
	if err := linkBot.AutoMigrate(context.Background()); err != nil {
		slog.Error("Failed to auto-migrate linkbot schema", "error", err)
	}

	// Initialize admin handlers
	adminH := handlers.NewAdminHandlers(
		renderer,
		repos,
		syncService,
		sitemapService,
		cacheService,
		taxonomyService,
		googleIndexingService,
		cfg,
	)
	adminH.SetLinkBot(linkBot)
	autoIndexer := services.NewAutoIndexerService(repos.Job, googleIndexingService, cfg.BaseURL)
	adminH.SetAutoIndexer(autoIndexer)
	indexNowService := services.NewIndexNowService(cfg.BaseURL, cfg.IndexNowKey)
	adminH.SetIndexNow(indexNowService)

	// Initialize Pinterest handlers with Redis rate limiter
	pinterestH := handlers.NewPinterestHandlers(
		renderer,
		repos,
		rdb,
		cfg,
	)

	// Initialize WhatsApp service, Google Contacts sync, and dedicated portal handlers
	waService := whatsapp.NewService(repos.WhatsApp, cfg.BaseURL)

	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	googleRedirectURL := os.Getenv("GOOGLE_REDIRECT_URL")
	if googleRedirectURL == "" {
		googleRedirectURL = strings.TrimRight(cfg.BaseURL, "/") + "/wa-portal/google-callback"
	}

	googleContactsService := services.NewGoogleContactsService(
		googleClientID,
		googleClientSecret,
		googleRedirectURL,
	)
	waService.SetContactSyncer(googleContactsService)

	alertDispatcher := whatsapp.NewAlertDispatcher(waService, repos, cfg.BaseURL)
	syncService.SetOnJobsSynced(func(ctx context.Context) {
		if cfg.EnableWhatsAppAlerts {
			alertDispatcher.TriggerImmediateCycle(ctx)
		}
	})

	waAdminH := handlers.NewWhatsAppAdminHandlers(renderer, repos, rdb, cfg, waService)
	waAdminH.SetGoogleContacts(googleContactsService)
	waAdminH.SetAlertDispatcher(alertDispatcher)

	waClientH := handlers.NewWhatsAppClientHandlers(renderer, repos, rdb, cfg, waService)
	waClientH.SetAlertDispatcher(alertDispatcher)

	// Build router
	r := chi.NewRouter()

	// Custom error handler for panic recovery
	panicRecoveryHandler := func(w http.ResponseWriter, r *http.Request, err interface{}) {
		errorData := handlers.PageData{
			Title:  "Internal Server Error | JobyUK.com",
			Robots: "noindex",
		}
		_ = renderer.RenderWithStatus(w, "500", http.StatusInternalServerError, errorData)
	}

	// Global middleware stack
	r.Use(chimw.RealIP)
	r.Use(chimw.GetHead)
	r.Use(middleware.MethodOverride)
	r.Use(middleware.Redirects(repos.Redirect))
	r.Use(middleware.EnforceTrailingSlash())
	r.Use(middleware.Recovery(logger, panicRecoveryHandler))
	r.Use(middleware.RequestLogger(logger))
	r.Use(middleware.Analytics(analyticsService, cfg.AnalyticsSalt))
	r.Use(middleware.SecurityHeaders(cfg.IsDev()))
	r.Use(middleware.Compress())
	r.Use(middleware.RateLimit(rdb, 1200, time.Minute)) // 1200 req/min (20 req/sec headroom for fast browsing, pagination, multi-tab, & search AJAX)

	// Serve static files with aggressive caching
	fileServer := http.FileServer(http.Dir("web/static"))
	r.Handle("/static/*", middleware.CacheControl("public, max-age=31536000, immutable")(
		http.StripPrefix("/static/", fileServer),
	))

	// Public routes
	r.Get("/", h.Home)
	r.Get("/health", h.Health)
	r.Get("/ad-test", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		http.ServeFile(w, r, "web/static/ad-test.html")
	})
	r.Get("/ad-test/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		http.ServeFile(w, r, "web/static/ad-test.html")
	})
	r.Get("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=604800")
		http.ServeFile(w, r, "web/static/images/favicon.ico")
	})
	r.Get("/site.webmanifest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/manifest+json")
		w.Header().Set("Cache-Control", "public, max-age=604800")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		http.ServeFile(w, r, "web/static/site.webmanifest")
	})
	r.Get("/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/manifest+json")
		w.Header().Set("Cache-Control", "public, max-age=604800")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		http.ServeFile(w, r, "web/static/site.webmanifest")
	})
	r.Get("/job/logo/{filename}", h.JobLogoProxy)

	// Job routes
	r.Get("/jobs", h.JobList)
	r.Get("/jobs/", h.JobList)
	r.Get("/jobs/{slug}", h.JobDetail)
	r.Get("/jobs/{slug}/", h.JobDetail)
	r.Get("/jobs/{slug}/og.svg", h.JobOGImage)
	r.Get("/jobs/{slug}/og.png", h.JobOGImage)
	r.Get("/jobs/{id}/apply", h.JobApplyRedirect)
	r.Get("/jobs/{id}/apply/", h.JobApplyRedirect)
	r.Get("/jobs/{id}/apply/out", h.JobApplyOut)
	r.Get("/jobs/{id}/apply/out/", h.JobApplyOut)
	r.Post("/jobs/{id}/report", h.ReportJob)
	r.Post("/jobs/{id}/report/", h.ReportJob)
	r.Get("/api/search/suggestions", h.SearchSuggestions)
	r.Get("/api/search/suggestions/", h.SearchSuggestions)

	// Taxonomy routes
	r.Get("/job-roles/", h.RoleIndex)
	r.Get("/job-roles/{slug}/", h.RoleDetail)
	r.Get("/job-roles/{roleSlug}/{locationSlug}/", h.RoleLocation)

	r.Get("/industries/", h.IndustryIndex)
	r.Get("/industries/{slug}/", h.IndustryDetail)

	r.Get("/locations/", h.LocationIndex)
	r.Get("/locations/{slug}/", h.LocationDetail)
	r.Get("/locations/{regionSlug}/{citySlug}/", h.LocationCity)

	r.Get("/job-types/", h.JobTypeIndex)
	r.Get("/job-types/{slug}/", h.JobTypeDetail)

	r.Get("/remote-jobs/", h.RemoteJobs)
	r.Get("/remote-jobs/{slug}/", h.RemoteJobsCategory)

	r.Get("/graduate-jobs/", h.GraduateJobs)
	r.Get("/apprenticeships/", h.Apprenticeships)

	// Company routes
	r.Get("/companies/", h.CompanyIndex)
	r.Get("/companies/{slug}/", h.CompanyDetail)

	// Saved Jobs (Figma 06. Saved Jobs)
	r.Get("/saved-jobs", h.SavedJobs)
	r.Get("/saved-jobs/", h.SavedJobs)
	r.Get("/saved", h.SavedJobs)
	r.Get("/saved/", h.SavedJobs)

	// Content routes
	r.Get("/salaries/", h.SalaryIndex)
	r.Get("/salaries/{slug}/", h.SalaryDetail)
	r.Get("/salary-calculator", h.SalaryCalculator)
	r.Get("/salary-calculator/", h.SalaryCalculator)

	r.Get("/career-advice", h.CareerAdviceIndex)
	r.Get("/career-advice/", h.CareerAdviceIndex)
	r.Get("/career-advice/{category}", h.CareerAdviceCategory)
	r.Get("/career-advice/{category}/", h.CareerAdviceCategory)
	r.Get("/career-advice/{category}/{slug}", h.CareerAdviceArticle)
	r.Get("/career-advice/{category}/{slug}/", h.CareerAdviceArticle)

	// Company Info & About
	r.Get("/about", h.AboutUs)
	r.Get("/about/", h.AboutUs)
	r.Get("/about-us", h.AboutUs)
	r.Get("/about-us/", h.AboutUs)

	// Contact Us & Inquiries
	r.Get("/contact", h.ContactPage)
	r.Get("/contact/", h.ContactPage)
	r.Post("/contact", h.ContactSubmit)
	r.Post("/contact/", h.ContactSubmit)
	r.Get("/contact-us", h.ContactPage)
	r.Get("/contact-us/", h.ContactPage)
	r.Post("/contact-us", h.ContactSubmit)
	r.Post("/contact-us/", h.ContactSubmit)

	// Employer Job Posting
	r.Get("/employers/post-job", h.EmployerPostJob)
	r.Get("/employers/post-job/", h.EmployerPostJob)
	r.Get("/post-a-job", h.EmployerPostJob)
	r.Get("/post-a-job/", h.EmployerPostJob)
	r.Get("/post-job", h.EmployerPostJob)
	r.Get("/post-job/", h.EmployerPostJob)

	// WhatsApp Job Alerts & VIP Subscription Landing Page
	r.Get("/whatsapp-alerts", h.WhatsAppAlertsPage)
	r.Get("/whatsapp-alerts/", h.WhatsAppAlertsPage)
	r.Get("/job-alerts", h.WhatsAppAlertsPage)
	r.Get("/job-alerts/", h.WhatsAppAlertsPage)
	r.Get("/subscribe", h.WhatsAppAlertsPage)
	r.Get("/subscribe/", h.WhatsAppAlertsPage)
	r.Get("/pricing", h.WhatsAppAlertsPage)
	r.Get("/pricing/", h.WhatsAppAlertsPage)

	// Legal pages & Aliases
	r.Get("/privacy-policy/", h.PrivacyPolicy)
	r.Get("/privacy-policy", h.PrivacyPolicy)
	r.Get("/privacy/", h.PrivacyPolicy)
	r.Get("/privacy", h.PrivacyPolicy)
	r.Get("/terms-of-service/", h.TermsOfService)
	r.Get("/terms-of-service", h.TermsOfService)
	r.Get("/terms/", h.TermsOfService)
	r.Get("/terms", h.TermsOfService)
	r.Get("/cookie-policy/", h.CookiePolicy)
	r.Get("/cookie-policy", h.CookiePolicy)
	r.Get("/cookies/", h.CookiePolicy)
	r.Get("/cookies", h.CookiePolicy)

	// SEO routes
	r.Get("/sitemap.xml", h.SitemapIndex)
	r.Get("/sitemap-jobs.xml", h.SitemapJobs)
	r.Get("/sitemap-taxonomy.xml", h.SitemapTaxonomy)
	r.Get("/sitemap-companies.xml", h.SitemapCompanies)
	r.Get("/sitemap-content.xml", h.SitemapContent)
	r.Get("/robots.txt", h.RobotsTxt)
	r.Get("/llms.txt", h.LLMsTxt)
	r.Get("/llms-full.txt", h.LLMsFullTxt)
	r.Get("/ads.txt", h.AdsTxt)
	r.Get("/ads.txt/", h.AdsTxt)

	// IndexNow verification key for Bing & Yahoo
	r.Get("/" + cfg.IndexNowKey + ".txt", h.IndexNowKey)
	r.Get("/ab4225315c73f110cb98d2abd9911d76.txt", h.IndexNowKey)

	// User-Facing Pinterest Poster Portal Routes
	r.Get("/pin-login", pinterestH.LoginPage)
	r.Get("/pin-login/", pinterestH.LoginPage)
	r.Post("/pin-login", pinterestH.LoginSubmit)
	r.Get("/pin-logout", pinterestH.Logout)
	r.Get("/pin-logout/", pinterestH.Logout)
	r.Get("/pin-dashboard", pinterestH.Dashboard)
	r.Get("/pin-dashboard/", pinterestH.Dashboard)
	r.Post("/pin-dashboard/submit", pinterestH.SubmitPin)
	r.Post("/pin-dashboard/handle", pinterestH.UpdateHandle)
	r.Post("/pin-dashboard/edit-pin", pinterestH.EditPin)
	r.Get("/pin-dashboard/heartbeat", pinterestH.Heartbeat)

	// Dedicated WhatsApp Client Portal Routes (/my-alerts)
	r.Group(func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !cfg.EnableWhatsAppAlerts {
					http.Redirect(w, r, "/", http.StatusFound)
					return
				}
				next.ServeHTTP(w, r)
			})
		})

	r.Get("/my-alerts/login", waClientH.LoginPage)
	r.Get("/my-alerts/login/", waClientH.LoginPage)
	r.Post("/my-alerts/login", waClientH.LoginWithPIN)
	r.Post("/my-alerts/login/", waClientH.LoginWithPIN)

	r.Get("/my-alerts/register", waClientH.RegisterPage)
	r.Get("/my-alerts/register/", waClientH.RegisterPage)
	r.Post("/my-alerts/register", waClientH.RegisterSubmit)
	r.Post("/my-alerts/register/", waClientH.RegisterSubmit)
	r.Post("/my-alerts/register/send-otp", waClientH.RegisterSubmit)
	r.Post("/my-alerts/register/verify", waClientH.RegisterVerify)

	r.Get("/my-alerts/verify", waClientH.VerifyPage)
	r.Get("/my-alerts/verify/", waClientH.VerifyPage)

	r.Get("/my-alerts/forgot-pin", waClientH.ForgotPINPage)
	r.Get("/my-alerts/forgot-pin/", waClientH.ForgotPINPage)
	r.Post("/my-alerts/forgot-pin/send-otp", waClientH.ForgotPINRequest)
	r.Get("/my-alerts/reset-pin", waClientH.ResetPINPage)
	r.Get("/my-alerts/reset-pin/", waClientH.ResetPINPage)
	r.Post("/my-alerts/forgot-pin/verify", waClientH.ForgotPINVerify)

	r.Get("/my-alerts/logout", waClientH.Logout)
	r.Get("/my-alerts/logout/", waClientH.Logout)

	r.Group(func(r chi.Router) {
		r.Use(waClientH.RequireSubscriber)
		r.Get("/my-alerts", waClientH.Dashboard)
		r.Get("/my-alerts/", waClientH.Dashboard)
		r.Post("/my-alerts/change-number", waClientH.ChangeNumber)
		r.Post("/my-alerts/preferences", waClientH.UpdatePreferences)
		r.Post("/my-alerts/toggle-pause", waClientH.TogglePause)
		r.Post("/my-alerts/checkout", waClientH.UpgradeCheckout)
		r.Post("/my-alerts/checkout/", waClientH.UpgradeCheckout)
		r.Post("/my-alerts/upgrade", waClientH.UpgradeCheckout)
		r.Post("/my-alerts/upgrade/", waClientH.UpgradeCheckout)
		r.Post("/my-alerts/crypto-payment", waClientH.SubmitCryptoPayment)
		r.Post("/my-alerts/crypto-payment/", waClientH.SubmitCryptoPayment)
		r.Post("/my-alerts/request-invoice", waClientH.RequestInvoice)
		r.Post("/my-alerts/request-invoice/", waClientH.RequestInvoice)
	})
	})

	// Legacy backward-compatibility 301 redirects & aliases for /wa-client
	legacyClientRedirect := func(w http.ResponseWriter, r *http.Request) {
		target := strings.Replace(r.URL.Path, "/wa-client", "/my-alerts", 1)
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	}
	r.Get("/wa-client", legacyClientRedirect)
	r.Get("/wa-client/*", legacyClientRedirect)

	// Keep POST handlers on /wa-client as aliases to ensure in-flight forms continue working
	r.Post("/wa-client/login", waClientH.LoginWithPIN)
	r.Post("/wa-client/login/", waClientH.LoginWithPIN)
	r.Post("/wa-client/register", waClientH.RegisterSubmit)
	r.Post("/wa-client/register/", waClientH.RegisterSubmit)
	r.Post("/wa-client/register/send-otp", waClientH.RegisterSubmit)
	r.Post("/wa-client/register/verify", waClientH.RegisterVerify)
	r.Post("/wa-client/forgot-pin/send-otp", waClientH.ForgotPINRequest)
	r.Post("/wa-client/forgot-pin/verify", waClientH.ForgotPINVerify)
	r.Group(func(r chi.Router) {
		r.Use(waClientH.RequireSubscriber)
		r.Post("/wa-client/change-number", waClientH.ChangeNumber)
		r.Post("/wa-client/preferences", waClientH.UpdatePreferences)
		r.Post("/wa-client/toggle-pause", waClientH.TogglePause)
		r.Post("/wa-client/checkout", waClientH.UpgradeCheckout)
		r.Post("/wa-client/checkout/", waClientH.UpgradeCheckout)
		r.Post("/wa-client/upgrade", waClientH.UpgradeCheckout)
		r.Post("/wa-client/upgrade/", waClientH.UpgradeCheckout)
		r.Post("/wa-client/crypto-payment", waClientH.SubmitCryptoPayment)
		r.Post("/wa-client/crypto-payment/", waClientH.SubmitCryptoPayment)
		r.Post("/wa-client/request-invoice", waClientH.RequestInvoice)
		r.Post("/wa-client/request-invoice/", waClientH.RequestInvoice)
	})

	// Public API for Homepage 3-Day Free Trial Subscription
	r.Post("/api/whatsapp/trial-subscribe", waClientH.TrialSubscribe)
	r.Post("/api/whatsapp/trial-subscribe/", waClientH.TrialSubscribe)
	r.Post("/api/whatsapp/trial-verify", waClientH.TrialVerify)
	r.Post("/api/whatsapp/trial-verify/", waClientH.TrialVerify)
	r.Post("/api/whatsapp/track-click", waClientH.TrackClickAPI)
	r.Post("/api/whatsapp/track-click/", waClientH.TrackClickAPI)

	// WhatsApp Contact Management & Phone Sync API (Anti-Ban Engine)
	r.Post("/api/wa/contacts", waClientH.SaveContactAPI)
	r.Post("/api/wa/contacts/", waClientH.SaveContactAPI)
	r.Get("/api/wa/contacts", waClientH.ListContactsAPI)
	r.Get("/api/wa/contacts/", waClientH.ListContactsAPI)
	r.Get("/api/wa/contacts/export.vcf", waClientH.ExportContactsVCF)
	r.Get("/api/wa/contacts/export.vcf/", waClientH.ExportContactsVCF)

	// Dedicated WhatsApp Gateway Management Portal Routes (/wa-portal)
	r.Get("/wa-portal/login", waAdminH.LoginPage)
	r.Get("/wa-portal/login/", waAdminH.LoginPage)
	r.Post("/wa-portal/login", waAdminH.LoginSubmit)
	r.Get("/wa-portal/logout", waAdminH.Logout)
	r.Get("/wa-portal/logout/", waAdminH.Logout)
	r.Get("/wa-portal/google-callback", waAdminH.GoogleCallback)
	r.Get("/wa-portal/google-callback/", waAdminH.GoogleCallback)

	r.Group(func(r chi.Router) {
		r.Use(waAdminH.RequireAuth)
		r.Get("/wa-portal", waAdminH.Dashboard)
		r.Get("/wa-portal/", waAdminH.Dashboard)
		r.Get("/wa-portal/qr-refresh", waAdminH.RefreshQR)
		r.Get("/wa-portal/qr-refresh/", waAdminH.RefreshQR)
		r.Post("/wa-portal/qr-refresh", waAdminH.RefreshQR)
		r.Post("/wa-portal/qr-refresh/", waAdminH.RefreshQR)
		r.Get("/wa-portal/gateway/status", waAdminH.GatewayStatusJSON)
		r.Get("/wa-portal/gateway/status/", waAdminH.GatewayStatusJSON)
		r.Post("/wa-portal/disconnect-device", waAdminH.DisconnectDevice)
		r.Post("/wa-portal/disconnect-device/", waAdminH.DisconnectDevice)
		r.Post("/wa-portal/gateway-routing/update", waAdminH.UpdateGatewayRouting)
		r.Post("/wa-portal/gateway-routing/update/", waAdminH.UpdateGatewayRouting)
		r.Get("/wa-portal/subscribers", waAdminH.SubscribersPage)
		r.Get("/wa-portal/subscribers/", waAdminH.SubscribersPage)
		r.Post("/wa-portal/subscribers/new", waAdminH.CreateSubscriber)
		r.Post("/wa-portal/subscribers/{id}/toggle", waAdminH.ToggleSubscriber)
		r.Post("/wa-portal/subscribers/{id}/delete", waAdminH.DeleteSubscriber)
		r.Post("/wa-portal/subscribers/{id}/activate-plan", waAdminH.ActivateSubscriberPlan)
		r.Post("/wa-portal/subscribers/{id}/activate-plan/", waAdminH.ActivateSubscriberPlan)
		r.Get("/wa-portal/logs", waAdminH.LogsPage)
		r.Get("/wa-portal/logs/", waAdminH.LogsPage)
		r.Get("/wa-portal/analytics", waAdminH.AnalyticsPage)
		r.Get("/wa-portal/analytics/", waAdminH.AnalyticsPage)
		r.Post("/wa-portal/analytics/reset", waAdminH.ResetAnalytics)
		r.Post("/wa-portal/analytics/reset/", waAdminH.ResetAnalytics)
		r.Post("/wa-portal/test-message", waAdminH.SendTestMessage)
		r.Post("/wa-portal/match-jobs", waAdminH.MatchAndDispatchManual)
		r.Post("/wa-portal/channel/post-now", waAdminH.PostChannelJobNow)
		r.Post("/wa-portal/channel/post-now/", waAdminH.PostChannelJobNow)
		r.Get("/wa-portal/crypto-payments", waAdminH.CryptoPaymentsPage)
		r.Get("/wa-portal/crypto-payments/", waAdminH.CryptoPaymentsPage)
		r.Get("/wa-portal/payment-settings", waAdminH.CryptoPaymentsPage)
		r.Get("/wa-portal/payment-settings/", waAdminH.CryptoPaymentsPage)
		r.Get("/wa-portal/payment-setting", waAdminH.CryptoPaymentsPage)
		r.Get("/wa-portal/payment-setting/", waAdminH.CryptoPaymentsPage)
		r.Get("/wa-portal/payments", waAdminH.CryptoPaymentsPage)
		r.Get("/wa-portal/payments/", waAdminH.CryptoPaymentsPage)
		r.Get("/wa-portal/wallets", waAdminH.CryptoPaymentsPage)
		r.Get("/wa-portal/wallets/", waAdminH.CryptoPaymentsPage)
		r.Post("/wa-portal/payment-settings/update", waAdminH.UpdatePaymentSettings)
		r.Post("/wa-portal/payment-setting/update", waAdminH.UpdatePaymentSettings)
		r.Post("/wa-portal/crypto-payments/{id}/verify", waAdminH.VerifyCryptoPayment)
		r.Post("/wa-portal/crypto-payments/{id}/reject", waAdminH.RejectCryptoPayment)
		r.Post("/wa-portal/crypto-wallets/update", waAdminH.UpdateCryptoWallets)
		r.Get("/wa-portal/contacts/export.vcf", waAdminH.ExportContactsVCF)
		r.Get("/wa-portal/contacts/export.vcf/", waAdminH.ExportContactsVCF)
		r.Get("/wa-portal/google-connect", waAdminH.GoogleConnect)
		r.Get("/wa-portal/google-connect/", waAdminH.GoogleConnect)
		r.Post("/wa-portal/google-disconnect", waAdminH.GoogleDisconnect)
		r.Post("/wa-portal/google-disconnect/", waAdminH.GoogleDisconnect)
	})

	// Admin routes (protected under secret route /admin-lol)
	r.Route("/admin-lol", func(r chi.Router) {
		// Login routes with dedicated rate limiting (5 attempts / 15 min)
		r.Get("/login", adminH.LoginPage)
		r.With(middleware.LoginRateLimit(rdb, 5, 15*time.Minute)).Post("/login", adminH.LoginSubmit)
		r.Get("/logout", adminH.Logout)
		r.Post("/logout", adminH.Logout)


		r.Group(func(r chi.Router) {
			r.Use(middleware.AdminAuth(cfg.JWTSecret))
			r.Use(middleware.CSRFProtect(cfg.CSRFSecret, cfg.IsDev()))

			r.Get("/", adminH.Dashboard)
			r.Get("/heartbeat", adminH.Heartbeat)
			r.Get("/pinterest", pinterestH.AdminPinterestManager)
			r.Post("/pinterest/users/new", pinterestH.AdminCreateUser)
			r.Post("/pinterest/users/{id}/quota", pinterestH.AdminUpdateUserQuota)
			r.Post("/pinterest/users/{id}/regenerate-code", pinterestH.AdminRegenerateCode)
			r.Post("/pinterest/users/{id}/toggle", pinterestH.AdminToggleUser)
			r.Post("/pinterest/users/{id}/delete", pinterestH.AdminDeleteUser)
			r.Get("/pins", adminH.PinAdmin)
			r.Post("/pins/{id}", adminH.PinUpdate)
			r.Get("/jobs", adminH.JobsManage)
			r.Get("/jobs/new", adminH.JobNew)
			r.Post("/jobs/new", adminH.JobCreate)
			r.Post("/jobs/{id}/expire", adminH.JobExpire)
			r.Post("/jobs/{id}/reactivate", adminH.JobReactivate)
			r.Post("/jobs/{id}/delete", adminH.JobDelete)

			r.Get("/reports", adminH.ReportsManage)
			r.Post("/reports/{id}/action", adminH.ReportAction)
			r.Post("/reports/blacklist", adminH.BlacklistIP)
			r.Post("/reports/blacklist/unblock", adminH.UnblockIP)
			r.Post("/reports/bulk-dismiss", adminH.BulkDismiss)

			// Contact Inquiries Management
			r.Get("/inquiries", adminH.InquiriesManage)
			r.Post("/inquiries/{id}/status", adminH.InquiryUpdateStatus)
			r.Post("/inquiries/{id}/delete", adminH.InquiryDelete)

			r.Get("/taxonomy", adminH.TaxonomyManage)
			r.Post("/taxonomy/roles", adminH.RoleCreate)
			r.Put("/taxonomy/roles/{id}", adminH.RoleUpdate)

			r.Get("/content", adminH.ContentManage)
			r.Get("/content/articles/new", adminH.ArticleNew)
			r.Post("/content/articles", adminH.ArticleCreate)
			r.Get("/content/articles/{id}/edit", adminH.ArticleEdit)
			r.Put("/content/articles/{id}", adminH.ArticleUpdate)

			r.Get("/seo", adminH.SEOManage)
			r.Post("/seo/redirects", adminH.RedirectCreate)
			r.Delete("/seo/redirects/{id}", adminH.RedirectDelete)

			r.Get("/analytics", adminH.AnalyticsPage)

			r.Get("/settings", adminH.SettingsPage)
			r.Post("/settings/site", adminH.UpdateSiteSettings)
			r.Post("/settings/ads-txt", adminH.UpdateAdsTxt)
			r.Post("/settings/anti-adblock/toggle", adminH.ToggleAntiAdblock)
			r.Post("/settings/whatsapp-alerts/toggle", adminH.ToggleWhatsAppAlerts)
			r.Post("/settings/google-indexing", adminH.UpdateGoogleIndexing)
			r.Post("/settings/google-indexing-test", adminH.TestGoogleIndexing)
			r.Post("/settings/google-auto-index", adminH.TriggerAutoIndex)
			r.Post("/settings/indexnow", adminH.TriggerIndexNow)
			r.Post("/settings/sync", adminH.TriggerSync)
			r.Post("/settings/link-check", adminH.TriggerLinkCheck)
			r.Post("/settings/cache-clear", adminH.ClearCache)
			r.Post("/settings/sitemap-rebuild", adminH.RebuildSitemap)
		})
	})

	// Custom 404
	r.NotFound(h.NotFound)

	// Start background workers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Drip-Feed Progressive Job Publishing Engine (Natural Human UK Cadence)
	dripFeedService := services.NewDripFeedService(db, sitemapService, indexNowService, cfg.BaseURL)
	go dripFeedService.StartWorker(ctx)

	// Reed API sync worker (every 4 hours)
	go syncService.StartScheduled(ctx, 4*time.Hour)

	// FreeHire API sync worker (every 6 hours)
	go freehireSyncService.StartScheduled(ctx, 6*time.Hour)

	// Smart Job Link Health & Expiry Bot (verifies active ATS URLs, auto-expires dead links, extends live links)
	go linkBot.Start(ctx, 15*time.Minute)

	// Job expiry checker (every 2 hours)
	go services.StartExpiryChecker(ctx, repos.Job, sitemapService, googleIndexingService, cfg.BaseURL, 2*time.Hour)

	// Sitemap regeneration (every 4 hours)
	go sitemapService.StartScheduled(ctx, 4*time.Hour)

	// Google Indexing API daily auto-indexer (fast-tracks up to 150 fresh jobs daily)
	go autoIndexer.StartScheduled(ctx, 24*time.Hour)

	// IndexNow automated submission (submits top fresh jobs to Bing & Yahoo every 12 hours)
	go indexNowService.StartScheduled(ctx, repos.Job, 12*time.Hour)

	// WhatsApp automated alert dispatcher (every 15 minutes)
	if cfg.EnableWhatsAppAlerts {
		go alertDispatcher.StartScheduled(ctx, 15*time.Minute)

		// WhatsApp Channel (Newsletter) automated job poster (max 20 jobs/day during peak UK hours)
		go alertDispatcher.StartChannelAutoPoster(ctx, 35*time.Minute)
	}

	// Configure HTTP server
	server := &http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.Port),
		Handler:           r,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
	}

	// Start server in goroutine
	go func() {
		slog.Info("Server starting", "port", cfg.Port, "base_url", cfg.BaseURL)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	sig := <-quit

	slog.Info("Shutting down server...", "signal", sig.String())

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	cancel() // Stop background workers

	slog.Info("Server stopped")
}
