package repository

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"jobyuk/internal/models"
)

// JobRepository handles job database operations.
type JobRepository struct {
	db *pgxpool.Pool
}

// NewJobRepository creates a new job repository.
func NewJobRepository(db *pgxpool.Pool) *JobRepository {
	return &JobRepository{db: db}
}

// Create inserts a new job.
func (r *JobRepository) Create(ctx context.Context, job *models.Job) error {
	if job.Source == "" {
		job.Source = "reed"
	}
	query := `
		INSERT INTO jobs (
			reed_job_id, source, external_slug, slug, title, description, employer_id, location_id,
			location_name, min_salary, max_salary, yearly_min, yearly_max,
			currency, salary_type, contract_type, job_type, is_graduate,
			is_remote, external_url, reed_url, date_posted, expiration_date,
			status, search_vector
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13,
			$14, $15, $16, $17, $18,
			$19, $20, $21, $22, $23,
			$24, to_tsvector('english', $5 || ' ' || $6)
		) RETURNING id, created_at, updated_at`

	return r.db.QueryRow(ctx, query,
		job.ReedJobID, job.Source, job.ExternalSlug, job.Slug, job.Title, job.Description, job.EmployerID, job.LocationID,
		job.LocationName, job.MinSalary, job.MaxSalary, job.YearlyMin, job.YearlyMax,
		job.Currency, job.SalaryType, job.ContractType, job.JobType, job.IsGraduate,
		job.IsRemote, job.ExternalURL, job.ReedURL, job.DatePosted, job.ExpirationDate,
		job.Status,
	).Scan(&job.ID, &job.CreatedAt, &job.UpdatedAt)
}

// ExistsByReedID checks if a job with the given Reed ID exists.
func (r *JobRepository) ExistsByReedID(ctx context.Context, reedJobID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM jobs WHERE reed_job_id = $1)`,
		reedJobID,
	).Scan(&exists)
	return exists, err
}

// ExistsByExternalSlug checks if a job from a given source with external_slug exists.
func (r *JobRepository) ExistsByExternalSlug(ctx context.Context, source string, externalSlug string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM jobs WHERE source = $1 AND external_slug = $2)`,
		source, externalSlug,
	).Scan(&exists)
	return exists, err
}

