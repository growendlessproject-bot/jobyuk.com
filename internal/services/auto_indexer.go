package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"jobyuk/internal/repository"
)

// AutoIndexerService handles automated background daily submission of fresh jobs to Google Indexing API.
type AutoIndexerService struct {
	jobRepo  *repository.JobRepository
	indexing *GoogleIndexingService
	baseURL  string

	mu               sync.Mutex
	isRunning        bool
	lastRun          time.Time
	lastIndexedCount int
	lastError        string
}

// NewAutoIndexerService creates a new instance of AutoIndexerService.
func NewAutoIndexerService(jobRepo *repository.JobRepository, indexing *GoogleIndexingService, baseURL string) *AutoIndexerService {
	prodURL := "https://jobyuk.com"
	if baseURL != "" && !strings.Contains(baseURL, "localhost") && !strings.Contains(baseURL, "127.0.0.1") {
		prodURL = strings.TrimRight(baseURL, "/")
	}

	return &AutoIndexerService{
		jobRepo:  jobRepo,
		indexing: indexing,
		baseURL:  prodURL,
	}
}

// RunBatch selects un-indexed active jobs and pushes them to Google Indexing API with polite rate limiting.
func (s *AutoIndexerService) RunBatch(ctx context.Context, limit int) (int, error) {
	if s.indexing == nil || !s.indexing.IsEnabled() {
		return 0, fmt.Errorf("Google Indexing API is not configured or disabled")
	}

	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return 0, fmt.Errorf("auto-indexer batch is already in progress")
	}
	s.isRunning = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.isRunning = false
		s.lastRun = time.Now()
		s.mu.Unlock()
	}()

	// Ensure database column exists
	_ = s.jobRepo.EnsureGoogleIndexedColumn(ctx)

	if limit <= 0 {
		limit = 150
	}

	slugs, err := s.jobRepo.FindJobsForIndexing(ctx, limit)
	if err != nil {
		s.recordError(err.Error())
		return 0, fmt.Errorf("failed to fetch unindexed jobs: %w", err)
	}

	if len(slugs) == 0 {
		slog.Info("[AutoIndexer] No unindexed jobs found. All active jobs have been submitted to Google.")
		return 0, nil
	}

	slog.Info("[AutoIndexer] Starting Google fast-track indexing batch", "total_jobs", len(slugs))

	successful := 0
	for i, slug := range slugs {
		select {
		case <-ctx.Done():
			slog.Warn("[AutoIndexer] Batch cancelled due to context timeout or shutdown")
			return successful, ctx.Err()
		default:
		}

		jobURL := fmt.Sprintf("%s/jobs/%s/", s.baseURL, slug)
		err := s.indexing.PublishUpdate(ctx, jobURL)
		if err != nil {
			// If Google returns 429 Too Many Requests, stop to avoid quota penalties
			if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "quota") {
				slog.Warn("[AutoIndexer] Google quota limit reached (429). Pausing batch until next cycle.", "error", err, "completed", successful)
				s.recordError("Google quota limit reached (429). Will retry next cycle.")
				break
			}
			slog.Warn("[AutoIndexer] Failed to notify Google of job", "url", jobURL, "error", err)
		} else {
			_ = s.jobRepo.MarkJobAsIndexed(ctx, slug)
			successful++
			if (i+1)%25 == 0 || i+1 == len(slugs) {
				slog.Info("[AutoIndexer] Progress update", "progress", fmt.Sprintf("%d/%d jobs indexed", i+1, len(slugs)))
			}
		}

		// Polite delay: 350ms between requests prevents Google per-minute rate-limiting
		time.Sleep(350 * time.Millisecond)
	}

	s.mu.Lock()
	s.lastIndexedCount = successful
	s.mu.Unlock()

	slog.Info("[AutoIndexer] Batch completed successfully", "indexed_count", successful, "requested", len(slugs))
	return successful, nil
}

// StartScheduled runs the auto-indexer daily in the background.
func (s *AutoIndexerService) StartScheduled(ctx context.Context, interval time.Duration) {
	// Self-healing migration on boot
	_ = s.jobRepo.EnsureGoogleIndexedColumn(ctx)

	// Wait 2 minutes after server boot before initial run
	go func() {
		time.Sleep(2 * time.Minute)
		if s.indexing != nil && s.indexing.IsEnabled() {
			slog.Info("[AutoIndexer] Running initial daily fast-track indexing cycle...")
			_, _ = s.RunBatch(context.Background(), 150)
		}
	}()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.indexing != nil && s.indexing.IsEnabled() {
				slog.Info("[AutoIndexer] Starting scheduled daily fast-track indexing cycle...")
				_, _ = s.RunBatch(context.Background(), 150)
			}
		}
	}
}

// GetStatus returns the current live status of the auto-indexer.
func (s *AutoIndexerService) GetStatus() (isRunning bool, lastRun time.Time, lastCount int, lastErr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isRunning, s.lastRun, s.lastIndexedCount, s.lastError
}

func (s *AutoIndexerService) recordError(msg string) {
	s.mu.Lock()
	s.lastError = msg
	s.mu.Unlock()
}

