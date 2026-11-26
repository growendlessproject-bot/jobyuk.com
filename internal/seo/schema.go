package seo

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"jobyuk/internal/models"
)

var (
	salaryRangeRe  = regexp.MustCompile(`(?i)£([0-9]{2,3}(?:,[0-9]{3})?)\s*(?:-|to|–)\s*£?([0-9]{2,3}(?:,[0-9]{3})?)`)
	singleSalaryRe = regexp.MustCompile(`(?i)£([0-9]{2,3}(?:,[0-9]{3})?)\s*(?:per\s+annum|p\.?a\.?|per\s+year|a\s+year|\/year|\/annum)`)
)

// SchemaGenerator produces JSON-LD structured data.
type SchemaGenerator struct {
	baseURL string
}

// NewSchemaGenerator creates a new schema generator.
func NewSchemaGenerator(baseURL string) *SchemaGenerator {
	prodURL := "https://jobyuk.com"
	if baseURL != "" && !strings.Contains(baseURL, "localhost") && !strings.Contains(baseURL, "127.0.0.1") {
		prodURL = strings.TrimRight(baseURL, "/")
	}
	return &SchemaGenerator{baseURL: prodURL}
}

// JobPostingSchema generates JSON-LD for a single job page according to Google Search Central guidelines.
// Only active (non-expired) jobs should have this markup.
func (g *SchemaGenerator) JobPostingSchema(job *models.Job) template.HTML {
	if job.IsExpired() {
		return "" // Critical: no JobPosting schema for expired jobs
	}

	empName := strings.TrimSpace(job.EmployerName)
	if empName == "" {
		empName = "Confidential Employer"
	}

	schema := map[string]interface{}{
		"@context":    "https://schema.org",
		"@type":       "JobPosting",
		"title":       job.Title,
		"description": job.Description,
		"datePosted":  job.DatePosted.Format("2006-01-02"),
		"url":         fmt.Sprintf("%s/jobs/%s/", g.baseURL, job.Slug),
		"identifier": map[string]interface{}{
			"@type": "PropertyValue",
			"name":  "JobyUK",
			"value": fmt.Sprintf("JOB-%d", job.ID),
		},
		"directApply": false,
	}

	// Valid through (required property for Google Jobs Rich Results)
	if job.ExpirationDate != nil && !job.ExpirationDate.IsZero() {
		schema["validThrough"] = job.ExpirationDate.UTC().Format("2006-01-02T15:04:05Z")
	} else {
		exp := job.DatePosted.AddDate(0, 0, 30)
		schema["validThrough"] = exp.UTC().Format("2006-01-02T15:04:05Z")
	}

	// Employment type (Google accepts FULL_TIME, PART_TIME, CONTRACTOR, TEMPORARY, INTERN)
	empType := mapEmploymentType(job.ContractType, job.JobType)
	if empType == "" {
		empType = "FULL_TIME"
	}
	schema["employmentType"] = empType

	// Hiring organization (asli employer name, sameAs and logo if available)
	hiringOrg := map[string]interface{}{
		"@type": "Organization",
		"name":  empName,
	}
	if job.CompanySlug != "" {
		hiringOrg["sameAs"] = fmt.Sprintf("%s/companies/%s/", g.baseURL, job.CompanySlug)
	}
	if job.CompanyLogoURL() != "" {
		logoURL := job.CompanyLogoURL()
		if strings.HasPrefix(logoURL, "/") {
			logoURL = g.baseURL + logoURL
		}
		hiringOrg["logo"] = logoURL
	}
	schema["hiringOrganization"] = hiringOrg

	// Job location
	loc := job.DisplayLocation()
	if loc == "" {
		loc = "United Kingdom"
	}
	region := ResolveUKRegion(loc)
	postcode := ResolveUKPostalCode(loc)

	addr := map[string]interface{}{
		"@type":           "PostalAddress",
		"addressLocality": loc,
		"addressRegion":   region,
		"addressCountry":  "GB",
	}
	if postcode != "" {
		addr["postalCode"] = postcode
	}

	schema["jobLocation"] = map[string]interface{}{
		"@type":   "Place",
		"address": addr,
	}

	// Remote / Telecommute location requirements (country: GB)
	if job.IsRemote {
		schema["jobLocationType"] = "TELECOMMUTE"
		schema["applicantLocationRequirements"] = map[string]interface{}{
			"@type": "Country",
			"name":  "GB",
		}
	}

	// Base salary: Sirf tab daalo jab employer ne publish ki ho (never fake or estimated)
	var minSal, maxSal *float64
	unit := "YEAR"
	salType := strings.ToLower(job.SalaryType)
	if strings.Contains(salType, "hour") {
		unit = "HOUR"
	} else if strings.Contains(salType, "day") {
		unit = "DAY"
	} else if strings.Contains(salType, "month") {
		unit = "MONTH"
	}

	if (job.MinSalary != nil && *job.MinSalary > 0) || (job.MaxSalary != nil && *job.MaxSalary > 0) {
		minSal = job.MinSalary
		maxSal = job.MaxSalary
	} else if (job.YearlyMin != nil && *job.YearlyMin > 0) || (job.YearlyMax != nil && *job.YearlyMax > 0) {
		minSal = job.YearlyMin
		maxSal = job.YearlyMax
		unit = "YEAR"
	}

	if (minSal != nil && *minSal > 0) || (maxSal != nil && *maxSal > 0) {
		salaryValue := map[string]interface{}{
			"@type":    "QuantitativeValue",
			"unitText": unit,
		}
		if minSal != nil && *minSal > 0 {
			salaryValue["minValue"] = *minSal
		}
		if maxSal != nil && *maxSal > 0 {
			salaryValue["maxValue"] = *maxSal
		}

		schema["baseSalary"] = map[string]interface{}{
			"@type":    "MonetaryAmount",
			"currency": "GBP",
			"value":    salaryValue,
		}
	}

	return g.toHTML(schema)
}

// OrganizationSchema generates sitewide Organization JSON-LD.
func (g *SchemaGenerator) OrganizationSchema() template.HTML {
	schema := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "Organization",
		"name":     "JobyUK.com",
		"slogan":   "Apply direct. Skip the middleman",
		"url":      g.baseURL,
		"logo":     fmt.Sprintf("%s/static/images/logo.svg", g.baseURL),
		"sameAs":   []string{},
	}
	return g.toHTML(schema)
}

// WebSiteSchema generates homepage WebSite schema with search action.
func (g *SchemaGenerator) WebSiteSchema() template.HTML {
	schema := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "WebSite",
		"name":     "JobyUK.com",
		"url":      g.baseURL,
		"potentialAction": map[string]interface{}{
			"@type":       "SearchAction",
			"target":      fmt.Sprintf("%s/jobs/?q={search_term_string}", g.baseURL),
			"query-input": "required name=search_term_string",
		},
	}
	return g.toHTML(schema)
}