// FindExistingMatch searches for an existing job by exact external_url OR (title + employer_id) for zero-duplication.
func (r *JobRepository) FindExistingMatch(ctx context.Context, externalURL string, title string, employerID *int64) (*models.Job, error) {
	cleanTitle := strings.ToLower(strings.TrimSpace(title))

	query := `
		SELECT j.id, j.reed_job_id, j.source, j.external_slug, j.slug, j.title, j.description,
			j.employer_id, j.location_id, j.location_name,
			j.min_salary, j.max_salary, j.yearly_min, j.yearly_max,
			j.currency, j.salary_type, j.contract_type, j.job_type,
			j.is_graduate, j.is_remote, j.external_url, j.reed_url,
			j.date_posted, j.expiration_date, j.status,
			j.created_at, j.updated_at,
			COALESCE(c.name, '') as employer_name,
			COALESCE(c.logo_url, '') as company_logo_url,
			COALESCE(c.slug, '') as company_slug
		FROM jobs j
		LEFT JOIN companies c ON j.employer_id = c.id
		WHERE ($1 != '' AND j.external_url = $1)
		   OR ($2 != '' AND $3 > 0 AND LOWER(TRIM(j.title)) = $2 AND j.employer_id = $3 AND j.status = 'active')
		ORDER BY j.id DESC
		LIMIT 1`

	var empID int64
	if employerID != nil {
		empID = *employerID
	}

	job := &models.Job{}
	err := r.db.QueryRow(ctx, query, externalURL, cleanTitle, empID).Scan(
		&job.ID, &job.ReedJobID, &job.Source, &job.ExternalSlug, &job.Slug, &job.Title, &job.Description,
		&job.EmployerID, &job.LocationID, &job.LocationName,
		&job.MinSalary, &job.MaxSalary, &job.YearlyMin, &job.YearlyMax,
		&job.Currency, &job.SalaryType, &job.ContractType, &job.JobType,
		&job.IsGraduate, &job.IsRemote, &job.ExternalURL, &job.ReedURL,
		&job.DatePosted, &job.ExpirationDate, &job.Status,
		&job.CreatedAt, &job.UpdatedAt, &job.EmployerName, &job.CompanyLogoURLValue, &job.CompanySlug,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return job, nil
}

// UpgradeJobApplyURL updates the external apply URL on an existing job to a direct ATS URL.
func (r *JobRepository) UpgradeJobApplyURL(ctx context.Context, jobID int64, directURL string) error {
	query := `UPDATE jobs SET external_url = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.Exec(ctx, query, directURL, jobID)
	return err
}

// FindBySlug finds a job by its URL slug, clean slug, or Reed Job ID.
func (r *JobRepository) FindBySlug(ctx context.Context, slug string) (*models.Job, error) {
	cleanSlug := strings.Trim(slug, "/")
	var reedJobID int64
	parts := strings.Split(cleanSlug, "-")
	if len(parts) > 0 {
		if id, err := strconv.ParseInt(parts[len(parts)-1], 10, 64); err == nil && id > 0 {
			reedJobID = id
		}
	}

	job := &models.Job{}
	err := r.db.QueryRow(ctx, `
		SELECT j.id, j.reed_job_id, j.slug, j.title, j.description,
			j.employer_id, j.location_id, j.location_name,
			j.min_salary, j.max_salary, j.yearly_min, j.yearly_max,
			j.currency, j.salary_type, j.contract_type, j.job_type,
			j.is_graduate, j.is_remote, j.external_url, j.reed_url,
			j.date_posted, j.expiration_date, j.status,
			j.created_at, j.updated_at,
			COALESCE(c.name, '') as employer_name,
			COALESCE(c.logo_url, '') as company_logo_url,
			COALESCE(c.slug, '') as company_slug
		FROM jobs j
		LEFT JOIN companies c ON j.employer_id = c.id
		WHERE j.slug = $1 OR j.slug = $2 OR ($3 > 0 AND j.reed_job_id = $3)
		LIMIT 1
	`, slug, cleanSlug, reedJobID).Scan(
		&job.ID, &job.ReedJobID, &job.Slug, &job.Title, &job.Description,
		&job.EmployerID, &job.LocationID, &job.LocationName,
		&job.MinSalary, &job.MaxSalary, &job.YearlyMin, &job.YearlyMax,
		&job.Currency, &job.SalaryType, &job.ContractType, &job.JobType,
		&job.IsGraduate, &job.IsRemote, &job.ExternalURL, &job.ReedURL,
		&job.DatePosted, &job.ExpirationDate, &job.Status,
		&job.CreatedAt, &job.UpdatedAt, &job.EmployerName, &job.CompanyLogoURLValue, &job.CompanySlug,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("job not found: %s", slug)
	}
	return job, err
}

// FindByID finds a job by its database ID.
func (r *JobRepository) FindByID(ctx context.Context, id int64) (*models.Job, error) {
	job := &models.Job{}
	err := r.db.QueryRow(ctx, `
		SELECT j.id, j.reed_job_id, j.slug, j.title, j.description,
			j.employer_id, j.location_id, j.location_name,
			j.min_salary, j.max_salary, j.yearly_min, j.yearly_max,
			j.currency, j.salary_type, j.contract_type, j.job_type,
			j.is_graduate, j.is_remote, j.external_url, j.reed_url,
			j.date_posted, j.expiration_date, j.status,
			j.created_at, j.updated_at,
			COALESCE(c.name, '') as employer_name,
			COALESCE(c.logo_url, '') as company_logo_url,
			COALESCE(c.slug, '') as company_slug
		FROM jobs j
		LEFT JOIN companies c ON j.employer_id = c.id
		WHERE j.id = $1
		LIMIT 1
	`, id).Scan(
		&job.ID, &job.ReedJobID, &job.Slug, &job.Title, &job.Description,
		&job.EmployerID, &job.LocationID, &job.LocationName,
		&job.MinSalary, &job.MaxSalary, &job.YearlyMin, &job.YearlyMax,
		&job.Currency, &job.SalaryType, &job.ContractType, &job.JobType,
		&job.IsGraduate, &job.IsRemote, &job.ExternalURL, &job.ReedURL,
		&job.DatePosted, &job.ExpirationDate, &job.Status,
		&job.CreatedAt, &job.UpdatedAt, &job.EmployerName, &job.CompanyLogoURLValue, &job.CompanySlug,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("job not found: %d", id)
	}
	return job, err
}

// Search performs a full-text search with filters.
func (r *JobRepository) Search(ctx context.Context, params models.JobSearchParams) (*models.JobSearchResult, error) {
	if params.PerPage == 0 {
		params.PerPage = 20
	}
	if params.Page == 0 {
		params.Page = 1
	}
	offset := (params.Page - 1) * params.PerPage

	// Build dynamic WHERE clause
	conditions := []string{"j.status = 'active'", "(j.expiration_date IS NULL OR j.expiration_date > NOW())"}
	args := []interface{}{}
	argNum := 1

	if params.Query != "" {
		conditions = append(conditions, fmt.Sprintf(
			"j.search_vector @@ plainto_tsquery('english', $%d)", argNum))
		args = append(args, params.Query)
		argNum++
	}

	if params.Location != "" {
		conditions = append(conditions, fmt.Sprintf(
			"j.location_name ILIKE $%d", argNum))
		args = append(args, "%"+params.Location+"%")
		argNum++
	}

	if params.ContractType != "" {
		conditions = append(conditions, fmt.Sprintf(
			"j.contract_type = $%d", argNum))
		args = append(args, params.ContractType)
		argNum++
	}

	if params.JobType != "" {
		conditions = append(conditions, fmt.Sprintf(
			"j.job_type = $%d", argNum))
		args = append(args, params.JobType)
		argNum++
	}

	if params.MinSalary > 0 {
		conditions = append(conditions, fmt.Sprintf(
			"j.yearly_max >= $%d", argNum))
		args = append(args, params.MinSalary)
		argNum++
	}

	if params.MaxSalary > 0 {
		conditions = append(conditions, fmt.Sprintf(
			"j.yearly_min <= $%d", argNum))
		args = append(args, params.MaxSalary)
		argNum++
	}

	if params.IsGraduate != nil && *params.IsGraduate {
		conditions = append(conditions, "j.is_graduate = true")
	}

	if params.IsRemote != nil && *params.IsRemote {
		conditions = append(conditions, "j.is_remote = true")
	}

	if params.EasyApply {
		conditions = append(conditions, "j.external_url != ''")
	}

	if params.ExperienceLevel != "" {
		switch params.ExperienceLevel {
		case "no_experience":
			conditions = append(conditions, "(j.title ILIKE '%no experience%' OR j.title ILIKE '%trainee%' OR j.title ILIKE '%apprentice%' OR j.is_graduate = true OR j.description ILIKE '%no experience needed%' OR j.description ILIKE '%no experience required%')")
		case "junior":
			conditions = append(conditions, "(j.title ILIKE '%junior%' OR j.title ILIKE '%entry level%' OR j.title ILIKE '%grad%' OR j.title ILIKE '%trainee%')")
		case "mid":
			conditions = append(conditions, "(j.title NOT ILIKE '%junior%' AND j.title NOT ILIKE '%senior%' AND j.title NOT ILIKE '%lead%' AND j.title NOT ILIKE '%dir%' AND j.title NOT ILIKE '%head%' AND j.title NOT ILIKE '%principal%' AND j.title NOT ILIKE '%trainee%' AND j.title NOT ILIKE '%apprentice%')")
		case "senior":
			conditions = append(conditions, "(j.title ILIKE '%senior%' OR j.title ILIKE '%lead%' OR j.title ILIKE '%principal%' OR j.title ILIKE '%director%' OR j.title ILIKE '%head%' OR j.title ILIKE '%mgr%' OR j.title ILIKE '%manager%')")
		}
	}

	if params.DatePostedRange != "" && params.DatePostedRange != "anytime" {
		switch params.DatePostedRange {
		case "30d":
			conditions = append(conditions, "j.date_posted >= NOW() - INTERVAL '30 days'")
		case "10d":
			conditions = append(conditions, "j.date_posted >= NOW() - INTERVAL '10 days'")
		case "5d":
			conditions = append(conditions, "j.date_posted >= NOW() - INTERVAL '5 days'")
		case "1d":
			conditions = append(conditions, "j.date_posted >= NOW() - INTERVAL '1 day'")
		}
	}

	if params.RemoteType != "" && params.RemoteType != "all" {
		switch params.RemoteType {
		case "on_site":
			conditions = append(conditions, "(j.is_remote = false AND j.title NOT ILIKE '%hybrid%' AND j.description NOT ILIKE '%hybrid%')")
		case "remote":
			conditions = append(conditions, "(j.is_remote = true OR j.location_name ILIKE '%remote%' OR j.title ILIKE '%remote%')")
		case "hybrid":
			conditions = append(conditions, "(j.title ILIKE '%hybrid%' OR j.description ILIKE '%hybrid%' OR j.location_name ILIKE '%hybrid%')")
		}
	}

	if params.LocationSlug != "" {
		conditions = append(conditions, fmt.Sprintf(
			"j.location_id IN (SELECT id FROM locations WHERE slug = $%d)", argNum))
		args = append(args, params.LocationSlug)
		argNum++
	}

	if params.RoleSlug != "" {
		roleKeywords := strings.ReplaceAll(params.RoleSlug, "-", " ")
		conditions = append(conditions, fmt.Sprintf(
			"(j.id IN (SELECT job_id FROM job_roles_mapping WHERE role_id IN (SELECT id FROM job_roles WHERE slug = $%d)) OR j.title ILIKE $%d OR j.search_vector @@ plainto_tsquery('english', $%d))",
			argNum, argNum+1, argNum+2))
		args = append(args, params.RoleSlug, "%"+roleKeywords+"%", roleKeywords)
		argNum += 3
	}

	if params.IndustrySlug != "" {
		indKeywords := strings.ReplaceAll(params.IndustrySlug, "-", " ")
		conditions = append(conditions, fmt.Sprintf(
			"(j.id IN (SELECT job_id FROM job_industries_mapping WHERE industry_id IN (SELECT id FROM industries WHERE slug = $%d)) OR j.title ILIKE $%d OR j.search_vector @@ plainto_tsquery('english', $%d))",
			argNum, argNum+1, argNum+2))
		args = append(args, params.IndustrySlug, "%"+indKeywords+"%", indKeywords)
		argNum += 3
	}

	whereClause := strings.Join(conditions, " AND ")

	// Order by
	orderBy := "j.date_posted DESC"
	switch params.SortBy {
	case "relevance":
		if params.Query != "" {
			orderBy = fmt.Sprintf(
				"ts_rank_cd(j.search_vector, plainto_tsquery('english', $1)) DESC, j.date_posted DESC")
		}
	case "salary_desc":
		orderBy = "j.yearly_max DESC NULLS LAST"
	case "salary_asc":
		orderBy = "j.yearly_min ASC NULLS LAST"
	}

	// Count query
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM jobs j WHERE %s", whereClause)
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("count query failed: %w", err)
	}

	// Main query
	query := fmt.Sprintf(`
		SELECT j.id, j.reed_job_id, j.slug, j.title, j.description,
			j.employer_id, j.location_id, j.location_name,
			j.min_salary, j.max_salary, j.yearly_min, j.yearly_max,
			j.currency, j.salary_type, j.contract_type, j.job_type,
			j.is_graduate, j.is_remote, j.external_url, j.reed_url,
			j.date_posted, j.expiration_date, j.status,
			j.created_at, j.updated_at,
			COALESCE(c.name, '') as employer_name,
			COALESCE(c.logo_url, '') as company_logo_url,
			COALESCE(c.slug, '') as company_slug
		FROM jobs j
		LEFT JOIN companies c ON j.employer_id = c.id
		WHERE %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, whereClause, orderBy, argNum, argNum+1)

	args = append(args, params.PerPage, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search query failed: %w", err)
	}
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		var job models.Job
		if err := rows.Scan(
			&job.ID, &job.ReedJobID, &job.Slug, &job.Title, &job.Description,
			&job.EmployerID, &job.LocationID, &job.LocationName,
			&job.MinSalary, &job.MaxSalary, &job.YearlyMin, &job.YearlyMax,
			&job.Currency, &job.SalaryType, &job.ContractType, &job.JobType,
			&job.IsGraduate, &job.IsRemote, &job.ExternalURL, &job.ReedURL,
			&job.DatePosted, &job.ExpirationDate, &job.Status,
			&job.CreatedAt, &job.UpdatedAt, &job.EmployerName, &job.CompanyLogoURLValue, &job.CompanySlug,
		); err != nil {
			return nil, fmt.Errorf("scan job failed: %w", err)
		}
		jobs = append(jobs, job)
	}

	totalPages := (total + params.PerPage - 1) / params.PerPage

	return &models.JobSearchResult{
		Jobs:       jobs,
		Total:      total,
		Page:       params.Page,
		PerPage:    params.PerPage,
		TotalPages: totalPages,
	}, nil
}

