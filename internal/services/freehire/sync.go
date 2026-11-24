package freehire

import (
	"context"
	"fmt"
	crand "crypto/rand"
	"log/slog"
	"math/big"
	"net/url"
	"regexp"
	"strings"
	"time"

	"jobyuk/internal/models"
	"jobyuk/internal/repository"
	"jobyuk/internal/services"
)

// SyncService handles background synchronization of FreeHire API data.
type SyncService struct {
	client   *Client
	repos    *repository.Repositories
	taxonomy *services.TaxonomyService
	indexing *services.GoogleIndexingService
	baseURL  string
}

// NewSyncService creates a new FreeHire sync service.
func NewSyncService(
	client *Client,
	repos *repository.Repositories,
	taxonomy *services.TaxonomyService,
	indexing *services.GoogleIndexingService,
	baseURL string,
) *SyncService {
	return &SyncService{
		client:   client,
		repos:    repos,
		taxonomy: taxonomy,
		indexing: indexing,
		baseURL:  baseURL,
	}
}

// StartScheduled runs the FreeHire sync process on a schedule.
func (s *SyncService) StartScheduled(ctx context.Context, interval time.Duration) {
	slog.Info("Starting FreeHire API sync worker", "interval", interval)

	// Stagger initial sync to avoid CPU/DB collision on boot
	select {
	case <-time.After(10 * time.Minute):
	case <-ctx.Done():
		return
	}
	s.RunSync(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("FreeHire API sync worker stopped")
			return
		case <-ticker.C:
			s.RunSync(ctx)
		}
	}
}

// RunSync performs a single sync cycle from FreeHire API with strict direct-link filtering and zero duplication.
func (s *SyncService) RunSync(ctx context.Context) {
	slog.Info("Starting FreeHire API sync cycle")
	start := time.Now()

	limit := 100
	maxPages := 10 // Scans up to 1,000 latest UK jobs per cycle

	var totalNew, totalUpgraded, totalSkipped int

	for page := 0; page < maxPages; page++ {
		if ctx.Err() != nil {
			return
		}

		offset := page * limit
		jobs, err := s.client.SearchUKJobs(ctx, limit, offset)
		if err != nil {
			slog.Error("Failed to fetch jobs from FreeHire", "page", page, "offset", offset, "error", err)
			break
		}

		if len(jobs) == 0 {
			break
		}

		slog.Info("Processing FreeHire batch", "page", page+1, "jobs_count", len(jobs))

		for _, fhJob := range jobs {
			if ctx.Err() != nil {
				return
			}

			// 1. Regional verification
			if !isUKJob(fhJob) {
				continue
			}

			// 2. Strict Direct Link Filter: reject aggregators
			if !IsTargetableUKDirectLink(fhJob.URL) {
				totalSkipped++
				continue
			}

			// 3. Ensure Company
			companyID, err := s.ensureCompany(ctx, fhJob.Company, fhJob.CompanySlug)
			var empIDPtr *int64
			if err == nil && companyID > 0 {
				empIDPtr = &companyID
			}

			// 4. Zero-Duplication Check: match existing job by URL or Title + Company
			existingJob, err := s.repos.Job.FindExistingMatch(ctx, fhJob.URL, fhJob.Title, empIDPtr)
			if err != nil {
				slog.Error("Error checking duplicate job", "title", fhJob.Title, "error", err)
			}

			if existingJob != nil {
				// Link Upgrade Logic:
				// If the existing job's external URL is not a direct ATS link, but FreeHire has a Direct ATS link, upgrade it!
				if IsDirectATS(fhJob.URL) && !IsDirectATS(existingJob.ExternalURL) {
					if err := s.repos.Job.UpgradeJobApplyURL(ctx, existingJob.ID, fhJob.URL); err == nil {
						slog.Info("Upgraded job apply link to Direct ATS", "job_id", existingJob.ID, "title", existingJob.Title, "url", fhJob.URL)
						totalUpgraded++
					}
				} else {
					totalSkipped++
				}
				continue
			}

			// Check if already indexed by external_slug
			if fhJob.PublicSlug != "" {
				exists, _ := s.repos.Job.ExistsByExternalSlug(ctx, "freehire", fhJob.PublicSlug)
				if exists {
					totalSkipped++
					continue
				}
			}

			// 5. Map FreeHire job to models.Job
			job := s.mapFreeHireJobToModel(fhJob, empIDPtr)

			// 6. Save job
			if err := s.repos.Job.Create(ctx, &job); err != nil {
				slog.Error("Failed to save FreeHire job", "title", fhJob.Title, "error", err)
				continue
			}

			// 7. Map taxonomy (roles & industries)
			s.taxonomy.MapJobToTaxonomy(ctx, job.ID, job.Title, job.Description)

			// 8. Notify Google Indexing API of new job URL
			go func(jobSlug string) {
				jobURL := fmt.Sprintf("%s/jobs/%s/", s.baseURL, jobSlug)
				_ = s.indexing.PublishUpdate(context.Background(), jobURL)
			}(job.Slug)

			totalNew++
			time.Sleep(100 * time.Millisecond) // Polite delay to prevent CPU spikes
		}

		time.Sleep(200 * time.Millisecond) // Polite delay between pages
	}

	// Update taxonomy job counts
	if err := s.taxonomy.UpdateAllCounts(ctx); err != nil {
		slog.Error("Failed to update taxonomy counts after FreeHire sync", "error", err)
	}

	duration := time.Since(start)
	slog.Info("FreeHire API sync cycle complete",
		"new_jobs", totalNew,
		"upgraded_links", totalUpgraded,
		"skipped_duplicates_or_aggregators", totalSkipped,
		"duration", duration,
	)
}

