package reed

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"jobyuk/internal/services"
)

var logoRegex = regexp.MustCompile(`https://resources\.reed\.co\.uk/profileimages/logos/[^"'\s>]+`)

const (
	baseURL        = "https://www.reed.co.uk/api/1.0"
	maxRetries     = 3
	requestTimeout = 10 * time.Second
)

// Client handles communication with the Reed.co.uk API.
type Client struct {
	apiKey     string
	httpClient *http.Client
	cache      *services.CacheService
}

// NewClient creates a new Reed API client.
func NewClient(apiKey string, cache *services.CacheService) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: requestTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		cache: cache,
	}
}

// SearchParams holds Reed API search parameters.
type SearchParams struct {
	Keywords             string
	LocationName         string
	DistanceFromLocation int
	Permanent            *bool
	Contract             *bool
	Temp                 *bool
	PartTime             *bool
	FullTime             *bool
	Graduate             *bool
	MinimumSalary        int
	MaximumSalary        int
	ResultsToTake        int
	ResultsToSkip        int
	EmployerID           int
}

// SearchResult represents the Reed API search response.
type SearchResult struct {
	Results      []SearchJob `json:"results"`
	TotalResults int         `json:"totalResults"`
	Ambiguous    []string    `json:"ambiguousLocations,omitempty"`
}

// SearchJob represents a job from the Reed search API.
type SearchJob struct {
	JobID              int64   `json:"jobId"`
	EmployerID         int64   `json:"employerId"`
	EmployerName       string  `json:"employerName"`
	EmployerProfileID  *int64  `json:"employerProfileId"`
	JobTitle           string  `json:"jobTitle"`
	LocationName       string  `json:"locationName"`
	MinimumSalary      *float64 `json:"minimumSalary"`
	MaximumSalary      *float64 `json:"maximumSalary"`
	Currency           string  `json:"currency"`
	ExpirationDate     string  `json:"expirationDate"`
	Date               string  `json:"date"`
	JobDescription     string  `json:"jobDescription"`
	Applications       int     `json:"applications"`
	JobURL             string  `json:"jobUrl"`
}

// JobDetail represents the full job details from the Reed API.
type JobDetail struct {
	EmployerID         int64    `json:"employerId"`
	EmployerName       string   `json:"employerName"`
	EmployerProfileID  *int64   `json:"employerProfileId"`
	JobID              int64    `json:"jobId"`
	JobTitle           string   `json:"jobTitle"`
	LocationName       string   `json:"locationName"`
	MinimumSalary      *float64 `json:"minimumSalary"`
	MaximumSalary      *float64 `json:"maximumSalary"`
	YearlyMinimumSalary *float64 `json:"yearlyMinimumSalary"`
	YearlyMaximumSalary *float64 `json:"yearlyMaximumSalary"`
	Currency           string   `json:"currency"`
	SalaryType         string   `json:"salaryType"`
	ContractType       string   `json:"contractType"`
	JobType            string   `json:"jobType"`
	JobDescription     string   `json:"jobDescription"`
	ExternalURL        string   `json:"externalUrl"`
	JobURL             string   `json:"jobUrl"`
	ExpirationDate     string   `json:"expirationDate"`
	DatePosted         string   `json:"datePosted"`
}

