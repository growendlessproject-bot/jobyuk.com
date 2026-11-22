package models

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Company represents an employer/company from the Reed API.
type Company struct {
	ID             int64     `json:"id" db:"id"`
	ReedEmployerID *int64    `json:"reed_employer_id,omitempty" db:"reed_employer_id"`
	Slug           string    `json:"slug" db:"slug"`
	Name           string    `json:"name" db:"name"`
	Description    string    `json:"description,omitempty" db:"description"`
	LogoURL        string    `json:"logo_url,omitempty" db:"logo_url"`
	WebsiteURL     string    `json:"website_url,omitempty" db:"website_url"`
	JobCount       int       `json:"job_count" db:"job_count"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
}

// Initial returns the first letter of the company name for logo fallback.
func (c *Company) Initial() string {
	if c.Name != "" {
		return string(c.Name[0])
	}
	return "?"
}

// FormattedJobCount returns "X jobs" or "X job" properly pluralized, or "Explore Jobs" if 0.
func (c *Company) FormattedJobCount() string {
	if c.JobCount == 1 {
		return "1 job"
	}
	if c.JobCount > 1 {
		return fmt.Sprintf("%d jobs", c.JobCount)
	}
	return "Explore Jobs"
}

// ProxiedLogoURL returns a brand logo image URL for the company proxied through our domain.
func (c *Company) ProxiedLogoURL() string {
	if c.LogoURL != "" {
		if strings.Contains(c.LogoURL, "/profileimages/logos/thumbs/") {
			parts := strings.Split(c.LogoURL, "/profileimages/logos/thumbs/")
			if len(parts) > 1 {
				filenamePart := parts[1]
				if idx := strings.Index(filenamePart, "?"); idx != -1 {
					filenamePart = filenamePart[:idx]
				}
				if filenamePart != "" && filenamePart != "." {
					return "/job/logo/" + filenamePart
				}
			}
		}
	}
	if c.Slug != "" {
		return "/job/logo/fh-" + c.Slug + ".png"
	}
	if c.Name != "" {
		return "/job/logo/fh-" + generateSlug(c.Name) + ".png"
	}
	return fmt.Sprintf("https://ui-avatars.com/api/?name=%s&background=0A66C2&color=fff&size=128&bold=true&font-size=0.4", url.QueryEscape(c.Name))
}