// isUKJob verifies whether the FreeHire job belongs to the UK.
func isUKJob(j FreeHireJob) bool {
	for _, r := range j.Regions {
		if strings.ToLower(r) == "uk" {
			return true
		}
	}
	for _, c := range j.Countries {
		if strings.ToLower(c) == "gb" || strings.ToLower(c) == "uk" {
			return true
		}
	}
	locLower := strings.ToLower(j.Location)
	return strings.Contains(locLower, "united kingdom") ||
		strings.Contains(locLower, "uk") ||
		strings.Contains(locLower, "london") ||
		strings.Contains(locLower, "manchester") ||
		strings.Contains(locLower, "birmingham") ||
		strings.Contains(locLower, "scotland") ||
		strings.Contains(locLower, "england") ||
		strings.Contains(locLower, "wales")
}

// ensureCompany finds or creates a company record from FreeHire data.
func (s *SyncService) ensureCompany(ctx context.Context, name string, slug string) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.EqualFold(name, "unavailable") {
		name = "Confidential Employer"
	}

	if slug == "" {
		slug = generateSlug(name)
	}

	logoURL := fmt.Sprintf("https://logo.freehire.me/%s", url.PathEscape(name))

	// 1. Check if company exists by slug or name
	company, err := s.repos.Company.FindBySlug(ctx, slug)
	if err == nil && company != nil {
		if company.LogoURL == "" {
			_ = s.repos.Company.UpdateLogoURL(ctx, company.ID, logoURL)
		}
		return company.ID, nil
	}

	company, err = s.repos.Company.FindByName(ctx, name)
	if err == nil && company != nil {
		if company.LogoURL == "" {
			_ = s.repos.Company.UpdateLogoURL(ctx, company.ID, logoURL)
		}
		return company.ID, nil
	}

	// 2. Create company
	newCompany := &models.Company{
		Slug:    slug,
		Name:    name,
		LogoURL: logoURL,
	}

	if err := s.repos.Company.Create(ctx, newCompany); err != nil {
		return 0, fmt.Errorf("failed to create company: %w", err)
	}

	return newCompany.ID, nil
}

