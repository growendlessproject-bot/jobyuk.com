package repository

import (
	"context"
	"time"
)

// SitemapJob holds sitemap data for a job.
type SitemapJob struct {
	Slug      string
	UpdatedAt time.Time
}

// SitemapArticle holds sitemap data for an article.
type SitemapArticle struct {
	Slug      string
	Category  string
	UpdatedAt time.Time
}

// FindSitemapJobs returns all active jobs for sitemap generation.
func (r *JobRepository) FindSitemapJobs(ctx context.Context) ([]SitemapJob, error) {
	rows, err := r.db.Query(ctx, `
		SELECT slug, COALESCE(updated_at, date_posted, NOW()) AS updated_at 
		FROM jobs 
		WHERE status = 'active' AND slug IS NOT NULL AND slug != ''
		ORDER BY date_posted DESC
		LIMIT 1500
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []SitemapJob
	for rows.Next() {
		var item SitemapJob
		if err := rows.Scan(&item.Slug, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// GetLatestJobUpdatedAt returns the most recent updated_at timestamp of active jobs.
func (r *JobRepository) GetLatestJobUpdatedAt(ctx context.Context) (time.Time, error) {
	var t time.Time
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(MAX(updated_at), NOW()) FROM jobs WHERE status = 'active'
	`).Scan(&t)
	return t, err
}

// FindSitemapCompanies returns all companies with active jobs.
func (r *CompanyRepository) FindSitemapCompanies(ctx context.Context) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT slug FROM companies 
		WHERE job_count > 0 AND slug IS NOT NULL AND slug != ''
		ORDER BY job_count DESC, name ASC
		LIMIT 500
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var slugs []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, err
		}
		slugs = append(slugs, slug)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return slugs, nil
}

// FindSitemapRoles returns all roles with active jobs.
func (r *RoleRepository) FindSitemapRoles(ctx context.Context) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT slug FROM job_roles 
		WHERE job_count > 0 AND slug IS NOT NULL AND slug != ''
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var slugs []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, err
		}
		slugs = append(slugs, slug)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return slugs, nil
}

// FindSitemapLocations returns all locations with active jobs.
func (r *LocationRepository) FindSitemapLocations(ctx context.Context) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT slug FROM locations 
		WHERE job_count > 0 AND slug IS NOT NULL AND slug != ''
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var slugs []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, err
		}
		slugs = append(slugs, slug)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return slugs, nil
}

// FindSitemapIndustries returns all industries with active jobs.
func (r *IndustryRepository) FindSitemapIndustries(ctx context.Context) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT slug FROM industries 
		WHERE job_count > 0 AND slug IS NOT NULL AND slug != ''
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var slugs []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, err
		}
		slugs = append(slugs, slug)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return slugs, nil
}

// FindSitemapArticles returns all published career advice articles.
func (r *ArticleRepository) FindSitemapArticles(ctx context.Context) ([]SitemapArticle, error) {
	rows, err := r.db.Query(ctx, `
		SELECT slug, category, COALESCE(updated_at, NOW()) AS updated_at FROM articles 
		WHERE published = true AND (published_at IS NULL OR published_at <= NOW()) AND slug IS NOT NULL AND slug != ''
		ORDER BY published_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []SitemapArticle
	for rows.Next() {
		var item SitemapArticle
		if err := rows.Scan(&item.Slug, &item.Category, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
