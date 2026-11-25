package services

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"strings"
	"sync"
	"time"

	"jobyuk/internal/repository"
)

// FeedService generates XML feeds according to aggregator specifications (Jobrapido, Jooble, Adzuna, Indeed, Careerjet, WhatJobs).
type FeedService struct {
	repos   *repository.Repositories
	baseURL string

	mu           sync.RWMutex
	jobrapido    []byte
	jobrapidoMod time.Time
	jooble       []byte
	joobleMod    time.Time
	adzuna       []byte
	adzunaMod    time.Time
	indeed       []byte
	indeedMod    time.Time
	cacheTTL     time.Duration
}

// NewFeedService creates an instance of FeedService.
func NewFeedService(repos *repository.Repositories, baseURL string) *FeedService {
	cleanURL := "https://jobyuk.com"
	if baseURL != "" && !strings.Contains(baseURL, "localhost") && !strings.Contains(baseURL, "127.0.0.1") {
		cleanURL = strings.TrimRight(baseURL, "/")
	}

	return &FeedService{
		repos:    repos,
		baseURL:  cleanURL,
		cacheTTL: 45 * time.Minute,
	}
}

// GetJobrapidoFeed returns cached Jobrapido-compliant XML or generates fresh.
func (s *FeedService) GetJobrapidoFeed(ctx context.Context) ([]byte, error) {
	s.mu.RLock()
	if len(s.jobrapido) > 0 && time.Since(s.jobrapidoMod) < s.cacheTTL {
		data := s.jobrapido
		s.mu.RUnlock()
		return data, nil
	}
	s.mu.RUnlock()

	jobs, err := s.repos.Job.FindFeedJobs(ctx, 1500)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.Grow(len(jobs)*600 + 512)
	buf.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	buf.WriteString("<jobrapido>\n")

	for _, j := range jobs {
		jobURL := fmt.Sprintf("%s/jobs/%s/?utm_source=jobrapido&utm_medium=referral&utm_campaign=jobrapido_feed", s.baseURL, j.Slug)
		city, country := s.parseLocation(j.LocationName, j.IsRemote)
		salStr := s.formatSalary(j.MinSalary, j.MaxSalary, j.Currency, j.SalaryType)
		jobType := s.formatJobType(j.ContractType, j.JobType, j.IsRemote)

		buf.WriteString("  <job>\n")
		buf.WriteString(fmt.Sprintf("    <id>%d</id>\n", j.ID))
		buf.WriteString(fmt.Sprintf("    <title>%s</title>\n", cdata(j.Title)))
		buf.WriteString(fmt.Sprintf("    <company>%s</company>\n", cdata(j.EmployerName)))
		buf.WriteString(fmt.Sprintf("    <description>%s</description>\n", cdata(cleanDescription(j.Description))))
		buf.WriteString(fmt.Sprintf("    <city>%s</city>\n", cdata(city)))
		buf.WriteString(fmt.Sprintf("    <country>%s</country>\n", country))
		buf.WriteString(fmt.Sprintf("    <publish_date>%s</publish_date>\n", j.DatePosted.UTC().Format("2006-01-02 15:04:05")))
		buf.WriteString(fmt.Sprintf("    <url>%s</url>\n", cdata(jobURL)))
		buf.WriteString(fmt.Sprintf("    <salary>%s</salary>\n", cdata(salStr)))
		buf.WriteString(fmt.Sprintf("    <jobtype>%s</jobtype>\n", cdata(jobType)))
		buf.WriteString(fmt.Sprintf("    <category>%s</category>\n", cdata(j.Category)))
		buf.WriteString("  </job>\n")
	}

	buf.WriteString("</jobrapido>\n")
	data := buf.Bytes()

	s.mu.Lock()
	s.jobrapido = data
	s.jobrapidoMod = time.Now()
	s.mu.Unlock()

	return data, nil
}

