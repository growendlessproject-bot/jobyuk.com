package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"jobyuk/internal/models"
)

// CompanyRepository handles company database operations.
type CompanyRepository struct {
	db *pgxpool.Pool
}

func NewCompanyRepository(db *pgxpool.Pool) *CompanyRepository {
	return &CompanyRepository{db: db}
}

func (r *CompanyRepository) Create(ctx context.Context, company *models.Company) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO companies (reed_employer_id, slug, name, description, logo_url, website_url)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
		RETURNING id, created_at
	`, company.ReedEmployerID, company.Slug, company.Name, company.Description,
		company.LogoURL, company.WebsiteURL,
	).Scan(&company.ID, &company.CreatedAt)
}

func (r *CompanyRepository) FindByReedID(ctx context.Context, reedID int64) (*models.Company, error) {
	company := &models.Company{}
	err := r.db.QueryRow(ctx, `
		SELECT id, reed_employer_id, slug, name, description, logo_url, website_url, job_count, created_at
		FROM companies WHERE reed_employer_id = $1
	`, reedID).Scan(
		&company.ID, &company.ReedEmployerID, &company.Slug, &company.Name,
		&company.Description, &company.LogoURL, &company.WebsiteURL,
		&company.JobCount, &company.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("company not found")
	}
	return company, err
}

func (r *CompanyRepository) FindBySlug(ctx context.Context, slug string) (*models.Company, error) {
	company := &models.Company{}
	err := r.db.QueryRow(ctx, `
		SELECT id, reed_employer_id, slug, name, description, logo_url, website_url, job_count, created_at
		FROM companies WHERE slug = $1
	`, slug).Scan(
		&company.ID, &company.ReedEmployerID, &company.Slug, &company.Name,
		&company.Description, &company.LogoURL, &company.WebsiteURL,
		&company.JobCount, &company.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("company not found: %s", slug)
	}
	return company, err
}

func (r *CompanyRepository) FindByName(ctx context.Context, name string) (*models.Company, error) {
	cleanName := strings.ToLower(strings.TrimSpace(name))
	company := &models.Company{}
	err := r.db.QueryRow(ctx, `
		SELECT id, reed_employer_id, slug, name, description, logo_url, website_url, job_count, created_at
		FROM companies WHERE LOWER(TRIM(name)) = $1
		LIMIT 1
	`, cleanName).Scan(
		&company.ID, &company.ReedEmployerID, &company.Slug, &company.Name,
		&company.Description, &company.LogoURL, &company.WebsiteURL,
		&company.JobCount, &company.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("company not found: %s", name)
	}
	return company, err
}

func (r *CompanyRepository) FindTop(ctx context.Context, limit int) ([]models.Company, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, slug, name, logo_url, job_count
		FROM companies
		WHERE job_count > 0
		ORDER BY job_count DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var companies []models.Company
	for rows.Next() {
		var c models.Company
		if err := rows.Scan(&c.ID, &c.Slug, &c.Name, &c.LogoURL, &c.JobCount); err != nil {
			return nil, err
		}
		companies = append(companies, c)
	}
	return companies, nil
}

func (r *CompanyRepository) UpdateJobCounts(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `
		UPDATE companies c
		SET job_count = COALESCE(sub.cnt, 0)
		FROM (
			SELECT employer_id, COUNT(*) as cnt
			FROM jobs
			WHERE status = 'active' AND employer_id IS NOT NULL
			GROUP BY employer_id
		) sub
		WHERE c.id = sub.employer_id;

		UPDATE companies
		SET job_count = 0
		WHERE id NOT IN (
			SELECT DISTINCT employer_id FROM jobs WHERE status = 'active' AND employer_id IS NOT NULL
		) AND job_count != 0;
	`)
	return err
}

// UpdateLogoURL updates the logo_url for a company.
func (r *CompanyRepository) UpdateLogoURL(ctx context.Context, companyID int64, logoURL string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE companies SET logo_url = $2 WHERE id = $1 AND (logo_url IS NULL OR logo_url = '')
	`, companyID, logoURL)
	return err
}

