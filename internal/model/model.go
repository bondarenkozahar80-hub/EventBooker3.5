package model

import "time"

type Event struct {
	ID                    int       `db:"id" json:"id"`
	Name                  string    `db:"name" json:"name"`
	Description           string    `db:"description,omitempty" json:"description,omitempty"`
	StartTime             time.Time `db:"start_time" json:"start_time"`
	EndTime               time.Time `db:"end_time,omitempty" json:"end_time,omitempty"`
	Location              string    `db:"location,omitempty" json:"location,omitempty"`
	Capacity              int       `db:"capacity" json:"capacity"`
	PaymentTimeoutMinutes int       `db:"payment_timeout_minutes" json:"payment_timeout_minutes"`
	CreatedAt             time.Time `db:"created_at" json:"created_at"`
	UpdatedAt             time.Time `db:"updated_at" json:"updated_at"`
}

type Registration struct {
	ID        int       `db:"id" json:"id"`
	EventID   int       `db:"event_id" json:"event_id"`
	FullName  string    `db:"full_name" json:"full_name"`
	Email     string    `db:"email,omitempty" json:"email,omitempty"`
	Phone     string    `db:"phone,omitempty" json:"phone,omitempty"`
	Status    string    `db:"status" json:"status"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}
