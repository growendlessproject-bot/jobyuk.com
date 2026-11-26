package repository

import (
	"context"
	"time"
)

// FeedJob represents an active job formatted for XML syndication feeds.
type FeedJob struct {
	ID             int64
	Slug           string
	Title          string
	Description    string
	LocationName   string
	EmployerName   string
	MinSalary      *float64
	MaxSalary      *float64
	Currency       string
	SalaryType     string
	ContractType   string
	JobType        string
	IsRemote       bool
	DatePosted     time.Time
	ExpirationDate *time.Time
	Category       string
}

// FindFeedJobs retrieves the most recent active jobs for aggregator XML feed distribution.
func (r *JobRepository) FindFeedJobs(ctx context.Context, limit int) ([]FeedJob, error) {
	if limit <= 0 || limit > 3000 {
		limit = 1500
	}

	query := `
		SELECT j.id, j.slug, j.title, j.description, COALESCE(j.location_name, 'United Kingdom'),
		       COALESCE(c.name, 'Verified UK Employer'), j.min_salary, j.max_salary,
		       COALESCE(j.currency, 'GBP'), COALESCE(j.salary_type, 'per_annum'),
		       COALESCE(j.contract_type, 'permanent'), COALESCE(j.job_type, 'full_time'),
		       j.is_remote, j.date_posted, j.expiration_date,
		       COALESCE((SELECT ind.name FROM job_industries_mapping jim JOIN industries ind ON ind.id = jim.industry_id WHERE jim.job_id = j.id LIMIT 1), 'General') AS category
		FROM jobs j
		LEFT JOIN companies c ON j.employer_id = c.id
		WHERE j.status = 'active' AND (j.expiration_date IS NULL OR j.expiration_date > NOW()) AND j.slug IS NOT NULL AND j.slug != ''
		ORDER BY j.date_posted DESC
		LIMIT $1
	`

	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []FeedJob
	for rows.Next() {
		var j FeedJob
		err := rows.Scan(
			&j.ID, &j.Slug, &j.Title, &j.Description, &j.LocationName,
			&j.EmployerName, &j.MinSalary, &j.MaxSalary,
			&j.Currency, &j.SalaryType, &j.ContractType, &j.JobType,
			&j.IsRemote, &j.DatePosted, &j.ExpirationDate, &j.Category,
		)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}

	return jobs, rows.Err()
}