// Search performs a job search on the Reed API.
func (c *Client) Search(ctx context.Context, params SearchParams) (*SearchResult, error) {
	// Build query string
	q := url.Values{}
	if params.Keywords != "" {
		q.Set("keywords", params.Keywords)
	}
	if params.LocationName != "" {
		q.Set("locationName", params.LocationName)
	}
	if params.DistanceFromLocation > 0 {
		q.Set("distanceFromLocation", strconv.Itoa(params.DistanceFromLocation))
	}
	if params.Permanent != nil {
		q.Set("permanent", strconv.FormatBool(*params.Permanent))
	}
	if params.Contract != nil {
		q.Set("contract", strconv.FormatBool(*params.Contract))
	}
	if params.Temp != nil {
		q.Set("temp", strconv.FormatBool(*params.Temp))
	}
	if params.PartTime != nil {
		q.Set("partTime", strconv.FormatBool(*params.PartTime))
	}
	if params.FullTime != nil {
		q.Set("fullTime", strconv.FormatBool(*params.FullTime))
	}
	if params.Graduate != nil {
		q.Set("graduate", strconv.FormatBool(*params.Graduate))
	}
	if params.MinimumSalary > 0 {
		q.Set("minimumSalary", strconv.Itoa(params.MinimumSalary))
	}
	if params.MaximumSalary > 0 {
		q.Set("maximumSalary", strconv.Itoa(params.MaximumSalary))
	}

	take := params.ResultsToTake
	if take == 0 || take > 100 {
		take = 100
	}
	q.Set("resultsToTake", strconv.Itoa(take))

	if params.ResultsToSkip > 0 {
		q.Set("resultsToSkip", strconv.Itoa(params.ResultsToSkip))
	}
	if params.EmployerID > 0 {
		q.Set("employerId", strconv.Itoa(params.EmployerID))
	}

	endpoint := fmt.Sprintf("%s/search?%s", baseURL, q.Encode())

	// Check cache first
	cacheKey := fmt.Sprintf("reed:search:%s", q.Encode())
	var result SearchResult
	if err := c.cache.Get(ctx, cacheKey, &result); err == nil {
		return &result, nil
	}

	// Make API request with retries
	body, err := c.doRequest(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("reed search request failed: %w", err)
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("reed search response parse failed: %w", err)
	}

	// Cache for 30 minutes
	_ = c.cache.Set(ctx, cacheKey, result, 30*time.Minute)

	return &result, nil
}

// GetJobDetails fetches detailed information for a specific job.
func (c *Client) GetJobDetails(ctx context.Context, jobID int64) (*JobDetail, error) {
	endpoint := fmt.Sprintf("%s/jobs/%d", baseURL, jobID)

	// Check cache first
	cacheKey := fmt.Sprintf("reed:job:%d", jobID)
	var detail JobDetail
	if err := c.cache.Get(ctx, cacheKey, &detail); err == nil {
		return &detail, nil
	}

	body, err := c.doRequest(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("reed job detail request failed: %w", err)
	}

	if err := json.Unmarshal(body, &detail); err != nil {
		return nil, fmt.Errorf("reed job detail response parse failed: %w", err)
	}

	// Cache for 30 minutes
	_ = c.cache.Set(ctx, cacheKey, detail, 30*time.Minute)

	return &detail, nil
}

// doRequest performs an HTTP request with Basic Auth and retries.
func (c *Client) doRequest(ctx context.Context, endpoint string) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Reed API uses Basic Auth with API key as username, empty password
		req.SetBasicAuth(c.apiKey, "")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "JobyUK.com/1.0")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			slog.Warn("Reed API request failed, retrying",
				"attempt", attempt+1,
				"endpoint", endpoint,
				"error", err,
			)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", err)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			slog.Warn("Reed API rate limited, backing off",
				"attempt", attempt+1,
			)
			lastErr = fmt.Errorf("rate limited (429)")
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
			if resp.StatusCode >= 500 {
				continue // Retry on server errors
			}
			return nil, lastErr // Don't retry on client errors
		}

		return body, nil
	}

	return nil, fmt.Errorf("all %d retries exhausted: %w", maxRetries, lastErr)
}

// FetchEmployerLogoURL scrapes a Reed job page to extract the employer logo URL.
// This is a best-effort operation — returns empty string on any failure.
func (c *Client) FetchEmployerLogoURL(ctx context.Context, jobURL string) string {
	if jobURL == "" {
		return ""
	}

	// Use a separate timeout for scraping (not API call)
	scrapeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(scrapeCtx, http.MethodGet, jobURL, nil)
	if err != nil {
		return ""
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; JobyUK.com/1.0)")
	req.Header.Set("Accept", "text/html")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Debug("Failed to fetch Reed job page for logo", "url", jobURL, "error", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	// Read up to 200KB — logo URL should be in the first part of the page
	body, err := io.ReadAll(io.LimitReader(resp.Body, 200*1024))
	if err != nil {
		return ""
	}

	match := logoRegex.Find(body)
	if match == nil {
		return ""
	}

	logoURL := string(match)
	slog.Debug("Found employer logo from Reed page", "url", jobURL, "logo", logoURL)
	return logoURL
}
