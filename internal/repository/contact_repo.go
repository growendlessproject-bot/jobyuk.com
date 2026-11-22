package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"jobyuk/internal/models"
)

// ContactRepository handles database queries for user contact inquiries.
type ContactRepository struct {
	db *pgxpool.Pool
}

// NewContactRepository creates a new ContactRepository instance.
func NewContactRepository(db *pgxpool.Pool) *ContactRepository {
	return &ContactRepository{db: db}
}

// AutoMigrate creates the contact_inquiries table and required indexes idempotently.
func (r *ContactRepository) AutoMigrate(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS contact_inquiries (
			id BIGSERIAL PRIMARY KEY,
			name VARCHAR(150) NOT NULL,
			email VARCHAR(255) NOT NULL,
			subject VARCHAR(255) NOT NULL,
			inquiry_type VARCHAR(50) NOT NULL DEFAULT 'general',
			message TEXT NOT NULL,
			ip_address VARCHAR(50) DEFAULT '',
			user_agent VARCHAR(500) DEFAULT '',
			status VARCHAR(30) NOT NULL DEFAULT 'unread',
			admin_notes TEXT DEFAULT '',
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		);`,
		`CREATE INDEX IF NOT EXISTS idx_contact_inquiries_created_at ON contact_inquiries(created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_contact_inquiries_status ON contact_inquiries(status);`,
		`CREATE INDEX IF NOT EXISTS idx_contact_inquiries_email ON contact_inquiries(email);`,
	}

	for _, query := range queries {
		if _, err := r.db.Exec(ctx, query); err != nil {
			return fmt.Errorf("failed to auto-migrate contact table: %w", err)
		}
	}

	return nil
}

// Create inserts a new contact inquiry record.
func (r *ContactRepository) Create(ctx context.Context, inq *models.ContactInquiry) error {
	query := `
		INSERT INTO contact_inquiries (
			name, email, subject, inquiry_type, message,
			ip_address, user_agent, status, admin_notes, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, NOW(), NOW()
		)
		RETURNING id, created_at, updated_at
	`
	status := inq.Status
	if status == "" {
		status = "unread"
	}

	err := r.db.QueryRow(ctx, query,
		inq.Name,
		inq.Email,
		inq.Subject,
		inq.InquiryType,
		inq.Message,
		inq.IPAddress,
		inq.UserAgent,
		status,
		inq.AdminNotes,
	).Scan(&inq.ID, &inq.CreatedAt, &inq.UpdatedAt)

	return err
}

// FindAll retrieves inquiries with optional status filtering, search term, and pagination.
func (r *ContactRepository) FindAll(ctx context.Context, statusFilter string, searchQuery string, limit int, offset int) ([]models.ContactInquiry, int, error) {
	whereClauses := []string{"1=1"}
	args := []interface{}{}
	argIdx := 1

	if statusFilter != "" && statusFilter != "all" {
		whereClauses = append(whereClauses, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, statusFilter)
		argIdx++
	}

	if searchQuery != "" {
		term := "%" + strings.ToLower(searchQuery) + "%"
		whereClauses = append(whereClauses, fmt.Sprintf("(LOWER(name) LIKE $%d OR LOWER(email) LIKE $%d OR LOWER(subject) LIKE $%d OR LOWER(message) LIKE $%d)", argIdx, argIdx, argIdx, argIdx))
		args = append(args, term)
		argIdx++
	}

	whereSQL := strings.Join(whereClauses, " AND ")

	// Count query
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM contact_inquiries WHERE %s", whereSQL)
	var total int
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Data query
	dataSQL := fmt.Sprintf(`
		SELECT id, name, email, subject, inquiry_type, message,
		       COALESCE(ip_address, ''), COALESCE(user_agent, ''), status,
		       COALESCE(admin_notes, ''), created_at, updated_at
		FROM contact_inquiries
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereSQL, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var inquiries []models.ContactInquiry
	for rows.Next() {
		var inq models.ContactInquiry
		if err := rows.Scan(
			&inq.ID,
			&inq.Name,
			&inq.Email,
			&inq.Subject,
			&inq.InquiryType,
			&inq.Message,
			&inq.IPAddress,
			&inq.UserAgent,
			&inq.Status,
			&inq.AdminNotes,
			&inq.CreatedAt,
			&inq.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		inquiries = append(inquiries, inq)
	}

	return inquiries, total, nil
}

// FindByID retrieves a single inquiry by its primary key ID.
func (r *ContactRepository) FindByID(ctx context.Context, id int64) (*models.ContactInquiry, error) {
	query := `
		SELECT id, name, email, subject, inquiry_type, message,
		       COALESCE(ip_address, ''), COALESCE(user_agent, ''), status,
		       COALESCE(admin_notes, ''), created_at, updated_at
		FROM contact_inquiries
		WHERE id = $1
	`
	var inq models.ContactInquiry
	err := r.db.QueryRow(ctx, query, id).Scan(
		&inq.ID,
		&inq.Name,
		&inq.Email,
		&inq.Subject,
		&inq.InquiryType,
		&inq.Message,
		&inq.IPAddress,
		&inq.UserAgent,
		&inq.Status,
		&inq.AdminNotes,
		&inq.CreatedAt,
		&inq.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &inq, nil
}

// UpdateStatus changes the status of a contact inquiry (e.g., 'read', 'resolved', 'unread').
func (r *ContactRepository) UpdateStatus(ctx context.Context, id int64, status string) error {
	query := `
		UPDATE contact_inquiries
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`
	_, err := r.db.Exec(ctx, query, status, id)
	return err
}

// Delete removes an inquiry record.
func (r *ContactRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM contact_inquiries WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	return err
}

// GetStats returns summary metrics for inquiries (total, unread, read, resolved).
func (r *ContactRepository) GetStats(ctx context.Context) (models.ContactStats, error) {
	var stats models.ContactStats
	query := `
		SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE status = 'unread') as unread,
			COUNT(*) FILTER (WHERE status = 'read') as read,
			COUNT(*) FILTER (WHERE status = 'resolved') as resolved
		FROM contact_inquiries
	`
	err := r.db.QueryRow(ctx, query).Scan(&stats.Total, &stats.Unread, &stats.Read, &stats.Resolved)
	return stats, err
}

// CountRecentByIP counts how many inquiries were submitted by this IP in the last windowMinutes.
func (r *ContactRepository) CountRecentByIP(ctx context.Context, ip string, windowMinutes int) (int, error) {
	if ip == "" {
		return 0, nil
	}
	var count int
	query := `
		SELECT COUNT(*)
		FROM contact_inquiries
		WHERE ip_address = $1 AND created_at > NOW() - ($2 || ' minutes')::INTERVAL
	`
	err := r.db.QueryRow(ctx, query, ip, windowMinutes).Scan(&count)
	return count, err
}
