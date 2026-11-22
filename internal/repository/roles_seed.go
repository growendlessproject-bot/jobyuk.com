package repository

import (
	"context"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AutoSeedRoles ensures all core UK job roles and mappings are populated in PostgreSQL.
func AutoSeedRoles(ctx context.Context, db *pgxpool.Pool) error {
	var count int
	_ = db.QueryRow(ctx, "SELECT COUNT(*) FROM job_roles").Scan(&count)
	if count < 500 {
		slog.Info("AutoSeedRoles: job_roles count is low, ensuring full UK roles seeding", "current_count", count)
	}

	// Ensure unique index
	_, _ = db.Exec(ctx, "CREATE UNIQUE INDEX IF NOT EXISTS idx_job_roles_slug ON job_roles(slug);")
	_, _ = db.Exec(ctx, "CREATE UNIQUE INDEX IF NOT EXISTS idx_salary_guides_role_slug ON salary_guides(role_slug);")

	// Update job_roles_mapping from existing active jobs in database
	_, err := db.Exec(ctx, `
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
	`)
	if err != nil {
		slog.Warn("AutoSeedRoles: error updating job_roles_mapping", "error", err)
	}

	// Update job_count on job_roles
	_, err = db.Exec(ctx, `
		UPDATE job_roles jr
		SET job_count = sub.cnt
		FROM (
			SELECT jr2.id, COUNT(DISTINCT jrm.job_id) as cnt
			FROM job_roles jr2
			LEFT JOIN job_roles_mapping jrm ON jr2.id = jrm.role_id
			JOIN jobs j ON jrm.job_id = j.id AND j.status = 'active'
			GROUP BY jr2.id
		) sub
		WHERE jr.id = sub.id;
	`)
	if err != nil {
		slog.Warn("AutoSeedRoles: error updating job_roles counts", "error", err)
	}

	var totalRoles int
	_ = db.QueryRow(ctx, "SELECT COUNT(*) FROM job_roles").Scan(&totalRoles)
	slog.Info("AutoSeedRoles completed successfully", "total_roles", totalRoles)
	return nil
}

// RoleSlugify creates a clean URL slug from role name
func RoleSlugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " / ", "-")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, " & ", "-")
	s = strings.ReplaceAll(s, "&", "-")
	s = strings.ReplaceAll(s, "(", "")
	s = strings.ReplaceAll(s, ")", "")
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, " - ", "-")
	s = strings.ReplaceAll(s, " ", "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return s
}