// FindLatest returns the most recently posted active jobs.
func (r *JobRepository) FindLatest(ctx context.Context, limit int) ([]models.Job, error) {
	rows, err := r.db.Query(ctx, `
		SELECT j.id, j.slug, j.title, j.location_name,
			j.yearly_min, j.yearly_max, j.contract_type, j.job_type,
			j.is_graduate, j.is_remote, j.date_posted,
			COALESCE(c.name, '') as employer_name,
			COALESCE(c.logo_url, '') as company_logo_url,
			COALESCE(c.slug, '') as company_slug
		FROM jobs j
		LEFT JOIN companies c ON j.employer_id = c.id
		WHERE j.status = 'active' AND (j.expiration_date IS NULL OR j.expiration_date > NOW())
		ORDER BY j.date_posted DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		var job models.Job
		if err := rows.Scan(
			&job.ID, &job.Slug, &job.Title, &job.LocationName,
			&job.YearlyMin, &job.YearlyMax, &job.ContractType, &job.JobType,
			&job.IsGraduate, &job.IsRemote, &job.DatePosted,
			&job.EmployerName, &job.CompanyLogoURLValue, &job.CompanySlug,
		); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

// ExpireOldJobs marks jobs as expired if past expiration date and returns their slugs.
func (r *JobRepository) ExpireOldJobs(ctx context.Context) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		UPDATE jobs SET status = 'expired', updated_at = NOW()
		WHERE status = 'active'
		  AND (
			  link_status = 'dead'
			  OR (
				  expiration_date < NOW()
				  AND (link_status != 'active' OR last_link_checked_at < NOW() - INTERVAL '7 days')
			  )
			  OR (
				  expiration_date IS NULL
				  AND date_posted < NOW() - INTERVAL '45 days'
				  AND (link_status != 'active' OR last_link_checked_at < NOW() - INTERVAL '7 days')
			  )
		  )
		RETURNING slug
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var slugs []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err == nil {
			slugs = append(slugs, slug)
		}
	}
	return slugs, nil
}

// CountByStatus returns job counts grouped by status.
func (r *JobRepository) CountByStatus(ctx context.Context) (map[string]int, error) {
	rows, err := r.db.Query(ctx, `
		SELECT status, COUNT(*) FROM jobs GROUP BY status
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts[status] = count
	}
	return counts, nil
}

