package services

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"time"
)

// XMLURL represents a single URL entry in a sitemap.
type XMLURL struct {
	Loc        string  `xml:"loc"`
	LastMod    string  `xml:"lastmod,omitempty"`
	ChangeFreq string  `xml:"changefreq,omitempty"`
	Priority   float64 `xml:"priority,omitempty"`
}

// XMLURLSet represents a set of URLs in a sitemap.
type XMLURLSet struct {
	XMLName xml.Name `xml:"urlset"`
	Xmlns   string   `xml:"xmlns,attr"`
	URLs    []XMLURL `xml:"url"`
}

const sitemapXmlns = "http://www.sitemaps.org/schemas/sitemap/0.9"

// RebuildAll regenerates all 4 sitemaps in the background and warms the memory cache.
func (s *SitemapService) RebuildAll(ctx context.Context) error {
	var firstErr error
	if _, err := s.GenerateJobsSitemap(ctx); err != nil && firstErr == nil {
		firstErr = err
	}
	if _, err := s.GenerateTaxonomySitemap(ctx); err != nil && firstErr == nil {
		firstErr = err
	}
	if _, err := s.GenerateCompaniesSitemap(ctx); err != nil && firstErr == nil {
		firstErr = err
	}
	if _, err := s.GenerateContentSitemap(ctx); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

// GetJobsSitemap returns the cached jobs sitemap instantly or generates it on first hit.
func (s *SitemapService) GetJobsSitemap(ctx context.Context) ([]byte, time.Time, error) {
	s.mu.RLock()
	if len(s.jobsCache) > 0 {
		data := s.jobsCache
		lastMod := s.jobsLastMod
		s.mu.RUnlock()
		return data, lastMod, nil
	}
	s.mu.RUnlock()

	data, err := s.GenerateJobsSitemap(ctx)
	if err != nil {
		return nil, time.Time{}, err
	}
	s.mu.RLock()
	lastMod := s.jobsLastMod
	s.mu.RUnlock()
	return data, lastMod, nil
}

// GenerateJobsSitemap generates sitemap xml for active jobs using a high-speed pre-allocated buffer.
func (s *SitemapService) GenerateJobsSitemap(ctx context.Context) ([]byte, error) {
	jobs, err := s.repos.Job.FindSitemapJobs(ctx)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	// Pre-allocate ~160 bytes per job entry + header
	buf.Grow(len(jobs)*160 + 512)
	buf.WriteString(xml.Header)
	buf.WriteString("<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n")

	var latestMod time.Time
	for _, job := range jobs {
		if job.UpdatedAt.After(latestMod) {
			latestMod = job.UpdatedAt
		}
		buf.WriteString("  <url>\n    <loc>")
		xml.EscapeText(&buf, []byte(fmt.Sprintf("%s/jobs/%s/", s.baseURL, job.Slug)))
		buf.WriteString("</loc>\n    <lastmod>")
		buf.WriteString(job.UpdatedAt.UTC().Format(time.RFC3339))
		buf.WriteString("</lastmod>\n    <changefreq>weekly</changefreq>\n    <priority>0.8</priority>\n  </url>\n")
	}
	buf.WriteString("</urlset>\n")

	data := buf.Bytes()
	if latestMod.IsZero() {
		latestMod = time.Now().UTC()
	}

	s.mu.Lock()
	s.jobsCache = data
	s.jobsLastMod = latestMod
	s.mu.Unlock()

	return data, nil
}

// GetTaxonomySitemap returns the cached taxonomy sitemap instantly or generates it on first hit.
func (s *SitemapService) GetTaxonomySitemap(ctx context.Context) ([]byte, time.Time, error) {
	s.mu.RLock()
	if len(s.taxonomyCache) > 0 {
		data := s.taxonomyCache
		lastMod := s.taxonomyLastMod
		s.mu.RUnlock()
		return data, lastMod, nil
	}
	s.mu.RUnlock()

	data, err := s.GenerateTaxonomySitemap(ctx)
	if err != nil {
		return nil, time.Time{}, err
	}
	s.mu.RLock()
	lastMod := s.taxonomyLastMod
	s.mu.RUnlock()
	return data, lastMod, nil
}

// GenerateTaxonomySitemap generates sitemap xml for job roles, locations, and industries.
func (s *SitemapService) GenerateTaxonomySitemap(ctx context.Context) ([]byte, error) {
	roles, err := s.repos.Role.FindSitemapRoles(ctx)
	if err != nil {
		return nil, err
	}

	locations, err := s.repos.Location.FindSitemapLocations(ctx)
	if err != nil {
		return nil, err
	}

	industries, err := s.repos.Industry.FindSitemapIndustries(ctx)
	if err != nil {
		return nil, err
	}

	urlset := XMLURLSet{
		Xmlns: sitemapXmlns,
		URLs:  []XMLURL{},
	}

	today := time.Now().UTC().Format(time.RFC3339)

	// Add roles
	for _, role := range roles {
		urlset.URLs = append(urlset.URLs, XMLURL{
			Loc:        fmt.Sprintf("%s/job-roles/%s/", s.baseURL, role),
			LastMod:    today,
			ChangeFreq: "daily",
			Priority:   0.7,
		})
	}

	// Add locations
	for _, loc := range locations {
		urlset.URLs = append(urlset.URLs, XMLURL{
			Loc:        fmt.Sprintf("%s/locations/%s/", s.baseURL, loc),
			LastMod:    today,
			ChangeFreq: "daily",
			Priority:   0.7,
		})
	}

	// Add industries
	for _, ind := range industries {
		urlset.URLs = append(urlset.URLs, XMLURL{
			Loc:        fmt.Sprintf("%s/industries/%s/", s.baseURL, ind),
			LastMod:    today,
			ChangeFreq: "daily",
			Priority:   0.6,
		})
	}

	data, err := xml.MarshalIndent(urlset, "", "  ")
	if err != nil {
		return nil, err
	}

	out := append([]byte(xml.Header), data...)

	s.mu.Lock()
	s.taxonomyCache = out
	s.taxonomyLastMod = time.Now().UTC()
	s.mu.Unlock()

	return out, nil
}

// GetCompaniesSitemap returns the cached companies sitemap instantly or generates it on first hit.
func (s *SitemapService) GetCompaniesSitemap(ctx context.Context) ([]byte, time.Time, error) {
	s.mu.RLock()
	if len(s.companiesCache) > 0 {
		data := s.companiesCache
		lastMod := s.companiesLastMod
		s.mu.RUnlock()
		return data, lastMod, nil
	}
	s.mu.RUnlock()

	data, err := s.GenerateCompaniesSitemap(ctx)
	if err != nil {
		return nil, time.Time{}, err
	}
	s.mu.RLock()
	lastMod := s.companiesLastMod
	s.mu.RUnlock()
	return data, lastMod, nil
}

// GenerateCompaniesSitemap generates sitemap xml for active employers.
func (s *SitemapService) GenerateCompaniesSitemap(ctx context.Context) ([]byte, error) {
	companies, err := s.repos.Company.FindSitemapCompanies(ctx)
	if err != nil {
		return nil, err
	}

	urlset := XMLURLSet{
		Xmlns: sitemapXmlns,
		URLs:  make([]XMLURL, 0, len(companies)),
	}

	today := time.Now().UTC().Format(time.RFC3339)
	for _, comp := range companies {
		urlset.URLs = append(urlset.URLs, XMLURL{
			Loc:        fmt.Sprintf("%s/companies/%s/", s.baseURL, comp),
			LastMod:    today,
			ChangeFreq: "weekly",
			Priority:   0.6,
		})
	}

	data, err := xml.MarshalIndent(urlset, "", "  ")
	if err != nil {
		return nil, err
	}

	out := append([]byte(xml.Header), data...)

	s.mu.Lock()
	s.companiesCache = out
	s.companiesLastMod = time.Now().UTC()
	s.mu.Unlock()

	return out, nil
}

// GetContentSitemap returns the cached content sitemap instantly or generates it on first hit.
func (s *SitemapService) GetContentSitemap(ctx context.Context) ([]byte, time.Time, error) {
	s.mu.RLock()
	if len(s.contentCache) > 0 {
		data := s.contentCache
		lastMod := s.contentLastMod
		s.mu.RUnlock()
		return data, lastMod, nil
	}
	s.mu.RUnlock()

	data, err := s.GenerateContentSitemap(ctx)
	if err != nil {
		return nil, time.Time{}, err
	}
	s.mu.RLock()
	lastMod := s.contentLastMod
	s.mu.RUnlock()
	return data, lastMod, nil
}

// GenerateContentSitemap generates sitemap xml for salary guides and articles.
func (s *SitemapService) GenerateContentSitemap(ctx context.Context) ([]byte, error) {
	articles, err := s.repos.Article.FindSitemapArticles(ctx)
	if err != nil {
		return nil, err
	}

	urlset := XMLURLSet{
		Xmlns: sitemapXmlns,
		URLs:  make([]XMLURL, 0, len(articles)+5),
	}

	// Add static authority and index pages
	urlset.URLs = append(urlset.URLs, XMLURL{
		Loc:        fmt.Sprintf("%s/about/", s.baseURL),
		ChangeFreq: "monthly",
		Priority:   0.8,
	})
	urlset.URLs = append(urlset.URLs, XMLURL{
		Loc:        fmt.Sprintf("%s/career-advice/", s.baseURL),
		ChangeFreq: "daily",
		Priority:   0.7,
	})
	urlset.URLs = append(urlset.URLs, XMLURL{
		Loc:        fmt.Sprintf("%s/salaries/", s.baseURL),
		ChangeFreq: "weekly",
		Priority:   0.7,
	})
	urlset.URLs = append(urlset.URLs, XMLURL{
		Loc:        fmt.Sprintf("%s/salary-calculator/", s.baseURL),
		ChangeFreq: "weekly",
		Priority:   0.8,
	})
	urlset.URLs = append(urlset.URLs, XMLURL{
		Loc:        fmt.Sprintf("%s/contact/", s.baseURL),
		ChangeFreq: "monthly",
		Priority:   0.6,
	})
	urlset.URLs = append(urlset.URLs, XMLURL{
		Loc:        fmt.Sprintf("%s/privacy-policy/", s.baseURL),
		ChangeFreq: "monthly",
		Priority:   0.3,
	})
	urlset.URLs = append(urlset.URLs, XMLURL{
		Loc:        fmt.Sprintf("%s/terms-of-service/", s.baseURL),
		ChangeFreq: "monthly",
		Priority:   0.3,
	})

	for _, cat := range []string{"cv", "interviews", "job-search", "salary"} {
		urlset.URLs = append(urlset.URLs, XMLURL{
			Loc:        fmt.Sprintf("%s/career-advice/%s/", s.baseURL, cat),
			ChangeFreq: "weekly",
			Priority:   0.7,
		})
	}

	for _, art := range articles {
		urlset.URLs = append(urlset.URLs, XMLURL{
			Loc:        fmt.Sprintf("%s/career-advice/%s/%s/", s.baseURL, art.Category, art.Slug),
			LastMod:    art.UpdatedAt.UTC().Format(time.RFC3339),
			ChangeFreq: "monthly",
			Priority:   0.6,
		})
	}

	data, err := xml.MarshalIndent(urlset, "", "  ")
	if err != nil {
		return nil, err
	}

	out := append([]byte(xml.Header), data...)

	s.mu.Lock()
	s.contentCache = out
	s.contentLastMod = time.Now().UTC()
	s.mu.Unlock()

	return out, nil
}