// GetJoobleFeed returns Jooble-compliant XML.
func (s *FeedService) GetJoobleFeed(ctx context.Context) ([]byte, error) {
	s.mu.RLock()
	if len(s.jooble) > 0 && time.Since(s.joobleMod) < s.cacheTTL {
		data := s.jooble
		s.mu.RUnlock()
		return data, nil
	}
	s.mu.RUnlock()

	jobs, err := s.repos.Job.FindFeedJobs(ctx, 1500)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.Grow(len(jobs)*600 + 512)
	buf.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	buf.WriteString("<jobs>\n")

	for _, j := range jobs {
		jobURL := fmt.Sprintf("%s/jobs/%s/?utm_source=jooble&utm_medium=referral&utm_campaign=jooble_feed", s.baseURL, j.Slug)
		city, country := s.parseLocation(j.LocationName, j.IsRemote)
		salStr := s.formatSalary(j.MinSalary, j.MaxSalary, j.Currency, j.SalaryType)
		jobType := s.formatJobType(j.ContractType, j.JobType, j.IsRemote)

		buf.WriteString(fmt.Sprintf("  <job id=\"%d\">\n", j.ID))
		buf.WriteString(fmt.Sprintf("    <link>%s</link>\n", cdata(jobURL)))
		buf.WriteString(fmt.Sprintf("    <name>%s</name>\n", cdata(j.Title)))
		buf.WriteString(fmt.Sprintf("    <region>%s</region>\n", cdata(city)))
		buf.WriteString(fmt.Sprintf("    <country>%s</country>\n", country))
		buf.WriteString(fmt.Sprintf("    <description>%s</description>\n", cdata(cleanDescription(j.Description))))
		buf.WriteString(fmt.Sprintf("    <company>%s</company>\n", cdata(j.EmployerName)))
		buf.WriteString(fmt.Sprintf("    <pubdate>%s</pubdate>\n", j.DatePosted.UTC().Format("02.01.2006")))
		buf.WriteString(fmt.Sprintf("    <updated>%s</updated>\n", j.DatePosted.UTC().Format("02.01.2006 15:04:05")))
		buf.WriteString(fmt.Sprintf("    <salary>%s</salary>\n", cdata(salStr)))
		buf.WriteString(fmt.Sprintf("    <jobtype>%s</jobtype>\n", cdata(jobType)))
		buf.WriteString("  </job>\n")
	}

	buf.WriteString("</jobs>\n")
	data := buf.Bytes()

	s.mu.Lock()
	s.jooble = data
	s.joobleMod = time.Now()
	s.mu.Unlock()

	return data, nil
}

// GetIndeedFeed returns universal XML compatible with Indeed, Adzuna, WhatJobs, Careerjet, Trovit, and Talent.com.
func (s *FeedService) GetIndeedFeed(ctx context.Context, sourceTag string) ([]byte, error) {
	if sourceTag == "" {
		sourceTag = "indeed"
	}

	jobs, err := s.repos.Job.FindFeedJobs(ctx, 1500)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.Grow(len(jobs)*600 + 512)
	buf.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	buf.WriteString("<source>\n")
	buf.WriteString(fmt.Sprintf("  <publisher>JobyUK.com</publisher>\n"))
	buf.WriteString(fmt.Sprintf("  <publisherurl>%s</publisherurl>\n", s.baseURL))
	buf.WriteString(fmt.Sprintf("  <lastBuildDate>%s</lastBuildDate>\n", time.Now().UTC().Format(time.RFC1123)))

	for _, j := range jobs {
		jobURL := fmt.Sprintf("%s/jobs/%s/?utm_source=%s&utm_medium=referral&utm_campaign=%s_feed", s.baseURL, j.Slug, sourceTag, sourceTag)
		city, country := s.parseLocation(j.LocationName, j.IsRemote)
		salStr := s.formatSalary(j.MinSalary, j.MaxSalary, j.Currency, j.SalaryType)
		jobType := s.formatJobType(j.ContractType, j.JobType, j.IsRemote)

		buf.WriteString("  <job>\n")
		buf.WriteString(fmt.Sprintf("    <title>%s</title>\n", cdata(j.Title)))
		buf.WriteString(fmt.Sprintf("    <date>%s</date>\n", j.DatePosted.UTC().Format(time.RFC1123)))
		buf.WriteString(fmt.Sprintf("    <referencenumber>%d</referencenumber>\n", j.ID))
		buf.WriteString(fmt.Sprintf("    <url>%s</url>\n", cdata(jobURL)))
		buf.WriteString(fmt.Sprintf("    <company>%s</company>\n", cdata(j.EmployerName)))
		buf.WriteString(fmt.Sprintf("    <city>%s</city>\n", cdata(city)))
		buf.WriteString(fmt.Sprintf("    <state>%s</state>\n", cdata(city)))
		buf.WriteString(fmt.Sprintf("    <country>%s</country>\n", country))
		buf.WriteString(fmt.Sprintf("    <description>%s</description>\n", cdata(cleanDescription(j.Description))))
		buf.WriteString(fmt.Sprintf("    <salary>%s</salary>\n", cdata(salStr)))
		buf.WriteString(fmt.Sprintf("    <jobtype>%s</jobtype>\n", cdata(jobType)))
		buf.WriteString(fmt.Sprintf("    <category>%s</category>\n", cdata(j.Category)))
		buf.WriteString("  </job>\n")
	}

	buf.WriteString("</source>\n")
	return buf.Bytes(), nil
}

