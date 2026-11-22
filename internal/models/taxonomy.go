package models

import (
	"fmt"
)

// Location represents a UK geographic location in the hierarchy.
type Location struct {
	ID        int     `json:"id" db:"id"`
	Slug      string  `json:"slug" db:"slug"`
	Name      string  `json:"name" db:"name"`
	ParentID  *int    `json:"parent_id,omitempty" db:"parent_id"`
	Level     string  `json:"level" db:"level"` // country, region, city
	Latitude  float64 `json:"latitude,omitempty" db:"latitude"`
	Longitude float64 `json:"longitude,omitempty" db:"longitude"`
	JobCount  int     `json:"job_count" db:"job_count"`

	// Joined fields
	ParentName string    `json:"parent_name,omitempty" db:"parent_name"`
	Children   []Location `json:"children,omitempty"`
}

// FormattedJobCount returns "X jobs" properly pluralized or "Explore Jobs" if 0.
func (l *Location) FormattedJobCount() string {
	if l.JobCount == 1 {
		return "1 job"
	}
	if l.JobCount > 1 {
		return fmt.Sprintf("%d jobs", l.JobCount)
	}
	return "Explore Jobs"
}

// JobRole represents a job role in the taxonomy.
type JobRole struct {
	ID             int    `json:"id" db:"id"`
	Slug           string `json:"slug" db:"slug"`
	Name           string `json:"name" db:"name"`
	ParentID       *int   `json:"parent_id,omitempty" db:"parent_id"`
	IndustryID     *int   `json:"industry_id,omitempty" db:"industry_id"`
	JobCount       int    `json:"job_count" db:"job_count"`
	SEOTitle       string `json:"seo_title,omitempty" db:"seo_title"`
	SEODescription string `json:"seo_description,omitempty" db:"seo_description"`

	// Joined fields
	IndustryName string    `json:"industry_name,omitempty" db:"industry_name"`
	Children     []JobRole `json:"children,omitempty"`
}

// FormattedJobCount returns "X jobs" properly pluralized or "Explore Jobs" if 0.
func (r *JobRole) FormattedJobCount() string {
	if r.JobCount == 1 {
		return "1 job"
	}
	if r.JobCount > 1 {
		return fmt.Sprintf("%d jobs", r.JobCount)
	}
	return "Explore Jobs"
}

// Industry represents an industry sector.
type Industry struct {
	ID             int    `json:"id" db:"id"`
	Slug           string `json:"slug" db:"slug"`
	Name           string `json:"name" db:"name"`
	JobCount       int    `json:"job_count" db:"job_count"`
	SEOTitle       string `json:"seo_title,omitempty" db:"seo_title"`
	SEODescription string `json:"seo_description,omitempty" db:"seo_description"`
}

// FormattedJobCount returns "X jobs" properly pluralized or "Explore Jobs" if 0.
func (i *Industry) FormattedJobCount() string {
	if i.JobCount == 1 {
		return "1 job"
	}
	if i.JobCount > 1 {
		return fmt.Sprintf("%d jobs", i.JobCount)
	}
	return "Explore Jobs"
}

// JobType represents an employment type (full-time, part-time, contract, etc.)
type JobType struct {
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	JobCount int    `json:"job_count"`
}