// BreadcrumbSchema generates Google-compliant BreadcrumbList structured data.
// Every ListItem MUST contain a valid "item" URL.
func (g *SchemaGenerator) BreadcrumbSchema(items []BreadcrumbItem) template.HTML {
	listItems := make([]map[string]interface{}, 0, len(items))
	pos := 1
	for _, item := range items {
		itemURL := item.URL
		if itemURL == "" {
			continue // Critical: Google rejects ListItems with missing "item" field
		}
		listItem := map[string]interface{}{
			"@type":    "ListItem",
			"position": pos,
			"name":     item.Name,
			"item":     itemURL,
		}
		listItems = append(listItems, listItem)
		pos++
	}

	schema := map[string]interface{}{
		"@context":        "https://schema.org",
		"@type":           "BreadcrumbList",
		"itemListElement": listItems,
	}
	return g.toHTML(schema)
}

// BreadcrumbItem represents a single breadcrumb entry.
type BreadcrumbItem struct {
	Name string
	URL  string
}

// HomeBreadcrumbs returns breadcrumbs starting from Home.
func (g *SchemaGenerator) HomeBreadcrumbs() []BreadcrumbItem {
	return []BreadcrumbItem{
		{Name: "Home", URL: g.baseURL + "/"},
	}
}

// JobBreadcrumbs returns breadcrumbs for a job detail page according to Google BreadcrumbList specs.
func (g *SchemaGenerator) JobBreadcrumbs(job *models.Job) []BreadcrumbItem {
	items := []BreadcrumbItem{
		{Name: "Home", URL: g.baseURL + "/"},
		{Name: "Jobs", URL: g.baseURL + "/jobs/"},
	}

	if locName, param := job.CleanPrimaryLocation(); locName != "" && param != "" {
		items = append(items, BreadcrumbItem{
			Name: locName + " Jobs",
			URL:  fmt.Sprintf("%s/jobs/?%s", g.baseURL, param),
		})
	}

	items = append(items, BreadcrumbItem{
		Name: job.Title,
		URL:  fmt.Sprintf("%s/jobs/%s/", g.baseURL, job.Slug),
	})

	return items
}

// RoleBreadcrumbs returns breadcrumbs for a role page.
func (g *SchemaGenerator) RoleBreadcrumbs(role *models.JobRole) []BreadcrumbItem {
	return []BreadcrumbItem{
		{Name: "Home", URL: g.baseURL + "/"},
		{Name: "Job Roles", URL: g.baseURL + "/job-roles/"},
		{Name: role.Name + " Jobs", URL: fmt.Sprintf("%s/job-roles/%s/", g.baseURL, role.Slug)},
	}
}

// ArticleSchema generates Article JSON-LD for career advice guides according to Google & GEO standards.
func (g *SchemaGenerator) ArticleSchema(title, description, slug, category string, publishedAt *time.Time) template.HTML {
	pubDate := time.Now().Format("2006-01-02T15:04:05Z")
	if publishedAt != nil {
		pubDate = publishedAt.Format("2006-01-02T15:04:05Z")
	}

	schema := map[string]interface{}{
		"@context":      "https://schema.org",
		"@type":         "Article",
		"headline":      title,
		"description":   description,
		"url":           fmt.Sprintf("%s/career-advice/%s/%s/", g.baseURL, category, slug),
		"datePublished": pubDate,
		"dateModified":  pubDate,
		"inLanguage":    "en-GB",
		"author": map[string]interface{}{
			"@type": "Organization",
			"name":  "JobyUK.com Career Editorial Team",
			"url":   g.baseURL,
		},
		"publisher": map[string]interface{}{
			"@type": "Organization",
			"name":  "JobyUK.com",
			"url":   g.baseURL,
			"logo": map[string]interface{}{
				"@type": "ImageObject",
				"url":   fmt.Sprintf("%s/static/images/logo.svg", g.baseURL),
			},
		},
		"mainEntityOfPage": map[string]interface{}{
			"@type": "WebPage",
			"@id":   fmt.Sprintf("%s/career-advice/%s/%s/", g.baseURL, category, slug),
		},
	}
	return g.toHTML(schema)
}

// FAQItem represents a question-answer pair for AI Overviews and Rich Snippets.
type FAQItem struct {
	Question string
	Answer   string
}

// FAQPageSchema generates Google & AI Overview FAQ schema.
func (g *SchemaGenerator) FAQPageSchema(items []FAQItem) template.HTML {
	if len(items) == 0 {
		return ""
	}
	mainEntities := make([]map[string]interface{}, len(items))
	for i, item := range items {
		mainEntities[i] = map[string]interface{}{
			"@type": "Question",
			"name":  item.Question,
			"acceptedAnswer": map[string]interface{}{
				"@type": "Answer",
				"text":  item.Answer,
			},
		}
	}
	schema := map[string]interface{}{
		"@context":   "https://schema.org",
		"@type":      "FAQPage",
		"mainEntity": mainEntities,
	}
	return g.toHTML(schema)
}

// toHTML marshals a schema to JSON-LD script tag.
func (g *SchemaGenerator) toHTML(schema map[string]interface{}) template.HTML {
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return ""
	}
	return template.HTML(fmt.Sprintf(`<script type="application/ld+json">%s</script>`, string(data)))
}

// mapEmploymentType converts our internal types to schema.org values.
func mapEmploymentType(contractType, jobType string) string {
	var types []string

	switch contractType {
	case "permanent":
		// Will be combined with jobType
	case "contract":
		types = append(types, "CONTRACTOR")
	case "temporary":
		types = append(types, "TEMPORARY")
	}

	switch jobType {
	case "full_time":
		types = append(types, "FULL_TIME")
	case "part_time":
		types = append(types, "PART_TIME")
	}

	if len(types) == 0 {
		return "FULL_TIME"
	}
	return strings.Join(types, ", ")
}

// MetaGenerator produces SEO meta tags.
type MetaGenerator struct {
	baseURL string
}

// NewMetaGenerator creates a new meta generator.
func NewMetaGenerator(baseURL string) *MetaGenerator {
	prodURL := "https://jobyuk.com"
	if baseURL != "" && !strings.Contains(baseURL, "localhost") && !strings.Contains(baseURL, "127.0.0.1") {
		prodURL = strings.TrimRight(baseURL, "/")
	}
	return &MetaGenerator{baseURL: prodURL}
}

