package services

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"jobyuk/internal/models"
	"jobyuk/internal/repository"
)

// SearchService handles job search operations with caching.
type SearchService struct {
	jobRepo *repository.JobRepository
	cache   *CacheService
}

func NewSearchService(jobRepo *repository.JobRepository, cache *CacheService) *SearchService {
	return &SearchService{jobRepo: jobRepo, cache: cache}
}

// Search performs a cached job search.
func (s *SearchService) Search(ctx context.Context, params models.JobSearchParams) (*models.JobSearchResult, error) {
	// Create a unique cache key based on search parameters
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%+v", params)))
	cacheKey := fmt.Sprintf("jobs:search:%x", h.Sum(nil))

	var result models.JobSearchResult
	if err := s.cache.Get(ctx, cacheKey, &result); err == nil {
		return &result, nil
	}

	// Cache miss: query repository
	res, err := s.jobRepo.Search(ctx, params)
	if err != nil {
		return nil, err
	}

	// Cache for 10 minutes
	_ = s.cache.Set(ctx, cacheKey, res, 10*time.Minute)

	return res, nil
}

// ClearCache clears all cached search results.
func (s *SearchService) ClearCache(ctx context.Context) error {
	return s.cache.ClearPattern(ctx, "jobs:search:*")
}

// SEOService handles SEO-related operations.
type SEOService struct {
	baseURL string
}

func NewSEOService(baseURL string) *SEOService {
	return &SEOService{baseURL: baseURL}
}

// SitemapService handles sitemap generation and in-memory caching.
type SitemapService struct {
	repos   *repository.Repositories
	baseURL string

	mu               sync.RWMutex
	jobsCache        []byte
	taxonomyCache    []byte
	companiesCache   []byte
	contentCache     []byte
	jobsLastMod      time.Time
	taxonomyLastMod  time.Time
	companiesLastMod time.Time
	contentLastMod   time.Time
}

func NewSitemapService(repos *repository.Repositories, baseURL string) *SitemapService {
	prodURL := "https://jobyuk.com"
	if baseURL != "" && !strings.Contains(baseURL, "localhost") && !strings.Contains(baseURL, "127.0.0.1") {
		prodURL = strings.TrimRight(baseURL, "/")
	}
	return &SitemapService{repos: repos, baseURL: prodURL}
}

func (s *SitemapService) StartScheduled(ctx context.Context, interval time.Duration) {
	// Pre-warm sitemaps on startup in background
	go func() {
		time.Sleep(3 * time.Second)
		if err := s.RebuildAll(ctx); err != nil {
			slog.Error("Initial sitemap generation failed", "error", err)
		} else {
			slog.Info("Initial sitemaps successfully generated and cached in memory")
		}
	}()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			slog.Info("Scheduled sitemap regeneration started")
			if err := s.RebuildAll(ctx); err != nil {
				slog.Error("Scheduled sitemap regeneration failed", "error", err)
			} else {
				slog.Info("Scheduled sitemaps updated successfully")
			}
		}
	}
}

// TaxonomyService manages taxonomy mapping and counts.
type TaxonomyService struct {
	repos *repository.Repositories
	cache *CacheService
}

func NewTaxonomyService(repos *repository.Repositories, cache *CacheService) *TaxonomyService {
	return &TaxonomyService{repos: repos, cache: cache}
}

// FindLocationID looks up a location ID by name, falling back to cleaned location parts if needed.
func (t *TaxonomyService) FindLocationID(ctx context.Context, locationName string) int {
	if strings.TrimSpace(locationName) == "" {
		return 0
	}
	loc, err := t.repos.Location.FindByName(ctx, locationName)
	if err == nil {
		return loc.ID
	}

	// Try with clean DisplayLocation parts
	dummyJob := models.Job{LocationName: locationName}
	cleanLoc := dummyJob.DisplayLocation()
	if cleanLoc != "" && cleanLoc != "United Kingdom" {
		parts := strings.Split(cleanLoc, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				if loc, err := t.repos.Location.FindByName(ctx, p); err == nil {
					return loc.ID
				}
			}
		}
	}

	return 0
}

