package handlers

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"jobyuk/internal/config"
	"jobyuk/internal/middleware"
	"jobyuk/internal/models"
	"jobyuk/internal/repository"
	"jobyuk/internal/seo"
	"jobyuk/internal/services"
	"jobyuk/internal/settings"

	"github.com/go-chi/chi/v5"
)

// Handlers holds all HTTP handler dependencies.
type Handlers struct {
	renderer  *Renderer
	repos     *repository.Repositories
	search    *services.SearchService
	seo       *services.SEOService
	sitemap   *services.SitemapService
	taxonomy  *services.TaxonomyService
	cache     *services.CacheService
	analytics *services.AnalyticsService
	feed      *services.FeedService
	cfg       *config.Config
	schema    *seo.SchemaGenerator
	meta      *seo.MetaGenerator
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(
	renderer *Renderer,
	repos *repository.Repositories,
	search *services.SearchService,
	seoSvc *services.SEOService,
	sitemapSvc *services.SitemapService,
	taxonomy *services.TaxonomyService,
	cache *services.CacheService,
	analytics *services.AnalyticsService,
	feedSvc *services.FeedService,
	cfg *config.Config,
) *Handlers {
	return &Handlers{
		renderer:  renderer,
		repos:     repos,
		search:    search,
		seo:       seoSvc,
		sitemap:   sitemapSvc,
		taxonomy:  taxonomy,
		cache:     cache,
		analytics: analytics,
		feed:      feedSvc,
		cfg:       cfg,
		schema:    seo.NewSchemaGenerator(cfg.BaseURL),
		meta:      seo.NewMetaGenerator(cfg.BaseURL),
	}
}

// Home renders the homepage.
func (h *Handlers) Home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("q") != "" || r.URL.Query().Get("location") != "" || r.URL.Query().Get("l") != "" {
		http.Redirect(w, r, "/jobs/?"+r.URL.RawQuery, http.StatusMovedPermanently)
		return
	}
	ctx := r.Context()

	// Fetch latest jobs (with caching)
	var latestJobs []models.Job
	if err := h.cache.Get(ctx, "jobs:home:latest", &latestJobs); err != nil {
		var err2 error
		latestJobs, err2 = h.repos.Job.FindLatest(ctx, 12)
		if err2 == nil {
			_ = h.cache.Set(ctx, "jobs:home:latest", latestJobs, 15*time.Minute)
		} else {
			slog.Error("Failed to fetch latest jobs", "error", err2)
			latestJobs = []models.Job{}
		}
	}

	// Fetch top companies (with caching)
	var topCompanies []models.Company
	if err := h.cache.Get(ctx, "jobs:home:companies", &topCompanies); err != nil {
		var err2 error
		topCompanies, err2 = h.repos.Company.FindTop(ctx, 39)
		if err2 == nil {
			_ = h.cache.Set(ctx, "jobs:home:companies", topCompanies, 15*time.Minute)
		} else {
			slog.Error("Failed to fetch top companies", "error", err2)
		}
	}

	// Fetch top roles (limit 30 for homepage tab, with caching)
	var topRoles []models.JobRole
	if err := h.cache.Get(ctx, "jobs:home:roles", &topRoles); err != nil {
		var err2 error
		topRoles, err2 = h.repos.Role.FindTop(ctx, 30)
		if err2 == nil {
			_ = h.cache.Set(ctx, "jobs:home:roles", topRoles, 15*time.Minute)
		} else {
			slog.Error("Failed to fetch top roles", "error", err2)
		}
	}

	// Fetch industries for browse tab
	industries, err := h.repos.Industry.FindAll(ctx)
	if err != nil {
		slog.Error("Failed to fetch industries", "error", err)
	}

	// Fetch active locations for browse tab
	locations, err := h.repos.Location.FindAllActive(ctx)
	if err != nil {
		slog.Error("Failed to fetch locations", "error", err)
	}

	// Fetch job counts
	counts, err := h.repos.Job.CountByStatus(ctx)
	if err != nil {
		slog.Error("Failed to fetch job counts", "error", err)
		counts = map[string]int{}
	}

	// Generate SEO data
	pageMeta := h.meta.HomeMeta()
	schemaHTML := h.schema.WebSiteSchema() + h.schema.OrganizationSchema()

	isMobile := isMobileDevice(r)

	data := PageData{
		Title:       pageMeta.Title,
		Description: pageMeta.Description,
		Keywords:    pageMeta.Keywords,
		Canonical:   pageMeta.Canonical,
		OGTitle:     pageMeta.OGTitle,
		OGDesc:      pageMeta.OGDesc,
		OGImage:     pageMeta.OGImage,
		OGType:      pageMeta.OGType,
		Robots:      pageMeta.Robots,
		Schema:      schemaHTML,
		ActiveNav:   "home",
		IsMobile:    isMobile,
		Data: map[string]interface{}{
			"LatestJobs":   latestJobs,
			"TopCompanies": topCompanies,
			"TopRoles":     topRoles,
			"Industries":   industries,
			"Locations":    locations,
			"ActiveCount":  counts["active"],
			"TotalCount":   counts["active"] + counts["expired"],
		},
	}

	if err := h.renderPage(w, r, "home", data); err != nil {
		slog.Error("Failed to render home page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// isMobileDevice checks if the incoming HTTP request is from a mobile device user agent.
func isMobileDevice(r *http.Request) bool {
	// Query param override for easy testing (?view=mobile or ?view=desktop)
	viewParam := strings.ToLower(r.URL.Query().Get("view"))
	if viewParam == "mobile" {
		return true
	}
	if viewParam == "desktop" {
		return false
	}

	ua := strings.ToLower(r.Header.Get("User-Agent"))
	if ua == "" {
		return false
	}

	// Comprehensive keywords covering Android, iOS, Windows Mobile, Googlebot-Mobile, Bingbot-Mobile, etc.
	mobileKeywords := []string{
		"mobile", "android", "iphone", "ipad", "ipod", "blackberry",
		"iemobile", "opera mini", "opera mobi", "windows phone", "silk/", "kindle", "webos", "ucbrowser", "samsung",
	}
	for _, keyword := range mobileKeywords {
		if strings.Contains(ua, keyword) {
			return true
		}
	}
	return false
}

// normalizeCanonical ensures canonical URLs are absolute HTTPS URLs on the production domain jobyuk.com
func normalizeCanonical(rawURL string) string {
	if rawURL == "" {
		return "https://jobyuk.com/"
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		clean := strings.TrimPrefix(rawURL, "http://")
		clean = strings.TrimPrefix(clean, "https://")
		if idx := strings.Index(clean, "/"); idx != -1 {
			clean = clean[idx:]
		} else {
			clean = "/"
		}
		if !strings.HasSuffix(clean, "/") && !strings.Contains(clean, ".") {
			clean += "/"
		}
		return "https://jobyuk.com" + clean
	}
	p := u.Path
	if p == "" {
		p = "/"
	}
	if !strings.HasSuffix(p, "/") && !strings.Contains(p, ".") {
		p += "/"
	}
	return "https://jobyuk.com" + p
}

// renderPage automatically injects mobile device state, CSRF token, and SEO headers for all pages.
func (h *Handlers) renderPage(w http.ResponseWriter, r *http.Request, templateName string, data PageData) error {
	data.IsMobile = isMobileDevice(r)
	data.CSRFToken = middleware.GetCSRFToken(r)
	w.Header().Set("Vary", "Accept, User-Agent")

	// Enforce absolute https://jobyuk.com canonical domain
	if data.Canonical == "" {
		cleanPath := r.URL.Path
		if !strings.HasSuffix(cleanPath, "/") && !strings.Contains(cleanPath, ".") {
			cleanPath += "/"
		}
		data.Canonical = "https://jobyuk.com" + cleanPath
	} else {
		data.Canonical = normalizeCanonical(data.Canonical)
	}

	// Markdown Negotiation: deliver clean, formatting-stripped text directly to AI agents & LLMs
	if isMarkdownRequest(r) {
		md := h.renderMarkdown(templateName, data)
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Signal", "search=yes, ai-input=yes, ai-train=yes")
		tokens := len(md) / 4
		if tokens < 10 {
			tokens = 10
		}
		w.Header().Set("x-markdown-tokens", strconv.Itoa(tokens))
		_, err := w.Write([]byte(md))
		return err
	}

	// OG Image URL normalization: absolute https://jobyuk.com
	if data.OGImage == "" {
		data.OGImage = "https://jobyuk.com/static/images/og-image.jpg"
	} else if strings.HasPrefix(data.OGImage, "/") {
		data.OGImage = "https://jobyuk.com" + data.OGImage
	} else if strings.Contains(data.OGImage, "localhost") || strings.Contains(data.OGImage, "127.0.0.1") {
		u, err := url.Parse(data.OGImage)
		if err == nil {
			data.OGImage = "https://jobyuk.com" + u.Path
		}
	}

	// Automatically inject Google-compliant BreadcrumbList schema if page has breadcrumbs
	if len(data.Breadcrumbs) > 0 && !strings.Contains(string(data.Schema), "BreadcrumbList") {
		var items []seo.BreadcrumbItem
		for _, b := range data.Breadcrumbs {
			itemURL := b.URL
			if itemURL != "" {
				itemURL = normalizeCanonical(itemURL)
			}
			items = append(items, seo.BreadcrumbItem{
				Name: b.Name,
				URL:  itemURL,
			})
		}
		data.Schema += h.schema.BreadcrumbSchema(items)
	}

	return h.renderer.Render(w, templateName, data)
}

// JobList renders the job listing page with search and filters.
func (h *Handlers) JobList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	loc := r.URL.Query().Get("location")
	if loc == "" {
		loc = r.URL.Query().Get("l")
	}

	params := models.JobSearchParams{
		Query:           r.URL.Query().Get("q"),
		Location:        loc,
		ContractType:    r.URL.Query().Get("type"),
		JobType:         r.URL.Query().Get("job_type"),
		SortBy:          r.URL.Query().Get("sort"),
		EasyApply:       r.URL.Query().Get("easy_apply") == "true",
		ExperienceLevel: r.URL.Query().Get("experience"),
		DatePostedRange: r.URL.Query().Get("date_posted"),
		RemoteType:      r.URL.Query().Get("remote_type"),
		Page:            page,
		PerPage:         20,
	}

	if minSal := r.URL.Query().Get("min_salary"); minSal != "" {
		params.MinSalary, _ = strconv.ParseFloat(minSal, 64)
	}
	if maxSal := r.URL.Query().Get("max_salary"); maxSal != "" {
		params.MaxSalary, _ = strconv.ParseFloat(maxSal, 64)
	}

	results, err := h.search.Search(ctx, params)
	if err != nil {
		slog.Error("Search failed", "error", err)
		http.Error(w, "Search Error", http.StatusInternalServerError)
		return
	}

	// Determine SEO behavior
	var pageMeta seo.PageMeta
	hasFilters := params.Query != "" || params.Location != "" || params.ContractType != ""
	if hasFilters {
		pageMeta = h.meta.SearchFilterMeta(params.Query, params.Location, results.Total)
	} else {
		robots := "index,follow"
		if results == nil || results.Total <= 2 {
			robots = "noindex,follow"
		}
		pageMeta = seo.PageMeta{
			Title:       "Browse UK Job Vacancies 2026 | Updated Daily | JobyUK",
			Description: "Browse the latest UK job vacancies updated daily. Filter by role, city, salary and hours, then apply straight to the employer with no middleman.",
			Canonical:   h.cfg.BaseURL + "/jobs/",
			Robots:      robots,
			OGType:      "website",
		}
	}

	// Pagination SEO rule: page 2+ is noindex,follow and canonicalises to clean base URL
	if page > 1 {
		pageMeta.Robots = "noindex,follow"
		pageMeta.Canonical = h.cfg.BaseURL + "/jobs/"
	}

	customH1 := "Browse Live UK Job Vacancies"
	introText := "Explore verified UK jobs updated in real time directly from trusted British employers, NHS trusts, and fast-growing organizations. Apply direct with zero middleman recruitment markup."
	if hasFilters {
		if params.Query != "" && params.Location != "" {
			customH1 = fmt.Sprintf("%s Jobs in %s", params.Query, params.Location)
			introText = fmt.Sprintf("Browse verified %s jobs in %s. Compare pay upfront, see who is actively hiring, and apply directly without recruiter fees or middleman delays.", params.Query, params.Location)
		} else if params.Query != "" {
			customH1 = fmt.Sprintf("%s Jobs across the UK", params.Query)
			introText = fmt.Sprintf("Discover hiring-now %s opportunities in the United Kingdom. Benchmark market rates and apply directly through verified company applicant portals.", params.Query)
		} else if params.Location != "" {
			customH1 = fmt.Sprintf("Live Jobs in %s", params.Location)
			introText = fmt.Sprintf("Find active vacancies in %s across full-time, part-time, remote, and hybrid arrangements with transparent salary details and direct applications.", params.Location)
		}
	}

	data := PageData{
		Title:       pageMeta.Title,
		Description: pageMeta.Description,
		Keywords:    fmt.Sprintf("UK jobs %d, find vacancies UK, full time part time remote jobs UK, apply jobs United Kingdom", time.Now().Year()),
		Canonical:   pageMeta.Canonical,
		OGTitle:     pageMeta.Title,
		OGDesc:      pageMeta.Description,
		OGImage:     h.cfg.BaseURL + "/static/images/og-image.jpg",
		Robots:      pageMeta.Robots,
		OGType:      pageMeta.OGType,
		ActiveNav:   "jobs",
		Breadcrumbs: []BreadcrumbData{
			{Name: "Home", URL: "/"},
			{Name: "Jobs"},
		},
		Data: map[string]interface{}{
			"Results":   results,
			"Params":    params,
			"CustomH1":  customH1,
			"IntroText": introText,
		},
	}

	if err := h.renderPage(w, r, "job_list", data); err != nil {
		slog.Error("Failed to render job list", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// SearchSuggestionItem represents an instant autocomplete suggestion for job search.
type SearchSuggestionItem struct {
	Value    string `json:"value"`
	Label    string `json:"label"`
	Category string `json:"category"` // "Job Role", "Company", "Location", "Workplace"
	Count    int    `json:"count,omitempty"`
	Icon     string `json:"icon"`     // "role", "company", "location", "remote"
}

// SearchSuggestions returns instant JSON autocomplete suggestions for jobs or locations.
func (h *Handlers) SearchSuggestions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	suggestType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	qLower := strings.ToLower(query)

	var suggestions []SearchSuggestionItem

	if suggestType == "location" {
		// 1. Remote suggestions
		if qLower == "" || strings.Contains("remote work from home hybrid", qLower) {
			suggestions = append(suggestions, SearchSuggestionItem{
				Value:    "Remote",
				Label:    "Remote (Work From Home UK)",
				Category: "Workplace",
				Icon:     "remote",
			})
		}

		// 2. Database Locations
		var locations []models.Location
		if err := h.cache.Get(ctx, "locations:active:all", &locations); err != nil {
			var err2 error
			locations, err2 = h.repos.Location.FindAllActive(ctx)
			if err2 == nil {
				_ = h.cache.Set(ctx, "locations:active:all", locations, 1*time.Hour)
			}
		}

		topCities := []string{"London", "Manchester", "Birmingham", "Leeds", "Glasgow", "Bristol", "Edinburgh", "Liverpool", "Sheffield", "Newcastle", "Nottingham", "Cardiff", "Belfast", "Cambridge", "Oxford", "Southampton"}
		if qLower == "" {
			for _, city := range topCities[:7] {
				suggestions = append(suggestions, SearchSuggestionItem{
					Value:    city,
					Label:    city + ", UK",
					Category: "City",
					Icon:     "location",
				})
			}
		} else {
			for _, loc := range locations {
				locLower := strings.ToLower(loc.Name)
				if strings.Contains(locLower, qLower) {
					suggestions = append(suggestions, SearchSuggestionItem{
						Value:    loc.Name,
						Label:    loc.Name + ", UK",
						Category: "City",
						Count:    loc.JobCount,
						Icon:     "location",
					})
					if len(suggestions) >= 8 {
						break
					}
				}
			}
			if len(suggestions) < 5 {
				for _, city := range topCities {
					if strings.Contains(strings.ToLower(city), qLower) {
						alreadyAdded := false
						for _, s := range suggestions {
							if strings.EqualFold(s.Value, city) {
								alreadyAdded = true
								break
							}
						}
						if !alreadyAdded {
							suggestions = append(suggestions, SearchSuggestionItem{
								Value:    city,
								Label:    city + ", UK",
								Category: "City",
								Icon:     "location",
							})
							if len(suggestions) >= 8 {
								break
							}
						}
					}
				}
			}
		}
	} else {
		// Job / Role suggestions
		var roles []models.JobRole
		if err := h.cache.Get(ctx, "roles:all", &roles); err != nil {
			var err2 error
			roles, err2 = h.repos.Role.FindAll(ctx)
			if err2 == nil {
				_ = h.cache.Set(ctx, "roles:all", roles, 1*time.Hour)
			}
		}

		popularRoles := []string{
			"Software Developer", "UI/UX Designer", "Product Manager", "Project Manager",
			"Data Analyst", "Accountant", "Registered Nurse", "Healthcare Assistant",
			"Customer Service Advisor", "Receptionist", "Sales Executive", "Marketing Manager",
			"Warehouse Operative", "DevOps Engineer", "Civil Engineer", "Chef",
		}

		if qLower == "" {
			for _, role := range popularRoles[:8] {
				suggestions = append(suggestions, SearchSuggestionItem{
					Value:    role,
					Label:    role,
					Category: "Popular Role",
					Icon:     "role",
				})
			}
		} else {
			// First match DB roles
			for _, role := range roles {
				if strings.Contains(strings.ToLower(role.Name), qLower) {
					suggestions = append(suggestions, SearchSuggestionItem{
						Value:    role.Name,
						Label:    role.Name,
						Category: "Job Role",
						Count:    role.JobCount,
						Icon:     "role",
					})
					if len(suggestions) >= 6 {
						break
					}
				}
			}
			// Match curated common roles
			for _, role := range popularRoles {
				if strings.Contains(strings.ToLower(role), qLower) {
					alreadyAdded := false
					for _, s := range suggestions {
						if strings.EqualFold(s.Value, role) {
							alreadyAdded = true
							break
						}
					}
					if !alreadyAdded {
						suggestions = append(suggestions, SearchSuggestionItem{
							Value:    role,
							Label:    role,
							Category: "Job Role",
							Icon:     "role",
						})
						if len(suggestions) >= 8 {
							break
						}
					}
				}
			}
			// Match top companies
			var companies []models.Company
			if h.cache.Get(ctx, "companies:all_active", &companies) == nil {
				for _, c := range companies {
					if strings.Contains(strings.ToLower(c.Name), qLower) {
						suggestions = append(suggestions, SearchSuggestionItem{
							Value:    c.Name,
							Label:    c.Name,
							Category: "Employer",
							Count:    c.JobCount,
							Icon:     "company",
						})
						if len(suggestions) >= 8 {
							break
						}
					}
				}
			}
		}
	}

	if len(suggestions) > 8 {
		suggestions = suggestions[:8]
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=60")
	_ = json.NewEncoder(w).Encode(suggestions)
}

// JobDetail renders an individual job page.
func (h *Handlers) JobDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	slug := chi.URLParam(r, "slug")

	// 1. If slug is purely a numeric ID (e.g. /jobs/46636/ or /jobs/45327/), lookup by ID and 301 Redirect to canonical slug URL
	if id, parseErr := strconv.ParseInt(slug, 10, 64); parseErr == nil {
		jobByID, errID := h.repos.Job.FindByID(ctx, id)
		if errID == nil && jobByID != nil {
			if jobByID.Status == "active" {
				http.Redirect(w, r, fmt.Sprintf("/jobs/%s/", jobByID.Slug), http.StatusMovedPermanently)
				return
			}
			// If expired or removed, render 410 Gone page with similar jobs
			similarJobs, _ := h.repos.Job.FindSimilar(ctx, jobByID.ID, 5)
			data := PageData{
				Title:     fmt.Sprintf("%s (Expired) | JobyUK.com", jobByID.Title),
				Robots:    "noindex,follow",
				ActiveNav: "jobs",
				Breadcrumbs: []BreadcrumbData{
					{Name: "Home", URL: "/"},
					{Name: "Jobs", URL: "/jobs/"},
					{Name: jobByID.Title + " (Expired)"},
				},
				Data: map[string]interface{}{
					"Job":         jobByID,
					"SimilarJobs": similarJobs,
					"IsExpired":   true,
					"IsGone":      true,
				},
			}
			if err := h.renderer.RenderWithStatus(w, "job_detail", http.StatusGone, data); err != nil {
				w.WriteHeader(http.StatusGone)
			}
			return
		}
		// If numeric ID is not found in database, 301 redirect to /jobs/ list to prevent 404
		http.Redirect(w, r, "/jobs/", http.StatusMovedPermanently)
		return
	}

	job, err := h.repos.Job.FindBySlug(ctx, slug)
	if err != nil {
		slog.Warn("JobDetail: FindBySlug not found", "slug", slug, "error", err)
		// 2. Slug not found — render 410 Gone with helpful active vacancies rather than 404 error
		similarJobs, _ := h.repos.Job.FindLatest(ctx, 6)
		cleanTitle := strings.ReplaceAll(slug, "-", " ")
		data := PageData{
			Title:     fmt.Sprintf("%s (Expired) | JobyUK.com", strings.Title(cleanTitle)),
			Robots:    "noindex,follow",
			ActiveNav: "jobs",
			Breadcrumbs: []BreadcrumbData{
				{Name: "Home", URL: "/"},
				{Name: "Jobs", URL: "/jobs/"},
				{Name: "Expired Vacancy"},
			},
			Data: map[string]interface{}{
				"Job": &models.Job{
					Title: strings.Title(cleanTitle),
					Slug:  slug,
				},
				"SimilarJobs": similarJobs,
				"IsExpired":   true,
				"IsGone":      true,
			},
		}
		if err := h.renderer.RenderWithStatus(w, "job_detail", http.StatusGone, data); err != nil {
			w.WriteHeader(http.StatusGone)
		}
		return
	}

	// Soft-deleted jobs by admin should return 410 Gone (fast Googlebot de-indexing)
	if job.Status == "removed" {
		similarJobs, _ := h.repos.Job.FindSimilar(ctx, job.ID, 5)
		data := PageData{
			Title:     fmt.Sprintf("%s (Expired) | JobyUK.com", job.Title),
			Robots:    "noindex,follow",
			ActiveNav: "jobs",
			Breadcrumbs: []BreadcrumbData{
				{Name: "Home", URL: "/"},
				{Name: "Jobs", URL: "/jobs/"},
				{Name: job.Title + " (Expired)"},
			},
			Data: map[string]interface{}{
				"Job":         job,
				"SimilarJobs": similarJobs,
				"IsExpired":   true,
				"IsGone":      true,
			},
		}
		if err := h.renderer.RenderWithStatus(w, "job_detail", http.StatusGone, data); err != nil {
			w.WriteHeader(http.StatusGone)
		}
		return
	}

	// Handle expired jobs
	if job.Status == "expired" && !job.IsRecentlyExpired() {
		// Job expired > 30 days ago — return 410 Gone with helpful page body
		similarJobs, _ := h.repos.Job.FindSimilar(ctx, job.ID, 5)
		data := PageData{
			Title:     fmt.Sprintf("%s (Expired) | JobyUK.com", job.Title),
			Robots:    "noindex,nofollow",
			ActiveNav: "jobs",
			Breadcrumbs: []BreadcrumbData{
				{Name: "Home", URL: "/"},
				{Name: "Jobs", URL: "/jobs/"},
				{Name: job.Title + " (Expired)"},
			},
			Data: map[string]interface{}{
				"Job":         job,
				"SimilarJobs": similarJobs,
				"IsExpired":   true,
				"IsGone":      true,
			},
		}
		if err := h.renderer.RenderWithStatus(w, "job_detail", http.StatusGone, data); err != nil {
			w.WriteHeader(http.StatusGone)
		}
		return
	}

	// Fetch similar jobs (for sidebar)
	similarJobs, _ := h.repos.Job.FindSimilar(ctx, job.ID, 6)

	// Fetch popular jobs (for bottom slider)
	popularRes, _ := h.repos.Job.Search(ctx, models.JobSearchParams{
		Page:    1,
		PerPage: 8,
		SortBy:  "relevance",
	})
	var popularJobs []models.Job
	if popularRes != nil {
		popularJobs = popularRes.Jobs
	}

	// Generate SEO
	if job.IsExpired() {
		w.Header().Set("X-Robots-Tag", "noindex, follow")
	}
	pageMeta := h.meta.JobMeta(job)
	schemaHTML := h.schema.JobPostingSchema(job)
	breadcrumbItems := h.schema.JobBreadcrumbs(job)
	schemaHTML += h.schema.BreadcrumbSchema(breadcrumbItems)

	// On-page FAQs accordion
	faqs := seo.GenerateJobFAQs(job)

	// Rate limiting report count by IP
	ip := getClientIP(r)
	reportCount, _ := h.repos.Job.CountReportsByIP(ctx, job.ID, ip)
	isBlacklisted, _ := h.repos.Job.IsIPBlacklisted(ctx, ip)

	// Build on-page breadcrumbs matching schema hierarchy
	crumbs := []BreadcrumbData{
		{Name: "Home", URL: "/"},
		{Name: "Jobs", URL: "/jobs/"},
	}
	if locName, param := job.CleanPrimaryLocation(); locName != "" && param != "" {
		crumbs = append(crumbs, BreadcrumbData{
			Name: locName + " Jobs",
			URL:  "/jobs/?" + param,
		})
	}
	crumbs = append(crumbs, BreadcrumbData{
		Name: job.Title,
	})

	data := PageData{
		Title:       pageMeta.Title,
		Description: pageMeta.Description,
		Keywords:    pageMeta.Keywords,
		Canonical:   pageMeta.Canonical,
		OGTitle:     pageMeta.OGTitle,
		OGDesc:      pageMeta.OGDesc,
		OGImage:     pageMeta.OGImage,
		Robots:      pageMeta.Robots,
		OGType:      pageMeta.OGType,
		Schema:      schemaHTML,
		ActiveNav:   "jobs",
		Breadcrumbs: crumbs,
		Data: map[string]interface{}{
			"Job":             job,
			"SimilarJobs":     similarJobs,
			"PopularJobs":     popularJobs,
			"FAQs":            faqs,
			"IsExpired":       job.IsExpired(),
			"AlreadyReported": reportCount >= 1,
			"IsBlacklisted":   isBlacklisted,
		},
	}

	if err := h.renderPage(w, r, "job_detail", data); err != nil {
		slog.Error("Failed to render job detail", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// JobOGImage generates a dynamic 1200x630 branded SVG social card for Twitter, LinkedIn, and WhatsApp.
func (h *Handlers) JobOGImage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	slug := chi.URLParam(r, "slug")

	job, err := h.repos.Job.FindBySlug(ctx, slug)
	if err != nil || job == nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")

	title := html.EscapeString(truncateOGString(job.Title, 45))
	company := html.EscapeString(truncateOGString(job.EmployerName, 30))
	if company == "" {
		company = "Verified UK Employer"
	}
	location := html.EscapeString(job.DisplayLocation())
	if location == "" {
		location = "United Kingdom"
	}
	salary := html.EscapeString(job.FormattedSalary())
	if salary == "" {
		salary = "Competitive UK Package"
	}

	workMode := "On-site / Hybrid"
	if job.IsRemote {
		workMode = "Remote Friendly UK"
	}

	svg := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<svg width="1200" height="630" viewBox="0 0 1200 630" fill="none" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <linearGradient id="bg" x1="0" y1="0" x2="1200" y2="630" gradientUnits="userSpaceOnUse">
      <stop offset="0%%" stop-color="#051B14"/>
      <stop offset="50%%" stop-color="#0B2E24"/>
      <stop offset="100%%" stop-color="#0F5132"/>
    </linearGradient>
    <linearGradient id="cardGrad" x1="0" y1="0" x2="0" y2="520" gradientUnits="userSpaceOnUse">
      <stop offset="0%%" stop-color="#FFFFFF"/>
      <stop offset="100%%" stop-color="#F8FAF8"/>
    </linearGradient>
  </defs>

  <!-- Background -->
  <rect width="1200" height="630" fill="url(#bg)"/>
  
  <!-- Ambient circles -->
  <circle cx="1100" cy="80" r="280" fill="#22C55E" fill-opacity="0.08"/>
  <circle cx="100" cy="550" r="220" fill="#00D4AA" fill-opacity="0.06"/>

  <!-- Main Card Container -->
  <rect x="50" y="45" width="1100" height="540" rx="28" fill="url(#cardGrad)" stroke="#E2E8F0" stroke-width="2"/>

  <!-- Top Brand Row -->
  <g transform="translate(100, 95)">
    <rect width="180" height="42" rx="12" fill="#0F5132"/>
    <text x="90" y="27" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif" font-size="20" font-weight="800" fill="#FFFFFF" text-anchor="middle" letter-spacing="-0.5">JOBYUK.COM</text>
    
    <rect x="200" y="0" width="170" height="42" rx="12" fill="#EAF4EE" stroke="#A7F3D0" stroke-width="1.5"/>
    <text x="285" y="26" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif" font-size="14" font-weight="700" fill="#0F5132" text-anchor="middle">✓ VERIFIED VACANCY</text>
  </g>

  <!-- Company Name -->
  <text x="100" y="200" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif" font-size="24" font-weight="700" fill="#64748B" letter-spacing="-0.2">%s</text>

  <!-- Job Title -->
  <text x="100" y="270" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif" font-size="44" font-weight="850" fill="#0B1F1A" letter-spacing="-1">%s</text>

  <!-- Metadata Badges Row -->
  <g transform="translate(100, 325)">
    <rect width="260" height="48" rx="14" fill="#0F5132"/>
    <text x="130" y="31" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif" font-size="19" font-weight="800" fill="#FFFFFF" text-anchor="middle">%s</text>

    <rect x="280" y="0" width="220" height="48" rx="14" fill="#F1F5F9" stroke="#E2E8F0" stroke-width="1.5"/>
    <text x="390" y="30" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif" font-size="17" font-weight="650" fill="#334155" text-anchor="middle">📍 %s</text>

    <rect x="520" y="0" width="230" height="48" rx="14" fill="#ECFDF5" stroke="#A7F3D0" stroke-width="1.5"/>
    <text x="635" y="30" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif" font-size="17" font-weight="700" fill="#065F46" text-anchor="middle">⚡ %s</text>
  </g>

  <!-- Bottom CTA Row -->
  <line x1="100" y1="440" x2="1050" y2="440" stroke="#EEF2F6" stroke-width="2"/>
  
  <g transform="translate(100, 485)">
    <text x="0" y="24" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif" font-size="18" font-weight="600" fill="#64748B">Direct Employer Application • Zero Sign-Up Required</text>
    
    <rect x="690" y="-8" width="260" height="48" rx="12" fill="#0F5132"/>
    <text x="820" y="23" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif" font-size="16" font-weight="800" fill="#FFFFFF" text-anchor="middle">Apply Now on JobyUK →</text>
  </g>
</svg>`, company, title, salary, location, workMode)

	w.Write([]byte(svg))
}

func truncateOGString(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}

// ReportJob handles user submission of a job report.
func (h *Handlers) ReportJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	jobIDStr := chi.URLParam(r, "id")
	jobID, err := strconv.ParseInt(jobIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid Job ID", http.StatusBadRequest)
		return
	}

	_ = r.ParseMultipartForm(10 << 20)
	_ = r.ParseForm()

	reason := strings.TrimSpace(r.FormValue("reason"))
	if reason == "" && r.MultipartForm != nil && len(r.MultipartForm.Value["reason"]) > 0 {
		reason = strings.TrimSpace(r.MultipartForm.Value["reason"][0])
	}
	details := strings.TrimSpace(r.FormValue("details"))
	if details == "" && r.MultipartForm != nil && len(r.MultipartForm.Value["details"]) > 0 {
		details = strings.TrimSpace(r.MultipartForm.Value["details"][0])
	}

	if reason == "" {
		http.Error(w, "Reason is required", http.StatusBadRequest)
		return
	}

	ip := getClientIP(r)

	// Block if IP is blacklisted
	isBlacklisted, _ := h.repos.Job.IsIPBlacklisted(ctx, ip)
	if isBlacklisted {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("You have exceeded the report limit. Sorry, you cannot report any more jobs."))
		return
	}
	
	// Enforce 1 report limit per job per IP address
	count, err := h.repos.Job.CountReportsByIP(ctx, jobID, ip)
	if err == nil && count >= 1 {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("Sorry, you have already reported this job."))
		return
	}

	// Enforce 5 reports total limit per IP address
	totalCount, err := h.repos.Job.CountTotalReportsByIP(ctx, ip)
	if err == nil && totalCount >= 5 {
		_ = h.repos.Job.BlacklistIP(ctx, ip, "Exceeded maximum report limit (5)")
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("You have exceeded the report limit. Sorry, you cannot report any more jobs."))
		return
	}

	report := &models.JobReport{
		JobID:     jobID,
		Reason:    reason,
		Details:   details,
		IPAddress: ip,
	}

	if err := h.repos.Job.Report(ctx, report); err != nil {
		slog.Error("Failed to save job report", "error", err)
		http.Error(w, "Failed to submit report", http.StatusInternalServerError)
		return
	}

	// Recheck total count to auto-blacklist if they just hit the 5th report
	if newTotal, err := h.repos.Job.CountTotalReportsByIP(ctx, ip); err == nil && newTotal >= 5 {
		_ = h.repos.Job.BlacklistIP(ctx, ip, "Exceeded maximum report limit (5)")
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Thank you for your report. Our moderators will review this listing."))
}

// RoleIndex renders the job roles index page.
func (h *Handlers) RoleIndex(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	letter := strings.TrimSpace(r.URL.Query().Get("letter"))
	pageStr := r.URL.Query().Get("page")

	page := 1
	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}

	limit := 36
	offset := (page - 1) * limit

	var roles []models.JobRole
	var total int
	type roleIndexCacheData struct {
		Roles []models.JobRole
		Total int
	}
	cacheKey := fmt.Sprintf("roles:filtered:q=%s:l=%s:p=%d", q, letter, page)
	var cachedData roleIndexCacheData
	if err := h.cache.Get(ctx, cacheKey, &cachedData); err == nil {
		roles = cachedData.Roles
		total = cachedData.Total
	} else {
		var err2 error
		roles, total, err2 = h.repos.Role.FindAllFiltered(ctx, q, letter, limit, offset)
		if err2 == nil {
			_ = h.cache.Set(ctx, cacheKey, roleIndexCacheData{Roles: roles, Total: total}, 15*time.Minute)
		} else {
			slog.Error("Failed to fetch roles", "error", err2)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	alphabet := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z"}

	// Noindex paginated and filtered pages to prevent duplicate content indexing
	robots := "index,follow"
	if page > 1 || q != "" || letter != "" {
		robots = "noindex,follow"
	}

	data := PageData{
		Title:       "UK Job Roles A–Z 2026: Pay & Openings by Title | JobyUK",
		Description: "Explore UK job titles from A to Z. See typical salaries, who is hiring and live vacancies for each role, then apply direct to the employer.",
		Keywords:    fmt.Sprintf("UK job roles %d, UK job titles directory, tech finance healthcare jobs UK, apply direct UK jobs", time.Now().Year()),
		Canonical:   h.cfg.BaseURL + "/job-roles/",
		OGTitle:     "Find UK Jobs by Role | JobyUK",
		OGDesc:      "Browse hundreds of UK job titles. Live employer vacancies, salary benchmarks, and direct ATS application links on JobyUK.",
		OGImage:     h.cfg.BaseURL + "/static/images/og-image.jpg",
		Robots:      robots,
		ActiveNav:   "roles",
		Breadcrumbs: []BreadcrumbData{
			{Name: "Home", URL: "/"},
			{Name: "Job Roles"},
		},
		Data: map[string]interface{}{
			"Roles":          roles,
			"TotalCount":     total,
			"Page":           page,
			"TotalPages":     totalPages,
			"Query":          q,
			"SelectedLetter": letter,
			"Alphabet":       alphabet,
		},
	}

	if err := h.renderPage(w, r, "role", data); err != nil {
		slog.Error("Failed to render role index", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// RoleDetail renders a specific role page.
func (h *Handlers) RoleDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	slug := chi.URLParam(r, "slug")

	role, err := h.repos.Role.FindBySlug(ctx, slug)
	if err != nil {
		roleName := strings.Title(strings.ReplaceAll(slug, "-", " "))
		role = &models.JobRole{
			Slug:           slug,
			Name:           roleName,
			SEOTitle:       fmt.Sprintf("%s Jobs UK 2026", roleName),
			SEODescription: fmt.Sprintf("Browse %s vacancies in the UK. Find hiring companies, salaries and apply today.", roleName),
		}
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	jobs, total, err := h.repos.Job.FindActiveByRole(ctx, role.ID, role.Name, 20, (page-1)*20)
	if err != nil {
		slog.Error("Failed to fetch role jobs", "error", err)
	}

	pageMeta := h.meta.RoleMeta(role, total)

	results := &models.JobSearchResult{
		Jobs:       jobs,
		Total:      total,
		Page:       page,
		PerPage:    20,
		TotalPages: (total + 19) / 20,
	}

	params := models.JobSearchParams{
		Query:           role.Name,
		Location:        r.URL.Query().Get("location"),
		ContractType:    r.URL.Query().Get("type"),
		JobType:         r.URL.Query().Get("job_type"),
		SortBy:          r.URL.Query().Get("sort"),
		EasyApply:       r.URL.Query().Get("easy_apply") == "true",
		ExperienceLevel: r.URL.Query().Get("experience"),
		DatePostedRange: r.URL.Query().Get("date_posted"),
		RemoteType:      r.URL.Query().Get("remote_type"),
		Page:            page,
		PerPage:         20,
	}

	data := PageData{
		Title:       pageMeta.Title,
		Description: pageMeta.Description,
		Keywords:    pageMeta.Keywords,
		Canonical:   pageMeta.Canonical,
		OGTitle:     pageMeta.Title,
		OGDesc:      pageMeta.Description,
		OGImage:     h.cfg.BaseURL + "/static/images/og-image.jpg",
		Robots:      pageMeta.Robots,
		ActiveNav:   "roles",
		Breadcrumbs: []BreadcrumbData{
			{Name: "Home", URL: "/"},
			{Name: "Job Roles", URL: "/job-roles/"},
			{Name: role.Name + " Jobs"},
		},
		Data: map[string]interface{}{
			"Role":      role,
			"Results":   results,
			"Params":    params,
			"IntroText": fmt.Sprintf("Explore verified %s jobs across England, Scotland, Wales, and Northern Ireland. JobyUK connects you straight to hiring employers, NHS trusts, and innovative organizations with upfront salary disclosure and direct ATS application links. Skip the recruitment agencies and middleman fees, benchmark typical UK market compensation for %s professionals, and submit your direct job application today.", role.Name, role.Name),
		},
	}

	if err := h.renderPage(w, r, "role", data); err != nil {
		slog.Error("Failed to render role detail", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Stub handlers for remaining routes
// RoleLocation renders jobs matching both a specific role and location.
func (h *Handlers) RoleLocation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	roleSlug := chi.URLParam(r, "roleSlug")
	locationSlug := chi.URLParam(r, "locationSlug")

	role, err := h.repos.Role.FindBySlug(ctx, roleSlug)
	if err != nil {
		roleName := strings.Title(strings.ReplaceAll(roleSlug, "-", " "))
		role = &models.JobRole{
			Slug: roleSlug,
			Name: roleName,
		}
	}

	location, err := h.repos.Location.FindBySlug(ctx, locationSlug)
	if err != nil {
		locName := strings.Title(strings.ReplaceAll(locationSlug, "-", " "))
		location = &models.Location{
			Slug:  locationSlug,
			Name:  locName,
			Level: "city",
		}
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	params := models.JobSearchParams{
		RoleSlug:     role.Slug,
		LocationSlug: location.Slug,
		Page:         page,
		PerPage:      20,
	}

	results, err := h.search.Search(ctx, params)
	if err != nil {
		slog.Error("Role location search failed", "error", err)
	}

	// High-CTR: {Role} Jobs in {Location} (Apply Direct) 2026 | JobyUK
	title := fmt.Sprintf("%s Jobs in %s (Apply Direct) 2026 | JobyUK", role.Name, location.Name)
	if len([]rune(title)) > 60 {
		title = fmt.Sprintf("%s Jobs in %s (Apply Direct) | JobyUK", role.Name, location.Name)
	}
	if len([]rune(title)) > 60 {
		title = fmt.Sprintf("%s Jobs in %s 2026 | JobyUK", role.Name, location.Name)
	}
	if len([]rune(title)) > 60 {
		title = truncate(title, 60)
	}

	desc := fmt.Sprintf("Apply direct for verified %s jobs in %s. Compare live salaries, see hiring employers, and skip recruitment agencies – updated daily on JobyUK.", role.Name, location.Name)
	if len([]rune(desc)) > 150 {
		desc = truncate(desc, 150)
	}
	keywords := fmt.Sprintf("%s jobs %s 2026, %s vacancies in %s, %s salaries %s, %s careers in %s UK", role.Name, location.Name, role.Name, location.Name, role.Name, location.Name, role.Name, location.Name)

	robots := "index,follow"
	if page > 1 || results == nil || results.Total <= 2 {
		robots = "noindex,follow"
	}

	data := PageData{
		Title:       title,
		Description: desc,
		Keywords:    keywords,
		Canonical:   fmt.Sprintf("%s/job-roles/%s/%s/", h.cfg.BaseURL, role.Slug, location.Slug),
		OGTitle:     title,
		OGDesc:      desc,
		OGImage:     h.cfg.BaseURL + "/static/images/og-image.jpg",
		Robots:      robots,
		ActiveNav:   "roles",
		Breadcrumbs: []BreadcrumbData{
			{Name: "Home", URL: "/"},
			{Name: "Job Roles", URL: "/job-roles/"},
			{Name: role.Name + " Jobs", URL: fmt.Sprintf("/job-roles/%s/", role.Slug)},
			{Name: location.Name},
		},
		Data: map[string]interface{}{
			"Results":   results,
			"Params":    params,
			"CustomH1":  fmt.Sprintf("%s Jobs in %s", role.Name, location.Name),
			"IntroText": fmt.Sprintf("Discover hiring-now %s jobs in %s with upfront pay and direct employer application links. Skip recruitment agency delays and middleman commissions by applying straight to verified %s employers in %s. Benchmark local compensation, review commute requirements, and take the next step in your %s career in %s today.", role.Name, location.Name, role.Name, location.Name, role.Name, location.Name),
		},
	}

	if err := h.renderPage(w, r, "job_list", data); err != nil {
		slog.Error("Failed to render role location list", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// IndustryIndex renders the list of industry categories.
func (h *Handlers) IndustryIndex(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var industries []models.Industry
	if err := h.cache.Get(ctx, "industries:all", &industries); err != nil {
		var err2 error
		industries, err2 = h.repos.Industry.FindAll(ctx)
		if err2 == nil {
			_ = h.cache.Set(ctx, "industries:all", industries, 30*time.Minute)
		} else {
			slog.Error("Failed to fetch industries", "error", err2)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	data := PageData{
		Title:       "UK Jobs by Industry 2026: Browse Every Sector | JobyUK",
		Description: "Browse UK jobs by industry, from healthcare and retail to tech, finance and construction. Compare sector pay and apply direct to employers.",
		Canonical:   h.cfg.BaseURL + "/industries/",
		Robots:      "index,follow",
		ActiveNav:   "jobs",
		Breadcrumbs: []BreadcrumbData{
			{Name: "Home", URL: "/"},
			{Name: "Industries"},
		},
		Data: map[string]interface{}{
			"Industries": industries,
		},
	}

	if err := h.renderPage(w, r, "industry", data); err != nil {
		slog.Error("Failed to render industry index", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// IndustryDetail renders jobs in a specific industry.
func (h *Handlers) IndustryDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	slug := chi.URLParam(r, "slug")

	industry, err := h.repos.Industry.FindBySlug(ctx, slug)
	if err != nil {
		indName := strings.Title(strings.ReplaceAll(slug, "-", " "))
		industry = &models.Industry{
			Slug: slug,
			Name: indName,
		}
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	params := models.JobSearchParams{
		IndustrySlug: industry.Slug,
		Page:         page,
		PerPage:      20,
	}

	results, err := h.search.Search(ctx, params)
	if err != nil {
		slog.Error("Industry detail search failed", "error", err)
	}

	// High-CTR: {Industry} Jobs UK 2026 (Apply Direct) | JobyUK
	title := fmt.Sprintf("%s Jobs UK 2026 (Apply Direct) | JobyUK", industry.Name)
	if len([]rune(title)) > 60 {
		title = fmt.Sprintf("%s Jobs UK 2026 | JobyUK", industry.Name)
	}
	if len([]rune(title)) > 60 {
		title = truncate(title, 60)
	}

	desc := fmt.Sprintf("Find verified %s jobs across the UK with upfront pay disclosure. See hiring companies and apply straight to the employer without recruiter middlemen.", industry.Name)
	if len([]rune(desc)) > 150 {
		desc = truncate(desc, 150)
	}

	// Noindex paginated industry pages or thin categories
	robots := "index,follow"
	if page > 1 || results == nil || results.Total <= 2 {
		robots = "noindex,follow"
	}

	data := PageData{
		Title:       title,
		Description: desc,
		Canonical:   fmt.Sprintf("%s/industries/%s/", h.cfg.BaseURL, industry.Slug),
		Robots:      robots,
		ActiveNav:   "jobs",
		Breadcrumbs: []BreadcrumbData{
			{Name: "Home", URL: "/"},
			{Name: "Industries", URL: "/industries/"},
			{Name: industry.Name},
		},
		Data: map[string]interface{}{
			"Industry":  industry,
			"Results":   results,
			"Params":    params,
			"IntroText": fmt.Sprintf("Browse verified %s vacancies across the United Kingdom. From entry-level trainee schemes to senior executive leadership roles, JobyUK displays live jobs from verified UK employers with upfront pay and direct application links. Bypass recruitment agencies and explore active hiring across the %s sector in London, Manchester, Birmingham, and throughout the UK.", industry.Name, industry.Name),
		},
	}

	if err := h.renderPage(w, r, "industry", data); err != nil {
		slog.Error("Failed to render industry detail", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// LocationIndex renders the list of hiring locations.
func (h *Handlers) LocationIndex(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var locations []models.Location
	if err := h.cache.Get(ctx, "locations:active:all", &locations); err != nil {
		var err2 error
		locations, err2 = h.repos.Location.FindAllActive(ctx)
		if err2 == nil {
			_ = h.cache.Set(ctx, "locations:active:all", locations, 30*time.Minute)
		} else {
			slog.Error("Failed to fetch locations", "error", err2)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	data := PageData{
		Title:       "Jobs by UK City & Region 2026 | Near You | JobyUK",
		Description: "Find jobs near you in London, Manchester, Birmingham, Leeds and more. Browse local vacancies with pay shown and apply direct to employers.",
		Keywords:    fmt.Sprintf("jobs by UK city %d, London jobs, Manchester vacancies, Birmingham careers, Scotland Wales Northern Ireland employment", time.Now().Year()),
		Canonical:   h.cfg.BaseURL + "/locations/",
		Robots:      "index,follow",
		ActiveNav:   "locations",
		Breadcrumbs: []BreadcrumbData{
			{Name: "Home", URL: "/"},
			{Name: "Locations"},
		},
		Data: map[string]interface{}{
			"Locations": locations,
		},
	}

	if err := h.renderPage(w, r, "location", data); err != nil {
		slog.Error("Failed to render location index", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// LocationDetail renders jobs in a specific location.
func (h *Handlers) LocationDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		slug = r.URL.Query().Get("slug")
	}

	location, err := h.repos.Location.FindBySlug(ctx, slug)
	if err != nil {
		locName := strings.Title(strings.ReplaceAll(slug, "-", " "))
		location = &models.Location{
			Slug:  slug,
			Name:  locName,
			Level: "city",
		}
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	params := models.JobSearchParams{
		LocationSlug: location.Slug,
		Page:         page,
		PerPage:      20,
	}

	results, err := h.search.Search(ctx, params)
	if err != nil {
		slog.Error("Location detail search failed", "error", err)
	}

	// High-CTR: Jobs in {Location} 2026 (Apply Direct) | JobyUK
	title := fmt.Sprintf("Jobs in %s 2026 (Apply Direct) | JobyUK", location.Name)
	if len([]rune(title)) > 60 {
		title = fmt.Sprintf("Jobs in %s 2026 | JobyUK", location.Name)
	}
	if len([]rune(title)) > 60 {
		title = truncate(title, 60)
	}

	desc := fmt.Sprintf("Browse verified jobs in %s: full-time, part-time, remote and hybrid. Local salaries shown, direct employer links, no agency spam.", location.Name)
	if len([]rune(desc)) > 150 {
		desc = truncate(desc, 150)
	}

	// Noindex paginated location pages or thin cities
	robots := "index,follow"
	if page > 1 || results == nil || results.Total <= 2 {
		robots = "noindex,follow"
	}

	data := PageData{
		Title:       title,
		Description: desc,
		Keywords:    fmt.Sprintf("jobs in %s %d, %s vacancies, work in %s, %s employment UK", location.Name, time.Now().Year(), location.Name, location.Name, location.Name),
		Canonical:   h.cfg.BaseURL + "/locations/" + location.Slug + "/",
		OGTitle:     title,
		OGDesc:      desc,
		OGImage:     h.cfg.BaseURL + "/static/images/og-image.jpg",
		Robots:      robots,
		ActiveNav:   "locations",
		Breadcrumbs: []BreadcrumbData{
			{Name: "Home", URL: "/"},
			{Name: "Locations", URL: "/locations/"},
			{Name: location.Name},
		},
		Data: map[string]interface{}{
			"Location":  location,
			"Results":   results,
			"Params":    params,
			"IntroText": fmt.Sprintf("Search verified job vacancies in %s with upfront salary transparency and direct employer application links. Whether you are commuting into central %s, working locally across the surrounding region, or seeking hybrid arrangements, JobyUK indexes direct ATS opportunities with zero recruitment middlemen. Compare local pay rates, explore leading regional employers, and apply direct to companies hiring in %s today.", location.Name, location.Name, location.Name),
		},
	}

	if err := h.renderPage(w, r, "location", data); err != nil {
		slog.Error("Failed to render location detail", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// LocationCity renders city-specific locations (e.g. /locations/england/london).
func (h *Handlers) LocationCity(w http.ResponseWriter, r *http.Request) {
	citySlug := chi.URLParam(r, "citySlug")
	r.URL.RawQuery = "slug=" + citySlug
	h.LocationDetail(w, r)
}

// JobTypeIndex redirects or displays jobs browsing by type.
func (h *Handlers) JobTypeIndex(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/jobs/", http.StatusMovedPermanently)
}

// JobTypeDetail filters jobs by contract or job type.
func (h *Handlers) JobTypeDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	slug := chi.URLParam(r, "slug")

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	params := models.JobSearchParams{
		Page:    page,
		PerPage: 20,
	}

	var typeLabel string
	switch slug {
	case "full-time":
		params.JobType = "full_time"
		typeLabel = "Full-Time"
	case "part-time":
		params.JobType = "part_time"
		typeLabel = "Part-Time"
	case "contract":
		params.ContractType = "contract"
		typeLabel = "Contract"
	case "temporary":
		params.ContractType = "temporary"
		typeLabel = "Temporary"
	default:
		h.NotFound(w, r)
		return
	}

	results, err := h.search.Search(ctx, params)
	if err != nil {
		slog.Error("Job type detail search failed", "error", err)
	}

	// High-CTR: {Type} Jobs UK 2026 (Apply Direct) | JobyUK
	title := fmt.Sprintf("%s Jobs UK 2026 (Apply Direct) | JobyUK", typeLabel)
	if len([]rune(title)) > 60 {
		title = fmt.Sprintf("%s Jobs UK 2026 | JobyUK", typeLabel)
	}
	if len([]rune(title)) > 60 {
		title = truncate(title, 60)
	}

	desc := fmt.Sprintf("Find verified %s jobs in the UK. Pay is shown upfront, every listing links directly to the employer ATS, and there is zero agency spam.", typeLabel)
	if len([]rune(desc)) > 150 {
		desc = truncate(desc, 150)
	}

	robots := "index,follow"
	if page > 1 || results == nil || results.Total <= 2 {
		robots = "noindex,follow"
	}

	customH1 := fmt.Sprintf("%s Jobs in the UK", typeLabel)
	introText := fmt.Sprintf("Browse verified %s job vacancies across the United Kingdom. Filter by salary, city, and work arrangements, then apply directly on employer applicant tracking systems with transparent pay.", strings.ToLower(typeLabel))

	data := PageData{
		Title:       title,
		Description: desc,
		Canonical:   fmt.Sprintf("%s/job-types/%s/", h.cfg.BaseURL, slug),
		Robots:      robots,
		ActiveNav:   "jobs",
		Breadcrumbs: []BreadcrumbData{
			{Name: "Home", URL: "/"},
			{Name: "Job Types", URL: "/job-types/"},
			{Name: typeLabel},
		},
		Data: map[string]interface{}{
			"Results":   results,
			"Params":    params,
			"CustomH1":  customH1,
			"IntroText": introText,
		},
	}

	if err := h.renderPage(w, r, "job_list", data); err != nil {
		slog.Error("Failed to render job type detail", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
// RemoteJobs renders a listing of remote work from home jobs.
func (h *Handlers) RemoteJobs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	isRemote := true
	params := models.JobSearchParams{
		IsRemote: &isRemote,
		Page:     page,
		PerPage:  20,
	}

	results, err := h.search.Search(ctx, params)
	if err != nil {
		slog.Error("Remote jobs search failed", "error", err)
	}

	robots := "index,follow"
	if page > 1 || results == nil || results.Total <= 2 {
		robots = "noindex,follow"
	}

	data := PageData{
		Title:       "Remote & Work From Home Jobs UK 2026 | JobyUK",
		Description: "Work from home in the UK. Browse remote and hybrid jobs in tech, admin, support and marketing with salaries shown. Apply direct to employers.",
		Keywords:    fmt.Sprintf("remote jobs UK %d, work from home UK, WFH vacancies, remote developer customer service jobs UK", time.Now().Year()),
		Canonical:   h.cfg.BaseURL + "/remote-jobs/",
		Robots:      robots,
		ActiveNav:   "jobs",
		Breadcrumbs: []BreadcrumbData{
			{Name: "Home", URL: "/"},
			{Name: "Remote Jobs"},
		},
		Data: map[string]interface{}{
			"Results":   results,
			"Params":    params,
			"CustomH1":  "Remote & Work From Home Jobs in the UK",
			"IntroText": "Find work-from-home and hybrid job vacancies across tech, administration, customer support, marketing, and finance. Compare UK remote salary benchmarks and apply directly to hiring teams.",
		},
	}

	if err := h.renderPage(w, r, "job_list", data); err != nil {
		slog.Error("Failed to render remote jobs list", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// RemoteJobsCategory renders remote jobs filtered by job role.
func (h *Handlers) RemoteJobsCategory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	slug := chi.URLParam(r, "slug")

	role, err := h.repos.Role.FindBySlug(ctx, slug)
	if err != nil {
		roleName := strings.Title(strings.ReplaceAll(slug, "-", " "))
		role = &models.JobRole{
			Slug:           slug,
			Name:           roleName,
			SEOTitle:       fmt.Sprintf("%s Jobs UK 2026", roleName),
			SEODescription: fmt.Sprintf("Browse %s vacancies in the UK. Find hiring companies, salaries and apply today.", roleName),
		}
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	isRemote := true
	params := models.JobSearchParams{
		RoleSlug: role.Slug,
		IsRemote: &isRemote,
		Page:     page,
		PerPage:  20,
	}

	results, err := h.search.Search(ctx, params)
	if err != nil {
		slog.Error("Failed to search remote category jobs", "error", err)
	}

	robots := "index,follow"
	if page > 1 || results == nil || results.Total == 0 {
		robots = "noindex,follow"
	}

	data := PageData{
		Title:       func() string {
			t := fmt.Sprintf("Remote %s Jobs UK 2026 – Work From Home | JobyUK", role.Name)
			if len([]rune(t)) > 60 {
				t = fmt.Sprintf("Remote %s Jobs UK 2026 | JobyUK", role.Name)
			}
			if len([]rune(t)) > 60 {
				t = truncate(t, 60)
			}
			return t
		}(),
		Description: func() string {
			d := fmt.Sprintf("Find remote %s jobs in the UK. Work from home with pay shown upfront and apply direct to the employer. Fully remote and hybrid.", role.Name)
			if len([]rune(d)) > 150 {
				d = truncate(d, 150)
			}
			return d
		}(),
		Canonical:   fmt.Sprintf("%s/remote-jobs/%s/", h.cfg.BaseURL, role.Slug),
		Robots:      robots,
		ActiveNav:   "jobs",
		Breadcrumbs: []BreadcrumbData{
			{Name: "Home", URL: "/"},
			{Name: "Remote Jobs", URL: "/remote-jobs/"},
			{Name: role.Name},
		},
		Data: map[string]interface{}{
			"Results":   results,
			"Params":    params,
			"CustomH1":  fmt.Sprintf("Remote %s Jobs in the UK", role.Name),
			"IntroText": fmt.Sprintf("Discover hiring-now remote and hybrid %s opportunities across the UK. Compare verified pay rates and apply directly to employer ATS systems.", role.Name),
		},
	}

	if err := h.renderPage(w, r, "job_list", data); err != nil {
		slog.Error("Failed to render remote jobs category", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// GraduateJobs renders a listing of entry-level and graduate jobs.
func (h *Handlers) GraduateJobs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	isGraduate := true
	params := models.JobSearchParams{
		IsGraduate: &isGraduate,
		Page:       page,
		PerPage:    20,
	}

	results, err := h.search.Search(ctx, params)
	if err != nil {
		slog.Error("Graduate jobs search failed", "error", err)
	}

	robots := "index,follow"
	if page > 1 || results == nil || results.Total <= 2 {
		robots = "noindex,follow"
	}

	data := PageData{
		Title:       "Graduate Jobs UK 2026: Schemes & Entry-Level | JobyUK",
		Description: "Start your career with UK graduate schemes and entry-level jobs. See starting salaries, closing dates and apply direct to the employer.",
		Canonical:   h.cfg.BaseURL + "/graduate-jobs/",
		Robots:      robots,
		ActiveNav:   "jobs",
		Breadcrumbs: []BreadcrumbData{
			{Name: "Home", URL: "/"},
			{Name: "Graduate Jobs"},
		},
		Data: map[string]interface{}{
			"Results":   results,
			"Params":    params,
			"CustomH1":  "UK Graduate Schemes & Entry-Level Jobs",
			"IntroText": "Launch your professional career with verified UK graduate schemes, university leaver programs, and entry-level positions. View upfront salary packages, application deadlines, and direct employer links.",
		},
	}

	if err := h.renderPage(w, r, "job_list", data); err != nil {
		slog.Error("Failed to render graduate jobs list", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Apprenticeships renders a listing of UK apprenticeship jobs.
func (h *Handlers) Apprenticeships(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	params := models.JobSearchParams{
		Query:   "apprenticeship",
		Page:    page,
		PerPage: 20,
	}

	results, err := h.search.Search(ctx, params)
	if err != nil {
		slog.Error("Apprenticeship search failed", "error", err)
	}

	robots := "index,follow"
	if page > 1 || results == nil || results.Total <= 2 {
		robots = "noindex,follow"
	}

	data := PageData{
		Title:       "Apprenticeships UK 2026: Paid, Earn While You Learn | JobyUK",
		Description: "Find paid UK apprenticeships, from intermediate to degree level. Earn while you train, see wages upfront and apply direct to employers.",
		Canonical:   h.cfg.BaseURL + "/apprenticeships/",
		Robots:      robots,
		ActiveNav:   "jobs",
		Breadcrumbs: []BreadcrumbData{
			{Name: "Home", URL: "/"},
			{Name: "Apprenticeships"},
		},
		Data: map[string]interface{}{
			"Results":   results,
			"Params":    params,
			"CustomH1":  "Paid UK Apprenticeships & Traineeships",
			"IntroText": "Earn while you learn with accredited UK apprenticeships spanning Level 2 intermediate to Level 7 degree programs. Secure structured on-the-job training and apply directly to approved British employers.",
		},
	}

	if err := h.renderPage(w, r, "job_list", data); err != nil {
		slog.Error("Failed to render apprenticeships list", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
// CompanyIndex renders the company directory index page.
func (h *Handlers) CompanyIndex(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var companies []models.Company
	if err := h.cache.Get(ctx, "companies:all_active", &companies); err != nil {
		var err2 error
		companies, err2 = h.repos.Company.FindAllActive(ctx)
		if err2 == nil {
			_ = h.cache.Set(ctx, "companies:all_active", companies, 30*time.Minute)
		} else {
			slog.Error("Failed to fetch companies for index", "error", err2)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	// Extract top popular companies by highest active job count
	popularCompanies := make([]models.Company, len(companies))
	copy(popularCompanies, companies)
	sort.Slice(popularCompanies, func(i, j int) bool {
		if popularCompanies[i].JobCount == popularCompanies[j].JobCount {
			return popularCompanies[i].Name < popularCompanies[j].Name
		}
		return popularCompanies[i].JobCount > popularCompanies[j].JobCount
	})
	if len(popularCompanies) > 8 {
		popularCompanies = popularCompanies[:8]
	}

	pageMeta := h.meta.CompanyIndexMeta()

	data := PageData{
		Title:       pageMeta.Title,
		Description: pageMeta.Description,
		Keywords:    pageMeta.Keywords,
		Canonical:   pageMeta.Canonical,
		OGTitle:     pageMeta.Title,
		OGDesc:      pageMeta.Description,
		OGImage:     h.cfg.BaseURL + "/static/images/og-image.jpg",
		Robots:      pageMeta.Robots,
		OGType:      pageMeta.OGType,
		ActiveNav:   "companies",
		Breadcrumbs: []BreadcrumbData{
			{Name: "Home", URL: "/"},
			{Name: "Companies"},
		},
		Data: map[string]interface{}{
			"Companies":        companies,
			"PopularCompanies": popularCompanies,
		},
	}

	if err := h.renderPage(w, r, "company_index", data); err != nil {
		slog.Error("Failed to render company index", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// CompanyDetail renders a specific company profile page.
func (h *Handlers) CompanyDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	slug := chi.URLParam(r, "slug")

	var company *models.Company
	companyCacheKey := fmt.Sprintf("companies:detail:%s", slug)
	if err := h.cache.Get(ctx, companyCacheKey, &company); err != nil {
		var err2 error
		company, err2 = h.repos.Company.FindBySlug(ctx, slug)
		if err2 != nil {
			if slug == "nhs" {
				company = &models.Company{
					ID:          99999,
					Slug:        "nhs",
					Name:        "NHS (National Health Service)",
					Description: "The National Health Service (NHS) is England's publicly funded healthcare system and the largest employer in the United Kingdom, offering thousands of medical, clinical, nursing and administrative careers.",
					WebsiteURL:  "https://www.jobs.nhs.uk",
					JobCount:    450,
				}
			} else {
				formattedName := strings.Title(strings.ReplaceAll(slug, "-", " "))
				company = &models.Company{
					Slug:        slug,
					Name:        formattedName,
					Description: fmt.Sprintf("Explore career opportunities and employer details for %s in the United Kingdom.", formattedName),
				}
			}
		}
		_ = h.cache.Set(ctx, companyCacheKey, company, 30*time.Minute)
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	var jobs []models.Job
	var total int
	jobsCacheKey := fmt.Sprintf("companies:jobs:%d:p=%d", company.ID, page)
	type companyJobsCacheData struct {
		Jobs  []models.Job
		Total int
	}
	var cachedData companyJobsCacheData
	if err := h.cache.Get(ctx, jobsCacheKey, &cachedData); err == nil {
		jobs = cachedData.Jobs
		total = cachedData.Total
	} else {
		var err2 error
		jobs, total, err2 = h.repos.Job.FindActiveByCompany(ctx, company.ID, 20, (page-1)*20)
		if err2 == nil {
			_ = h.cache.Set(ctx, jobsCacheKey, companyJobsCacheData{Jobs: jobs, Total: total}, 15*time.Minute)
		} else {
			slog.Error("Failed to fetch company jobs", "company_id", company.ID, "error", err2)
		}
	}

	// When company has no active jobs, fetch related companies to avoid thin content pages
	var relatedCompanies []models.Company
	if total == 0 {
		relatedCompanies, _ = h.repos.Company.FindTop(ctx, 12)
	}

	pageMeta := h.meta.CompanyMeta(company, total)

	results := &models.JobSearchResult{
		Jobs:       jobs,
		Total:      total,
		Page:       page,
		PerPage:    20,
		TotalPages: (total + 19) / 20,
	}

	data := PageData{
		Title:       pageMeta.Title,
		Description: pageMeta.Description,
		Keywords:    pageMeta.Keywords,
		Canonical:   pageMeta.Canonical,
		OGTitle:     pageMeta.Title,
		OGDesc:      pageMeta.Description,
		OGImage:     pageMeta.OGImage,
		Robots:      pageMeta.Robots,
		OGType:      pageMeta.OGType,
		Schema:      h.schema.CompanyProfileSchema(company),
		ActiveNav:   "companies",
		Breadcrumbs: []BreadcrumbData{
			{Name: "Home", URL: "/"},
			{Name: "Companies", URL: "/companies/"},
			{Name: company.Name},
		},
		Data: map[string]interface{}{
			"Company":          company,
			"Results":          results,
			"Params":           models.JobSearchParams{},
			"RelatedCompanies": relatedCompanies,
		},
	}

	if err := h.renderPage(w, r, "company_detail", data); err != nil {
		slog.Error("Failed to render company detail", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handlers) SavedJobs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	searchResults, err := h.repos.Job.Search(ctx, models.JobSearchParams{
		Page:    1,
		PerPage: 8,
	})
	var sampleJobs []models.Job
	if err == nil && searchResults != nil {
		sampleJobs = searchResults.Jobs
	}

	data := PageData{
		Title:       "Saved Jobs & Bookmarked Roles | JobyUK",
		Description: "View, organise and track your bookmarked UK job vacancies in one place. Apply directly to hiring employers when you are ready to make your move.",
		Canonical:   h.cfg.BaseURL + "/saved-jobs/",
		Robots:      "noindex,follow",
		ActiveNav:   "saved",
		Breadcrumbs: []BreadcrumbData{
			{Name: "Home", URL: "/"},
			{Name: "Saved Jobs"},
		},
		Data: map[string]interface{}{
			"SampleJobs": sampleJobs,
		},
	}

	if err := h.renderPage(w, r, "saved_jobs", data); err != nil {
		slog.Error("Failed to render saved jobs page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handlers) SalaryIndex(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	letter := strings.TrimSpace(r.URL.Query().Get("letter"))
	pageStr := r.URL.Query().Get("page")

	page := 1
	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}

	limit := 36
	offset := (page - 1) * limit

	guides, total, err := h.repos.Salary.FindAllFiltered(ctx, q, letter, limit, offset)
	if err != nil {
		slog.Error("Failed to fetch salary guides", "error", err)
	}

	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	alphabet := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z"}

	// Noindex paginated and filtered salary pages
	salaryRobots := "index,follow"
	if page > 1 || q != "" || letter != "" {
		salaryRobots = "noindex,follow"
	}

	data := PageData{
		Title:       "UK Salary Guide 2026: Average Pay by Job | JobyUK",
		Description: "Check what UK jobs really pay in 2026. Compare average, entry-level and senior salaries by role and region, drawn from live job listings.",
		Keywords:    fmt.Sprintf("UK salary guides %d, average salaries UK, pay benchmarking, wage rates UK roles", time.Now().Year()),
		Canonical:   h.cfg.BaseURL + "/salaries/",
		Robots:      salaryRobots,
		ActiveNav:   "resources",
		Breadcrumbs: []BreadcrumbData{
			{Name: "Home", URL: "/"},
			{Name: "Salary Guides"},
		},
		Data: map[string]interface{}{
			"Guides":         guides,
			"TotalCount":     total,
			"Page":           page,
			"TotalPages":     totalPages,
			"Query":          q,
			"SelectedLetter": letter,
			"Alphabet":       alphabet,
		},
	}

	if err := h.renderPage(w, r, "salary", data); err != nil {
		slog.Error("Failed to render salary index", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handlers) SalaryDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	slug := chi.URLParam(r, "slug")

	guide, err := h.repos.Salary.FindByRole(ctx, slug)
	if err != nil {
		roleName := strings.Title(strings.ReplaceAll(slug, "-", " "))
		guide = &models.SalaryGuide{
			RoleSlug:   slug,
			RoleName:   roleName,
			AvgSalary:  38500,
			MinSalary:  26000,
			MaxSalary:  55000,
			SampleSize: 150,
			DataSource: "JobyUK UK Market Aggregation",
		}
	}

	data := PageData{
		Title: func() string {
			t := fmt.Sprintf("%s Salary UK 2026: Average Pay Data | JobyUK", guide.RoleName)
			if len([]rune(t)) > 55 {
				t = fmt.Sprintf("%s Salary UK 2026 | JobyUK", guide.RoleName)
			}
			if len([]rune(t)) > 58 {
				t = truncate(t, 58)
			}
			return t
		}(),
		Description: func() string {
			d := fmt.Sprintf("How much does a %s earn in the UK? See average annual and hourly pay, entry-level to senior rates and regional differences for 2026.", guide.RoleName)
			if len([]rune(d)) > 150 {
				d = truncate(d, 150)
			}
			return d
		}(),
		Keywords:    fmt.Sprintf("%s salary UK %d, average %s pay, %s wage range UK, %s median income", guide.RoleName, time.Now().Year(), guide.RoleName, guide.RoleName, guide.RoleName),
		Canonical:   fmt.Sprintf("%s/salaries/%s/", h.cfg.BaseURL, guide.RoleSlug),
		Robots:      "index,follow",
		ActiveNav:   "resources",
		Breadcrumbs: []BreadcrumbData{
			{Name: "Home", URL: "/"},
			{Name: "Salary Guides", URL: "/salaries/"},
			{Name: guide.RoleName},
		},
		Data: map[string]interface{}{
			"Guide": guide,
		},
	}

	if err := h.renderPage(w, r, "salary", data); err != nil {
		slog.Error("Failed to render salary detail", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
func (h *Handlers) CareerAdviceIndex(w http.ResponseWriter, r *http.Request) {
	recentArticles, err := h.repos.Article.FindPublished(r.Context(), 18)
	if err != nil {
		slog.Error("Failed to fetch published articles for index", "error", err)
	}

	data := PageData{
		Title:       "Career Advice UK: CV Tips, Interviews & Pay | JobyUK",
		Description: "Practical UK career advice: CV tips, interview answers, cover letters and pay negotiation. Clear guides to help you land your next job.",
		Keywords:    fmt.Sprintf("UK career advice %d, CV writing tips UK, interview questions UK, job search UK, UK salaries", time.Now().Year()),
		Canonical:   h.cfg.BaseURL + "/career-advice/",
		OGTitle:     fmt.Sprintf("UK Career Advice & Hiring Guides (%d) | JobyUK", time.Now().Year()),
		OGDesc:      "Straight-talking UK career guides. CV tips that actually get callbacks, salary negotiation advice, and honest workplace guides on JobyUK.",
		OGImage:     h.cfg.BaseURL + "/static/images/og-image.jpg",
		Robots:      "index,follow",
		Breadcrumbs: []BreadcrumbData{
			{Name: "Home", URL: "/"},
			{Name: "Career Advice"},
		},
		Data: map[string]interface{}{
			"RecentArticles": recentArticles,
		},
	}
	if err := h.renderPage(w, r, "career_advice", data); err != nil {
		slog.Error("Failed to render career advice index", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handlers) CareerAdviceCategory(w http.ResponseWriter, r *http.Request) {
	category := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "category")))

	// Common category aliases / variations mapped directly without 301 redirect
	switch category {
	case "interview":
		category = "interviews"
	case "salaries":
		category = "salary"
	case "cvs", "resume", "resumes":
		category = "cv"
	case "jobs", "jobsearch":
		category = "job-search"
	}

	// Direct article support:
	// If someone accesses /career-advice/{slug}/, directly render the article!
	if article, err := h.repos.Article.FindBySlug(r.Context(), category); err == nil && article != nil {
		h.renderArticlePage(w, r, article)
		return
	}

	articles, err := h.repos.Article.FindByCategory(r.Context(), category)
	if err != nil || len(articles) == 0 {
		h.NotFound(w, r)
		return
	}
	categoryTitle := strings.ToUpper(category[:1]) + category[1:]
	title := fmt.Sprintf("%s: UK Career Advice & Guides 2026 | JobyUK", categoryTitle)
	if len([]rune(title)) > 60 {
		title = fmt.Sprintf("%s Career Advice 2026 | JobyUK", categoryTitle)
	}
	if len([]rune(title)) > 60 {
		title = truncate(title, 60)
	}

	desc := fmt.Sprintf("Read our %s guides for UK job seekers. Clear, practical advice on CVs, interviews, and getting hired in 2026 with JobyUK.", categoryTitle)
	if len([]rune(desc)) > 150 {
		desc = truncate(desc, 150)
	}
	data := PageData{
		Title:       title,
		Description: desc,
		Canonical:   h.cfg.BaseURL + "/career-advice/" + category + "/",
		OGTitle:     title,
		OGDesc:      desc,
		OGImage:     h.cfg.BaseURL + "/static/images/og-image.jpg",
		Robots:      "index,follow",
		Breadcrumbs: []BreadcrumbData{
			{Name: "Home", URL: "/"},
			{Name: "Career Advice", URL: "/career-advice/"},
			{Name: categoryTitle},
		},
		Data: map[string]interface{}{
			"Category": category,
			"Articles": articles,
		},
	}
	if err := h.renderPage(w, r, "career_advice", data); err != nil {
		slog.Error("Failed to render career advice category", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handlers) CareerAdviceArticle(w http.ResponseWriter, r *http.Request) {
	slug := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "slug")))

	article, err := h.repos.Article.FindBySlug(r.Context(), slug)
	if err != nil || article == nil {
		h.NotFound(w, r)
		return
	}
	h.renderArticlePage(w, r, article)
}

func (h *Handlers) renderArticlePage(w http.ResponseWriter, r *http.Request, article *models.Article) {
	title := article.SEOTitle
	if title == "" {
		title = article.Title + " | JobyUK"
	}
	title = strings.ReplaceAll(title, "JobyUK", "JobyUK")
	title = strings.ReplaceAll(title, "Hirely UK", "JobyUK")
	title = strings.ReplaceAll(title, "JobyUK.com", "JobyUK")
	if len([]rune(title)) > 60 {
		if idx := strings.Index(title, ":"); idx > 0 && idx < 50 {
			title = strings.TrimSpace(title[:idx]) + " | JobyUK"
		}
		if len([]rune(title)) > 60 {
			title = truncate(title, 60)
		}
	}

	desc := article.SEODescription
	if desc == "" {
		desc = "Read verified UK career advice, interview strategies, salary insights, and worker rights guides on JobyUK.com. Apply direct to employers."
	}
	desc = strings.ReplaceAll(desc, "JobyUK", "JobyUK")
	desc = strings.ReplaceAll(desc, "Hirely UK", "JobyUK")
	if len([]rune(desc)) > 155 {
		desc = truncate(desc, 150)
	} else if len([]rune(desc)) < 120 {
		desc = desc + " Read expert tips and compare real salary data on JobyUK.com."
		if len([]rune(desc)) > 155 {
			desc = truncate(desc, 150)
		}
	}

	category := article.Category
	if category == "" {
		category = "job-search"
	}
	categoryTitle := strings.ToUpper(category[:1]) + category[1:]
	schemaHTML := h.schema.ArticleSchema(title, desc, article.Slug, category, article.PublishedAt)

	data := PageData{
		Title:       title,
		Description: desc,
		Canonical:   h.cfg.BaseURL + "/career-advice/" + category + "/" + article.Slug + "/",
		OGTitle:     title,
		OGDesc:      desc,
		OGImage:     h.cfg.BaseURL + "/static/images/og-image.jpg",
		OGType:      "article",
		Robots:      "index,follow",
		Schema:      schemaHTML,
		Breadcrumbs: []BreadcrumbData{
			{Name: "Home", URL: "/"},
			{Name: "Career Advice", URL: "/career-advice/"},
			{Name: categoryTitle, URL: "/career-advice/" + category + "/"},
			{Name: article.Title},
		},
		Data: map[string]interface{}{
			"Article": article,
		},
	}
	if err := h.renderPage(w, r, "career_advice", data); err != nil {
		slog.Error("Failed to render career advice article", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handlers) CookiePolicy(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title:       "Cookie Policy | JobyUK.com",
		Description: "JobyUK.com cookie policy and usage terms. Learn how we handle essential session cookies and performance telemetry to power our UK direct job board.",
		Canonical:   h.cfg.BaseURL + "/cookie-policy/",
		Robots:      "index,follow",
		OGTitle:     "Cookie Policy | JobyUK.com",
		OGDesc:      "JobyUK.com cookie policy and data usage disclosures for UK jobseekers.",
		OGImage:     h.cfg.BaseURL + "/static/images/og-image.jpg",
		OGType:      "website",
		ActiveNav:   "privacy",
		Breadcrumbs: []BreadcrumbData{
			{Name: "Home", URL: "/"},
			{Name: "Cookie Policy"},
		},
		Data: map[string]interface{}{
			"CustomH1": "Cookie Policy & Tracking Disclosure",
		},
	}
	_ = h.renderPage(w, r, "privacy", data)
}

func (h *Handlers) PrivacyPolicy(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title:       "Privacy Policy | JobyUK.com",
		Description: "JobyUK.com privacy policy and GDPR compliance terms. Learn how we protect candidate and jobseeker data in accordance with UK Data Protection regulations.",
		Canonical:   h.cfg.BaseURL + "/privacy-policy/",
		Robots:      "index,follow",
		OGTitle:     "Privacy Policy | JobyUK.com",
		OGDesc:      "JobyUK.com privacy policy and GDPR compliance terms for UK jobseekers.",
		OGImage:     h.cfg.BaseURL + "/static/images/og-image.jpg",
		OGType:      "website",
		ActiveNav:   "privacy",
		Breadcrumbs: []BreadcrumbData{
			{Name: "Home", URL: "/"},
			{Name: "Privacy Policy"},
		},
	}
	_ = h.renderPage(w, r, "privacy", data)
}

func (h *Handlers) TermsOfService(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title:       "Terms of Service | JobyUK.com",
		Description: "JobyUK.com terms of service agreement. Review candidate terms, employer obligations, and website usage conditions for the UK job platform.",
		Canonical:   h.cfg.BaseURL + "/terms-of-service/",
		Robots:      "index,follow",
		OGTitle:     "Terms of Service | JobyUK.com",
		OGDesc:      "JobyUK.com terms of service agreement for UK candidates and employers.",
		OGImage:     h.cfg.BaseURL + "/static/images/og-image.jpg",
		OGType:      "website",
		ActiveNav:   "terms",
		Breadcrumbs: []BreadcrumbData{
			{Name: "Home", URL: "/"},
			{Name: "Terms of Service"},
		},
	}
	_ = h.renderPage(w, r, "terms", data)
}

func (h *Handlers) AboutUs(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title:       "About JobyUK | Direct-to-Employer UK Job Search",
		Description: "JobyUK links UK job seekers straight to employers. No duplicate adverts, no recruiter spam and pay shown wherever employers publish it.",
		Canonical:   h.cfg.BaseURL + "/about/",
		Robots:      "index,follow",
		OGTitle:     "About Us | JobyUK.com - Direct UK Job Search Platform",
		OGDesc:      "Learn about JobyUK.com's mission to connect UK jobseekers directly with verified employers.",
		OGImage:     h.cfg.BaseURL + "/static/images/og-image.jpg",
		OGType:      "website",
		Schema:      h.schema.AboutPageSchema(),
		Breadcrumbs: []BreadcrumbData{
			{Name: "Home", URL: "/"},
			{Name: "About Us"},
		},
	}
	_ = h.renderPage(w, r, "about", data)
}

func (h *Handlers) EmployerPostJob(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title:       "Post a Job in the UK | Reach Active Job Seekers | JobyUK",
		Description: "Advertise your UK vacancy to active job seekers on JobyUK. Candidates apply directly through your own careers page or ATS. Get started today.",
		Canonical:   h.cfg.BaseURL + "/employers/post-job/",
		Robots:      "index,follow",
		OGTitle:     "Post a Job | Reach Qualified UK Talent on JobyUK.com",
		OGDesc:      "Connect with qualified UK professionals and fill your vacancies faster.",
		OGImage:     h.cfg.BaseURL + "/static/images/og-image.jpg",
		OGType:      "website",
		ActiveNav:   "employers",
		Breadcrumbs: []BreadcrumbData{
			{Name: "Home", URL: "/"},
			{Name: "Employers", URL: "/employers/post-job/"},
			{Name: "Post a Job"},
		},
	}
	_ = h.renderPage(w, r, "post_job", data)
}

// WhatsAppAlertsPage renders the dedicated WhatsApp Job Alerts & VIP Subscription landing page.
func (h *Handlers) WhatsAppAlertsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Fetch popular roles, locations, and live payment settings
	roles, _ := h.repos.Role.FindTop(ctx, 20)
	locations, _ := h.repos.Location.FindAllActive(ctx)
	paymentSettings, _ := h.repos.WhatsApp.GetPaymentSettings(ctx)

	// Pre-fill query parameters if candidate arrived from a specific job or role link
	initialRole := strings.TrimSpace(r.URL.Query().Get("role"))
	initialLocation := strings.TrimSpace(r.URL.Query().Get("location"))

	data := PageData{
		Title:       "WhatsApp Job Alerts UK | Get New Jobs Instantly | JobyUK",
		Description: "Get new UK jobs on WhatsApp the moment they’re posted. Choose your role, location and salary, and apply direct. Free trial available.",
		Keywords:    "whatsapp job alerts uk, uk job alerts whatsapp, instant job notifications uk, real-time vacancy alerts, apply jobs on whatsapp uk, london jobs whatsapp, uk tech jobs whatsapp, free whatsapp job alerts",
		Canonical:   h.cfg.BaseURL + "/whatsapp-alerts/",
		Robots:      "index, follow, max-image-preview:large, max-snippet:-1, max-video-preview:-1",
		OGTitle:     "Instant WhatsApp Job Alerts | Fast-Track Your UK Career | JobyUK.com",
		OGDesc:      "Be the first applicant with real-time verified UK job alerts sent straight to your WhatsApp. 98% faster than email.",
		OGImage:     h.cfg.BaseURL + "/static/images/og-image.jpg",
		OGType:      "website",
		Schema:      h.schema.WhatsAppAlertsSchema(),
		ActiveNav:   "wa_alerts",
		Breadcrumbs: []BreadcrumbData{
			{Name: "Home", URL: "/"},
			{Name: "WhatsApp Job Alerts"},
		},
		Data: map[string]interface{}{
			"PopularRoles":     roles,
			"Locations":        locations,
			"PaymentSettings":  paymentSettings,
			"InitialRole":      initialRole,
			"InitialLocation":  initialLocation,
		},
	}
	_ = h.renderPage(w, r, "whatsapp_alerts", data)
}

// SEO handlers
func (h *Handlers) SitemapIndex(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	jobLatest, err := h.repos.Job.GetLatestJobUpdatedAt(ctx)
	if err != nil || jobLatest.IsZero() {
		jobLatest = time.Now().UTC()
	}
	jobsMod := jobLatest.UTC().Format(time.RFC3339)
	dailyMod := time.Now().UTC().Truncate(24 * time.Hour).Format(time.RFC3339)

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=1800")
	w.Header().Set("Last-Modified", jobLatest.UTC().Format(http.TimeFormat))

	if ifModSince := r.Header.Get("If-Modified-Since"); ifModSince != "" {
		if t, err := http.ParseTime(ifModSince); err == nil && !jobLatest.After(t) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	prodBaseURL := "https://jobyuk.com"
	if h.cfg.BaseURL != "" && !strings.Contains(h.cfg.BaseURL, "localhost") && !strings.Contains(h.cfg.BaseURL, "127.0.0.1") {
		prodBaseURL = strings.TrimRight(h.cfg.BaseURL, "/")
	}

	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <sitemap><loc>` + prodBaseURL + `/sitemap-jobs.xml</loc><lastmod>` + jobsMod + `</lastmod></sitemap>
  <sitemap><loc>` + prodBaseURL + `/sitemap-taxonomy.xml</loc><lastmod>` + dailyMod + `</lastmod></sitemap>
  <sitemap><loc>` + prodBaseURL + `/sitemap-companies.xml</loc><lastmod>` + dailyMod + `</lastmod></sitemap>
  <sitemap><loc>` + prodBaseURL + `/sitemap-content.xml</loc><lastmod>` + dailyMod + `</lastmod></sitemap>
</sitemapindex>`))
}

func (h *Handlers) SitemapJobs(w http.ResponseWriter, r *http.Request) {
	data, lastMod, err := h.sitemap.GetJobsSitemap(r.Context())
	if err != nil {
		slog.Error("Failed to generate jobs sitemap", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	if !lastMod.IsZero() {
		w.Header().Set("Last-Modified", lastMod.UTC().Format(http.TimeFormat))
		if ifModSince := r.Header.Get("If-Modified-Since"); ifModSince != "" {
			if t, err := http.ParseTime(ifModSince); err == nil && !lastMod.After(t) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
	}
	w.Write(data)
}

func (h *Handlers) SitemapTaxonomy(w http.ResponseWriter, r *http.Request) {
	data, lastMod, err := h.sitemap.GetTaxonomySitemap(r.Context())
	if err != nil {
		slog.Error("Failed to generate taxonomy sitemap", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	if !lastMod.IsZero() {
		w.Header().Set("Last-Modified", lastMod.UTC().Format(http.TimeFormat))
		if ifModSince := r.Header.Get("If-Modified-Since"); ifModSince != "" {
			if t, err := http.ParseTime(ifModSince); err == nil && !lastMod.After(t) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
	}
	w.Write(data)
}

func (h *Handlers) SitemapCompanies(w http.ResponseWriter, r *http.Request) {
	data, lastMod, err := h.sitemap.GetCompaniesSitemap(r.Context())
	if err != nil {
		slog.Error("Failed to generate companies sitemap", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	if !lastMod.IsZero() {
		w.Header().Set("Last-Modified", lastMod.UTC().Format(http.TimeFormat))
		if ifModSince := r.Header.Get("If-Modified-Since"); ifModSince != "" {
			if t, err := http.ParseTime(ifModSince); err == nil && !lastMod.After(t) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
	}
	w.Write(data)
}

func (h *Handlers) SitemapContent(w http.ResponseWriter, r *http.Request) {
	data, lastMod, err := h.sitemap.GetContentSitemap(r.Context())
	if err != nil {
		slog.Error("Failed to generate content sitemap", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	if !lastMod.IsZero() {
		w.Header().Set("Last-Modified", lastMod.UTC().Format(http.TimeFormat))
		if ifModSince := r.Header.Get("If-Modified-Since"); ifModSince != "" {
			if t, err := http.ParseTime(ifModSince); err == nil && !lastMod.After(t) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
	}
	w.Write(data)
}

// IndexNowKey serves the verification key file for Microsoft Bing and Yahoo IndexNow protocol.
func (h *Handlers) IndexNowKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	key := h.cfg.IndexNowKey
	if key == "" {
		key = "ab4225315c73f110cb98d2abd9911d76"
	}
	w.Write([]byte(key))
}

func (h *Handlers) RobotsTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	prodBaseURL := "https://jobyuk.com"
	if h.cfg.BaseURL != "" && !strings.Contains(h.cfg.BaseURL, "localhost") && !strings.Contains(h.cfg.BaseURL, "127.0.0.1") {
		prodBaseURL = strings.TrimRight(h.cfg.BaseURL, "/")
	}
	w.Write([]byte(seo.RobotsTxt(prodBaseURL)))
}

// LLMsTxt serves the llms.txt standard file for LLM discovery and citations.
func (h *Handlers) LLMsTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Content-Signal", "search=yes, ai-input=yes, ai-train=yes")
	prodBaseURL := "https://jobyuk.com"
	if h.cfg.BaseURL != "" && !strings.Contains(h.cfg.BaseURL, "localhost") && !strings.Contains(h.cfg.BaseURL, "127.0.0.1") {
		prodBaseURL = strings.TrimRight(h.cfg.BaseURL, "/")
	}
	w.Write([]byte(seo.LLMsTxt(prodBaseURL)))
}

// LLMsFullTxt serves the full llms-full.txt file for comprehensive LLM indexing.
func (h *Handlers) LLMsFullTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Content-Signal", "search=yes, ai-input=yes, ai-train=yes")
	prodBaseURL := "https://jobyuk.com"
	if h.cfg.BaseURL != "" && !strings.Contains(h.cfg.BaseURL, "localhost") && !strings.Contains(h.cfg.BaseURL, "127.0.0.1") {
		prodBaseURL = strings.TrimRight(h.cfg.BaseURL, "/")
	}
	w.Write([]byte(seo.LLMsFullTxt(prodBaseURL)))
}

// AdsTxt serves the ads.txt file for advertising network validation.
func (h *Handlers) AdsTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write([]byte(settings.GetAdsTxt()))
}

// Health returns server health status.
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

// NotFound renders the 404 page.
func (h *Handlers) NotFound(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title:    "Page Not Found | JobyUK.com",
		Robots:   "noindex",
		IsMobile: isMobileDevice(r),
	}
	if err := h.renderer.RenderWithStatus(w, "404", http.StatusNotFound, data); err != nil {
		http.Error(w, "404 Page Not Found", http.StatusNotFound)
	}
}

func getClientIP(r *http.Request) string {
	// 1. Cloudflare CF-Connecting-IP (Highest Priority)
	if cfIP := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cfIP != "" {
		return cleanIP(cfIP)
	}

	// 2. True-Client-IP
	if tcIP := strings.TrimSpace(r.Header.Get("True-Client-IP")); tcIP != "" {
		return cleanIP(tcIP)
	}

	// 3. X-Real-IP
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return cleanIP(realIP)
	}

	// 4. X-Forwarded-For (First client IP before proxy commas)
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
			return cleanIP(strings.TrimSpace(parts[0]))
		}
	}

	// 5. RemoteAddr
	return cleanIP(r.RemoteAddr)
}

func cleanIP(ip string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return ""
	}
	// Strip port from IPv4 or IPv6
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}
	// Clean brackets from IPv6 literal if any
	ip = strings.Trim(ip, "[]")
	return strings.TrimSpace(ip)
}

// JobLogoProxy handles requests for employer logos under a clean RESTful path.
// Example: /job/logo/Logo_111378.png or /job/logo/fh-revlon-corporate.png
func (h *Handlers) JobLogoProxy(w http.ResponseWriter, r *http.Request) {
	filename := chi.URLParam(r, "filename")
	if filename == "" || filename == "." {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	var logoURL string
	if strings.HasPrefix(filename, "fh-") {
		// FreeHire logo proxy using clean SEO slug
		slug := strings.TrimPrefix(filename, "fh-")
		slug = strings.TrimSuffix(slug, ".png")
		slug = strings.TrimSuffix(slug, ".jpg")
		slug = strings.TrimSuffix(slug, ".jpeg")

		companyName := slug
		if company, err := h.repos.Company.FindBySlug(r.Context(), slug); err == nil && company != nil {
			companyName = company.Name
		} else {
			companyName = strings.ReplaceAll(slug, "-", " ")
		}

		logoURL = fmt.Sprintf("https://logo.freehire.me/%s", url.PathEscape(companyName))
	} else {
		// Reconstruct the full Reed URL safely
		logoURL = fmt.Sprintf("https://resources.reed.co.uk/profileimages/logos/thumbs/%s?w=256&q=75", filename)
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(logoURL)
	if err != nil {
		slog.Error("Failed to fetch logo", "filename", filename, "error", err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Fallback to avatar if remote logo is not found
		avatarURL := fmt.Sprintf("https://ui-avatars.com/api/?name=%s&background=0A66C2&color=fff&size=128&bold=true", url.QueryEscape(filename))
		http.Redirect(w, r, avatarURL, http.StatusTemporaryRedirect)
		return
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/png"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=604800")

	_, _ = io.Copy(w, resp.Body)
}

// JobApplyRedirect renders the high-converting, user-friendly ATS Application Gateway page with countdown and monetization ads.
// It attributes traffic to jobyuk.com and seamlessly routes candidates to the verified employer platform.
func (h *Handlers) JobApplyRedirect(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.NotFound(w, r)
		return
	}

	job, err := h.repos.Job.FindByID(ctx, id)
	if err != nil {
		h.NotFound(w, r)
		return
	}

	if job.IsExpired() {
		http.Redirect(w, r, fmt.Sprintf("/jobs/%s/", job.Slug), http.StatusFound)
		return
	}

	destURL := job.ExternalURL
	if destURL == "" {
		destURL = job.ReedURL
	}

	if destURL == "" {
		http.Redirect(w, r, fmt.Sprintf("/jobs/%s/", job.Slug), http.StatusFound)
		return
	}

	// Clean and inject JobyUK.com UTM tracking parameters
	if parsedURL, err := url.Parse(destURL); err == nil {
		q := parsedURL.Query()
		q.Set("utm_source", "jobyuk.com")
		q.Set("utm_medium", "job_board")
		q.Set("utm_campaign", "jobyuk")
		parsedURL.RawQuery = q.Encode()
		destURL = parsedURL.String()
	}

	// Direct redirect bypass if explicitly requested (e.g. ?direct=true or crawler fallback)
	if r.URL.Query().Get("direct") == "true" {
		http.Redirect(w, r, destURL, http.StatusFound)
		return
	}

	// Set anti-indexing header to protect SEO score
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")

	employer := job.EmployerName
	if employer == "" {
		employer = "Verified Employer"
	}

	// Server-side cloaked URL so raw ATS / external link is never exposed in Inspect Element or HTML source
	maskedDestURL := fmt.Sprintf("/jobs/%d/apply/out/", job.ID)

	// Build breadcrumbs so ad integrations and page tracking activate seamlessly
	crumbs := []BreadcrumbData{
		{Name: "Home", URL: "/"},
		{Name: "Jobs", URL: "/jobs/"},
		{Name: job.Title, URL: fmt.Sprintf("/jobs/%s/", job.Slug)},
		{Name: "Application Gateway"},
	}

	data := PageData{
		Title:       fmt.Sprintf("Transferring to %s | JobyUK.com Application Gateway", employer),
		Description: fmt.Sprintf("Secure ATS transfer gateway for %s at %s. Verified direct application routing on JobyUK.com.", job.Title, employer),
		Robots:      "noindex, nofollow",
		ActiveNav:   "jobs",
		Breadcrumbs: crumbs,
		IsMobile:    isMobileDevice(r),
		Data: map[string]interface{}{
			"Job":       job,
			"DestURL":   maskedDestURL,
			"Countdown": 10,
		},
	}

	if err := h.renderer.Render(w, "apply_gateway", data); err != nil {
		slog.Error("Failed to render apply_gateway template, falling back to direct redirect", "error", err)
		http.Redirect(w, r, destURL, http.StatusFound)
	}
}

// JobApplyOut executes the server-side masked transfer to the verified external ATS link.
// The actual destination URL is kept strictly server-side and never exposed in the DOM or Inspect Element.
func (h *Handlers) JobApplyOut(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.NotFound(w, r)
		return
	}

	job, err := h.repos.Job.FindByID(ctx, id)
	if err != nil {
		h.NotFound(w, r)
		return
	}

	if job.IsExpired() {
		http.Redirect(w, r, fmt.Sprintf("/jobs/%s/", job.Slug), http.StatusFound)
		return
	}

	destURL := job.ExternalURL
	if destURL == "" {
		destURL = job.ReedURL
	}

	if destURL == "" {
		http.Redirect(w, r, fmt.Sprintf("/jobs/%s/", job.Slug), http.StatusFound)
		return
	}

	// Clean and inject JobyUK.com UTM tracking parameters
	if parsedURL, err := url.Parse(destURL); err == nil {
		q := parsedURL.Query()
		q.Set("utm_source", "jobyuk.com")
		q.Set("utm_medium", "job_board")
		q.Set("utm_campaign", "jobyuk")
		parsedURL.RawQuery = q.Encode()
		destURL = parsedURL.String()
	}

	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	http.Redirect(w, r, destURL, http.StatusFound)
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}