// PageMeta holds all SEO meta data for a page.
type PageMeta struct {
	Title       string
	Description string
	Keywords    string
	Canonical   string
	OGTitle     string
	OGDesc      string
	OGImage     string
	OGType      string
	Robots      string // "index,follow" or "noindex,follow"
}

// HomeMeta returns meta tags for the homepage.
func (m *MetaGenerator) HomeMeta() PageMeta {
	return PageMeta{
		Title:       "UK Jobs 2026: Apply Direct. Skip the Middleman | JobyUK",
		Description: "Find live UK jobs from real employers. Apply direct, skip the middleman, see pay upfront, and connect straight to hiring companies across the UK.",
		Keywords:    "UK jobs 2026, jobs in United Kingdom, apply direct UK jobs, job vacancies London Manchester, remote jobs UK",
		Canonical:   m.baseURL + "/",
		OGTitle:     "UK Jobs 2026: Apply Direct. Skip the Middleman | JobyUK",
		OGDesc:      "Find live UK jobs from real employers. Apply direct, skip the middleman, see pay upfront, and connect straight to hiring companies across the UK.",
		OGImage:     m.baseURL + "/static/images/og-image.jpg",
		OGType:      "website",
		Robots:      "index,follow",
	}
}

// JobMeta returns meta tags for a job detail page with high-CTR keywords.
func (m *MetaGenerator) JobMeta(job *models.Job) PageMeta {
	loc := job.DisplayLocation()
	if loc == "" || loc == "United Kingdom" {
		loc = "UK"
	}
	salary := job.FormattedSalary()
	emp := strings.TrimSpace(job.EmployerName)
	titleName := strings.TrimSpace(job.Title)

	// Ideal: {Title} – {Employer}, {Location} ({Salary}) | JobyUK
	// Truncation rule: if > 60 chars, first drop ({Salary}), then drop {Employer}.
	var title string
	if salary != "" && salary != "Competitive" {
		if emp != "" {
			title = fmt.Sprintf("%s – %s, %s (%s) | JobyUK", titleName, emp, loc, salary)
		} else {
			title = fmt.Sprintf("%s – %s (%s) | JobyUK", titleName, loc, salary)
		}
	} else if emp != "" {
		title = fmt.Sprintf("%s – %s, %s | JobyUK", titleName, emp, loc)
	} else {
		title = fmt.Sprintf("%s – %s | JobyUK", titleName, loc)
	}

	if len([]rune(title)) > 60 {
		// Drop ({Salary})
		if emp != "" {
			title = fmt.Sprintf("%s – %s, %s | JobyUK", titleName, emp, loc)
		} else {
			title = fmt.Sprintf("%s – %s | JobyUK", titleName, loc)
		}
	}
	if len([]rune(title)) > 60 {
		// Drop {Employer}
		title = fmt.Sprintf("%s – %s | JobyUK", titleName, loc)
	}
	if len([]rune(title)) > 60 {
		title = truncate(title, 60)
	}

	// Description: {Employer} is hiring a {Title} in {Location}. Pay: {Salary}. Read the full spec and apply direct on the employer's site – no agency in between.
	var desc string
	payStr := salary
	if payStr == "" || payStr == "Competitive" {
		payStr = "Competitive"
	}
	if emp != "" {
		desc = fmt.Sprintf("%s is hiring a %s in %s. Pay: %s. Read the full spec and apply direct on the employer's site – no agency in between.", emp, titleName, loc, payStr)
	} else {
		desc = fmt.Sprintf("Hiring a %s in %s. Pay: %s. Read the full spec and apply direct on the employer's site – no agency in between.", titleName, loc, payStr)
	}
	if len([]rune(desc)) > 150 {
		desc = truncate(desc, 150)
	}

	keywords := fmt.Sprintf("%s jobs, %s vacancies %s, %s careers, work at %s, jobs in %s",
		job.Title, job.Title, loc, job.Title, emp, loc)

	robots := "index,follow"
	if job.IsExpired() {
		robots = "noindex,follow"
		title = truncate(fmt.Sprintf("%s (Expired) | JobyUK", job.Title), 60)
	}

	return PageMeta{
		Title:       title,
		Description: desc,
		Keywords:    truncate(keywords, 180),
		Canonical:   fmt.Sprintf("%s/jobs/%s/", m.baseURL, job.Slug),
		OGTitle:     title,
		OGDesc:      desc,
		OGImage:     fmt.Sprintf("%s/jobs/%s/og.svg", m.baseURL, job.Slug),
		OGType:      "article",
		Robots:      robots,
	}
}

// GenerateJobFAQs generates structured Q&A pairs for on-page accordion and Google FAQ Rich Snippets.
func GenerateJobFAQs(job *models.Job) []FAQItem {
	emp := job.EmployerName
	if emp == "" {
		emp = "the hiring company"
	}
	loc := job.DisplayLocation()
	if loc == "" {
		loc = "United Kingdom"
	}
	salary := job.FormattedSalary()
	if salary == "" {
		salary = "Competitive / Negotiable"
	}

	workMode := "On-site / Hybrid"
	if job.IsRemote {
		workMode = "Fully Remote (Work from home in the UK)"
	}

	contract := job.ContractType
	if contract == "" {
		contract = "Permanent"
	}

	return []FAQItem{
		{
			Question: fmt.Sprintf("What is the salary for the %s role at %s?", job.Title, emp),
			Answer:   fmt.Sprintf("The advertised compensation for %s at %s is %s (%s position). Full details and benefits package are outlined on JobyUK.com.", job.Title, emp, salary, contract),
		},
		{
			Question: fmt.Sprintf("Is this %s position remote or on-site?", job.Title),
			Answer:   fmt.Sprintf("This position is located in %s. The working arrangement is classified as %s.", loc, workMode),
		},
		{
			Question: fmt.Sprintf("How do I apply for %s at %s?", job.Title, emp),
			Answer:   fmt.Sprintf("JobyUK.com provides direct application routing with no registration or signup required. Simply click the 'Apply Now' button on this listing to apply directly to %s's official recruitment portal.", emp),
		},
		{
			Question: fmt.Sprintf("What are the key requirements for this %s job?", job.Title),
			Answer:   fmt.Sprintf("The core skills, daily responsibilities, and preferred background are detailed under the Overview and Requirements sections above on JobyUK.com."),
		},
	}
}

