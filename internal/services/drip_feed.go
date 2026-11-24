package services

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DripFeedService handles the organic, progressive publishing of jobs from 'staged' to 'active'.
// It simulates natural human recruiting patterns across UK business hours with anti-bot jitter.
type DripFeedService struct {
	db         *pgxpool.Pool
	sitemap    *SitemapService
	indexNow   *IndexNowService
	baseURL    string
	launchDate time.Time
	mu         sync.Mutex
	lastPublish time.Time
}

// NewDripFeedService creates and initializes a DripFeedService.
func NewDripFeedService(db *pgxpool.Pool, sitemap *SitemapService, indexNow *IndexNowService, baseURL string) *DripFeedService {
	prodURL := "https://jobyuk.com"
	if baseURL != "" && !strings.Contains(baseURL, "localhost") && !strings.Contains(baseURL, "127.0.0.1") {
		prodURL = strings.TrimRight(baseURL, "/")
	}

	// Default launch date is today's midnight UTC, or configured via DRIP_FEED_START_DATE (e.g. 2026-11-23)
	launchDate := time.Now().UTC().Truncate(24 * time.Hour)
	if envDate := os.Getenv("DRIP_FEED_START_DATE"); envDate != "" {
		if parsed, err := time.Parse("2006-01-02", envDate); err == nil {
			launchDate = parsed.UTC()
		}
	}

	return &DripFeedService{
		db:         db,
		sitemap:    sitemap,
		indexNow:   indexNow,
		baseURL:    prodURL,
		launchDate: launchDate,
	}
}

// GetDailyQuota returns the target number of jobs to publish on a given day of the campaign (1..30+).
func (s *DripFeedService) GetDailyQuota(day int) int {
	if day < 1 {
		day = 1
	}
	switch day {
	case 1:
		return 20 // Day 1: Exactly 20 jobs
	case 2:
		return 25 // Day 2: 25 jobs
	case 3:
		return 30 // Day 3: 30 jobs
	case 4:
		return 40
	case 5:
		return 50 // Day 5: 50 jobs
	case 6:
		return 60
	case 7:
		return 70 // Day 7: 70 jobs
	case 8:
		return 85
	case 9:
		return 100
	case 10:
		return 100 // Day 10: 100 jobs
	case 11:
		return 120
	case 12:
		return 120 // Day 12: 120 jobs
	case 13:
		return 130
	case 14:
		return 140 // Day 14: 140 jobs
	case 15:
		return 150
	case 16:
		return 160
	case 17:
		return 170 // Day 17: 170 jobs
	case 18:
		return 180
	case 19:
		return 190
	case 20:
		return 200 // Day 20: 200 jobs
	case 21:
		return 220
	case 22:
		return 235
	case 23:
		return 250 // Day 23: 250 jobs
	case 24:
		return 270
	case 25:
		return 285
	case 26:
		return 300 // Day 26: 300 jobs
	case 27:
		return 350
	case 28:
		return 400
	case 29:
		return 450
	default:
		return 450 // Day 30+: 450 jobs/day established cadence
	}
}

// InitStagedPool ensures the database starts in clean drip-feed mode:
// If there are many active jobs on fresh launch, retain exactly Day 1 quota (20) as active,
// and move all remaining jobs into the 'staged' pool.
func (s *DripFeedService) InitStagedPool(ctx context.Context) error {
	var activeCount, stagedCount int
	_ = s.db.QueryRow(ctx, "SELECT count(*) FROM jobs WHERE status = 'active'").Scan(&activeCount)
	_ = s.db.QueryRow(ctx, "SELECT count(*) FROM jobs WHERE status = 'staged'").Scan(&stagedCount)

	slog.Info("Drip-feed pool status check", "active_jobs", activeCount, "staged_jobs", stagedCount)

	// If staged count is 0 and active count is high (> Day 1 quota 20), stage the overflow
	if stagedCount == 0 && activeCount > 20 {
		day1Quota := 20
		query := `
			WITH keep_active AS (
				SELECT id FROM jobs 
				WHERE status = 'active' 
				ORDER BY (employer_id IS NOT NULL) DESC, is_remote DESC, yearly_max DESC, id ASC 
				LIMIT $1
			)
			UPDATE jobs 
			SET status = 'staged', updated_at = NOW() 
			WHERE status = 'active' AND id NOT IN (SELECT id FROM keep_active)
		`
		tag, err := s.db.Exec(ctx, query, day1Quota)
		if err != nil {
			return fmt.Errorf("failed to initialize staged pool: %w", err)
		}
		slog.Info("Initialized Drip-Feed staged pool", "jobs_staged", tag.RowsAffected(), "active_retained", day1Quota)

		// Invalidate sitemap cache so it reflects the clean 20 Day-1 jobs
		if s.sitemap != nil {
			s.sitemap.InvalidateJobsCache()
		}
	}
	return nil
}