// FindAllActive returns top active companies with at least one active job.
func (r *CompanyRepository) FindAllActive(ctx context.Context) ([]models.Company, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, slug, name, logo_url, job_count
		FROM companies
		WHERE job_count > 0
		ORDER BY job_count DESC, name ASC
		LIMIT 150
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var companies []models.Company
	for rows.Next() {
		var c models.Company
		if err := rows.Scan(&c.ID, &c.Slug, &c.Name, &c.LogoURL, &c.JobCount); err != nil {
			return nil, err
		}
		companies = append(companies, c)
	}
	return companies, nil
}

// LocationRepository handles location database operations.
type LocationRepository struct {
	db *pgxpool.Pool
}

func NewLocationRepository(db *pgxpool.Pool) *LocationRepository {
	return &LocationRepository{db: db}
}

func (r *LocationRepository) FindBySlug(ctx context.Context, slug string) (*models.Location, error) {
	loc := &models.Location{}
	err := r.db.QueryRow(ctx, `
		SELECT l.id, l.slug, l.name, l.parent_id, l.level,
			COALESCE(l.latitude, 0.0) as latitude,
			COALESCE(l.longitude, 0.0) as longitude,
			l.job_count,
			COALESCE(p.name, '') as parent_name
		FROM locations l
		LEFT JOIN locations p ON l.parent_id = p.id
		WHERE l.slug = $1
	`, slug).Scan(
		&loc.ID, &loc.Slug, &loc.Name, &loc.ParentID, &loc.Level,
		&loc.Latitude, &loc.Longitude, &loc.JobCount, &loc.ParentName,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("location not found: %s", slug)
	}
	return loc, err
}

func (r *LocationRepository) FindByLevel(ctx context.Context, level string) ([]models.Location, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, slug, name, parent_id, level, job_count
		FROM locations
		WHERE level = $1
		ORDER BY job_count DESC
	`, level)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var locations []models.Location
	for rows.Next() {
		var loc models.Location
		if err := rows.Scan(&loc.ID, &loc.Slug, &loc.Name, &loc.ParentID, &loc.Level, &loc.JobCount); err != nil {
			return nil, err
		}
		locations = append(locations, loc)
	}
	return locations, nil
}

func (r *LocationRepository) FindChildren(ctx context.Context, parentID int) ([]models.Location, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, slug, name, parent_id, level, job_count
		FROM locations
		WHERE parent_id = $1
		ORDER BY job_count DESC
	`, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var locations []models.Location
	for rows.Next() {
		var loc models.Location
		if err := rows.Scan(&loc.ID, &loc.Slug, &loc.Name, &loc.ParentID, &loc.Level, &loc.JobCount); err != nil {
			return nil, err
		}
		locations = append(locations, loc)
	}
	return locations, nil
}

func (r *LocationRepository) FindByName(ctx context.Context, name string) (*models.Location, error) {
	loc := &models.Location{}
	err := r.db.QueryRow(ctx, `
		SELECT id, slug, name, level FROM locations
		WHERE LOWER(name) = LOWER($1)
		LIMIT 1
	`, name).Scan(&loc.ID, &loc.Slug, &loc.Name, &loc.Level)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("location not found: %s", name)
	}
	return loc, err
}

func (r *LocationRepository) UpdateJobCounts(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `
		UPDATE locations l
		SET job_count = COALESCE(sub.cnt, 0)
		FROM (
			SELECT location_id, COUNT(*) as cnt
			FROM jobs
			WHERE status = 'active' AND location_id IS NOT NULL
			GROUP BY location_id
		) sub
		WHERE l.id = sub.location_id;

		UPDATE locations
		SET job_count = 0
		WHERE id NOT IN (
			SELECT DISTINCT location_id FROM jobs WHERE status = 'active' AND location_id IS NOT NULL
		) AND job_count != 0;
	`)
	return err
}

// FindAllActive returns all locations for directory rendering and SEO exposure.
func (r *LocationRepository) FindAllActive(ctx context.Context) ([]models.Location, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, slug, name, level, job_count
		FROM locations
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var locations []models.Location
	for rows.Next() {
		var l models.Location
		if err := rows.Scan(&l.ID, &l.Slug, &l.Name, &l.Level, &l.JobCount); err != nil {
			return nil, err
		}
		locations = append(locations, l)
	}
	return locations, nil
}