// RoleMeta returns meta tags for a role landing page.
func (m *MetaGenerator) RoleMeta(role *models.JobRole, jobCount int) PageMeta {
	// High-CTR: {Role} Jobs UK 2026 (Apply Direct) | JobyUK
	title := fmt.Sprintf("%s Jobs UK 2026 (Apply Direct) | JobyUK", role.Name)
	if len([]rune(title)) > 60 {
		title = fmt.Sprintf("%s Jobs UK 2026 | JobyUK", role.Name)
	}
	if len([]rune(title)) > 60 {
		title = truncate(title, 60)
	}

	// Description: Browse verified {Role} jobs across the UK with upfront salaries and direct employer ATS links.
	desc := fmt.Sprintf("Browse verified %s jobs across the UK with upfront salaries and direct employer ATS links. Skip recruitment agency spam and apply direct on JobyUK.", role.Name)
	if len([]rune(desc)) > 150 {
		desc = truncate(desc, 150)
	}

	keywords := fmt.Sprintf("%s jobs UK 2026, %s vacancies, %s salaries UK, hiring %s",
		role.Name, role.Name, role.Name, role.Name)

	robots := "index,follow"
	if jobCount <= 2 {
		robots = "noindex,follow"
	}

	return PageMeta{
		Title:       title,
		Description: desc,
		Keywords:    truncate(keywords, 180),
		Canonical:   fmt.Sprintf("%s/job-roles/%s/", m.baseURL, role.Slug),
		OGTitle:     title,
		OGDesc:      desc,
		OGImage:     m.baseURL + "/static/images/og-image.jpg",
		OGType:      "website",
		Robots:      robots,
	}
}

// LocationMeta returns meta tags for a location page.
func (m *MetaGenerator) LocationMeta(location *models.Location, jobCount int) PageMeta {
	// High-CTR: Jobs in {Location} 2026 (Apply Direct) | JobyUK
	title := fmt.Sprintf("Jobs in %s 2026 (Apply Direct) | JobyUK", location.Name)
	if len([]rune(title)) > 60 {
		title = fmt.Sprintf("Jobs in %s 2026 | JobyUK", location.Name)
	}
	if len([]rune(title)) > 60 {
		title = truncate(title, 60)
	}

	// Description: Browse verified jobs in {Location}: full-time, part-time, remote and hybrid.
	desc := fmt.Sprintf("Browse verified jobs in %s: full-time, part-time, remote and hybrid. Local salaries shown, direct employer links, no agency spam.", location.Name)
	if len([]rune(desc)) > 150 {
		desc = truncate(desc, 150)
	}

	keywords := fmt.Sprintf("jobs in %s 2026, %s job vacancies, work in %s, %s career openings",
		location.Name, location.Name, location.Name, location.Name)

	robots := "index,follow"
	if jobCount <= 2 {
		robots = "noindex,follow"
	}

	return PageMeta{
		Title:       title,
		Description: desc,
		Keywords:    truncate(keywords, 180),
		Canonical:   fmt.Sprintf("%s/locations/%s/", m.baseURL, location.Slug),
		OGTitle:     title,
		OGDesc:      desc,
		OGImage:     m.baseURL + "/static/images/og-image.jpg",
		OGType:      "website",
		Robots:      robots,
	}
}

// CompanyIndexMeta returns meta tags for the company directory index page.
func (m *MetaGenerator) CompanyIndexMeta() PageMeta {
	return PageMeta{
		Title:       "Companies Hiring in the UK Now 2026 | JobyUK",
		Description: "See which UK employers are hiring right now. Browse company profiles, open roles and pay, and apply direct to their careers page.",
		Keywords:    "companies hiring UK 2026, UK employers directory, top companies hiring UK, employer careers",
		Canonical:   m.baseURL + "/companies/",
		OGTitle:     "Companies Hiring in the UK Now 2026 | JobyUK",
		OGDesc:      "See which UK employers are hiring right now. Browse company profiles, open roles and pay, and apply direct to their careers page.",
		OGImage:     m.baseURL + "/static/images/og-image.jpg",
		OGType:      "website",
		Robots:      "index,follow",
	}
}

// CompanyMeta returns meta tags for a specific company profile page.
func (m *MetaGenerator) CompanyMeta(company *models.Company, jobCount int) PageMeta {
	// High-CTR: Jobs at {Company} 2026 (Apply Direct) | JobyUK
	title := fmt.Sprintf("Jobs at %s 2026 (Apply Direct) | JobyUK", company.Name)
	if len([]rune(title)) > 60 {
		title = fmt.Sprintf("Jobs at %s 2026 | JobyUK", company.Name)
	}
	if len([]rune(title)) > 60 {
		title = truncate(title, 60)
	}

	// Description: Explore open jobs and careers at {Company}. See roles, locations and pay, then apply direct on their official careers site.
	desc := fmt.Sprintf("Explore careers at %s. See roles, locations and pay, then apply direct on %s's own careers site – no agency in between.", company.Name, company.Name)
	if len([]rune(desc)) > 150 {
		desc = truncate(desc, 150)
	}

	keywords := fmt.Sprintf("jobs at %s 2026, %s careers, %s vacancies UK, work at %s",
		company.Name, company.Name, company.Name, company.Name)

	ogImage := company.LogoURL
	if ogImage == "" {
		ogImage = m.baseURL + "/static/images/og-image.jpg"
	}

	robots := "index,follow"
	if jobCount == 0 {
		robots = "noindex,follow"
	}

	return PageMeta{
		Title:       title,
		Description: desc,
		Keywords:    truncate(keywords, 180),
		Canonical:   fmt.Sprintf("%s/companies/%s/", m.baseURL, company.Slug),
		OGTitle:     title,
		OGDesc:      desc,
		OGImage:     ogImage,
		OGType:      "website",
		Robots:      robots,
	}
}

