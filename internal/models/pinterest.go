package models

import (
	"time"
)

// PinterestUser represents an authorized team member/poster for Pinterest.
type PinterestUser struct {
	ID         int       `json:"id" db:"id"`
	Name       string    `json:"name" db:"name"`
	Email      string    `json:"email" db:"email"`
	Phone      string    `json:"phone" db:"phone"`
	AccessCode      string    `json:"access_code" db:"access_code"` // Unique 6-digit numeric login code
	PinterestHandle string    `json:"pinterest_handle" db:"pinterest_handle"` // e.g. @jobyuk
	DailyLimit         int       `json:"daily_limit" db:"daily_limit"` // e.g. 5
	IsActive           bool      `json:"is_active" db:"is_active"`
	DeactivationReason string    `json:"deactivation_reason,omitempty" db:"deactivation_reason"`
	CreatedAt          time.Time `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time `json:"updated_at" db:"updated_at"`

	// Derived / Computed stats
	TodayPinsCount int `json:"today_pins_count,omitempty"`
	TotalPinsCount int `json:"total_pins_count,omitempty"`
}

// PinterestAssignment represents a job assigned exclusively to a Pinterest user.
type PinterestAssignment struct {
	ID          int64      `json:"id" db:"id"`
	UserID      int        `json:"user_id" db:"user_id"`
	JobID       int64      `json:"job_id" db:"job_id"`
	PinURL      string     `json:"pin_url" db:"pin_url"`
	Status      string     `json:"status" db:"status"` // 'assigned', 'submitted'
	AssignedAt  time.Time  `json:"assigned_at" db:"assigned_at"`
	SubmittedAt *time.Time `json:"submitted_at,omitempty" db:"submitted_at"`
	PinDate     time.Time  `json:"pin_date" db:"pin_date"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`

	// Joined job fields
	JobTitle      string `json:"job_title,omitempty"`
	JobSlug       string `json:"job_slug,omitempty"`
	LocationName  string `json:"location_name,omitempty"`
	EmployerName  string `json:"employer_name,omitempty"`
	WorkMode      string `json:"work_mode,omitempty"`
	ContractType  string `json:"contract_type,omitempty"`
	JobType       string `json:"job_type,omitempty"`
	FormattedSalary string `json:"formatted_salary,omitempty"`
	UserName      string `json:"user_name,omitempty"`
	UserEmail     string `json:"user_email,omitempty"`
}

// PinterestUserDailyProgress represents the user's progress for today.
type PinterestUserDailyProgress struct {
	UserID         int    `json:"user_id"`
	UserName       string `json:"user_name"`
	DailyLimit     int    `json:"daily_limit"`
	TodayCompleted int    `json:"today_completed"`
	Remaining      int    `json:"remaining"`
	IsCompleted    bool   `json:"is_completed"`
	CurrentJob     *Job   `json:"current_job,omitempty"`
}

// PinterestDailyAuditItem represents an item in the admin daily audit report.
type PinterestDailyAuditItem struct {
	Date          string `json:"date"`
	UserID        int    `json:"user_id"`
	UserName      string `json:"user_name"`
	DailyLimit    int    `json:"daily_limit"`
	PinsSubmitted int    `json:"pins_submitted"`
	Status        string `json:"status"` // 'Completed', 'Partial', 'Missed'
}

// OptimalPostingSlot represents a recommended posting time slot based on UK peak traffic.
type OptimalPostingSlot struct {
	SlotNumber  int    `json:"slot_number"`
	Title       string `json:"title"`
	UKTime      string `json:"uk_time"`
	PKTime      string `json:"pk_time"`
	Description string `json:"description"`
	IsPrime     bool   `json:"is_prime"`
	Badge       string `json:"badge"`
}