// FindActiveByRole returns active jobs for a specific role matching mappings, title keywords, and search vector.
func (r *JobRepository) FindActiveByRole(ctx context.Context, roleID int, roleName string, limit, offset int) ([]models.Job, int, error) {
	keywords := strings.TrimSpace(roleName)
	cleanKeywords := strings.ReplaceAll(keywords, "-", " ")
	words := strings.Fields(strings.ToLower(cleanKeywords))

	var wordConditions []string
	var args []interface{}
	args = append(args, roleID) // $1
	argNum := 2

	// Exact phrase pattern
	exactPattern := "%" + cleanKeywords + "%"
	args = append(args, exactPattern) // $2
	exactArg := argNum
	argNum++

	// Build individual word ILIKE conditions if role has words
	for _, w := range words {
		if len(w) > 2 && w != "and" && w != "the" && w != "for" {
			wordConditions = append(wordConditions, fmt.Sprintf("j.title ILIKE $%d", argNum))
			args = append(args, "%"+w+"%")
			argNum++
		}
	}

	whereClause := fmt.Sprintf("(jrm.role_id = $1 OR j.title ILIKE $%d", exactArg)
	if len(wordConditions) > 1 {
		whereClause += " OR (" + strings.Join(wordConditions, " AND ") + ")"
		whereClause += " OR (" + strings.Join(wordConditions, " OR ") + ")"
	} else if len(wordConditions) == 1 {
		whereClause += " OR " + wordConditions[0]
	}
	whereClause += ") AND j.status = 'active' AND (j.expiration_date IS NULL OR j.expiration_date > NOW())"

	countQuery := fmt.Sprintf(`
		SELECT COUNT(DISTINCT j.id) 
		FROM jobs j
		LEFT JOIN job_roles_mapping jrm ON j.id = jrm.job_id AND jrm.role_id = $1
		WHERE %s
	`, whereClause)

	var total int
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	selectArgs := append(args, limit, offset)
	limitArg := argNum
	offsetArg := argNum + 1

	selectQuery := fmt.Sprintf(`
		SELECT j.id, j.slug, j.title, j.location_name,
			j.yearly_min, j.yearly_max, j.contract_type, j.job_type,
			j.is_graduate, j.is_remote, j.date_posted,
			COALESCE(c.name, '') as employer_name,
			COALESCE(c.logo_url, '') as company_logo_url,
			COALESCE(c.slug, '') as company_slug
		FROM jobs j
		LEFT JOIN job_roles_mapping jrm ON j.id = jrm.job_id AND jrm.role_id = $1
		LEFT JOIN companies c ON j.employer_id = c.id
		WHERE %s
		GROUP BY j.id, c.name, c.logo_url, c.slug
		ORDER BY 
			CASE 
				WHEN j.title ILIKE $%d THEN 1 
				WHEN MAX(CASE WHEN jrm.role_id = $1 THEN 1 ELSE 0 END) = 1 THEN 2
				ELSE 3 
			END ASC,
			j.date_posted DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, exactArg, limitArg, offsetArg)

	rows, err := r.db.Query(ctx, selectQuery, selectArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		var job models.Job
		if err := rows.Scan(
			&job.ID, &job.Slug, &job.Title, &job.LocationName,
			&job.YearlyMin, &job.YearlyMax, &job.ContractType, &job.JobType,
			&job.IsGraduate, &job.IsRemote, &job.DatePosted,
			&job.EmployerName, &job.CompanyLogoURLValue, &job.CompanySlug,
		); err != nil {
			return nil, 0, err
		}
		jobs = append(jobs, job)
	}
	return jobs, total, nil
}

// FindSimilar finds similar jobs to a given job.
func (r *JobRepository) FindSimilar(ctx context.Context, jobID int64, limit int) ([]models.Job, error) {
	rows, err := r.db.Query(ctx, `
		SELECT j.id, j.slug, j.title, j.location_name,
			j.yearly_min, j.yearly_max, j.date_posted,
			COALESCE(c.name, '') as employer_name,
			COALESCE(c.logo_url, '') as company_logo_url,
			COALESCE(c.slug, '') as company_slug
		FROM jobs j
		LEFT JOIN companies c ON j.employer_id = c.id
		WHERE j.status = 'active'
			AND (j.expiration_date IS NULL OR j.expiration_date > NOW())
			AND j.id != $1
			AND j.search_vector @@ plainto_tsquery('english', (SELECT title FROM jobs WHERE id = $1))
		ORDER BY j.date_posted DESC
		LIMIT $2
	`, jobID, limit)
	if err != nil {
		// Fallback: just return latest jobs
		return r.FindLatest(ctx, limit)
	}
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		var job models.Job
		if err := rows.Scan(
			&job.ID, &job.Slug, &job.Title, &job.LocationName,
			&job.YearlyMin, &job.YearlyMax, &job.DatePosted,
			&job.EmployerName, &job.CompanyLogoURLValue, &job.CompanySlug,
		); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}

	if len(jobs) < limit {
		// Not enough similar jobs, fill with latest
		latest, _ := r.FindLatest(ctx, limit-len(jobs))
		jobs = append(jobs, latest...)
	}

	return jobs, nil
}

// FindActiveByCompany returns active jobs for a specific company.
func (r *JobRepository) FindActiveByCompany(ctx context.Context, companyID int64, limit, offset int) ([]models.Job, int, error) {
	var total int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM jobs WHERE employer_id = $1 AND status = 'active' AND (expiration_date IS NULL OR expiration_date > NOW())
	`, companyID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT j.id, j.slug, j.title, j.location_name,
			j.yearly_min, j.yearly_max, j.contract_type, j.job_type,
			j.is_graduate, j.is_remote, j.date_posted,
			COALESCE(c.name, '') as employer_name,
			COALESCE(c.logo_url, '') as company_logo_url,
			COALESCE(c.slug, '') as company_slug
		FROM jobs j
		LEFT JOIN companies c ON j.employer_id = c.id
		WHERE j.employer_id = $1 AND j.status = 'active' AND (j.expiration_date IS NULL OR j.expiration_date > NOW())
		ORDER BY j.date_posted DESC
		LIMIT $2 OFFSET $3
	`, companyID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		var job models.Job
		if err := rows.Scan(
			&job.ID, &job.Slug, &job.Title, &job.LocationName,
			&job.YearlyMin, &job.YearlyMax, &job.ContractType, &job.JobType,
			&job.IsGraduate, &job.IsRemote, &job.DatePosted,
			&job.EmployerName, &job.CompanyLogoURLValue, &job.CompanySlug,
		); err != nil {
			return nil, 0, err
		}
		jobs = append(jobs, job)
	}
	return jobs, total, nil
}

// Expire manually sets the job status to 'expired' and sets the expiration_date to now.
func (r *JobRepository) Expire(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `
		UPDATE jobs
		SET status = 'expired', expiration_date = $2
		WHERE id = $1
	`, id, time.Now())
	return err
}

// Reactivate sets the job status back to 'active' and extends the expiration_date to 30 days from now.
func (r *JobRepository) Reactivate(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `
		UPDATE jobs
		SET status = 'active', expiration_date = $2, date_posted = $3
		WHERE id = $1
	`, id, time.Now().Add(30*24*time.Hour), time.Now())
	return err
}

// AdminSearch performs a search across all jobs (both active and expired) for the administration portal.
func (r *JobRepository) AdminSearch(ctx context.Context, query string, limit, offset int) ([]models.Job, int, error) {
	var total int
	var err error
	if query != "" {
		err = r.db.QueryRow(ctx, `
			SELECT COUNT(*) FROM jobs WHERE title ILIKE $1 OR location_name ILIKE $1
		`, "%"+query+"%").Scan(&total)
		if err != nil {
			return nil, 0, err
		}
		rows, err := r.db.Query(ctx, `
			SELECT j.id, j.slug, j.title, j.location_name, j.status, COALESCE(c.name, ''), j.date_posted, j.expiration_date, j.created_at
			FROM jobs j
			LEFT JOIN companies c ON j.employer_id = c.id
			WHERE j.title ILIKE $1 OR j.location_name ILIKE $1
			ORDER BY j.date_posted DESC
			LIMIT $2 OFFSET $3
		`, "%"+query+"%", limit, offset)
		if err != nil {
			return nil, 0, err
		}
		defer rows.Close()
		var jobs []models.Job
		for rows.Next() {
			var j models.Job
			if err := rows.Scan(&j.ID, &j.Slug, &j.Title, &j.LocationName, &j.Status, &j.EmployerName, &j.DatePosted, &j.ExpirationDate, &j.CreatedAt); err != nil {
				return nil, 0, err
			}
			jobs = append(jobs, j)
		}
		return jobs, total, nil
	} else {
		err = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM jobs`).Scan(&total)
		if err != nil {
			return nil, 0, err
		}
		rows, err := r.db.Query(ctx, `
			SELECT j.id, j.slug, j.title, j.location_name, j.status, COALESCE(c.name, ''), j.date_posted, j.expiration_date, j.created_at
			FROM jobs j
			LEFT JOIN companies c ON j.employer_id = c.id
			ORDER BY j.date_posted DESC
			LIMIT $1 OFFSET $2
		`, limit, offset)
		if err != nil {
			return nil, 0, err
		}
		defer rows.Close()
		var jobs []models.Job
		for rows.Next() {
			var j models.Job
			if err := rows.Scan(&j.ID, &j.Slug, &j.Title, &j.LocationName, &j.Status, &j.EmployerName, &j.DatePosted, &j.ExpirationDate, &j.CreatedAt); err != nil {
				return nil, 0, err
			}
			jobs = append(jobs, j)
		}
		return jobs, total, nil
	}
}