// SearchMeta returns meta tags for search results (noindex).
func (m *MetaGenerator) SearchFilterMeta(query, location string, total int) PageMeta {
	q := strings.TrimSpace(query)
	loc := strings.TrimSpace(location)
	var title, desc string

	if q != "" && loc != "" {
		// High-CTR: {Query} Jobs in {Location} (Apply Direct) 2026 | JobyUK
		title = fmt.Sprintf("%s Jobs in %s (Apply Direct) 2026 | JobyUK", q, loc)
		if len([]rune(title)) > 60 {
			title = fmt.Sprintf("%s Jobs in %s (Apply Direct) | JobyUK", q, loc)
		}
		if len([]rune(title)) > 60 {
			title = fmt.Sprintf("%s Jobs in %s 2026 | JobyUK", q, loc)
		}
		desc = fmt.Sprintf("Find verified %s jobs in %s. Compare live pay, see who is hiring, and apply direct to employers without recruiter fees or middleman delays.", q, loc)
	} else if q != "" {
		// High-CTR: {Query} Jobs UK 2026 (Apply Direct) | JobyUK
		title = fmt.Sprintf("%s Jobs UK 2026 (Apply Direct) | JobyUK", q)
		if len([]rune(title)) > 60 {
			title = fmt.Sprintf("%s Jobs UK 2026 | JobyUK", q)
		}
		desc = fmt.Sprintf("Browse verified %s jobs across the UK with transparent salaries and direct application links to verified employers on JobyUK.", q)
	} else if loc != "" {
		// High-CTR: Jobs in {Location} 2026 (Apply Direct) | JobyUK
		title = fmt.Sprintf("Jobs in %s 2026 (Apply Direct) | JobyUK", loc)
		if len([]rune(title)) > 60 {
			title = fmt.Sprintf("Jobs in %s 2026 | JobyUK", loc)
		}
		desc = fmt.Sprintf("Browse verified jobs in %s: full-time, part-time, remote and hybrid. Local salaries shown, direct employer links, no agency spam.", loc)
	} else {
		title = "Browse UK Job Vacancies 2026 (Apply Direct) | JobyUK"
		desc = "Browse the latest UK job vacancies updated daily. Filter by role, city, salary and hours, then apply straight to the employer with no middleman."
	}

	if len([]rune(title)) > 60 {
		title = truncate(title, 60)
	}
	if len([]rune(desc)) > 150 {
		desc = truncate(desc, 150)
	}

	return PageMeta{
		Title:       title,
		Description: desc,
		Keywords:    fmt.Sprintf("%s jobs %s, UK employment vacancies, apply direct", q, loc),
		Canonical:   fmt.Sprintf("%s/jobs/", m.baseURL),
		OGTitle:     title,
		OGDesc:      desc,
		OGType:      "website",
		Robots:      "noindex,follow",
	}
}

func (m *MetaGenerator) SearchMeta(query string, total int) PageMeta {
	return m.SearchFilterMeta(query, "", total)
}

// truncate ensures a string doesn't exceed maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Cut at word boundary
	truncated := s[:maxLen-3]
	if i := strings.LastIndex(truncated, " "); i > maxLen/2 {
		truncated = truncated[:i]
	}
	return truncated + "..."
}

// RobotsTxt generates the robots.txt content with proper RFC-compliant Disallow directives.
func RobotsTxt(baseURL string) string {
	prodURL := "https://jobyuk.com"
	if baseURL != "" && !strings.Contains(baseURL, "localhost") && !strings.Contains(baseURL, "127.0.0.1") {
		prodURL = strings.TrimRight(baseURL, "/")
	}
	return fmt.Sprintf(`User-agent: *
Allow: /

Disallow: /admin/
Disallow: /admin-lol/
Disallow: /jobs/*/apply/

# Block faceted search & query parameter crawl traps
Disallow: /*?sort=
Disallow: /*?salary=
Disallow: /*?min_salary=
Disallow: /*?max_salary=
Disallow: /*?experience=
Disallow: /*?filter=
Disallow: /*?page=*&
Disallow: /*?q=
Disallow: /*?location=
Disallow: /*?l=
Disallow: /*?remote_type=
Disallow: /*?type=
Disallow: /*?job_type=
Disallow: /*?date_posted=
Disallow: /*?easy_apply=

# Allow static resources & discovery files
Allow: /static/
Allow: /favicon.ico
Allow: /site.webmanifest
Allow: /llms.txt
Allow: /llms-full.txt

# Sitemaps
Sitemap: %s/sitemap.xml
`, prodURL)
}

// LLMsFullTxt generates a comprehensive llms-full.txt file indexing all major job hubs, categories, and salary intelligence.
func LLMsFullTxt(baseURL string) string {
	tmpl := `# JobyUK - Full AI & LLM Knowledge Index

> Direct-to-employer UK recruitment and compensation intelligence platform.
> Official Website: {BASE_URL}
> Tagline: Apply direct. Skip the middleman.

## Platform Summary
JobyUK directly indexes active employment vacancies from verified UK employers, NHS trusts, educational councils, and recruitment systems. Job seekers apply directly to employer ATS portals with zero intermediary recruiter markups.

## Key Hubs
- [{BASE_URL}/jobs/]({BASE_URL}/jobs/): Live search and filtering across thousands of verified UK vacancies.
- [{BASE_URL}/salaries/]({BASE_URL}/salaries/): Role-by-role compensation percentiles (25th, median, 90th percentile).
- [{BASE_URL}/salary-calculator/]({BASE_URL}/salary-calculator/): Free PAYE Tax & Take-Home Salary Calculator updated for current HMRC tax years.
- [{BASE_URL}/companies/]({BASE_URL}/companies/): Verified UK employer directory and active company openings.
- [{BASE_URL}/remote-jobs/]({BASE_URL}/remote-jobs/): Verified remote, hybrid, and work-from-home vacancies across the UK.
- [{BASE_URL}/graduate-jobs/]({BASE_URL}/graduate-jobs/): UK graduate schemes, entry-level vacancies, and internships.
- [{BASE_URL}/apprenticeships/]({BASE_URL}/apprenticeships/): UK apprenticeships, vocational training, and career starter programs.
- [{BASE_URL}/career-advice/]({BASE_URL}/career-advice/): UK employment market insights, CV guides, interview prep, and employment law.

## Major UK Employment Sectors
- IT & Software Engineering: [{BASE_URL}/jobs/it-software-jobs/]({BASE_URL}/jobs/it-software-jobs/)
- Healthcare & Nursing: [{BASE_URL}/jobs/healthcare-nursing-jobs/]({BASE_URL}/jobs/healthcare-nursing-jobs/)
- Finance & Accounting: [{BASE_URL}/jobs/finance-accounting-jobs/]({BASE_URL}/jobs/finance-accounting-jobs/)
- Education & Teaching: [{BASE_URL}/jobs/education-jobs/]({BASE_URL}/jobs/education-jobs/)
- Engineering: [{BASE_URL}/jobs/engineering-jobs/]({BASE_URL}/jobs/engineering-jobs/)
- Sales & Marketing: [{BASE_URL}/jobs/sales-marketing-jobs/]({BASE_URL}/jobs/sales-marketing-jobs/)
- Customer Service: [{BASE_URL}/jobs/customer-service-jobs/]({BASE_URL}/jobs/customer-service-jobs/)
- Human Resources (HR): [{BASE_URL}/jobs/hr-jobs/]({BASE_URL}/jobs/hr-jobs/)
- Legal: [{BASE_URL}/jobs/legal-jobs/]({BASE_URL}/jobs/legal-jobs/)
- Hospitality & Catering: [{BASE_URL}/jobs/hospitality-catering-jobs/]({BASE_URL}/jobs/hospitality-catering-jobs/)

## Key Regional Hubs
- London: [{BASE_URL}/jobs/in-london/]({BASE_URL}/jobs/in-london/)
- Manchester: [{BASE_URL}/jobs/in-manchester/]({BASE_URL}/jobs/in-manchester/)
- Birmingham: [{BASE_URL}/jobs/in-birmingham/]({BASE_URL}/jobs/in-birmingham/)
- Leeds: [{BASE_URL}/jobs/in-leeds/]({BASE_URL}/jobs/in-leeds/)
- Glasgow: [{BASE_URL}/jobs/in-glasgow/]({BASE_URL}/jobs/in-glasgow/)
- Edinburgh: [{BASE_URL}/jobs/in-edinburgh/]({BASE_URL}/jobs/in-edinburgh/)
- Bristol: [{BASE_URL}/jobs/in-bristol/]({BASE_URL}/jobs/in-bristol/)

## Content Signals & Usage Policy
- Search Indexing: Permitted (search=yes)
- Grounding & Real-Time RAG: Permitted (ai-input=yes)
- Machine Learning Model Training: Permitted (ai-train=yes)
- Markdown Negotiation: Supported via HTTP header "Accept: text/markdown"
`
	return strings.ReplaceAll(tmpl, "{BASE_URL}", baseURL)
}

