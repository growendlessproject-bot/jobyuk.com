package handlers

import (
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"jobyuk/internal/models"
)

var (
	tagRegex     = regexp.MustCompile(`<[^>]+>`)
	pRegex       = regexp.MustCompile(`(?i)</?(p|div|section|article)[^>]*>`)
	brRegex      = regexp.MustCompile(`(?i)<br\s*/?>`)
	liRegex      = regexp.MustCompile(`(?i)<li[^>]*>`)
	endLiRegex   = regexp.MustCompile(`(?i)</li>`)
	bRegex       = regexp.MustCompile(`(?i)<(?:strong|b)[^>]*>(.*?)</(?:strong|b)>`)
	iRegex       = regexp.MustCompile(`(?i)<(?:em|i)[^>]*>(.*?)</(?:em|i)>`)
	hRegex       = regexp.MustCompile(`(?i)<h([1-6])[^>]*>(.*?)</h[1-6]>`)
	aRegex       = regexp.MustCompile(`(?i)<a[^>]*href=["']([^"']*)["'][^>]*>(.*?)</a>`)
	multiNLRegex = regexp.MustCompile(`\n{3,}`)
)

// htmlToMarkdown converts raw HTML into clean, human and LLM-readable Markdown.
func htmlToMarkdown(raw string) string {
	if raw == "" {
		return ""
	}
	s := raw
	// Replace headings with Markdown equivalents
	s = hRegex.ReplaceAllStringFunc(s, func(m string) string {
		match := hRegex.FindStringSubmatch(m)
		if len(match) == 3 {
			level, _ := strconv.Atoi(match[1])
			if level < 1 {
				level = 1
			}
			return "\n\n" + strings.Repeat("#", level) + " " + strings.TrimSpace(match[2]) + "\n\n"
		}
		return m
	})
	// Bold & italics
	s = bRegex.ReplaceAllString(s, "**$1**")
	s = iRegex.ReplaceAllString(s, "*$1*")
	// Links
	s = aRegex.ReplaceAllString(s, "[$2]($1)")
	// Line breaks and list items
	s = brRegex.ReplaceAllString(s, "\n")
	s = liRegex.ReplaceAllString(s, "\n- ")
	s = endLiRegex.ReplaceAllString(s, "")
	s = pRegex.ReplaceAllString(s, "\n\n")
	// Strip all remaining HTML tags
	s = tagRegex.ReplaceAllString(s, "")
	// Unescape HTML entities (&amp;, &pound;, &#39;, &quot;, &nbsp;)
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\u00a0", " ")
	// Collapse excessive newlines
	s = multiNLRegex.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// isMarkdownRequest checks if client asked for Markdown via Accept header or query parameter.
func isMarkdownRequest(r *http.Request) bool {
	if r.URL.Query().Get("format") == "markdown" || r.URL.Query().Get("markdown") == "1" {
		return true
	}
	accept := strings.ToLower(r.Header.Get("Accept"))
	if accept == "" {
		return false
	}
	return strings.Contains(accept, "text/markdown") || strings.Contains(accept, "text/x-markdown")
}

// formatNumber formats an integer with thousands commas.
func formatNumber(n int) string {
	in := strconv.Itoa(n)
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

// renderMarkdown constructs structured, token-optimized Markdown for AI agents and LLMs.
func (h *Handlers) renderMarkdown(templateName string, data PageData) string {
	dataMap, _ := data.Data.(map[string]interface{})
	if dataMap == nil {
		dataMap = make(map[string]interface{})
	}

	switch templateName {
	case "job_detail":
		job, ok := dataMap["Job"].(*models.Job)
		if !ok || job == nil {
			return genericMarkdown(data)
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# %s\n\n", job.Title))
		if job.EmployerName != "" {
			if job.CompanySlug != "" {
				sb.WriteString(fmt.Sprintf("**Company:** [%s](https://jobyuk.com/companies/%s/)\n", job.EmployerName, job.CompanySlug))
			} else {
				sb.WriteString(fmt.Sprintf("**Company:** %s\n", job.EmployerName))
			}
		}
		sb.WriteString(fmt.Sprintf("**Location:** %s\n", job.DisplayLocation()))
		sb.WriteString(fmt.Sprintf("**Salary:** %s\n", job.FormattedSalary()))
		sb.WriteString(fmt.Sprintf("**Job Type:** %s (%s)\n", job.JobTypeSimple(), job.ContractTypeSimple()))
		sb.WriteString(fmt.Sprintf("**Work Mode:** %s\n", job.WorkMode()))
		sb.WriteString(fmt.Sprintf("**Posted:** %s\n", job.DatePosted.Format("02 Jan 2006")))
		if job.ExpirationDate != nil {
			sb.WriteString(fmt.Sprintf("**Application Deadline:** %s\n", job.ExpirationDate.Format("02 Jan 2006")))
		}
		sb.WriteString(fmt.Sprintf("**Direct Apply Link:** https://jobyuk.com/jobs/%d/apply/\n", job.ID))
		sb.WriteString(fmt.Sprintf("**Canonical URL:** https://jobyuk.com/jobs/%s/\n\n", job.Slug))
		sb.WriteString("---\n\n## Job Description\n\n")
		sb.WriteString(htmlToMarkdown(job.Description))
		sb.WriteString("\n\n---\n\n")
		sb.WriteString("## Direct-to-Employer Guarantee\n")
		sb.WriteString("JobyUK indexes direct vacancies from verified UK employers. Apply directly to the official employer tracking portal with zero agency markup.\n")
		return sb.String()

	case "home":
		var sb strings.Builder
		sb.WriteString("# JobyUK - UK Direct-to-Employer Job Discovery & Salary Intelligence\n\n")
		sb.WriteString("> \"Apply direct. Skip the middleman.\"\n\n")
		sb.WriteString("JobyUK (https://jobyuk.com) is the United Kingdom's premier direct-to-employer employment discovery platform. Candidates bypass recruitment middlemen and apply directly to verified employers, NHS trusts, educational bodies, and corporate ATS systems.\n\n")
		if activeCount, ok := dataMap["ActiveCount"].(int); ok && activeCount > 0 {
			sb.WriteString(fmt.Sprintf("- **Active UK Vacancies:** %s verified live roles\n", formatNumber(activeCount)))
		}
		sb.WriteString("\n## Key Discovery Hubs\n")
		sb.WriteString("- [Search All UK Jobs](https://jobyuk.com/jobs/)\n")
		sb.WriteString("- [UK Salary Benchmarks](https://jobyuk.com/salaries/)\n")
		sb.WriteString("- [HMRC PAYE Salary & Take-Home Tax Calculator](https://jobyuk.com/salary-calculator/)\n")
		sb.WriteString("- [Verified UK Employer Directory](https://jobyuk.com/companies/)\n")
		sb.WriteString("- [Remote Jobs UK](https://jobyuk.com/remote-jobs/)\n")
		sb.WriteString("- [Career Advice & Guides](https://jobyuk.com/career-advice/)\n\n")

		if latestJobs, ok := dataMap["LatestJobs"].([]models.Job); ok && len(latestJobs) > 0 {
			sb.WriteString("## Featured Live Vacancies\n\n")
			limit := len(latestJobs)
			if limit > 20 {
				limit = 20
			}
			for _, j := range latestJobs[:limit] {
				sb.WriteString(fmt.Sprintf("- [%s](https://jobyuk.com/jobs/%s/) — %s | %s | %s\n",
					j.Title, j.Slug, j.EmployerName, j.DisplayLocation(), j.FormattedSalary()))
			}
		}
		return sb.String()

	case "job_list", "role", "location", "industry":
		var sb strings.Builder
		title := data.Title
		if title == "" {
			title = "UK Job Vacancies | JobyUK.com"
		}
		sb.WriteString(fmt.Sprintf("# %s\n\n", title))
		if data.Description != "" {
			sb.WriteString(fmt.Sprintf("%s\n\n", data.Description))
		}
		if data.Canonical != "" {
			sb.WriteString(fmt.Sprintf("**Canonical URL:** %s\n\n", data.Canonical))
		}
		var jobs []models.Job
		if jList, ok := dataMap["Jobs"].([]models.Job); ok {
			jobs = jList
		}
		total := len(jobs)
		if t, ok := dataMap["Total"].(int); ok && t > 0 {
			total = t
		}
		sb.WriteString(fmt.Sprintf("**Total Matching Vacancies:** %s\n\n", formatNumber(total)))
		sb.WriteString("## Vacancies\n\n")
		limit := len(jobs)
		if limit > 25 {
			limit = 25
		}
		for i := 0; i < limit; i++ {
			j := jobs[i]
			sb.WriteString(fmt.Sprintf("### [%s](https://jobyuk.com/jobs/%s/)\n", j.Title, j.Slug))
			sb.WriteString(fmt.Sprintf("- **Company:** %s\n", j.EmployerName))
			sb.WriteString(fmt.Sprintf("- **Location:** %s\n", j.DisplayLocation()))
			sb.WriteString(fmt.Sprintf("- **Salary:** %s\n", j.FormattedSalary()))
			sb.WriteString(fmt.Sprintf("- **Type:** %s\n", j.JobTypeSimple()))
			sb.WriteString(fmt.Sprintf("- **Direct Apply:** https://jobyuk.com/jobs/%d/apply/\n", j.ID))
			if j.Description != "" {
				descClean := htmlToMarkdown(j.Description)
				if len(descClean) > 280 {
					descClean = descClean[:280] + "..."
				}
				sb.WriteString(fmt.Sprintf("- **Summary:** %s\n", descClean))
			}
			sb.WriteString("\n")
		}
		return sb.String()

	case "salary":
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# %s\n\n", data.Title))
		if data.Description != "" {
			sb.WriteString(fmt.Sprintf("%s\n\n", data.Description))
		}
		if data.Canonical != "" {
			sb.WriteString(fmt.Sprintf("**URL:** %s\n\n", data.Canonical))
		}
		sb.WriteString("## UK Compensation Intelligence\n")
		sb.WriteString("Benchmarked against thousands of verified active UK employer job advertisements. Percentile figures reflect true advertised compensation across England, Scotland, Wales, and Northern Ireland.\n\n")
		sb.WriteString("- Free PAYE Tax Calculator: https://jobyuk.com/salary-calculator/\n")
		sb.WriteString("- Search live vacancies: https://jobyuk.com/jobs/\n")
		return sb.String()

	case "salary_calculator":
		var sb strings.Builder
		sb.WriteString("# UK Take-Home Pay & Salary Calculator | JobyUK.com\n\n")
		sb.WriteString("Free UK Salary & PAYE Tax Calculator updated for current HMRC fiscal tax rates.\n")
		sb.WriteString("Calculate net take-home pay, income tax bands, National Insurance contributions, student loan deductions, and workplace pension contributions.\n\n")
		sb.WriteString("- Tool URL: https://jobyuk.com/salary-calculator/\n")
		sb.WriteString("- Explore UK Salaries: https://jobyuk.com/salaries/\n")
		sb.WriteString("- Search Jobs: https://jobyuk.com/jobs/\n")
		return sb.String()

	case "company_detail":
		company, _ := dataMap["Company"].(*models.Company)
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# %s\n\n", data.Title))
		if company != nil {
			sb.WriteString(fmt.Sprintf("**Employer:** %s\n", company.Name))
			if company.WebsiteURL != "" {
				sb.WriteString(fmt.Sprintf("**Careers Portal:** %s\n", company.WebsiteURL))
			}
		}
		sb.WriteString(fmt.Sprintf("\n%s\n\n", data.Description))
		if jobs, ok := dataMap["Jobs"].([]models.Job); ok && len(jobs) > 0 {
			sb.WriteString("## Active Openings\n\n")
			for _, j := range jobs {
				sb.WriteString(fmt.Sprintf("- [%s](https://jobyuk.com/jobs/%s/) — %s | %s\n",
					j.Title, j.Slug, j.DisplayLocation(), j.FormattedSalary()))
			}
		}
		return sb.String()

	case "company_index":
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# %s\n\n", data.Title))
		sb.WriteString("Directory of verified UK employers and organizations hiring directly on JobyUK.com.\n\n")
		if companies, ok := dataMap["Companies"].([]models.Company); ok {
			for _, c := range companies {
				sb.WriteString(fmt.Sprintf("- [%s](https://jobyuk.com/companies/%s/) (%s)\n", c.Name, c.Slug, c.FormattedJobCount()))
			}
		}
		return sb.String()

	case "career_advice":
		article, _ := dataMap["Article"].(*models.Article)
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# %s\n\n", data.Title))
		if article != nil {
			sb.WriteString(fmt.Sprintf("**Category:** %s\n", article.Category))
			sb.WriteString(fmt.Sprintf("**URL:** %s\n\n", data.Canonical))
			sb.WriteString(htmlToMarkdown(article.Content))
		} else {
			sb.WriteString(data.Description)
		}
		return sb.String()

	default:
		return genericMarkdown(data)
	}
}

// genericMarkdown provides a structured Markdown fallback for any other page.
func genericMarkdown(data PageData) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", data.Title))
	if data.Description != "" {
		sb.WriteString(fmt.Sprintf("%s\n\n", data.Description))
	}
	if data.Canonical != "" {
		sb.WriteString(fmt.Sprintf("**URL:** %s\n", data.Canonical))
	}
	return sb.String()
}
