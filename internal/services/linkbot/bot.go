package linkbot

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"jobyuk/internal/repository"
	"jobyuk/internal/services"
)

// Bot manages scheduled and on-demand verification of job listing destination links.
type Bot struct {
	db           *pgxpool.Pool
	jobRepo      *repository.JobRepository
	sitemapSvc   *services.SitemapService
	indexingSvc  *services.GoogleIndexingService
	baseURL      string
	checker      *Checker
	batchSize    int
	concurrency  int
	mu           sync.Mutex
	isProcessing bool
}

// JobCandidate holds fields needed to verify and update a job listing.
type JobCandidate struct {
	ID             int64
	Slug           string
	ExternalURL    string
	ReedURL        string
	DatePosted     time.Time
	ExpirationDate *time.Time
	FailCount      int
}

// NewBot constructs a link verification bot instance.
func NewBot(
	db *pgxpool.Pool,
	jobRepo *repository.JobRepository,
	sitemapSvc *services.SitemapService,
	indexingSvc *services.GoogleIndexingService,
	baseURL string,
) *Bot {
	return &Bot{
		db:          db,
		jobRepo:     jobRepo,
		sitemapSvc:  sitemapSvc,
		indexingSvc: indexingSvc,
		baseURL:     baseURL,
		checker:     NewChecker(10 * time.Second),
		batchSize:   35,
		concurrency: 3,
	}
}

// AutoMigrate applies schema enhancements to the jobs table for tracking link health.
func (b *Bot) AutoMigrate(ctx context.Context) error {
	queries := []string{
		`ALTER TABLE jobs ADD COLUMN IF NOT EXISTS last_link_checked_at TIMESTAMPTZ`,
		`ALTER TABLE jobs ADD COLUMN IF NOT EXISTS link_http_status INT DEFAULT 0`,
		`ALTER TABLE jobs ADD COLUMN IF NOT EXISTS link_status VARCHAR(20) DEFAULT 'unknown'`,
		`ALTER TABLE jobs ADD COLUMN IF NOT EXISTS link_fail_count INT DEFAULT 0`,
		`ALTER TABLE jobs ADD COLUMN IF NOT EXISTS link_check_reason TEXT DEFAULT ''`,
		`CREATE INDEX IF NOT EXISTS idx_jobs_link_health ON jobs(status, last_link_checked_at ASC NULLS FIRST) WHERE status = 'active'`,
	}

	for _, q := range queries {
		if _, err := b.db.Exec(ctx, q); err != nil {
			return fmt.Errorf("migration query failed (%s): %w", q, err)
		}
	}

	slog.Info("[LinkBot] Database schema columns and index verified successfully")
	return nil
}

// Start begins the scheduled background link health check loop.
func (b *Bot) Start(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 15 * time.Minute
	}

	slog.Info("[LinkBot] Link Health & Expiry Bot started", "interval", interval.String(), "batchSize", b.batchSize)

	// Small initial delay before first pass to allow server startup to finish
	select {
	case <-ctx.Done():
		return
	case <-time.After(20 * time.Second):
		b.safeRunCycle(ctx)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("[LinkBot] Stopping link health bot...")
			return
		case <-ticker.C:
			b.safeRunCycle(ctx)
		}
	}
}

func (b *Bot) safeRunCycle(ctx context.Context) {
	b.mu.Lock()
	if b.isProcessing {
		b.mu.Unlock()
		slog.Debug("[LinkBot] Cycle already in progress, skipping")
		return
	}
	b.isProcessing = true
	b.mu.Unlock()

	defer func() {
		b.mu.Lock()
		b.isProcessing = false
		b.mu.Unlock()
	}()

	checked, expired, err := b.RunCycle(ctx)
	if err != nil {
		slog.Error("[LinkBot] Cycle encountered an error", "error", err)
	} else if checked > 0 {
		slog.Info("[LinkBot] Cycle completed", "checked", checked, "expired", expired)
	}
}

// RunCycle executes a prioritized verification pass on a batch of active jobs.
func (b *Bot) RunCycle(ctx context.Context) (int, int, error) {
	// Query prioritizing:
	// 1. Unchecked active jobs (last_link_checked_at IS NULL)
	// 2. Oldest checked active jobs
	rows, err := b.db.Query(ctx, `
		SELECT id, slug, COALESCE(external_url, ''), COALESCE(reed_url, ''), date_posted, expiration_date, link_fail_count
		FROM jobs
		WHERE status = 'active'
		ORDER BY 
			(last_link_checked_at IS NULL) DESC,
			last_link_checked_at ASC NULLS FIRST
		LIMIT $1
	`, b.batchSize)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to query jobs for link check: %w", err)
	}
	defer rows.Close()

	var candidates []JobCandidate
	for rows.Next() {
		var j JobCandidate
		if err := rows.Scan(&j.ID, &j.Slug, &j.ExternalURL, &j.ReedURL, &j.DatePosted, &j.ExpirationDate, &j.FailCount); err != nil {
			return 0, 0, fmt.Errorf("failed to scan candidate job: %w", err)
		}
		candidates = append(candidates, j)
	}

	if len(candidates) == 0 {
		return 0, 0, nil
	}

	jobsChan := make(chan JobCandidate, len(candidates))
	for _, j := range candidates {
		jobsChan <- j
	}
	close(jobsChan)

	var wg sync.WaitGroup
	var expiredCountMu sync.Mutex
	expiredCount := 0
	var newlyExpiredSlugs []string

	workerCount := b.concurrency
	if workerCount > len(candidates) {
		workerCount = len(candidates)
	}

	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobsChan {
				select {
				case <-ctx.Done():
					return
				default:
				}

				targetURL := job.ExternalURL
				if targetURL == "" {
					targetURL = job.ReedURL
				}

				// Check the link
				res := b.checker.CheckURL(ctx, targetURL)

				// Apply health update and policy
				expired := b.applyLinkResult(ctx, job, res)
				if expired {
					expiredCountMu.Lock()
					expiredCount++
					newlyExpiredSlugs = append(newlyExpiredSlugs, job.Slug)
					expiredCountMu.Unlock()
				}

				// Polite delay to prevent connection saturation
				time.Sleep(150 * time.Millisecond)
			}
		}()
	}

	wg.Wait()

	// Notify Google Indexing API & Rebuild Sitemaps for expired listings
	if len(newlyExpiredSlugs) > 0 {
		slog.Info("[LinkBot] Immediately notifying Google Indexing API of closed jobs", "count", len(newlyExpiredSlugs))
		for _, slug := range newlyExpiredSlugs {
			jobURL := fmt.Sprintf("%s/jobs/%s/", b.baseURL, slug)
			go func(u string) {
				_ = b.indexingSvc.PublishDelete(context.Background(), u)
			}(jobURL)
		}

		go func() {
			time.Sleep(2 * time.Second)
			_ = b.sitemapSvc.RebuildAll(context.Background())
		}()
	}

	return len(candidates), expiredCount, nil
}