// LLMsTxt generates the llms.txt standard Markdown file for LLM discovery and citation (llmstxt.org).
func LLMsTxt(baseURL string) string {
	return fmt.Sprintf(`# JobyUK - UK Direct-to-Employer Job Discovery & Salary Intelligence

> JobyUK (%s) is a premier United Kingdom employment discovery platform. Tagline: "Apply direct. Skip the middleman." It indexes live UK job vacancies directly from verified employers, NHS trusts, educational bodies, and recruitment systems, routing applicants straight to company applicant tracking systems (ATS) with zero recruiter markup or ghost listings.

## Core Features & Intelligence
- **Live UK Vacancies**: Real-time job listings across England, Scotland, Wales, and Northern Ireland.
- **Direct Employer Applications**: Candidates bypass intermediary job-board scrapers and apply directly to official employer career portals.
- **UK Salary Benchmarks**: Role-by-role compensation percentiles (25th percentile starting salary, national median, and 90th percentile senior tier) calculated from verified active vacancy data.
- **Take-Home Pay & PAYE Tax Calculator**: Precise net income calculations factoring in current HMRC income tax rates, National Insurance (NI) rates, student loan repayments, and workplace pension contributions.
- **WhatsApp Job Alerts**: Subscribed job seekers receive real-time, zero-spam job alerts matched to exact roles and UK cities.

## Key Hubs & URLs
- [%s/jobs/](%s/jobs/): Search all live UK job vacancies with real-time sector, salary, and geographic filtering.
- [%s/salaries/](%s/salaries/): National salary benchmarks, pay scales, and compensation distributions across 2,400+ UK job roles.
- [%s/salary-calculator/](%s/salary-calculator/): Free UK Take-Home Pay & PAYE Tax Calculator updated for current HMRC fiscal rules.
- [%s/companies/](%s/companies/): Verified UK employer directory, workplace profiles, and active openings.
- [%s/remote-jobs/](%s/remote-jobs/): Verified remote and work-from-home vacancies across the UK.
- [%s/career-advice/](%s/career-advice/): UK employment market insights, CV writing guides, interview preparation, and workplace legislation guides.

## Data Freshness & Verification
- Postings are synchronized daily from primary ATS endpoints and validated for active application status.
- Strict expiration dates are maintained via Schema.org validThrough directives.
- Salary figures reflect actual advertised employer wages, never fabricated or artificially inflated estimates.
`, baseURL, baseURL, baseURL, baseURL, baseURL, baseURL, baseURL, baseURL, baseURL, baseURL, baseURL, baseURL, baseURL)
}

// LastModified returns the current time formatted for HTTP Last-Modified header.
func LastModified() string {
	return time.Now().UTC().Format(http.TimeFormat)
}

