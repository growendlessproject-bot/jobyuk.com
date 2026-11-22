package models

import (
	"time"
)

// Article represents a career advice or content page.
type Article struct {
	ID             int        `json:"id" db:"id"`
	Slug           string     `json:"slug" db:"slug"`
	Title          string     `json:"title" db:"title"`
	Content        string     `json:"content" db:"content"`
	Category       string     `json:"category" db:"category"`
	SEOTitle       string     `json:"seo_title,omitempty" db:"seo_title"`
	SEODescription string     `json:"seo_description,omitempty" db:"seo_description"`
	Published      bool       `json:"published" db:"published"`
	PublishedAt    *time.Time `json:"published_at,omitempty" db:"published_at"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
}

// SalaryGuide holds salary information for a role/location combination.
type SalaryGuide struct {
	ID             int       `json:"id" db:"id"`
	RoleSlug       string    `json:"role_slug" db:"role_slug"`
	RoleName       string    `json:"role_name" db:"role_name"`
	LocationSlug   string    `json:"location_slug,omitempty" db:"location_slug"`
	LocationName   string    `json:"location_name,omitempty" db:"location_name"`
	MinSalary      float64   `json:"min_salary" db:"min_salary"`
	MaxSalary      float64   `json:"max_salary" db:"max_salary"`
	AvgSalary      float64   `json:"avg_salary" db:"avg_salary"`
	SampleSize     int       `json:"sample_size" db:"sample_size"`
	DataSource     string    `json:"data_source" db:"data_source"`
	SEOTitle       string    `json:"seo_title,omitempty" db:"seo_title"`
	SEODescription string    `json:"seo_description,omitempty" db:"seo_description"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// Redirect represents an SEO redirect rule.
type Redirect struct {
	ID         int       `json:"id" db:"id"`
	FromPath   string    `json:"from_path" db:"from_path"`
	ToPath     string    `json:"to_path" db:"to_path"`
	StatusCode int       `json:"status_code" db:"status_code"` // 301 or 410
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// PageView represents an analytics event.
type PageView struct {
	ID         int64     `json:"id" db:"id"`
	Path       string    `json:"path" db:"path"`
	Referrer   string    `json:"referrer,omitempty" db:"referrer"`
	UserAgent  string    `json:"user_agent,omitempty" db:"user_agent"`
	IPHash     string    `json:"ip_hash" db:"ip_hash"`
	Country    string    `json:"country,omitempty" db:"country"`
	DeviceType string    `json:"device_type" db:"device_type"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// AdminUser represents an admin user.
type AdminUser struct {
	ID           int        `json:"id" db:"id"`
	Username     string     `json:"username" db:"username"`
	PasswordHash string     `json:"-" db:"password_hash"`
	Role         string     `json:"role" db:"role"`
	LastLogin    *time.Time `json:"last_login,omitempty" db:"last_login"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
}