// mapFreeHireJobToModel maps FreeHire job data to the internal Job model.
func (s *SyncService) mapFreeHireJobToModel(fh FreeHireJob, employerID *int64) models.Job {
	job := models.Job{
		Source:       "freehire",
		ExternalSlug: fh.PublicSlug,
		Title:        fh.Title,
		Description:  fh.Description,
		LocationName: fh.Location,
		EmployerID:   employerID,
		ExternalURL:  fh.URL,
		Status:       "staged",
		Currency:     "GBP",
	}

	// Assign LocationID if resolved from taxonomy
	if locID := s.taxonomy.FindLocationID(context.Background(), fh.Location); locID > 0 {
		job.LocationID = &locID
	}

	// Parse Posted Date
	if fh.PostedAt != "" {
		if t, err := time.Parse(time.RFC3339, fh.PostedAt); err == nil {
			job.DatePosted = t
		} else if t, err := time.Parse("2006-01-02T15:04:05Z", fh.PostedAt); err == nil {
			job.DatePosted = t
		} else if t, err := time.Parse("2006-01-02", fh.PostedAt); err == nil {
			job.DatePosted = t
		} else {
			job.DatePosted = time.Now()
		}
	} else {
		job.DatePosted = time.Now()
	}

	// Parse Expiration / Closed Date
	if fh.ClosedAt != nil && *fh.ClosedAt != "" {
		if t, err := time.Parse(time.RFC3339, *fh.ClosedAt); err == nil {
			job.ExpirationDate = &t
			job.Status = "expired"
		}
	} else {
		// Standard 30-day expiration window for active UK jobs (Google for Jobs best practice)
		exp := job.DatePosted.AddDate(0, 0, 30)
		job.ExpirationDate = &exp
	}

	// Work Mode detection
	workModeLower := strings.ToLower(fh.WorkMode)
	titleLower := strings.ToLower(fh.Title)
	descLower := strings.ToLower(fh.Description)

	job.IsRemote = workModeLower == "remote" ||
		strings.Contains(titleLower, "remote") ||
		strings.Contains(titleLower, "work from home") ||
		strings.Contains(descLower, "fully remote") ||
		strings.Contains(descLower, "100% remote")

	// Graduate detection
	job.IsGraduate = strings.Contains(titleLower, "graduate") ||
		strings.Contains(titleLower, "entry level") ||
		strings.Contains(titleLower, "junior") ||
		strings.Contains(titleLower, "trainee")

	job.ContractType = "Permanent"
	job.JobType = "full_time"

	// Generate clean SEO slug with unique parameter: {title}-{location}-{cleanCompany}-{token}
	token := generateJobyUKCode(8)
	job.Slug = BuildSEOSlug(fh.Title, fh.Location, job.IsRemote, fh.Company, token)

	return job
}

func generateJobyUKCode(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		num, err := crand.Int(crand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			b[i] = chars[time.Now().UnixNano()%int64(len(chars))]
		} else {
			b[i] = chars[num.Int64()]
		}
	}
	return string(b)
}

func cleanCompanySlug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	suffixes := []string{
		" careers", " career", " ltd", " limited", " plc", " inc", " llc", " group", " services",
		"-careers", "-career", "-ltd", "-limited", "-plc", "-inc", "-llc",
	}
	for _, suf := range suffixes {
		s = strings.TrimSuffix(s, suf)
	}
	slug := generateSlug(s)
	for _, suf := range []string{"-careers", "-career", "-ltd", "-plc", "-limited", "-inc", "-llc"} {
		slug = strings.TrimSuffix(slug, suf)
	}
	return slug
}

func cleanLocationSlug(loc string, isRemote bool) string {
	locLower := strings.ToLower(strings.TrimSpace(loc))
	if isRemote {
		return "remote"
	}
	if locLower == "" {
		return ""
	}

	parts := strings.Split(locLower, ",")
	primary := strings.TrimSpace(parts[0])
	primary = strings.TrimSuffix(primary, " uk")
	primary = strings.TrimSuffix(primary, " united kingdom")
	primary = strings.TrimSpace(primary)

	slug := generateSlug(primary)
	if slug == "" || slug == "united-kingdom" || slug == "uk" || slug == "united-states" || slug == "multiple-locations" || slug == "various-locations" {
		return ""
	}
	return slug
}

// BuildSEOSlug constructs an optimized, human-friendly URL slug containing Title, Location, and clean Company name.
func BuildSEOSlug(title string, location string, isRemote bool, company string, token string) string {
	titleSlug := generateSlug(title)
	locSlug := cleanLocationSlug(location, isRemote)
	compSlug := cleanCompanySlug(company)

	var parts []string
	parts = append(parts, titleSlug)

	if locSlug != "" && !strings.Contains(titleSlug, locSlug) {
		parts = append(parts, locSlug)
	}

	if compSlug != "" && !strings.Contains(titleSlug, compSlug) {
		parts = append(parts, compSlug)
	}

	base := strings.Join(parts, "-")
	if len(base) > 65 {
		base = base[:65]
		if i := strings.LastIndex(base, "-"); i > 30 {
			base = base[:i]
		}
	}
	base = strings.Trim(base, "-")
	return fmt.Sprintf("%s-%s", base, token)
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