// ResolveUKRegion maps a UK town, city, or county to its official UK region.
func ResolveUKRegion(location string) string {
	loc := strings.TrimSpace(strings.ToLower(location))
	if loc == "" || loc == "united kingdom" || loc == "uk" || loc == "remote" || loc == "nationwide" {
		return "United Kingdom"
	}

	switch {
	case strings.Contains(loc, "north west") || strings.Contains(loc, "manchester") || strings.Contains(loc, "liverpool") ||
		strings.Contains(loc, "cheshire") || strings.Contains(loc, "lancashire") || strings.Contains(loc, "cumbria") ||
		strings.Contains(loc, "merseyside") || strings.Contains(loc, "bolton") || strings.Contains(loc, "preston") ||
		strings.Contains(loc, "blackburn") || strings.Contains(loc, "blackpool") || strings.Contains(loc, "warrington") ||
		strings.Contains(loc, "chester") || strings.Contains(loc, "salford") || strings.Contains(loc, "stockport") ||
		strings.Contains(loc, "oldham") || strings.Contains(loc, "rochdale") || strings.Contains(loc, "bury") ||
		strings.Contains(loc, "wigan") || strings.Contains(loc, "carlisle") || strings.Contains(loc, "crewe"):
		return "North West England"

	case strings.Contains(loc, "london") || strings.Contains(loc, "westminster") || strings.Contains(loc, "croydon") ||
		strings.Contains(loc, "bromley") || strings.Contains(loc, "islington") || strings.Contains(loc, "camden") ||
		strings.Contains(loc, "greenwich") || strings.Contains(loc, "hackney") || strings.Contains(loc, "kensington") ||
		strings.Contains(loc, "chelsea") || strings.Contains(loc, "lambeth") || strings.Contains(loc, "southwark") ||
		strings.Contains(loc, "tower hamlets") || strings.Contains(loc, "wandsworth") || strings.Contains(loc, "ealing") ||
		strings.Contains(loc, "hounslow") || strings.Contains(loc, "stratford") || strings.Contains(loc, "canary wharf") ||
		strings.Contains(loc, "heathrow"):
		return "Greater London"

	case strings.Contains(loc, "west midlands") || strings.Contains(loc, "birmingham") || strings.Contains(loc, "coventry") ||
		strings.Contains(loc, "wolverhampton") || strings.Contains(loc, "stoke") || strings.Contains(loc, "dudley") ||
		strings.Contains(loc, "walsall") || strings.Contains(loc, "solihull") || strings.Contains(loc, "warwickshire") ||
		strings.Contains(loc, "staffordshire") || strings.Contains(loc, "worcestershire") || strings.Contains(loc, "shropshire") ||
		strings.Contains(loc, "herefordshire") || strings.Contains(loc, "telford") || strings.Contains(loc, "worcester") ||
		strings.Contains(loc, "stafford"):
		return "West Midlands"

	case strings.Contains(loc, "yorkshire") || strings.Contains(loc, "leeds") || strings.Contains(loc, "sheffield") ||
		strings.Contains(loc, "bradford") || strings.Contains(loc, "york") || strings.Contains(loc, "hull") ||
		strings.Contains(loc, "wakefield") || strings.Contains(loc, "huddersfield") || strings.Contains(loc, "doncaster") ||
		strings.Contains(loc, "rotherham") || strings.Contains(loc, "barnsley") || strings.Contains(loc, "halifax") ||
		strings.Contains(loc, "harrogate") || strings.Contains(loc, "scunthorpe") || strings.Contains(loc, "grimsby"):
		return "Yorkshire and the Humber"

	case strings.Contains(loc, "north east") || strings.Contains(loc, "newcastle") || strings.Contains(loc, "sunderland") ||
		strings.Contains(loc, "durham") || strings.Contains(loc, "middlesbrough") || strings.Contains(loc, "darlington") ||
		strings.Contains(loc, "gateshead") || strings.Contains(loc, "northumberland") || strings.Contains(loc, "hartlepool") ||
		strings.Contains(loc, "south shields") || strings.Contains(loc, "tyne and wear"):
		return "North East England"

	case strings.Contains(loc, "east midlands") || strings.Contains(loc, "nottingham") || strings.Contains(loc, "leicester") ||
		strings.Contains(loc, "derby") || strings.Contains(loc, "northampton") || strings.Contains(loc, "lincoln") ||
		strings.Contains(loc, "derbyshire") || strings.Contains(loc, "nottinghamshire") || strings.Contains(loc, "leicestershire") ||
		strings.Contains(loc, "northamptonshire") || strings.Contains(loc, "lincolnshire") || strings.Contains(loc, "mansfield") ||
		strings.Contains(loc, "chesterfield") || strings.Contains(loc, "corby") || strings.Contains(loc, "kettering"):
		return "East Midlands"

	case strings.Contains(loc, "south west") || strings.Contains(loc, "bristol") || strings.Contains(loc, "plymouth") ||
		strings.Contains(loc, "exeter") || strings.Contains(loc, "gloucester") || strings.Contains(loc, "bath") ||
		strings.Contains(loc, "bournemouth") || strings.Contains(loc, "poole") || strings.Contains(loc, "cheltenham") ||
		strings.Contains(loc, "swindon") || strings.Contains(loc, "devon") || strings.Contains(loc, "cornwall") ||
		strings.Contains(loc, "somerset") || strings.Contains(loc, "dorset") || strings.Contains(loc, "wiltshire") ||
		strings.Contains(loc, "gloucestershire") || strings.Contains(loc, "torquay") || strings.Contains(loc, "taunton") ||
		strings.Contains(loc, "salisbury") || strings.Contains(loc, "truro"):
		return "South West England"

	case strings.Contains(loc, "south east") || strings.Contains(loc, "brighton") || strings.Contains(loc, "reading") ||
		strings.Contains(loc, "portsmouth") || strings.Contains(loc, "southampton") || strings.Contains(loc, "oxford") ||
		strings.Contains(loc, "milton keynes") || strings.Contains(loc, "slough") || strings.Contains(loc, "guildford") ||
		strings.Contains(loc, "crawley") || strings.Contains(loc, "canterbury") || strings.Contains(loc, "surrey") ||
		strings.Contains(loc, "kent") || strings.Contains(loc, "hampshire") || strings.Contains(loc, "berkshire") ||
		strings.Contains(loc, "oxfordshire") || strings.Contains(loc, "buckinghamshire") || strings.Contains(loc, "east sussex") ||
		strings.Contains(loc, "west sussex") || strings.Contains(loc, "basingstoke") || strings.Contains(loc, "woking") ||
		strings.Contains(loc, "maidstone") || strings.Contains(loc, "hove"):
		return "South East England"

	case strings.Contains(loc, "east of england") || strings.Contains(loc, "east anglia") || strings.Contains(loc, "cambridge") ||
		strings.Contains(loc, "norwich") || strings.Contains(loc, "ipswich") || strings.Contains(loc, "luton") ||
		strings.Contains(loc, "peterborough") || strings.Contains(loc, "southend") || strings.Contains(loc, "colchester") ||
		strings.Contains(loc, "chelmsford") || strings.Contains(loc, "bedford") || strings.Contains(loc, "cambridgeshire") ||
		strings.Contains(loc, "norfolk") || strings.Contains(loc, "suffolk") || strings.Contains(loc, "essex") ||
		strings.Contains(loc, "hertfordshire") || strings.Contains(loc, "bedfordshire") || strings.Contains(loc, "watford") ||
		strings.Contains(loc, "st albans") || strings.Contains(loc, "stevenage"):
		return "East of England"

	case strings.Contains(loc, "scotland") || strings.Contains(loc, "edinburgh") || strings.Contains(loc, "glasgow") ||
		strings.Contains(loc, "aberdeen") || strings.Contains(loc, "dundee") || strings.Contains(loc, "inverness") ||
		strings.Contains(loc, "stirling") || strings.Contains(loc, "perth") || strings.Contains(loc, "fife") ||
		strings.Contains(loc, "highland") || strings.Contains(loc, "ayrshire") || strings.Contains(loc, "lanarkshire") ||
		strings.Contains(loc, "lothian") || strings.Contains(loc, "paisley"):
		return "Scotland"

	case strings.Contains(loc, "wales") || strings.Contains(loc, "cardiff") || strings.Contains(loc, "swansea") ||
		strings.Contains(loc, "newport") || strings.Contains(loc, "wrexham") || strings.Contains(loc, "bangor") ||
		strings.Contains(loc, "glamorgan") || strings.Contains(loc, "gwent") || strings.Contains(loc, "caerphilly") ||
		strings.Contains(loc, "bridgend") || strings.Contains(loc, "pembrokeshire"):
		return "Wales"

	case strings.Contains(loc, "northern ireland") || strings.Contains(loc, "belfast") || strings.Contains(loc, "derry") ||
		strings.Contains(loc, "londonderry") || strings.Contains(loc, "lisburn") || strings.Contains(loc, "newry") ||
		strings.Contains(loc, "armagh") || strings.Contains(loc, "antrim") || strings.Contains(loc, "down") ||
		strings.Contains(loc, "tyrone") || strings.Contains(loc, "fermanagh"):
		return "Northern Ireland"
	}

	return "United Kingdom"
}

