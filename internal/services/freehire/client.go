package freehire

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	defaultBaseURL = "https://freehire.me/api/v1"
	defaultTimeout = 20 * time.Second
)

// FreeHireJob represents the job object returned by FreeHire OpenAPI search endpoint.
type FreeHireJob struct {
	PublicSlug  string   `json:"public_slug"`
	Title       string   `json:"title"`
	Company     string   `json:"company"`
	CompanySlug string   `json:"company_slug"`
	URL         string   `json:"url"`
	Location    string   `json:"location"`
	Regions     []string `json:"regions"`
	Countries   []string `json:"countries"`
	WorkMode    string   `json:"work_mode"`
	Skills      []string `json:"skills"`
	Source      string   `json:"source"`
	PostedAt    string   `json:"posted_at"`
	CreatedAt   string   `json:"created_at"`
	ClosedAt    *string  `json:"closed_at"`
	Description string   `json:"description"`
}

type SearchResponse struct {
	Data []FreeHireJob `json:"data"`
}

// Client connects to the public FreeHire API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new FreeHire API client.
func NewClient() *Client {
	return &Client{
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// SearchUKJobs fetches the latest UK jobs from FreeHire.
func (c *Client) SearchUKJobs(ctx context.Context, limit int, offset int) ([]FreeHireJob, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}

	apiURL := fmt.Sprintf("%s/agent/jobs/search?regions=uk&limit=%d&offset=%d&sort=posted_at&order=desc", c.baseURL, limit, offset)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create freehire request: %w", err)
	}

	req.Header.Set("User-Agent", "JobyUK.com-Sync/1.0 (+https://jobyuk.com)")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute freehire request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("freehire api returned status: %d", resp.StatusCode)
	}

	var res SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode freehire json response: %w", err)
	}

	return res.Data, nil
}