// RoleRepository handles job role database operations.
type RoleRepository struct {
	db *pgxpool.Pool
}

func NewRoleRepository(db *pgxpool.Pool) *RoleRepository {
	return &RoleRepository{db: db}
}

func (r *RoleRepository) FindBySlug(ctx context.Context, slug string) (*models.JobRole, error) {
	role := &models.JobRole{}
	err := r.db.QueryRow(ctx, `
		SELECT id, slug, name, parent_id, industry_id, job_count, seo_title, seo_description
		FROM job_roles WHERE slug = $1
	`, slug).Scan(
		&role.ID, &role.Slug, &role.Name, &role.ParentID, &role.IndustryID,
		&role.JobCount, &role.SEOTitle, &role.SEODescription,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("role not found: %s", slug)
	}
	return role, err
}

func (r *RoleRepository) FindAll(ctx context.Context) ([]models.JobRole, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, slug, name, parent_id, industry_id, job_count
		FROM job_roles
		WHERE parent_id IS NULL
		ORDER BY job_count DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []models.JobRole
	for rows.Next() {
		var role models.JobRole
		if err := rows.Scan(&role.ID, &role.Slug, &role.Name, &role.ParentID, &role.IndustryID, &role.JobCount); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, nil
}

func (r *RoleRepository) FindTop(ctx context.Context, limit int) ([]models.JobRole, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, slug, name, parent_id, industry_id, job_count
		FROM job_roles
		WHERE parent_id IS NULL
		ORDER BY job_count DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []models.JobRole
	for rows.Next() {
		var role models.JobRole
		if err := rows.Scan(&role.ID, &role.Slug, &role.Name, &role.ParentID, &role.IndustryID, &role.JobCount); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, nil
}

func (r *RoleRepository) FindAllFiltered(ctx context.Context, search, letter string, limit, offset int) ([]models.JobRole, int, error) {
	whereClauses := []string{"parent_id IS NULL"}
	args := []interface{}{}
	argID := 1

	searchTrimmed := strings.TrimSpace(search)
	prefixArg := ""

	if searchTrimmed != "" {
		if len(searchTrimmed) == 1 {
			// Single letter query is an exact prefix search (e.g. 'B' -> starts with 'B')
			whereClauses = append(whereClauses, fmt.Sprintf("name ILIKE $%d", argID))
			args = append(args, searchTrimmed+"%")
			argID++
		} else {
			whereClauses = append(whereClauses, fmt.Sprintf("name ILIKE $%d", argID))
			args = append(args, "%"+searchTrimmed+"%")
			argID++
			prefixArg = searchTrimmed + "%"
		}
	}

	if letter != "" && len(letter) == 1 {
		whereClauses = append(whereClauses, fmt.Sprintf("name ILIKE $%d", argID))
		args = append(args, letter+"%")
		argID++
	}

	whereSQL := strings.Join(whereClauses, " AND ")

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM job_roles WHERE %s", whereSQL)
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	var query string
	if prefixArg != "" {
		query = fmt.Sprintf(`
			SELECT id, slug, name, parent_id, industry_id, job_count
			FROM job_roles
			WHERE %s
			ORDER BY 
				CASE WHEN name ILIKE $%d THEN 0 ELSE 1 END ASC,
				CASE WHEN job_count > 0 THEN 1 ELSE 2 END ASC, 
				job_count DESC, 
				name ASC
		`, whereSQL, argID)
		args = append(args, prefixArg)
		argID++
	} else {
		query = fmt.Sprintf(`
			SELECT id, slug, name, parent_id, industry_id, job_count
			FROM job_roles
			WHERE %s
			ORDER BY CASE WHEN job_count > 0 THEN 1 ELSE 2 END ASC, job_count DESC, name ASC
		`, whereSQL)
	}

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argID, argID+1)
		args = append(args, limit, offset)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var roles []models.JobRole
	for rows.Next() {
		var role models.JobRole
		if err := rows.Scan(&role.ID, &role.Slug, &role.Name, &role.ParentID, &role.IndustryID, &role.JobCount); err != nil {
			return nil, 0, err
		}
		roles = append(roles, role)
	}
	return roles, total, nil
}

func (r *RoleRepository) UpdateJobCounts(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO job_roles_mapping (role_id, job_id)
		SELECT jr.id, j.id
		FROM job_roles jr
		JOIN jobs j ON (
			j.status = 'active' AND (
				j.title ILIKE '%' || jr.name || '%'
				OR j.title ILIKE '%' || REPLACE(jr.slug, '-', ' ') || '%'
			)
		)
		ON CONFLICT (role_id, job_id) DO NOTHING;

		UPDATE job_roles jr
		SET job_count = COALESCE(sub.cnt, 0)
		FROM (
			SELECT jrm.role_id, COUNT(DISTINCT j.id) as cnt
			FROM job_roles_mapping jrm
			JOIN jobs j ON jrm.job_id = j.id AND j.status = 'active'
			GROUP BY jrm.role_id
		) sub
		WHERE jr.id = sub.role_id;

		UPDATE job_roles
		SET job_count = 0
		WHERE id NOT IN (
			SELECT DISTINCT jrm.role_id FROM job_roles_mapping jrm JOIN jobs j ON jrm.job_id = j.id AND j.status = 'active'
		) AND job_count != 0;
	`)
	return err
}

// IndustryRepository handles industry database operations.
type IndustryRepository struct {
	db *pgxpool.Pool
}

func NewIndustryRepository(db *pgxpool.Pool) *IndustryRepository {
	return &IndustryRepository{db: db}
}

func (r *IndustryRepository) FindBySlug(ctx context.Context, slug string) (*models.Industry, error) {
	ind := &models.Industry{}
	err := r.db.QueryRow(ctx, `
		SELECT id, slug, name, job_count, seo_title, seo_description
		FROM industries WHERE slug = $1
	`, slug).Scan(&ind.ID, &ind.Slug, &ind.Name, &ind.JobCount, &ind.SEOTitle, &ind.SEODescription)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("industry not found: %s", slug)
	}
	return ind, err
}

func (r *IndustryRepository) FindAll(ctx context.Context) ([]models.Industry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, slug, name, job_count FROM industries ORDER BY job_count DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var industries []models.Industry
	for rows.Next() {
		var ind models.Industry
		if err := rows.Scan(&ind.ID, &ind.Slug, &ind.Name, &ind.JobCount); err != nil {
			return nil, err
		}
		industries = append(industries, ind)
	}
	return industries, nil
}

func (r *IndustryRepository) UpdateJobCounts(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `
		UPDATE industries ind
		SET job_count = COALESCE(sub.cnt, 0)
		FROM (
			SELECT jim.industry_id, COUNT(DISTINCT j.id) as cnt
			FROM job_industries_mapping jim
			JOIN jobs j ON jim.job_id = j.id AND j.status = 'active'
			GROUP BY jim.industry_id
		) sub
		WHERE ind.id = sub.industry_id;

		UPDATE industries
		SET job_count = 0
		WHERE id NOT IN (
			SELECT DISTINCT jim.industry_id FROM job_industries_mapping jim JOIN jobs j ON jim.job_id = j.id AND j.status = 'active'
		) AND job_count != 0;
	`)
	return err
}

// ArticleRepository handles article database operations.
type ArticleRepository struct {
	db *pgxpool.Pool
}

func NewArticleRepository(db *pgxpool.Pool) *ArticleRepository {
	return &ArticleRepository{db: db}
}

func (r *ArticleRepository) FindBySlug(ctx context.Context, slug string) (*models.Article, error) {
	article := &models.Article{}
	err := r.db.QueryRow(ctx, `
		SELECT id, slug, title, content, category, COALESCE(seo_title, ''), COALESCE(seo_description, ''),
			published, published_at, created_at, updated_at
		FROM articles 
		WHERE slug = $1 AND published = true AND (published_at IS NULL OR published_at <= NOW())
	`, slug).Scan(
		&article.ID, &article.Slug, &article.Title, &article.Content,
		&article.Category, &article.SEOTitle, &article.SEODescription,
		&article.Published, &article.PublishedAt, &article.CreatedAt, &article.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("article not found: %s", slug)
	}
	return article, err
}

func (r *ArticleRepository) FindByCategory(ctx context.Context, category string) ([]models.Article, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, slug, title, category, published_at, COALESCE(seo_description, '')
		FROM articles
		WHERE category = $1 AND published = true AND (published_at IS NULL OR published_at <= NOW())
		ORDER BY published_at DESC
	`, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []models.Article
	for rows.Next() {
		var a models.Article
		if err := rows.Scan(&a.ID, &a.Slug, &a.Title, &a.Category, &a.PublishedAt, &a.SEODescription); err != nil {
			return nil, err
		}
		articles = append(articles, a)
	}
	return articles, nil
}

// FindPublished returns the latest publicly visible articles (excluding future scheduled drip-feed).
func (r *ArticleRepository) FindPublished(ctx context.Context, limit int) ([]models.Article, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, slug, title, category, published_at, COALESCE(seo_description, '')
		FROM articles
		WHERE published = true AND (published_at IS NULL OR published_at <= NOW())
		ORDER BY published_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []models.Article
	for rows.Next() {
		var a models.Article
		if err := rows.Scan(&a.ID, &a.Slug, &a.Title, &a.Category, &a.PublishedAt, &a.SEODescription); err != nil {
			return nil, err
		}
		articles = append(articles, a)
	}
	return articles, nil
}

func (r *ArticleRepository) FindAll(ctx context.Context) ([]models.Article, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, slug, title, category, published, published_at, created_at, updated_at
		FROM articles
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []models.Article
	for rows.Next() {
		var a models.Article
		if err := rows.Scan(
			&a.ID, &a.Slug, &a.Title, &a.Category,
			&a.Published, &a.PublishedAt, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, err
		}
		articles = append(articles, a)
	}
	return articles, nil
}

func (r *ArticleRepository) FindByID(ctx context.Context, id int) (*models.Article, error) {
	article := &models.Article{}
	err := r.db.QueryRow(ctx, `
		SELECT id, slug, title, content, category, seo_title, seo_description,
			published, published_at, created_at, updated_at
		FROM articles WHERE id = $1
	`, id).Scan(
		&article.ID, &article.Slug, &article.Title, &article.Content,
		&article.Category, &article.SEOTitle, &article.SEODescription,
		&article.Published, &article.PublishedAt, &article.CreatedAt, &article.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("article not found with id: %d", id)
	}
	return article, err
}

func (r *ArticleRepository) Create(ctx context.Context, a *models.Article) error {
	now := time.Now()
	a.CreatedAt = now
	a.UpdatedAt = now
	var pubAt *time.Time
	if a.Published {
		pubAt = &now
	}
	a.PublishedAt = pubAt

	return r.db.QueryRow(ctx, `
		INSERT INTO articles (slug, title, content, category, seo_title, seo_description, published, published_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`, a.Slug, a.Title, a.Content, a.Category, a.SEOTitle, a.SEODescription, a.Published, a.PublishedAt, a.CreatedAt, a.UpdatedAt).Scan(&a.ID)
}

func (r *ArticleRepository) Update(ctx context.Context, a *models.Article) error {
	a.UpdatedAt = time.Now()
	var pubAt *time.Time
	if a.Published {
		if a.PublishedAt != nil {
			pubAt = a.PublishedAt
		} else {
			now := time.Now()
			pubAt = &now
		}
	}
	a.PublishedAt = pubAt

	_, err := r.db.Exec(ctx, `
		UPDATE articles
		SET slug = $1, title = $2, content = $3, category = $4, seo_title = $5,
			seo_description = $6, published = $7, published_at = $8, updated_at = $9
		WHERE id = $10
	`, a.Slug, a.Title, a.Content, a.Category, a.SEOTitle, a.SEODescription, a.Published, a.PublishedAt, a.UpdatedAt, a.ID)
	return err
}

func (r *ArticleRepository) Delete(ctx context.Context, id int) error {
	_, err := r.db.Exec(ctx, "DELETE FROM articles WHERE id = $1", id)
	return err
}

// SalaryRepository handles salary guide database operations.
type SalaryRepository struct {
	db *pgxpool.Pool
}

func NewSalaryRepository(db *pgxpool.Pool) *SalaryRepository {
	return &SalaryRepository{db: db}
}

func (r *SalaryRepository) FindByRole(ctx context.Context, roleSlug string) (*models.SalaryGuide, error) {
	guide := &models.SalaryGuide{}
	err := r.db.QueryRow(ctx, `
		SELECT id, role_slug, role_name, COALESCE(location_slug, ''), COALESCE(location_name, ''),
			min_salary, max_salary, avg_salary, sample_size, COALESCE(data_source, ''),
			COALESCE(seo_title, ''), COALESCE(seo_description, ''), updated_at
		FROM salary_guides WHERE role_slug = $1
	`, roleSlug).Scan(
		&guide.ID, &guide.RoleSlug, &guide.RoleName, &guide.LocationSlug,
		&guide.LocationName, &guide.MinSalary, &guide.MaxSalary, &guide.AvgSalary,
		&guide.SampleSize, &guide.DataSource, &guide.SEOTitle, &guide.SEODescription,
		&guide.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("salary guide not found: %s", roleSlug)
	}
	return guide, err
}

func (r *SalaryRepository) FindAll(ctx context.Context) ([]models.SalaryGuide, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, role_slug, role_name, COALESCE(location_slug, ''), COALESCE(location_name, ''),
			min_salary, max_salary, avg_salary, sample_size, COALESCE(data_source, ''),
			COALESCE(seo_title, ''), COALESCE(seo_description, ''), updated_at
		FROM salary_guides
		ORDER BY role_name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var guides []models.SalaryGuide
	for rows.Next() {
		var g models.SalaryGuide
		if err := rows.Scan(
			&g.ID, &g.RoleSlug, &g.RoleName, &g.LocationSlug,
			&g.LocationName, &g.MinSalary, &g.MaxSalary, &g.AvgSalary,
			&g.SampleSize, &g.DataSource, &g.SEOTitle, &g.SEODescription,
			&g.UpdatedAt,
		); err != nil {
			return nil, err
		}
		guides = append(guides, g)
	}
	return guides, nil
}

func (r *SalaryRepository) FindAllFiltered(ctx context.Context, search, letter string, limit, offset int) ([]models.SalaryGuide, int, error) {
	whereClauses := []string{"1=1"}
	args := []interface{}{}
	argID := 1

	searchTrimmed := strings.TrimSpace(search)
	prefixArg := ""

	if searchTrimmed != "" {
		if len(searchTrimmed) == 1 {
			whereClauses = append(whereClauses, fmt.Sprintf("role_name ILIKE $%d", argID))
			args = append(args, searchTrimmed+"%")
			argID++
		} else {
			whereClauses = append(whereClauses, fmt.Sprintf("role_name ILIKE $%d", argID))
			args = append(args, "%"+searchTrimmed+"%")
			argID++
			prefixArg = searchTrimmed + "%"
		}
	}

	if letter != "" && len(letter) == 1 {
		whereClauses = append(whereClauses, fmt.Sprintf("role_name ILIKE $%d", argID))
		args = append(args, letter+"%")
		argID++
	}

	whereSQL := strings.Join(whereClauses, " AND ")

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM salary_guides WHERE %s", whereSQL)
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	var query string
	if prefixArg != "" {
		query = fmt.Sprintf(`
			SELECT id, role_slug, role_name, COALESCE(location_slug, ''), COALESCE(location_name, ''),
				min_salary, max_salary, avg_salary, sample_size, COALESCE(data_source, ''),
				COALESCE(seo_title, ''), COALESCE(seo_description, ''), updated_at
			FROM salary_guides
			WHERE %s
			ORDER BY CASE WHEN role_name ILIKE $%d THEN 0 ELSE 1 END ASC, role_name ASC
		`, whereSQL, argID)
		args = append(args, prefixArg)
		argID++
	} else {
		query = fmt.Sprintf(`
			SELECT id, role_slug, role_name, COALESCE(location_slug, ''), COALESCE(location_name, ''),
				min_salary, max_salary, avg_salary, sample_size, COALESCE(data_source, ''),
				COALESCE(seo_title, ''), COALESCE(seo_description, ''), updated_at
			FROM salary_guides
			WHERE %s
			ORDER BY role_name ASC
		`, whereSQL)
	}

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argID, argID+1)
		args = append(args, limit, offset)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var guides []models.SalaryGuide
	for rows.Next() {
		var g models.SalaryGuide
		if err := rows.Scan(
			&g.ID, &g.RoleSlug, &g.RoleName, &g.LocationSlug,
			&g.LocationName, &g.MinSalary, &g.MaxSalary, &g.AvgSalary,
			&g.SampleSize, &g.DataSource, &g.SEOTitle, &g.SEODescription,
			&g.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		guides = append(guides, g)
	}
	return guides, total, nil
}

// AdminRepository handles admin user database operations.
type AdminRepository struct {
	db *pgxpool.Pool
}

func NewAdminRepository(db *pgxpool.Pool) *AdminRepository {
	return &AdminRepository{db: db}
}

func (r *AdminRepository) FindByUsername(ctx context.Context, username string) (*models.AdminUser, error) {
	user := &models.AdminUser{}
	err := r.db.QueryRow(ctx, `
		SELECT id, username, password_hash, role, last_login, created_at
		FROM admin_users WHERE username = $1
	`, username).Scan(
		&user.ID, &user.Username, &user.PasswordHash, &user.Role,
		&user.LastLogin, &user.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("admin user not found: %s", username)
	}
	return user, err
}

func (r *AdminRepository) UpdateLastLogin(ctx context.Context, id int) error {
	_, err := r.db.Exec(ctx, "UPDATE admin_users SET last_login = NOW() WHERE id = $1", id)
	return err
}

// RedirectRepository handles redirect database operations.
type RedirectRepository struct {
	db *pgxpool.Pool
}

func NewRedirectRepository(db *pgxpool.Pool) *RedirectRepository {
	return &RedirectRepository{db: db}
}

func (r *RedirectRepository) FindByPath(ctx context.Context, path string) (*models.Redirect, error) {
	redirect := &models.Redirect{}
	err := r.db.QueryRow(ctx, `
		SELECT id, from_path, to_path, status_code FROM redirects WHERE from_path = $1
	`, path).Scan(&redirect.ID, &redirect.FromPath, &redirect.ToPath, &redirect.StatusCode)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("redirect not found: %s", path)
	}
	return redirect, err
}