// applyLinkResult updates database records according to link verification findings.
func (b *Bot) applyLinkResult(ctx context.Context, job JobCandidate, res CheckResult) bool {
	switch res.Status {
	case StatusDead:
		// Hard 404, 410, or verified ATS soft 404 -> Expire job immediately!
		_, err := b.db.Exec(ctx, `
			UPDATE jobs
			SET status = 'expired',
				link_status = 'dead',
				link_http_status = $1,
				link_check_reason = $2,
				last_link_checked_at = NOW(),
				updated_at = NOW()
			WHERE id = $3
		`, res.StatusCode, res.Reason, job.ID)
		if err != nil {
			slog.Error("[LinkBot] Failed to mark job as expired", "job_id", job.ID, "error", err)
			return false
		}
		slog.Warn("[LinkBot] Closed expired vacancy",
			"job_id", job.ID,
			"slug", job.Slug,
			"http_status", res.StatusCode,
			"reason", res.Reason,
		)
		return true

	case StatusActive:
		// Destination is LIVE and actively accepting applications!
		// Automatically extend expiration date so it does NOT get expired by naive 30-day logic.
		_, err := b.db.Exec(ctx, `
			UPDATE jobs
			SET link_status = 'active',
				link_http_status = $1,
				link_fail_count = 0,
				link_check_reason = $2,
				last_link_checked_at = NOW(),
				expiration_date = CASE
					WHEN expiration_date IS NULL OR expiration_date < NOW() + INTERVAL '7 days'
					THEN NOW() + INTERVAL '14 days'
					ELSE expiration_date
				END,
				updated_at = NOW()
			WHERE id = $3
		`, res.StatusCode, res.Reason, job.ID)
		if err != nil {
			slog.Error("[LinkBot] Failed to update active job status", "job_id", job.ID, "error", err)
		} else {
			slog.Debug("[LinkBot] Verified job alive & extended validity", "job_id", job.ID, "slug", job.Slug)
		}
		return false

	case StatusUnreachable:
		// Transient network failure or 5xx
		newFailCount := job.FailCount + 1
		if newFailCount >= 3 {
			// After 3 consecutive failed checks over multiple runs, expire the dead vacancy
			_, err := b.db.Exec(ctx, `
				UPDATE jobs
				SET status = 'expired',
					link_status = 'unreachable',
					link_http_status = $1,
					link_fail_count = $2,
					link_check_reason = $3,
					last_link_checked_at = NOW(),
					updated_at = NOW()
				WHERE id = $4
			`, res.StatusCode, newFailCount, "Expired: 3 consecutive unreachable attempts ("+res.Reason+")", job.ID)
			if err != nil {
				slog.Error("[LinkBot] Failed to expire unreachable job", "job_id", job.ID, "error", err)
				return false
			}
			slog.Warn("[LinkBot] Closed repeatedly unreachable vacancy", "job_id", job.ID, "failCount", newFailCount)
			return true
		}

		// Otherwise just record the failure count and retry in next cycle
		_, err := b.db.Exec(ctx, `
			UPDATE jobs
			SET link_status = 'unreachable',
				link_http_status = $1,
				link_fail_count = $2,
				link_check_reason = $3,
				last_link_checked_at = NOW(),
				updated_at = NOW()
			WHERE id = $4
		`, res.StatusCode, newFailCount, res.Reason, job.ID)
		if err != nil {
			slog.Error("[LinkBot] Failed to record unreachable link check", "job_id", job.ID, "error", err)
		}
		return false

	default:
		return false
	}
}

// GetStats returns current health statistics of the jobs database.
func (b *Bot) GetStats(ctx context.Context) (map[string]int, error) {
	rows, err := b.db.Query(ctx, `
		SELECT COALESCE(link_status, 'unknown'), COUNT(*)
		FROM jobs
		WHERE status = 'active'
		GROUP BY link_status
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var st string
		var count int
		if err := rows.Scan(&st, &count); err == nil {
			stats[st] = count
		}
	}
	return stats, nil
}