// MapJobToTaxonomy maps a job to roles and industries based on title/description keywords.
func (t *TaxonomyService) MapJobToTaxonomy(ctx context.Context, jobID int64, title, description string) {
	// Taxonomy mapping logic — keyword-based role/industry detection
	// This will be expanded with more sophisticated matching
}

// UpdateAllCounts refreshes job counts on all taxonomy entries.
func (t *TaxonomyService) UpdateAllCounts(ctx context.Context) error {
	if err := t.repos.Role.UpdateJobCounts(ctx); err != nil {
		return err
	}
	if err := t.repos.Industry.UpdateJobCounts(ctx); err != nil {
		return err
	}
	if err := t.repos.Location.UpdateJobCounts(ctx); err != nil {
		return err
	}
	if err := t.repos.Company.UpdateJobCounts(ctx); err != nil {
		return err
	}

	// Clear cached taxonomy, search, home, company, role, industry, and location data
	_ = t.cache.ClearPattern(ctx, "taxonomy:*")
	_ = t.cache.ClearPattern(ctx, "jobs:search:*")
	_ = t.cache.ClearPattern(ctx, "jobs:home:*")
	_ = t.cache.ClearPattern(ctx, "companies:*")
	_ = t.cache.ClearPattern(ctx, "roles:filtered:*")
	_ = t.cache.Delete(ctx, "industries:all")
	_ = t.cache.Delete(ctx, "locations:active:all")

	return nil
}

// AnalyticsService handles server-side analytics.
type AnalyticsService struct {
	pageViewRepo *repository.PageViewRepository
}

func NewAnalyticsService(pageViewRepo *repository.PageViewRepository) *AnalyticsService {
	return &AnalyticsService{pageViewRepo: pageViewRepo}
}

// RecordPageView hashes the IP and classifies the device type, then saves the page view event.
func (s *AnalyticsService) RecordPageView(ctx context.Context, path, referrer, userAgent, ip, country, salt string) error {
	// Hash IP for GDPR compliance
	hashedIP := fmt.Sprintf("%x", sha256.Sum256([]byte(ip+salt)))

	// Classify device type
	deviceType := classifyDevice(userAgent)

	pv := &models.PageView{
		Path:       path,
		Referrer:   referrer,
		UserAgent:  userAgent,
		IPHash:     hashedIP,
		Country:    country,
		DeviceType: deviceType,
	}

	return s.pageViewRepo.Record(ctx, pv)
}

func classifyDevice(ua string) string {
	uaLower := strings.ToLower(ua)
	if strings.Contains(uaLower, "ipad") || strings.Contains(uaLower, "tablet") || (strings.Contains(uaLower, "android") && !strings.Contains(uaLower, "mobile")) {
		return "tablet"
	}
	if strings.Contains(uaLower, "mobile") || strings.Contains(uaLower, "iphone") || strings.Contains(uaLower, "ipod") || strings.Contains(uaLower, "windows phone") {
		return "mobile"
	}
	return "desktop"
}

// StartExpiryChecker runs periodic job expiry checks and notifies Google of expired URLs.
func StartExpiryChecker(
	ctx context.Context,
	jobRepo *repository.JobRepository,
	sitemapSvc *SitemapService,
	indexing *GoogleIndexingService,
	baseURL string,
	interval time.Duration,
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			expiredSlugs, err := jobRepo.ExpireOldJobs(ctx)
			if err != nil {
				slog.Error("Failed to expire old jobs", "error", err)
				continue
			}
			if len(expiredSlugs) > 0 {
				slog.Info("Expired old jobs", "count", len(expiredSlugs))
				for _, slug := range expiredSlugs {
					jobURL := fmt.Sprintf("%s/jobs/%s/", baseURL, slug)
					go func(urlStr string) {
						_ = indexing.PublishDelete(context.Background(), urlStr)
					}(jobURL)
				}
				// Rebuild sitemaps since sitemaps changed
				_ = sitemapSvc.RebuildAll(ctx)
			}
		}
	}
}