// StartWorker runs the background drip-feed scheduler.
func (s *DripFeedService) StartWorker(ctx context.Context) {
	// First initialize staged pool if needed
	if err := s.InitStagedPool(ctx); err != nil {
		slog.Warn("Notice on InitStagedPool", "error", err)
	}

	slog.Info("Starting Drip-Feed Publishing Worker", "launch_date", s.launchDate.Format("2006-01-02"))

	// Check periodically (every 2 to 4 minutes) whether a job is due for release
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.processDrip(ctx)
		}
	}
}

// processDrip determines if a job should be released right now based on UK business hours & quota.
func (s *DripFeedService) processDrip(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	startOfToday := now.Truncate(24 * time.Hour)

	// Calculate campaign day number (Day 1, 2, 3...)
	daysSinceLaunch := int(now.Sub(s.launchDate).Hours() / 24)
	currentDay := daysSinceLaunch + 1
	if currentDay < 1 {
		currentDay = 1
	}

	dailyTarget := s.GetDailyQuota(currentDay)

	// Count jobs published/active today
	var publishedToday int
	err := s.db.QueryRow(ctx, "SELECT count(*) FROM jobs WHERE status = 'active' AND date_posted >= $1", startOfToday).Scan(&publishedToday)
	if err != nil {
		slog.Warn("Drip-feed query error", "error", err)
		return
	}

	if publishedToday >= dailyTarget {
		// Quota for today reached!
		return
	}

	// Calculate time pacing:
	// UK Hour (GMT / UTC)
	ukHour := now.Hour()
	isPeakHours := ukHour >= 8 && ukHour <= 18    // 08:00 - 18:00 GMT: Core UK working day
	isEvening := ukHour > 18 && ukHour <= 23     // 18:00 - 23:00 GMT: Evening posting
	isNight := ukHour < 8                        // 23:00 - 08:00 GMT: Night quiet period

	// Average base interval in minutes needed to distribute remaining quota across remaining hours
	hoursRemaining := 24 - ukHour
	if hoursRemaining < 1 {
		hoursRemaining = 1
	}
	remainingQuota := dailyTarget - publishedToday
	baseIntervalMins := (hoursRemaining * 60) / remainingQuota
	if baseIntervalMins < 4 {
		baseIntervalMins = 4 // Minimum 4 mins between jobs even on high-volume days
	}

	// Adjust interval by time-of-day weight:
	weightMultiplier := 1.0
	if isPeakHours {
		weightMultiplier = 0.75 // 25% faster during peak office hours
	} else if isEvening {
		weightMultiplier = 1.10 // slightly slower in evening
	} else if isNight {
		weightMultiplier = 2.50 // much slower late at night (simulating human rest)
	}

	targetInterval := time.Duration(float64(baseIntervalMins)*weightMultiplier) * time.Minute

	// Add anti-bot random jitter (+/- 20%)
	jitterPercent := 0.80 + rand.Float64()*0.40 // 0.80 to 1.20
	targetInterval = time.Duration(float64(targetInterval) * jitterPercent)

	// Check if enough time has elapsed since last publish
	if !s.lastPublish.IsZero() && now.Sub(s.lastPublish) < targetInterval {
		return // Not yet time for next post
	}

	// Release 1 job from 'staged' pool
	var jobID int64
	var slug, title string
	query := `
		SELECT id, slug, title 
		FROM jobs 
		WHERE status = 'staged' 
		ORDER BY (employer_id IS NOT NULL) DESC, is_remote DESC, yearly_max DESC, id ASC 
		LIMIT 1
	`
	err = s.db.QueryRow(ctx, query).Scan(&jobID, &slug, &title)
	if err != nil {
		// No staged jobs remaining in pool
		return
	}

	// Publish job
	updateQuery := `
		UPDATE jobs 
		SET status = 'active', date_posted = NOW(), updated_at = NOW() 
		WHERE id = $1
	`
	_, err = s.db.Exec(ctx, updateQuery, jobID)
	if err != nil {
		slog.Error("Drip-feed: failed to activate job", "job_id", jobID, "error", err)
		return
	}

	s.lastPublish = now
	slog.Info("Drip-feed: published job",
		"job_id", jobID,
		"slug", slug,
		"title", title,
		"campaign_day", currentDay,
		"progress_today", fmt.Sprintf("%d/%d", publishedToday+1, dailyTarget),
		"next_in_mins", int(targetInterval.Minutes()),
	)

	// Invalidate sitemap cache so the new job smoothly enters XML sitemaps
	if s.sitemap != nil {
		s.sitemap.InvalidateJobsCache()
	}

	// Submit fresh URL to IndexNow for Bing/Yahoo (pure organic discovery)
	if s.indexNow != nil && s.indexNow.GetKey() != "" {
		jobURL := fmt.Sprintf("%s/jobs/%s/", s.baseURL, slug)
		go func() {
			ctxBg, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, _ = s.indexNow.SubmitURLs(ctxBg, []string{jobURL})
		}()
	}
}
