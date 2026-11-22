package reed

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"jobyuk/internal/models"
	"jobyuk/internal/repository"
	"jobyuk/internal/services"
)

// SyncService handles background synchronization of Reed API data.
type SyncService struct {
	client   *Client
	repos    *repository.Repositories
	taxonomy     *services.TaxonomyService
	indexing     *services.GoogleIndexingService
	baseURL      string
	onJobsSynced func(ctx context.Context)
}

// NewSyncService creates a new sync service.
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

// SetOnJobsSynced registers a callback triggered when new jobs are ingested.
func (s *SyncService) SetOnJobsSynced(cb func(ctx context.Context)) {
	s.onJobsSynced = cb
}

// StartScheduled runs the sync process on a schedule.
func (s *SyncService) StartScheduled(ctx context.Context, interval time.Duration) {
	slog.Info("Starting Reed API sync worker", "interval", interval)

	// Stagger initial sync to avoid CPU/DB collision on boot
	select {
	case <-time.After(5 * time.Minute):
	case <-ctx.Done():
		return
	}
	s.RunSync(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Reed API sync worker stopped")
			return
		case <-ticker.C:
			s.RunSync(ctx)
		}
	}
}

// RunSync performs a single sync cycle.
func (s *SyncService) RunSync(ctx context.Context) {
	slog.Info("Starting Reed API sync cycle")
	start := time.Now()

	// Top search categories to sync
	categories := []SearchParams{
		{Keywords: "software developer"},
		{Keywords: "software engineer"},
		{Keywords: "data analyst"},
		{Keywords: "project manager"},
		{Keywords: "accountant"},
		{Keywords: "nurse"},
		{Keywords: "marketing manager"},
		{Keywords: "sales"},
		{Keywords: "teacher"},
		{Keywords: "hr"},
	}

	var totalNew, totalUpdated int

	for _, params := range categories {
		if ctx.Err() != nil {
			return
		}

		params.ResultsToTake = 100

		result, err := s.client.Search(ctx, params)
		if err != nil {
			slog.Error("Reed search failed",
				"keywords", params.Keywords,
				"error", err,
			)
			continue
		}

		slog.Info("Reed search results",
			"keywords", params.Keywords,
			"total", result.TotalResults,
			"returned", len(result.Results),
		)

		for _, reedJob := range result.Results {
			if ctx.Err() != nil {
				return
			}

			// Check if job already exists
			exists, err := s.repos.Job.ExistsByReedID(ctx, reedJob.JobID)
			if err != nil {
				slog.Error("Failed to check job existence", "reed_id", reedJob.JobID, "error", err)
				continue
			}

			if exists {
				totalUpdated++
				continue
			}

			// Fetch full job details
			detail, err := s.client.GetJobDetails(ctx, reedJob.JobID)
			if err != nil {
				slog.Error("Failed to fetch job details", "reed_id", reedJob.JobID, "error", err)
				continue
			}

			// Find or create company
			companyID, err := s.ensureCompany(ctx, detail.EmployerID, detail.EmployerName, detail.JobURL)
			var empIDPtr *int64
			if err == nil && companyID > 0 {
				empIDPtr = &companyID
			}

			// Zero-Duplication Check against existing jobs (from any source)
			existingMatch, _ := s.repos.Job.FindExistingMatch(ctx, detail.ExternalURL, detail.JobTitle, empIDPtr)
			if existingMatch != nil {
				// If existing job has no external URL, and Reed provides one, update it
				if existingMatch.ExternalURL == "" && detail.ExternalURL != "" {
					_ = s.repos.Job.UpgradeJobApplyURL(ctx, existingMatch.ID, detail.ExternalURL)
				}
				totalUpdated++
				continue
			}

			// Map to our model and save
			job := mapReedJobToModel(detail)
			job.EmployerID = empIDPtr

			// Find location
			locationID := s.taxonomy.FindLocationID(ctx, detail.LocationName)
			if locationID > 0 {
				job.LocationID = &locationID
			}

			// Save job
			if err := s.repos.Job.Create(ctx, &job); err != nil {
				slog.Error("Failed to save job", "reed_id", reedJob.JobID, "error", err)
				continue
			}

			// Map to roles/industries
			s.taxonomy.MapJobToTaxonomy(ctx, job.ID, job.Title, job.Description)

			// Notify Google Indexing API of new job URL
			go func(jobSlug string) {
				jobURL := fmt.Sprintf("%s/jobs/%s/", s.baseURL, jobSlug)
				_ = s.indexing.PublishUpdate(context.Background(), jobURL)
			}(job.Slug)

			totalNew++
			time.Sleep(100 * time.Millisecond) // Polite delay to prevent 1-core CPU lock
		}
	}

	// Update job counts on taxonomy
	if err := s.taxonomy.UpdateAllCounts(ctx); err != nil {
		slog.Error("Failed to update taxonomy counts", "error", err)
	}

	duration := time.Since(start)
	slog.Info("Reed API sync cycle complete",
		"new_jobs", totalNew,
		"existing_jobs", totalUpdated,
		"duration", duration,
	)

	if totalNew > 0 && s.onJobsSynced != nil {
		go s.onJobsSynced(context.Background())
	}
}

// TriggerSync allows manual sync from admin dashboard.
func (s *SyncService) TriggerSync(ctx context.Context) {
	go s.RunSync(ctx)
}