// Report inserts a user report for a job.
func (r *JobRepository) Report(ctx context.Context, report *models.JobReport) error {
	query := `
		INSERT INTO job_reports (job_id, reason, details, ip_address, created_at)
		VALUES ($1, $2, $3, $4, NOW())
		RETURNING id, created_at
	`
	err := r.db.QueryRow(ctx, query, report.JobID, report.Reason, report.Details, report.IPAddress).
		Scan(&report.ID, &report.CreatedAt)
	if err != nil {
		_, execErr := r.db.Exec(ctx, "INSERT INTO job_reports (job_id, reason, details, ip_address) VALUES ($1, $2, $3, $4)", report.JobID, report.Reason, report.Details, report.IPAddress)
		if execErr == nil {
			return nil
		}
		return fmt.Errorf("failed to insert report: %w", err)
	}
	return nil
}

// CountReportsByIP returns the number of reports submitted by a specific IP for a job.
func (r *JobRepository) CountReportsByIP(ctx context.Context, jobID int64, ip string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM job_reports WHERE job_id = $1 AND ip_address = $2
	`, jobID, ip).Scan(&count)
	return count, err
}

// FindAllReports returns all submitted job reports.
func (r *JobRepository) FindAllReports(ctx context.Context) ([]models.JobReport, error) {
	rows, err := r.db.Query(ctx, `
		SELECT r.id, r.job_id, r.reason, r.details, COALESCE(r.ip_address, '') as ip_address, r.created_at,
			j.title as job_title, j.slug as job_slug, j.status as job_status
		FROM job_reports r
		INNER JOIN jobs j ON r.job_id = j.id
		ORDER BY r.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reports []models.JobReport
	for rows.Next() {
		var rep models.JobReport
		if err := rows.Scan(
			&rep.ID, &rep.JobID, &rep.Reason, &rep.Details, &rep.IPAddress, &rep.CreatedAt,
			&rep.JobTitle, &rep.JobSlug, &rep.JobStatus,
		); err != nil {
			return nil, err
		}
		reports = append(reports, rep)
	}
	return reports, nil
}

