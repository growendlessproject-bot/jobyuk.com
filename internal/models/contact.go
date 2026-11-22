package models

import (
	"time"
)

// ContactInquiry represents a message submitted via the public contact form.
type ContactInquiry struct {
	ID          int64     `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Email       string    `json:"email" db:"email"`
	Subject     string    `json:"subject" db:"subject"`
	InquiryType string    `json:"inquiry_type" db:"inquiry_type"` // general, employer, candidate, partnership, technical
	Message     string    `json:"message" db:"message"`
	IPAddress   string    `json:"ip_address" db:"ip_address"`
	UserAgent   string    `json:"user_agent" db:"user_agent"`
	Status      string    `json:"status" db:"status"` // unread, read, resolved, spam
	AdminNotes  string    `json:"admin_notes" db:"admin_notes"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// ContactStats provides inquiry metrics for the admin dashboard.
type ContactStats struct {
	Total    int `json:"total"`
	Unread   int `json:"unread"`
	Resolved int `json:"resolved"`
	Read     int `json:"read"`
}
