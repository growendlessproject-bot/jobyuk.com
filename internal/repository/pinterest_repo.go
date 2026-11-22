package repository

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"jobyuk/internal/models"
)

// PinterestRepository handles database operations for Pinterest users and exclusive assignments.
type PinterestRepository struct {
	db *pgxpool.Pool
}

// NewPinterestRepository creates a new Pinterest repository.
func NewPinterestRepository(db *pgxpool.Pool) *PinterestRepository {
	return &PinterestRepository{db: db}
}

// generate6DigitCode creates a secure 6-digit numeric access code.
func generate6DigitCode() string {
	nBig, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	}
	return fmt.Sprintf("%06d", nBig.Int64()+100000)
}

// CreatePinterestUser creates a new Pinterest poster with an auto-generated unique 6-digit code.
func (r *PinterestRepository) CreatePinterestUser(ctx context.Context, user *models.PinterestUser) error {
	if user.DailyLimit <= 0 {
		user.DailyLimit = 5
	}
	user.Name = strings.TrimSpace(user.Name)
	user.Email = strings.ToLower(strings.TrimSpace(user.Email))
	user.Phone = strings.TrimSpace(user.Phone)

	// Generate a unique 6-digit code
	for i := 0; i < 10; i++ {
		code := generate6DigitCode()
		var exists bool
		_ = r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pinterest_users WHERE access_code = $1)`, code).Scan(&exists)
		if !exists {
			user.AccessCode = code
			break
		}
	}
	if user.AccessCode == "" {
		user.AccessCode = generate6DigitCode()
	}

	query := `
		INSERT INTO pinterest_users (name, email, phone, access_code, daily_limit, is_active)
		VALUES ($1, $2, $3, $4, $5, true)
		RETURNING id, created_at, updated_at
	`
	err := r.db.QueryRow(ctx, query, user.Name, user.Email, user.Phone, user.AccessCode, user.DailyLimit).
		Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return err
	}
	_, _ = r.db.Exec(ctx, `
		INSERT INTO pinterest_user_daily_quotas (user_id, quota_date, daily_limit)
		VALUES ($1, CURRENT_DATE, $2)
		ON CONFLICT (user_id, quota_date) DO UPDATE SET daily_limit = EXCLUDED.daily_limit
	`, user.ID, user.DailyLimit)
	return nil
}

// GetPinterestUserByCode retrieves a user by their 6-digit access code.
func (r *PinterestRepository) GetPinterestUserByCode(ctx context.Context, code string) (*models.PinterestUser, error) {
	code = strings.TrimSpace(code)
	query := `
		SELECT id, name, email, phone, access_code, COALESCE(pinterest_handle, ''), daily_limit, is_active, COALESCE(deactivation_reason, ''), created_at, updated_at
		FROM pinterest_users
		WHERE access_code = $1
	`
	var u models.PinterestUser
	err := r.db.QueryRow(ctx, query, code).
		Scan(&u.ID, &u.Name, &u.Email, &u.Phone, &u.AccessCode, &u.PinterestHandle, &u.DailyLimit, &u.IsActive, &u.DeactivationReason, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetPinterestUserByID retrieves a user by ID.
func (r *PinterestRepository) GetPinterestUserByID(ctx context.Context, id int) (*models.PinterestUser, error) {
	query := `
		SELECT id, name, email, phone, access_code, COALESCE(pinterest_handle, ''), daily_limit, is_active, COALESCE(deactivation_reason, ''), created_at, updated_at
		FROM pinterest_users
		WHERE id = $1
	`
	var u models.PinterestUser
	err := r.db.QueryRow(ctx, query, id).
		Scan(&u.ID, &u.Name, &u.Email, &u.Phone, &u.AccessCode, &u.PinterestHandle, &u.DailyLimit, &u.IsActive, &u.DeactivationReason, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// ListPinterestUsers lists all users with their today's and lifetime pin counts.
func (r *PinterestRepository) ListPinterestUsers(ctx context.Context) ([]models.PinterestUser, error) {
	query := `
		SELECT 
			u.id, u.name, u.email, u.phone, u.access_code, COALESCE(u.pinterest_handle, ''), u.daily_limit, u.is_active, COALESCE(u.deactivation_reason, ''), u.created_at, u.updated_at,
			COALESCE((SELECT COUNT(*) FROM pinterest_assignments pa WHERE pa.user_id = u.id AND pa.status = 'submitted' AND pa.submitted_at::date = CURRENT_DATE), 0) AS today_pins,
			COALESCE((SELECT COUNT(*) FROM pinterest_assignments pa WHERE pa.user_id = u.id AND pa.status = 'submitted'), 0) AS total_pins
		FROM pinterest_users u
		ORDER BY u.created_at DESC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.PinterestUser
	for rows.Next() {
		var u models.PinterestUser
		if err := rows.Scan(
			&u.ID, &u.Name, &u.Email, &u.Phone, &u.AccessCode, &u.PinterestHandle, &u.DailyLimit, &u.IsActive, &u.DeactivationReason, &u.CreatedAt, &u.UpdatedAt,
			&u.TodayPinsCount, &u.TotalPinsCount,
		); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

// UpdatePinterestHandle updates the user's saved Pinterest handle.
func (r *PinterestRepository) UpdatePinterestHandle(ctx context.Context, userID int, handle string) error {
	handle = strings.TrimSpace(handle)
	if strings.HasPrefix(handle, "@") {
		handle = strings.TrimPrefix(handle, "@")
	}
	_, err := r.db.Exec(ctx, `
		UPDATE pinterest_users
		SET pinterest_handle = $2, updated_at = NOW()
		WHERE id = $1
	`, userID, handle)
	return err
}

// UpdatePinterestUserQuota updates the daily quota and status of a user.
func (r *PinterestRepository) UpdatePinterestUserQuota(ctx context.Context, id int, dailyLimit int) error {
	if dailyLimit <= 0 {
		dailyLimit = 5
	}
	_, err := r.db.Exec(ctx, `
		UPDATE pinterest_users
		SET daily_limit = $2, updated_at = NOW()
		WHERE id = $1
	`, id, dailyLimit)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO pinterest_user_daily_quotas (user_id, quota_date, daily_limit)
		VALUES ($1, CURRENT_DATE, $2)
		ON CONFLICT (user_id, quota_date) DO UPDATE SET daily_limit = EXCLUDED.daily_limit
	`, id, dailyLimit)
	return err
}

// TogglePinterestUserActive toggles a user's active state.
func (r *PinterestRepository) TogglePinterestUserActive(ctx context.Context, id int) error {
	_, err := r.db.Exec(ctx, `
		UPDATE pinterest_users
		SET is_active = NOT is_active,
		    deactivation_reason = CASE WHEN is_active = true THEN 'Deactivated by admin. Please contact admin.' ELSE '' END,
		    updated_at = NOW()
		WHERE id = $1
	`, id)
	return err
}

// RegeneratePinterestCode generates a new 6-digit access code for the user.
func (r *PinterestRepository) RegeneratePinterestCode(ctx context.Context, id int) (string, error) {
	newCode := generate6DigitCode()
	_, err := r.db.Exec(ctx, `
		UPDATE pinterest_users
		SET access_code = $2, updated_at = NOW()
		WHERE id = $1
	`, id, newCode)
	return newCode, err
}

// DeletePinterestUser permanently deletes a Pinterest poster and all their database records.
func (r *PinterestRepository) DeletePinterestUser(ctx context.Context, id int) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// 1. Delete assignments associated with this user
	_, err = tx.Exec(ctx, `DELETE FROM pinterest_assignments WHERE user_id = $1`, id)
	if err != nil {
		return err
	}

	// 2. Delete daily quotas associated with this user
	_, err = tx.Exec(ctx, `DELETE FROM pinterest_user_daily_quotas WHERE user_id = $1`, id)
	if err != nil {
		return err
	}

	// 3. Delete user record
	_, err = tx.Exec(ctx, `DELETE FROM pinterest_users WHERE id = $1`, id)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// GetUserTodayProgress returns the current progress of a user today.
func (r *PinterestRepository) GetUserTodayProgress(ctx context.Context, userID int) (*models.PinterestUserDailyProgress, error) {
	user, err := r.GetPinterestUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	var todayCompleted int
	err = r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM pinterest_assignments
		WHERE user_id = $1 AND status = 'submitted' AND submitted_at::date = CURRENT_DATE
	`, userID).Scan(&todayCompleted)
	if err != nil {
		return nil, err
	}

	remaining := user.DailyLimit - todayCompleted
	if remaining < 0 {
		remaining = 0
	}

	progress := &models.PinterestUserDailyProgress{
		UserID:         user.ID,
		UserName:       user.Name,
		DailyLimit:     user.DailyLimit,
		TodayCompleted: todayCompleted,
		Remaining:      remaining,
		IsCompleted:    todayCompleted >= user.DailyLimit,
	}

	// If not completed, get the active 1 job
	if !progress.IsCompleted {
		job, err := r.getOrAssignNextJob(ctx, user.ID)
		if err == nil {
			progress.CurrentJob = job
		}
	}

	return progress, nil
}

// getOrAssignNextJob gets the currently pending job for the user, or atomically assigns the next unique active job.
func (r *PinterestRepository) getOrAssignNextJob(ctx context.Context, userID int) (*models.Job, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// 1. Check if user already has a pending assigned job
	var pendingJobID int64
	err = tx.QueryRow(ctx, `
		SELECT job_id
		FROM pinterest_assignments
		WHERE user_id = $1 AND status = 'assigned'
		ORDER BY assigned_at ASC
		LIMIT 1
	`, userID).Scan(&pendingJobID)

	if err == nil && pendingJobID > 0 {
		// Fetch job details
		job, err := r.fetchJobByID(ctx, tx, pendingJobID)
		if err == nil {
			_ = tx.Commit(ctx)
			return job, nil
		}
	}

	// 2. Atomically find the next available active job that has NEVER been assigned to ANY user
	query := `
		SELECT j.id
		FROM jobs j
		WHERE j.status = 'active'
		  AND NOT EXISTS (
			  SELECT 1 FROM pinterest_assignments pa WHERE pa.job_id = j.id
		  )
		ORDER BY j.date_posted DESC
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`
	var nextJobID int64
	err = tx.QueryRow(ctx, query).Scan(&nextJobID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("no more active unassigned jobs available")
		}
		return nil, err
	}

	// 3. Assign this job exclusively to the user
	_, err = tx.Exec(ctx, `
		INSERT INTO pinterest_assignments (user_id, job_id, status, assigned_at)
		VALUES ($1, $2, 'assigned', NOW())
	`, userID, nextJobID)
	if err != nil {
		return nil, err
	}

	// 4. Fetch the full job data
	job, err := r.fetchJobByID(ctx, tx, nextJobID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return job, nil
}

// fetchJobByID helper for transaction queries.
func (r *PinterestRepository) fetchJobByID(ctx context.Context, tx pgx.Tx, jobID int64) (*models.Job, error) {
	query := `
		SELECT 
			j.id, j.slug, j.title, j.location_name, j.is_remote,
			COALESCE(j.contract_type, ''), COALESCE(j.job_type, ''),
			j.min_salary, j.max_salary, j.yearly_min, j.yearly_max,
			j.currency, j.salary_type,
			COALESCE(c.name, ''),
			COALESCE(j.pinterest_pin_url, ''),
			COALESCE(j.description, ''),
			j.date_posted, j.created_at
		FROM jobs j
		LEFT JOIN companies c ON j.employer_id = c.id
		WHERE j.id = $1
	`
	var j models.Job
	err := tx.QueryRow(ctx, query, jobID).Scan(
		&j.ID, &j.Slug, &j.Title, &j.LocationName, &j.IsRemote,
		&j.ContractType, &j.JobType,
		&j.MinSalary, &j.MaxSalary, &j.YearlyMin, &j.YearlyMax,
		&j.Currency, &j.SalaryType,
		&j.EmployerName,
		&j.PinterestPinURL,
		&j.Description,
		&j.DatePosted, &j.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &j, nil
}

// SubmitJobPin records the valid Pinterest Pin URL for the assigned job.
func (r *PinterestRepository) SubmitJobPin(ctx context.Context, userID int, jobID int64, pinURL string) error {
	pinURL = strings.TrimSpace(pinURL)
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Check if this pin URL was already submitted (by this user or any other)
	var existingCount int
	err = tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM pinterest_assignments
		WHERE pin_url = $1 AND status = 'submitted'
	`, pinURL).Scan(&existingCount)
	if err == nil && existingCount > 0 {
		return fmt.Errorf("This Pinterest Pin URL has already been submitted. Please create a unique pin for each job and paste the new URL.")
	}

	// Update assignment
	res, err := tx.Exec(ctx, `
		UPDATE pinterest_assignments
		SET pin_url = $3, status = 'submitted', submitted_at = NOW(), pin_date = CURRENT_DATE
		WHERE user_id = $1 AND job_id = $2
	`, userID, jobID, pinURL)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		// Auto-upsert assignment if not already pre-created
		_, err = tx.Exec(ctx, `
			INSERT INTO pinterest_assignments (user_id, job_id, pin_url, status, assigned_at, submitted_at, pin_date)
			VALUES ($1, $2, $3, 'submitted', NOW(), NOW(), CURRENT_DATE)
			ON CONFLICT (job_id) DO UPDATE
			SET user_id = EXCLUDED.user_id, pin_url = EXCLUDED.pin_url, status = 'submitted', submitted_at = NOW(), pin_date = CURRENT_DATE
		`, userID, jobID, pinURL)
		if err != nil {
			return err
		}
	}

	// Update job record as well
	_, err = tx.Exec(ctx, `
		UPDATE jobs
		SET pinterest_pin_url = $2, updated_at = NOW()
		WHERE id = $1
	`, jobID, pinURL)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// UpdateSubmissionPinURL allows a user to edit/correct their submitted Pinterest Pin URL.
func (r *PinterestRepository) UpdateSubmissionPinURL(ctx context.Context, userID int, assignmentID int64, pinURL string) error {
	pinURL = strings.TrimSpace(pinURL)
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var jobID int64
	err = tx.QueryRow(ctx, `
		SELECT job_id
		FROM pinterest_assignments
		WHERE id = $1 AND user_id = $2
	`, assignmentID, userID).Scan(&jobID)
	if err != nil {
		return fmt.Errorf("submission not found or unauthorized")
	}

	// Check if this pin URL was already submitted by another submission
	var existingCount int
	err = tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM pinterest_assignments
		WHERE pin_url = $1 AND id != $2 AND status = 'submitted'
	`, pinURL, assignmentID).Scan(&existingCount)
	if err == nil && existingCount > 0 {
		return fmt.Errorf("This Pinterest Pin URL is already used for another job submission. Please enter a unique pin URL.")
	}

	// Update assignment pin_url
	_, err = tx.Exec(ctx, `
		UPDATE pinterest_assignments
		SET pin_url = $3
		WHERE id = $1 AND user_id = $2
	`, assignmentID, userID, pinURL)
	if err != nil {
		return err
	}

	// Update job record
	_, err = tx.Exec(ctx, `
		UPDATE jobs
		SET pinterest_pin_url = $2, updated_at = NOW()
		WHERE id = $1
	`, jobID, pinURL)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// GetUserSubmissions retrieves history of submitted pins for a specific user.
func (r *PinterestRepository) GetUserSubmissions(ctx context.Context, userID int, limit, offset int) ([]models.PinterestAssignment, int, error) {
	var total int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM pinterest_assignments
		WHERE user_id = $1 AND status = 'submitted'
	`, userID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT 
			pa.id, pa.user_id, pa.job_id, pa.pin_url, pa.status, pa.assigned_at, pa.submitted_at, pa.pin_date, pa.created_at,
			j.title, j.slug, j.location_name, COALESCE(c.name, '')
		FROM pinterest_assignments pa
		JOIN jobs j ON pa.job_id = j.id
		LEFT JOIN companies c ON j.employer_id = c.id
		WHERE pa.user_id = $1 AND pa.status = 'submitted'
		ORDER BY pa.submitted_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []models.PinterestAssignment
	for rows.Next() {
		var a models.PinterestAssignment
		if err := rows.Scan(
			&a.ID, &a.UserID, &a.JobID, &a.PinURL, &a.Status, &a.AssignedAt, &a.SubmittedAt, &a.PinDate, &a.CreatedAt,
			&a.JobTitle, &a.JobSlug, &a.LocationName, &a.EmployerName,
		); err != nil {
			return nil, 0, err
		}
		list = append(list, a)
	}
	return list, total, nil
}

// GetAdminDailyAuditReport returns a 14-day audit of user completion vs daily target.
func (r *PinterestRepository) GetAdminDailyAuditReport(ctx context.Context, days int) ([]models.PinterestDailyAuditItem, error) {
	if days <= 0 {
		days = 14
	}
	query := `
		WITH dates AS (
			SELECT generate_series(CURRENT_DATE - INTERVAL '1 day' * $1, CURRENT_DATE, '1 day')::date AS d
		),
		user_dates AS (
			SELECT 
				d.d as audit_date, 
				u.id as user_id, 
				u.name as user_name, 
				COALESCE(
					pudq.daily_limit,
					(SELECT pudq2.daily_limit FROM pinterest_user_daily_quotas pudq2 WHERE pudq2.user_id = u.id AND pudq2.quota_date <= d.d ORDER BY pudq2.quota_date DESC LIMIT 1),
					CASE WHEN d.d = CURRENT_DATE THEN u.daily_limit ELSE 5 END
				) as daily_limit
			FROM dates d
			CROSS JOIN pinterest_users u
			LEFT JOIN pinterest_user_daily_quotas pudq 
				ON pudq.user_id = u.id AND pudq.quota_date = d.d
			WHERE u.is_active = true AND d.d >= u.created_at::date
		)
		SELECT 
			TO_CHAR(ud.audit_date, 'YYYY-MM-DD') as formatted_date,
			ud.user_id,
			ud.user_name,
			ud.daily_limit,
			COALESCE(COUNT(pa.id), 0) as pins_submitted,
			CASE 
				WHEN COALESCE(COUNT(pa.id), 0) >= ud.daily_limit THEN 'Completed'
				WHEN COALESCE(COUNT(pa.id), 0) > 0 THEN 'Partial'
				ELSE 'Missed'
			END as status
		FROM user_dates ud
		LEFT JOIN pinterest_assignments pa 
			ON pa.user_id = ud.user_id 
			AND pa.status = 'submitted' 
			AND pa.submitted_at::date = ud.audit_date
		GROUP BY ud.audit_date, ud.user_id, ud.user_name, ud.daily_limit
		ORDER BY ud.audit_date DESC, ud.user_name ASC
	`
	rows, err := r.db.Query(ctx, query, days-1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.PinterestDailyAuditItem
	for rows.Next() {
		var item models.PinterestDailyAuditItem
		if err := rows.Scan(
			&item.Date, &item.UserID, &item.UserName, &item.DailyLimit, &item.PinsSubmitted, &item.Status,
		); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, nil
}

// GetAdminDailyAuditReportForUser returns a daily breakdown for a specific user.
func (r *PinterestRepository) GetAdminDailyAuditReportForUser(ctx context.Context, userID int, days int) ([]models.PinterestDailyAuditItem, error) {
	if days <= 0 {
		days = 30
	}
	query := `
		WITH dates AS (
			SELECT generate_series(CURRENT_DATE - INTERVAL '1 day' * $2, CURRENT_DATE, '1 day')::date AS d
		),
		user_dates AS (
			SELECT 
				d.d as audit_date, 
				u.id as user_id, 
				u.name as user_name, 
				COALESCE(
					pudq.daily_limit,
					(SELECT pudq2.daily_limit FROM pinterest_user_daily_quotas pudq2 WHERE pudq2.user_id = u.id AND pudq2.quota_date <= d.d ORDER BY pudq2.quota_date DESC LIMIT 1),
					CASE WHEN d.d = CURRENT_DATE THEN u.daily_limit ELSE 5 END
				) as daily_limit
			FROM dates d
			CROSS JOIN pinterest_users u
			LEFT JOIN pinterest_user_daily_quotas pudq 
				ON pudq.user_id = u.id AND pudq.quota_date = d.d
			WHERE u.id = $1 AND d.d >= u.created_at::date
		)
		SELECT 
			TO_CHAR(ud.audit_date, 'YYYY-MM-DD') as formatted_date,
			ud.user_id,
			ud.user_name,
			ud.daily_limit,
			COALESCE(COUNT(pa.id), 0) as pins_submitted,
			CASE 
				WHEN COALESCE(COUNT(pa.id), 0) >= ud.daily_limit THEN 'Completed'
				WHEN COALESCE(COUNT(pa.id), 0) > 0 THEN 'Partial'
				ELSE 'Missed'
			END as status
		FROM user_dates ud
		LEFT JOIN pinterest_assignments pa 
			ON pa.user_id = ud.user_id 
			AND pa.status = 'submitted' 
			AND pa.submitted_at::date = ud.audit_date
		GROUP BY ud.audit_date, ud.user_id, ud.user_name, ud.daily_limit
		ORDER BY ud.audit_date DESC
	`
	rows, err := r.db.Query(ctx, query, userID, days-1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.PinterestDailyAuditItem
	for rows.Next() {
		var item models.PinterestDailyAuditItem
		if err := rows.Scan(
			&item.Date, &item.UserID, &item.UserName, &item.DailyLimit, &item.PinsSubmitted, &item.Status,
		); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, nil
}

// GetAdminAllSubmissions lists all submissions across all users with job details.
func (r *PinterestRepository) GetAdminAllSubmissions(ctx context.Context, userID int, limit, offset int) ([]models.PinterestAssignment, int, error) {
	var whereSQL string
	var args []interface{}
	if userID > 0 {
		whereSQL = "WHERE pa.status = 'submitted' AND pa.user_id = $1"
		args = append(args, userID)
	} else {
		whereSQL = "WHERE pa.status = 'submitted'"
	}

	countSQL := fmt.Sprintf(`SELECT COUNT(*) FROM pinterest_assignments pa %s`, whereSQL)
	var total int
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	dataSQL := fmt.Sprintf(`
		SELECT 
			pa.id, pa.user_id, pa.job_id, pa.pin_url, pa.status, pa.assigned_at, pa.submitted_at, pa.pin_date, pa.created_at,
			j.title, j.slug, j.location_name, COALESCE(c.name, ''),
			u.name, u.email
		FROM pinterest_assignments pa
		JOIN jobs j ON pa.job_id = j.id
		LEFT JOIN companies c ON j.employer_id = c.id
		JOIN pinterest_users u ON pa.user_id = u.id
		%s
		ORDER BY pa.submitted_at DESC
		LIMIT $%d OFFSET $%d
	`, whereSQL, len(args)+1, len(args)+2)

	args = append(args, limit, offset)
	rows, err := r.db.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []models.PinterestAssignment
	for rows.Next() {
		var a models.PinterestAssignment
		if err := rows.Scan(
			&a.ID, &a.UserID, &a.JobID, &a.PinURL, &a.Status, &a.AssignedAt, &a.SubmittedAt, &a.PinDate, &a.CreatedAt,
			&a.JobTitle, &a.JobSlug, &a.LocationName, &a.EmployerName,
			&a.UserName, &a.UserEmail,
		); err != nil {
			return nil, 0, err
		}
		list = append(list, a)
	}
	return list, total, nil
}

// CheckAndApplyAutoSuspension evaluates 5-consecutive missed days or 5 total missed days in last 30 days.
func (r *PinterestRepository) CheckAndApplyAutoSuspension(ctx context.Context, userID int) (bool, string, error) {
	u, err := r.GetPinterestUserByID(ctx, userID)
	if err != nil {
		return false, "", err
	}

	// If already deactivated, return current deactivation status and reason
	if !u.IsActive {
		reason := u.DeactivationReason
		if reason == "" {
			reason = "Suspended account please contact admin."
		}
		return true, reason, nil
	}

	// Query daily submission count for past completed days (excluding today) up to 30 days back,
	// only considering days on or after account creation date.
	query := `
		WITH dates AS (
			SELECT generate_series(CURRENT_DATE - INTERVAL '30 days', CURRENT_DATE - INTERVAL '1 day', '1 day')::date AS d
		)
		SELECT 
			d.d,
			COALESCE(COUNT(pa.id), 0) as pins_submitted
		FROM dates d
		LEFT JOIN pinterest_assignments pa 
			ON pa.user_id = $1 
			AND pa.status = 'submitted' 
			AND pa.submitted_at::date = d.d
		WHERE d.d >= $2::date
		GROUP BY d.d
		ORDER BY d.d DESC
	`
	rows, err := r.db.Query(ctx, query, userID, u.CreatedAt)
	if err != nil {
		return false, "", err
	}
	defer rows.Close()

	type dayItem struct {
		submitted int
	}
	var pastDays []dayItem
	for rows.Next() {
		var d time.Time
		var sub int
		if err := rows.Scan(&d, &sub); err != nil {
			return false, "", err
		}
		pastDays = append(pastDays, dayItem{submitted: sub})
	}

	// Rule 1: 5 Consecutive Missed Days in most recent past days
	if len(pastDays) >= 5 {
		consecutiveMissed := 0
		for _, day := range pastDays {
			if day.submitted == 0 {
				consecutiveMissed++
			} else {
				break
			}
		}
		if consecutiveMissed >= 5 {
			reason := fmt.Sprintf("Suspended account: %d consecutive missed days (0 pins). Please contact admin.", consecutiveMissed)
			_, _ = r.db.Exec(ctx, `UPDATE pinterest_users SET is_active = false, deactivation_reason = $2, updated_at = NOW() WHERE id = $1`, userID, reason)
			return true, reason, nil
		}
	}

	// Rule 2: 5 or more total missed days in the 30-day window
	if len(pastDays) >= 5 {
		totalMissed := 0
		for _, day := range pastDays {
			if day.submitted == 0 {
				totalMissed++
			}
		}
		if totalMissed >= 5 {
			reason := fmt.Sprintf("Suspended account: %d missed days in the last 30 days. Please contact admin.", totalMissed)
			_, _ = r.db.Exec(ctx, `UPDATE pinterest_users SET is_active = false, deactivation_reason = $2, updated_at = NOW() WHERE id = $1`, userID, reason)
			return true, reason, nil
		}
	}

	return false, "", nil
}

// RunAllUsersAutoSuspension runs the 5-missed-days suspension check across all active posters.
func (r *PinterestRepository) RunAllUsersAutoSuspension(ctx context.Context) error {
	users, err := r.ListPinterestUsers(ctx)
	if err != nil {
		return err
	}
	for _, u := range users {
		if u.IsActive {
			_, _, _ = r.CheckAndApplyAutoSuspension(ctx, u.ID)
		}
	}
	return nil
}