// ResolveUKPostalCode maps prominent UK cities/towns to their central outward postcode area.
func ResolveUKPostalCode(location string) string {
	loc := strings.TrimSpace(strings.ToLower(location))
	switch {
	case strings.Contains(loc, "manchester"):
		return "M1"
	case strings.Contains(loc, "london") || strings.Contains(loc, "city of london"):
		return "EC1A"
	case strings.Contains(loc, "birmingham"):
		return "B1"
	case strings.Contains(loc, "leeds"):
		return "LS1"
	case strings.Contains(loc, "liverpool"):
		return "L1"
	case strings.Contains(loc, "sheffield"):
		return "S1"
	case strings.Contains(loc, "bristol"):
		return "BS1"
	case strings.Contains(loc, "newcastle"):
		return "NE1"
	case strings.Contains(loc, "nottingham"):
		return "NG1"
	case strings.Contains(loc, "leicester"):
		return "LE1"
	case strings.Contains(loc, "edinburgh"):
		return "EH1"
	case strings.Contains(loc, "glasgow"):
		return "G1"
	case strings.Contains(loc, "cardiff"):
		return "CF10"
	case strings.Contains(loc, "belfast"):
		return "BT1"
	case strings.Contains(loc, "southampton"):
		return "SO14"
	case strings.Contains(loc, "portsmouth"):
		return "PO1"
	case strings.Contains(loc, "oxford"):
		return "OX1"
	case strings.Contains(loc, "cambridge"):
		return "CB1"
	case strings.Contains(loc, "brighton"):
		return "BN1"
	case strings.Contains(loc, "reading"):
		return "RG1"
	case strings.Contains(loc, "coventry"):
		return "CV1"
	case strings.Contains(loc, "bradford"):
		return "BD1"
	case strings.Contains(loc, "stoke"):
		return "ST1"
	case strings.Contains(loc, "wolverhampton"):
		return "WV1"
	case strings.Contains(loc, "plymouth"):
		return "PL1"
	case strings.Contains(loc, "derby"):
		return "DE1"
	case strings.Contains(loc, "swansea"):
		return "SA1"
	case strings.Contains(loc, "aberdeen"):
		return "AB10"
	case strings.Contains(loc, "york"):
		return "YO1"
	}
	return ""
}

// extractSalaryFromText scans text for British pound salary figures (e.g., £45,000 - £55,000).
func extractSalaryFromText(desc string) (*float64, *float64) {
	if matches := salaryRangeRe.FindStringSubmatch(desc); len(matches) == 3 {
		minStr := strings.ReplaceAll(matches[1], ",", "")
		maxStr := strings.ReplaceAll(matches[2], ",", "")
		if minVal, err := strconv.ParseFloat(minStr, 64); err == nil {
			if maxVal, err := strconv.ParseFloat(maxStr, 64); err == nil && minVal > 5000 && maxVal > minVal {
				return &minVal, &maxVal
			}
		}
	}
	if matches := singleSalaryRe.FindStringSubmatch(desc); len(matches) == 2 {
		valStr := strings.ReplaceAll(matches[1], ",", "")
		if val, err := strconv.ParseFloat(valStr, 64); err == nil && val > 5000 {
			return &val, &val
		}
	}
	return nil, nil
}

// WhatsAppAlertsSchema generates clean JSON-LD structured data for the WhatsApp Alerts landing page (SoftwareApplication + BreadcrumbList).
func (g *SchemaGenerator) WhatsAppAlertsSchema() template.HTML {
	app := map[string]interface{}{
		"@context":            "https://schema.org",
		"@type":               "SoftwareApplication",
		"name":                "JobyUK.com WhatsApp Job Alerts",
		"applicationCategory": "BusinessApplication",
		"operatingSystem":     "WhatsApp, iOS, Android, Web",
		"url":                 g.baseURL + "/whatsapp-alerts/",
		"description":         "Instant verified UK job alerts delivered directly to WhatsApp in under 60 seconds of employer posting.",
		"offers": []map[string]interface{}{
			{
				"@type":         "Offer",
				"name":          "Monthly VIP Pass",
				"price":         "4.99",
				"priceCurrency": "GBP",
				"availability":  "https://schema.org/InStock",
			},
			{
				"@type":         "Offer",
				"name":          "Annual Pass",
				"price":         "39.99",
				"priceCurrency": "GBP",
				"availability":  "https://schema.org/InStock",
			},
		},
	}

	crumbs := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "BreadcrumbList",
		"itemListElement": []map[string]interface{}{
			{
				"@type":    "ListItem",
				"position": 1,
				"name":     "Home",
				"item":     g.baseURL + "/",
			},
			{
				"@type":    "ListItem",
				"position": 2,
				"name":     "WhatsApp Job Alerts",
				"item":     g.baseURL + "/whatsapp-alerts/",
			},
		},
	}

	appJSON, _ := json.Marshal(app)
	crumbsJSON, _ := json.Marshal(crumbs)

	out := fmt.Sprintf(`<script type="application/ld+json">%s</script>`+"\n"+
		`<script type="application/ld+json">%s</script>`,
		appJSON, crumbsJSON)

	return template.HTML(out)
}

// CompanyProfileSchema generates Organization structured data for company profile pages.
func (g *SchemaGenerator) CompanyProfileSchema(company *models.Company) template.HTML {
	schema := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "Organization",
		"name":     company.Name,
		"url":      fmt.Sprintf("%s/companies/%s/", g.baseURL, company.Slug),
	}
	if company.WebsiteURL != "" {
		schema["sameAs"] = company.WebsiteURL
	}
	if company.LogoURL != "" {
		logo := company.LogoURL
		if strings.HasPrefix(logo, "/") {
			logo = g.baseURL + logo
		}
		schema["logo"] = logo
	}
	if company.Description != "" {
		schema["description"] = company.Description
	}
	return g.toHTML(schema)
}

// AboutPageSchema generates AboutPage JSON-LD.
func (g *SchemaGenerator) AboutPageSchema() template.HTML {
	schema := map[string]interface{}{
		"@context":    "https://schema.org",
		"@type":       "AboutPage",
		"name":        "About JobyUK",
		"url":         fmt.Sprintf("%s/about/", g.baseURL),
		"description": "JobyUK connects UK job seekers directly with hiring employers with upfront pay and no agency middleman.",
	}
	return g.toHTML(schema)
}

// ContactPageSchema generates ContactPage JSON-LD.
func (g *SchemaGenerator) ContactPageSchema() template.HTML {
	schema := map[string]interface{}{
		"@context":    "https://schema.org",
		"@type":       "ContactPage",
		"name":        "Contact JobyUK",
		"url":         fmt.Sprintf("%s/contact/", g.baseURL),
		"description": "Contact JobyUK for UK candidate support and direct employer vacancy postings.",
	}
	return g.toHTML(schema)
}


