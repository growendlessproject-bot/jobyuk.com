package linkbot

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// LinkStatus represents the health status of a job link.
type LinkStatus string

const (
	StatusUnknown     LinkStatus = "unknown"
	StatusActive      LinkStatus = "active"
	StatusDead        LinkStatus = "dead"
	StatusUnreachable LinkStatus = "unreachable"
)

// CheckResult contains detailed findings of a URL verification pass.
type CheckResult struct {
	Status     LinkStatus
	StatusCode int
	Reason     string
	FinalURL   string
	Duration   time.Duration
}

// Checker evaluates destination URLs for hard 404s, redirects, and ATS soft 404s.
type Checker struct {
	client          *http.Client
	softClosePhrases []string
}

// NewChecker initializes a tuned HTTP client for ATS link verification.
func NewChecker(timeout time.Duration) *Checker {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	transport := &http.Transport{
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true}, // Many internal ATS certs expire or have subdomains
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     30 * time.Second,
		DisableKeepAlives:   false,
	}

	client := &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			// Retain browser User-Agent across redirects
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36 JobyUKLinkVerifier/1.0")
			req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
			req.Header.Set("Accept-Language", "en-GB,en;q=0.9")
			return nil
		},
	}

	phrases := []string{
		"this vacancy has closed",
		"this vacancy is closed",
		"this vacancy is no longer available",
		"this job is no longer available",
		"job is no longer available",
		"no longer accepting applications",
		"not currently accepting applications",
		"applications for this role have closed",
		"applications are now closed",
		"applications have closed",
		"this position has been filled",
		"this position is no longer available",
		"this position is closed",
		"job has expired",
		"job expired",
		"vacancy expired",
		"this posting has expired",
		"the job you are looking for has expired",
		"this role is now closed",
		"this advert has expired",
		"this job advert has closed",
		"this listing is no longer active",
		"position has been filled",
		"we are no longer accepting applications for this position",
		"this role has now closed",
		"page not found",
		"error 404",
		"404 not found",
		"404 - page not found",
	}

	return &Checker{
		client:          client,
		softClosePhrases: phrases,
	}
}

// CheckURL verifies whether a job link is still live, closed, or dead.
func (c *Checker) CheckURL(ctx context.Context, targetURL string) CheckResult {
	start := time.Now()
	targetURL = strings.TrimSpace(targetURL)
	if targetURL == "" {
		return CheckResult{
			Status:   StatusDead,
			Reason:   "Empty URL",
			Duration: time.Since(start),
		}
	}

	parsedOriginal, err := url.Parse(targetURL)
	if err != nil || (parsedOriginal.Scheme != "http" && parsedOriginal.Scheme != "https") {
		return CheckResult{
			Status:   StatusDead,
			Reason:   "Invalid URL scheme",
			Duration: time.Since(start),
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return CheckResult{
			Status:   StatusDead,
			Reason:   "Failed to construct HTTP request: " + err.Error(),
			Duration: time.Since(start),
		}
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36 JobyUKLinkVerifier/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-GB,en;q=0.9")
	req.Header.Set("Sec-Ch-Ua", "\"Not/A)Brand\";v=\"8\", \"Chromium\";v=\"126\", \"Google Chrome\";v=\"126\"")

	resp, err := c.client.Do(req)
	if err != nil {
		return CheckResult{
			Status:   StatusUnreachable,
			Reason:   "Network error: " + err.Error(),
			Duration: time.Since(start),
		}
	}
	defer resp.Body.Close()

	finalURLStr := targetURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURLStr = resp.Request.URL.String()
	}

	// 1. Hard HTTP dead statuses
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		return CheckResult{
			Status:     StatusDead,
			StatusCode: resp.StatusCode,
			FinalURL:   finalURLStr,
			Reason:     http.StatusText(resp.StatusCode),
			Duration:   time.Since(start),
		}
	}

	// 2. Server or rate-limiting errors (unreachable / transient)
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return CheckResult{
			Status:     StatusUnreachable,
			StatusCode: resp.StatusCode,
			FinalURL:   finalURLStr,
			Reason:     "Transient status: " + http.StatusText(resp.StatusCode),
			Duration:   time.Since(start),
		}
	}

	// 3. Check for generic homepage / careers listing redirect
	if c.isRedirectedToGenericIndex(parsedOriginal, resp.Request.URL) {
		return CheckResult{
			Status:     StatusDead,
			StatusCode: resp.StatusCode,
			FinalURL:   finalURLStr,
			Reason:     "Redirected away from job to generic careers/home index",
			Duration:   time.Since(start),
		}
	}

	// 4. Read first 128KB to detect ATS soft-404 closure announcements
	bodyPreview, err := io.ReadAll(io.LimitReader(resp.Body, 128*1024))
	if err == nil && len(bodyPreview) > 0 {
		bodyLower := strings.ToLower(string(bodyPreview))
		for _, phrase := range c.softClosePhrases {
			if strings.Contains(bodyLower, phrase) {
				return CheckResult{
					Status:     StatusDead,
					StatusCode: resp.StatusCode,
					FinalURL:   finalURLStr,
					Reason:     "Soft 404: vacancy closed (" + phrase + ")",
					Duration:   time.Since(start),
				}
			}
		}
	}

	// 5. If status is 200/300-range without closure markers, link is confirmed ACTIVE!
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return CheckResult{
			Status:     StatusActive,
			StatusCode: resp.StatusCode,
			FinalURL:   finalURLStr,
			Reason:     "Verified active and accessible",
			Duration:   time.Since(start),
		}
	}

	// Fallback for unexpected status codes (401, 403 without body closure, etc.)
	return CheckResult{
		Status:     StatusUnreachable,
		StatusCode: resp.StatusCode,
		FinalURL:   finalURLStr,
		Reason:     "Unexpected HTTP status: " + http.StatusText(resp.StatusCode),
		Duration:   time.Since(start),
	}
}

// isRedirectedToGenericIndex detects if an ATS redirects an expired job URL back to its root careers listing.
func (c *Checker) isRedirectedToGenericIndex(origURL, finalURL *url.URL) bool {
	if origURL == nil || finalURL == nil {
		return false
	}

	origPath := strings.TrimRight(strings.ToLower(origURL.Path), "/")
	finalPath := strings.TrimRight(strings.ToLower(finalURL.Path), "/")

	// If original path had segments (e.g. /jobs/12345 or /vacancies/developer) but final path is root or generic
	if len(strings.Split(origPath, "/")) > 2 {
		genericRoots := []string{"", "/", "/jobs", "/jobs/", "/careers", "/careers/", "/jobs", "/jobs/", "/search", "/search/"}
		for _, gr := range genericRoots {
			if finalPath == strings.TrimRight(gr, "/") {
				return true
			}
		}
	}

	// Check query params commonly appended on closure
	q := finalURL.Query()
	if q.Get("closed") == "true" || q.Get("expired") == "true" || q.Get("status") == "closed" {
		return true
	}

	return false
}