// DeleteReport deletes a report by its ID.
func (r *JobRepository) DeleteReport(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, "DELETE FROM job_reports WHERE id = $1", id)
	return err
}

// Delete removes a job by its ID.
func (r *JobRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, "DELETE FROM jobs WHERE id = $1", id)
	return err
}

// SoftDelete sets the job status to 'removed' and updates the expiration_date to now.
func (r *JobRepository) SoftDelete(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `
		UPDATE jobs
		SET status = 'removed', expiration_date = $2
		WHERE id = $1
	`, id, time.Now())
	return err
}

// BlacklistIP blocks a specific IP address with a reason.
func (r *JobRepository) BlacklistIP(ctx context.Context, ip string, reason string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO blacklisted_ips (ip_address, reason)
		VALUES ($1, $2)
		ON CONFLICT (ip_address) DO UPDATE SET reason = EXCLUDED.reason
	`, ip, reason)
	return err
}

// UnblacklistIP unblocks a specific IP address.
func (r *JobRepository) UnblacklistIP(ctx context.Context, ip string) error {
	_, err := r.db.Exec(ctx, "DELETE FROM blacklisted_ips WHERE ip_address = $1", ip)
	return err
}

// IsIPBlacklisted returns true if the IP address is blocked.
func (r *JobRepository) IsIPBlacklisted(ctx context.Context, ip string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM blacklisted_ips WHERE ip_address = $1)", ip).Scan(&exists)
	return exists, err
}

// GetBlacklistedIPs returns the list of all currently blocked IP addresses.
func (r *JobRepository) GetBlacklistedIPs(ctx context.Context) ([]models.BlacklistedIP, error) {
	rows, err := r.db.Query(ctx, "SELECT ip_address, COALESCE(reason, '') as reason, created_at FROM blacklisted_ips ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.BlacklistedIP
	for rows.Next() {
		var item models.BlacklistedIP
		if err := rows.Scan(&item.IPAddress, &item.Reason, &item.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, nil
}

// CountTotalReportsByIP returns the total number of reports submitted across all jobs by an IP.
func (r *JobRepository) CountTotalReportsByIP(ctx context.Context, ip string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM job_reports WHERE ip_address = $1", ip).Scan(&count)
	return count, err
}

// BulkDeleteReports dismisses/deletes multiple reports by their IDs.
func (r *JobRepository) BulkDeleteReports(ctx context.Context, ids []int64) error {
	_, err := r.db.Exec(ctx, "DELETE FROM job_reports WHERE id = ANY($1)", ids)
	return err
}

// UpdatePinterestPin updates the Pinterest Pin URL for a specific job.
func (r *JobRepository) UpdatePinterestPin(ctx context.Context, jobID int64, pinURL string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE jobs
		SET pinterest_pin_url = $2, updated_at = NOW()
		WHERE id = $1
	`, jobID, strings.TrimSpace(pinURL))
	return err
}

