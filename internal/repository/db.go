package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewDB creates a new PostgreSQL connection pool.
func NewDB(databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Connection pool settings
	// Connection pool settings optimized for 1-Core VPS (multi-tenant)
	config.MaxConns = 12
	config.MinConns = 2
	config.MaxConnIdleTime = 3 * time.Minute
	config.MaxConnLifetime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}

// Repositories holds all repository instances.
type Repositories struct {
	db       *pgxpool.Pool
	Job      *JobRepository
	Company  *CompanyRepository
	Location *LocationRepository
	Role     *RoleRepository
	Industry *IndustryRepository
	Article  *ArticleRepository
	Salary   *SalaryRepository
	Admin    *AdminRepository
	Redirect *RedirectRepository
	PageView *PageViewRepository
	Pinterest *PinterestRepository
	WhatsApp  *WhatsAppRepository
	Contact   *ContactRepository
}

// NewRepositories creates all repository instances.
func NewRepositories(db *pgxpool.Pool) *Repositories {
	return &Repositories{
		db:        db,
		Job:       NewJobRepository(db),
		Company:   NewCompanyRepository(db),
		Location:  NewLocationRepository(db),
		Role:      NewRoleRepository(db),
		Industry:  NewIndustryRepository(db),
		Article:   NewArticleRepository(db),
		Salary:    NewSalaryRepository(db),
		Admin:     NewAdminRepository(db),
		Redirect:  NewRedirectRepository(db),
		PageView:  NewPageViewRepository(db),
		Pinterest: NewPinterestRepository(db),
		WhatsApp:  NewWhatsAppRepository(db),
		Contact:   NewContactRepository(db),
	}
}

// DB returns the underlying pgxpool connection pool.
func (r *Repositories) DB() *pgxpool.Pool {
	return r.db
}