func (r *RedirectRepository) FindAll(ctx context.Context) ([]models.Redirect, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, from_path, to_path, status_code, created_at
		FROM redirects
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var redirects []models.Redirect
	for rows.Next() {
		var rd models.Redirect
		if err := rows.Scan(&rd.ID, &rd.FromPath, &rd.ToPath, &rd.StatusCode, &rd.CreatedAt); err != nil {
			return nil, err
		}
		redirects = append(redirects, rd)
	}
	return redirects, nil
}

func (r *RedirectRepository) Create(ctx context.Context, rd *models.Redirect) error {
	rd.CreatedAt = time.Now()
	return r.db.QueryRow(ctx, `
		INSERT INTO redirects (from_path, to_path, status_code, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, rd.FromPath, rd.ToPath, rd.StatusCode, rd.CreatedAt).Scan(&rd.ID)
}

func (r *RedirectRepository) Delete(ctx context.Context, id int) error {
	_, err := r.db.Exec(ctx, "DELETE FROM redirects WHERE id = $1", id)
	return err
}

// PageViewRepository handles analytics database operations.
type PageViewRepository struct {
	db *pgxpool.Pool
}

func NewPageViewRepository(db *pgxpool.Pool) *PageViewRepository {
	return &PageViewRepository{db: db}
}

func (r *PageViewRepository) Record(ctx context.Context, pv *models.PageView) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO page_views (path, referrer, user_agent, ip_hash, country, device_type)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, pv.Path, pv.Referrer, pv.UserAgent, pv.IPHash, pv.Country, pv.DeviceType)
	return err
}