// GetPinterestStats returns total active jobs and total active jobs with pinterest pin URLs.
func (r *JobRepository) GetPinterestStats(ctx context.Context) (totalActive int, totalPinned int, err error) {
	err = r.db.QueryRow(ctx, `
		SELECT 
			COUNT(*),
			COUNT(CASE WHEN pinterest_pin_url IS NOT NULL AND TRIM(pinterest_pin_url) != '' THEN 1 END)
		FROM jobs
		WHERE status = 'active'
	`).Scan(&totalActive, &totalPinned)
	return totalActive, totalPinned, err
}

// FindPinterestJobs retrieves active jobs for the Pinterest Admin dashboard.
func (r *JobRepository) FindPinterestJobs(ctx context.Context, pinnedOnly bool, query string, limit, offset int) ([]models.Job, int, error) {
	query = strings.TrimSpace(query)

	var whereClauses []string
	var args []interface{}
	argIdx := 1

	whereClauses = append(whereClauses, "j.status = 'active'")

	if pinnedOnly {
		whereClauses = append(whereClauses, "j.pinterest_pin_url IS NOT NULL AND TRIM(j.pinterest_pin_url) != ''")
	}

	if query != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("(j.title ILIKE $%d OR j.location_name ILIKE $%d OR c.name ILIKE $%d)", argIdx, argIdx, argIdx))
		args = append(args, "%"+query+"%")
		argIdx++
	}

	whereSQL := strings.Join(whereClauses, " AND ")

	// Count query
	countSQL := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM jobs j
		LEFT JOIN companies c ON j.employer_id = c.id
		WHERE %s
	`, whereSQL)

	var total int
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Data query
	dataSQL := fmt.Sprintf(`
		SELECT 
			j.id, j.slug, j.title, j.location_name, j.is_remote,
			COALESCE(j.contract_type, ''), COALESCE(j.job_type, ''),
			j.min_salary, j.max_salary, j.yearly_min, j.yearly_max,
			j.currency, j.salary_type,
			COALESCE(c.name, ''),
			COALESCE(j.pinterest_pin_url, ''),
			j.date_posted, j.created_at
		FROM jobs j
		LEFT JOIN companies c ON j.employer_id = c.id
		WHERE %s
		ORDER BY j.date_posted DESC
		LIMIT $%d OFFSET $%d
	`, whereSQL, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		var j models.Job
		if err := rows.Scan(
			&j.ID, &j.Slug, &j.Title, &j.LocationName, &j.IsRemote,
			&j.ContractType, &j.JobType,
			&j.MinSalary, &j.MaxSalary, &j.YearlyMin, &j.YearlyMax,
			&j.Currency, &j.SalaryType,
			&j.EmployerName,
			&j.PinterestPinURL,
			&j.DatePosted, &j.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		jobs = append(jobs, j)
	}

	return jobs, total, nil
}






// EnsureGoogleIndexedColumn ensures the google_indexed_at column exists.
func (r *JobRepository) EnsureGoogleIndexedColumn(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `
		ALTER TABLE jobs ADD COLUMN IF NOT EXISTS google_indexed_at TIMESTAMPTZ;
		CREATE INDEX IF NOT EXISTS idx_jobs_google_indexed_at ON jobs (google_indexed_at) WHERE status = 'active';
	`)
	return err
}

// FindJobsForIndexing returns up to limit active jobs that have not been indexed by Google recently.
func (r *JobRepository) FindJobsForIndexing(ctx context.Context, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 150
	}
	rows, err := r.db.Query(ctx, `
		SELECT slug FROM jobs 
		WHERE status = 'active' 
		  AND (google_indexed_at IS NULL OR google_indexed_at < NOW() - INTERVAL '14 days')
		  AND slug IS NOT NULL AND slug != ''
		ORDER BY date_posted DESC 
		LIMIT $1
	`, limit)
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
	return slugs, rows.Err()
}

// MarkJobAsIndexed updates the google_indexed_at timestamp for a job.
func (r *JobRepository) MarkJobAsIndexed(ctx context.Context, slug string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE jobs SET google_indexed_at = NOW() WHERE slug = $1
	`, slug)
	return err
}