// ensureCompany finds or creates a company record.
func (s *SyncService) ensureCompany(ctx context.Context, reedEmployerID int64, name string, jobURL string) (int64, error) {
	// Check if exists
	company, err := s.repos.Company.FindByReedID(ctx, reedEmployerID)
	if err == nil {
		// If existing company has no logo, try to fetch one
		if company.LogoURL == "" && jobURL != "" {
			if logoURL := s.client.FetchEmployerLogoURL(ctx, jobURL); logoURL != "" {
				_ = s.repos.Company.UpdateLogoURL(ctx, company.ID, logoURL)
				slog.Info("Updated employer logo", "company", name, "logo", logoURL)
			}
		}
		return company.ID, nil
	}

	// Create new company
	newCompany := &models.Company{
		ReedEmployerID: &reedEmployerID,
		Slug:           generateSlug(name),
		Name:           name,
	}

	if err := s.repos.Company.Create(ctx, newCompany); err != nil {
		return 0, fmt.Errorf("failed to create company: %w", err)
	}

	// Try to fetch employer logo from Reed job page
	if jobURL != "" {
		if logoURL := s.client.FetchEmployerLogoURL(ctx, jobURL); logoURL != "" {
			_ = s.repos.Company.UpdateLogoURL(ctx, newCompany.ID, logoURL)
			newCompany.LogoURL = logoURL
			slog.Info("Fetched employer logo for new company", "company", name, "logo", logoURL)
		}
	}

	return newCompany.ID, nil
}

// mapReedJobToModel converts a Reed API job to our internal model.
func mapReedJobToModel(detail *JobDetail) models.Job {
	jobID := detail.JobID
	job := models.Job{
		ReedJobID:    &jobID,
		Source:       "reed",
		Title:        detail.JobTitle,
		Description:  detail.JobDescription,
		LocationName: detail.LocationName,
		MinSalary:    detail.MinimumSalary,
		MaxSalary:    detail.MaximumSalary,
		YearlyMin:    detail.YearlyMinimumSalary,
		YearlyMax:    detail.YearlyMaximumSalary,
		Currency:     detail.Currency,
		SalaryType:   normalizeSalaryType(detail.SalaryType),
		ContractType: normalizeContractType(detail.ContractType),
		JobType:      normalizeJobType(detail.JobType),
		ExternalURL:  detail.ExternalURL,
		ReedURL:      detail.JobURL,
		Status:       "active",
	}

	// Generate slug
	slug := generateSlug(detail.JobTitle)
	if detail.LocationName != "" {
		slug += "-" + generateSlug(detail.LocationName)
	}
	slug += fmt.Sprintf("-%d", detail.JobID)
	job.Slug = slug

	// Parse dates
	if t, err := time.Parse("02/01/2006", detail.DatePosted); err == nil {
		job.DatePosted = t
	} else {
		job.DatePosted = time.Now()
	}

	if detail.ExpirationDate != "" {
		if t, err := time.Parse("02/01/2006", detail.ExpirationDate); err == nil {
			job.ExpirationDate = &t
		}
	}

	// Detect remote jobs
	titleLower := strings.ToLower(detail.JobTitle)
	descLower := strings.ToLower(detail.JobDescription)
	job.IsRemote = strings.Contains(titleLower, "remote") ||
		strings.Contains(titleLower, "work from home") ||
		strings.Contains(descLower, "fully remote") ||
		strings.Contains(descLower, "100% remote")

	// Detect graduate jobs
	job.IsGraduate = strings.Contains(titleLower, "graduate") ||
		strings.Contains(titleLower, "entry level") ||
		strings.Contains(titleLower, "junior")

	// Compute yearly salary if not provided
	if job.YearlyMin == nil && job.MinSalary != nil {
		yearly := convertToYearly(*job.MinSalary, job.SalaryType)
		job.YearlyMin = &yearly
	}
	if job.YearlyMax == nil && job.MaxSalary != nil {
		yearly := convertToYearly(*job.MaxSalary, job.SalaryType)
		job.YearlyMax = &yearly
	}

	return job
}

var slugRegex = regexp.MustCompile(`[^a-z0-9]+`)

func generateSlug(s string) string {
	slug := strings.ToLower(strings.TrimSpace(s))
	slug = slugRegex.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	// Limit slug length
	if len(slug) > 80 {
		slug = slug[:80]
		// Don't cut in the middle of a word
		if i := strings.LastIndex(slug, "-"); i > 40 {
			slug = slug[:i]
		}
	}
	return slug
}

func normalizeSalaryType(s string) string {
	s = strings.ToLower(s)
	switch {
	case strings.Contains(s, "hour"):
		return "per_hour"
	case strings.Contains(s, "day"):
		return "per_day"
	case strings.Contains(s, "week"):
		return "per_week"
	case strings.Contains(s, "month"):
		return "per_month"
	default:
		return "per_annum"
	}
}

func normalizeContractType(s string) string {
	s = strings.ToLower(s)
	switch {
	case strings.Contains(s, "permanent"):
		return "permanent"
	case strings.Contains(s, "contract"):
		return "contract"
	case strings.Contains(s, "temp"):
		return "temporary"
	default:
		return "permanent"
	}
}

func normalizeJobType(s string) string {
	s = strings.ToLower(s)
	if strings.Contains(s, "part") {
		return "part_time"
	}
	return "full_time"
}

func convertToYearly(amount float64, salaryType string) float64 {
	switch salaryType {
	case "per_hour":
		return amount * 37.5 * 52 // 37.5 hours/week, 52 weeks
	case "per_day":
		return amount * 260 // 260 working days
	case "per_week":
		return amount * 52
	case "per_month":
		return amount * 12
	default:
		return amount
	}
}
