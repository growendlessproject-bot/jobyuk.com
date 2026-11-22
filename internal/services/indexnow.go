package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"jobyuk/internal/repository"
)

// IndexNowPayload represents the official IndexNow JSON request format.
type IndexNowPayload struct {
	Host        string   `json:"host"`
	Key         string   `json:"key"`
	KeyLocation string   `json:"keyLocation,omitempty"`
	URLList     []string `json:"urlList"`
}

// IndexNowService manages submitting URLs to Bing, Yahoo, Yandex, and Seznam.
type IndexNowService struct {
	host        string
	key         string
	keyLocation string
	client      *http.Client
}

// NewIndexNowService initializes the IndexNow service.
func NewIndexNowService(baseURL, key string) *IndexNowService {
	if key == "" {
		key = "ab4225315c73f110cb98d2abd9911d76"
	}

	prodURL := "https://jobyuk.com"
	host := "jobyuk.com"
	if baseURL != "" && !strings.Contains(baseURL, "localhost") && !strings.Contains(baseURL, "127.0.0.1") {
		prodURL = strings.TrimRight(baseURL, "/")
		host = strings.TrimPrefix(prodURL, "https://")
		host = strings.TrimPrefix(host, "http://")
	}

	keyLocation := fmt.Sprintf("%s/%s.txt", prodURL, key)

	return &IndexNowService{
		host:        host,
		key:         key,
		keyLocation: keyLocation,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// GetKey returns the configured IndexNow key.
func (s *IndexNowService) GetKey() string {
	return s.key
}

// GetKeyLocation returns the full URL location of the verification key file.
func (s *IndexNowService) GetKeyLocation() string {
	return s.keyLocation
}

// SubmitURLs sends a batch of URLs (up to 10,000) to IndexNow (Bing & Yahoo).
func (s *IndexNowService) SubmitURLs(ctx context.Context, urls []string) (int, error) {
	if len(urls) == 0 {
		return 0, nil
	}

	// IndexNow allows max 10,000 URLs per request
	chunkSize := 1000
	totalSubmitted := 0

	for i := 0; i < len(urls); i += chunkSize {
		end := i + chunkSize
		if end > len(urls) {
			end = len(urls)
		}
		chunk := urls[i:end]

		payload := IndexNowPayload{
			Host:        s.host,
			Key:         s.key,
			KeyLocation: s.keyLocation,
			URLList:     chunk,
		}

		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			return totalSubmitted, fmt.Errorf("failed to marshal IndexNow payload: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", "https://api.indexnow.org/indexnow", bytes.NewBuffer(bodyBytes))
		if err != nil {
			return totalSubmitted, fmt.Errorf("failed to create IndexNow request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json; charset=utf-8")

		resp, err := s.client.Do(req)
		if err != nil {
			return totalSubmitted, fmt.Errorf("IndexNow HTTP request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
			return totalSubmitted, fmt.Errorf("IndexNow returned HTTP %s", resp.Status)
		}

		totalSubmitted += len(chunk)
		slog.Info("[IndexNow] Successfully submitted URLs to Bing & Yahoo", "chunk_count", len(chunk), "status", resp.Status)
	}

	return totalSubmitted, nil
}

// SubmitTopJobs fetches fresh active jobs from repository and submits them to IndexNow.
func (s *IndexNowService) SubmitTopJobs(ctx context.Context, jobRepo *repository.JobRepository, limit int) (int, error) {
	if limit <= 0 {
		limit = 200
	}

	// Always include core hubs
	urls := []string{
		fmt.Sprintf("https://%s/", s.host),
		fmt.Sprintf("https://%s/jobs/", s.host),
		fmt.Sprintf("https://%s/salaries/", s.host),
		fmt.Sprintf("https://%s/salary-calculator/", s.host),
		fmt.Sprintf("https://%s/career-advice/", s.host),
	}

	// Fetch fresh jobs
	slugs, err := jobRepo.FindJobsForIndexing(ctx, limit)
	if err == nil {
		for _, slug := range slugs {
			urls = append(urls, fmt.Sprintf("https://%s/jobs/%s/", s.host, slug))
		}
	}

	return s.SubmitURLs(ctx, urls)
}

// StartScheduled runs IndexNow submission every 12 hours for new and fresh jobs.
func (s *IndexNowService) StartScheduled(ctx context.Context, jobRepo *repository.JobRepository, interval time.Duration) {
	// Initial submission 3 minutes after boot
	go func() {
		time.Sleep(3 * time.Minute)
		slog.Info("[IndexNow] Running initial submission of top fresh jobs to Bing & Yahoo...")
		_, _ = s.SubmitTopJobs(context.Background(), jobRepo, 200)
	}()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			slog.Info("[IndexNow] Running scheduled submission of top jobs to Bing & Yahoo...")
			_, _ = s.SubmitTopJobs(context.Background(), jobRepo, 200)
		}
	}
}