// InvalidateCache resets all in-memory feed caches.
func (s *FeedService) InvalidateCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobrapido = nil
	s.jooble = nil
	s.adzuna = nil
	s.indeed = nil
}

func cdata(s string) string {
	clean := strings.ReplaceAll(s, "]]>", "]]]]><![CDATA[>")
	return "<![CDATA[" + clean + "]]>"
}

func cleanDescription(d string) string {
	d = strings.TrimSpace(d)
	if d == "" {
		return "Apply directly on JobyUK for this verified UK vacancy."
	}
	// Decode html entities then sanitize
	decoded := html.UnescapeString(d)
	return decoded
}

func (s *FeedService) parseLocation(loc string, isRemote bool) (city, country string) {
	country = "GB"
	loc = strings.TrimSpace(loc)
	if isRemote {
		return "Remote (UK)", country
	}
	if loc == "" || strings.EqualFold(loc, "United Kingdom") || strings.EqualFold(loc, "UK") {
		return "United Kingdom", country
	}
	parts := strings.Split(loc, ",")
	return strings.TrimSpace(parts[0]), country
}

func (s *FeedService) formatSalary(min, max *float64, curr, salType string) string {
	symbol := "£"
	if curr == "EUR" {
		symbol = "€"
	} else if curr == "USD" {
		symbol = "$"
	}

	period := "per annum"
	if strings.Contains(salType, "hour") {
		period = "per hour"
	} else if strings.Contains(salType, "day") {
		period = "per day"
	} else if strings.Contains(salType, "month") {
		period = "per month"
	}

	if min != nil && max != nil && *min > 0 && *max > 0 {
		if *min == *max {
			return fmt.Sprintf("%s%s %s", symbol, formatNumber(*min), period)
		}
		return fmt.Sprintf("%s%s - %s%s %s", symbol, formatNumber(*min), symbol, formatNumber(*max), period)
	} else if min != nil && *min > 0 {
		return fmt.Sprintf("From %s%s %s", symbol, formatNumber(*min), period)
	} else if max != nil && *max > 0 {
		return fmt.Sprintf("Up to %s%s %s", symbol, formatNumber(*max), period)
	}

	return "Competitive salary"
}

func (s *FeedService) formatJobType(contractType, jobType string, isRemote bool) string {
	parts := []string{}
	ct := strings.ToLower(contractType)
	jt := strings.ToLower(jobType)

	if strings.Contains(ct, "perm") {
		parts = append(parts, "Permanent")
	} else if strings.Contains(ct, "contract") {
		parts = append(parts, "Contract")
	} else if strings.Contains(ct, "temp") {
		parts = append(parts, "Temporary")
	}

	if strings.Contains(jt, "full") {
		parts = append(parts, "Full-time")
	} else if strings.Contains(jt, "part") {
		parts = append(parts, "Part-time")
	}

	if isRemote {
		parts = append(parts, "Remote")
	}

	if len(parts) == 0 {
		return "Full-time, Permanent"
	}
	return strings.Join(parts, ", ")
}

func formatNumber(f float64) string {
	n := int64(f)
	in := fmt.Sprintf("%d", n)
	out := make([]byte, len(in)+(len(in)-1)/3)
	for i, j, k := len(in)-1, len(out)-1, 0; i >= 0; i, j, k = i-1, j-1, k+1 {
		if k > 0 && k%3 == 0 {
			out[j] = ','
			j--
		}
		out[j] = in[i]
	}
	return string(out)
}
