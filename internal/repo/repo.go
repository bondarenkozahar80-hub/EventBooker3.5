package repo

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/wb-go/wbf/dbpg"
	"github.com/wb-go/wbf/ginext"

	"fifthOne/internal/model"
)

var (
	ErrEventNotFound         = errors.New("event not found")
	ErrEventFull             = errors.New("event is full")
	ErrDuplicateRegistration = errors.New("duplicate registration")
)

type Repository interface {
	CreateEvent(ctx *ginext.Context, e *model.Event) (int64, error)
	GetEventByID(ctx context.Context, id int64) (*model.Event, error)
	GetAllEvents(ctx context.Context) ([]model.Event, error)
	BookRegistrationTx(ctx context.Context, reg *model.Registration) (int64, int, error)
	GetRegistrationByID(ctx context.Context, id int64) (*model.Registration, error)
	UpdateRegistrationStatusTx(ctx context.Context, registrationID int64, newStatus string) error
	GetRegistrationByRegID(ctx context.Context, regID int64) (*model.Registration, error)
	CountRegistrations(ctx context.Context, eventID int64) (int, error)
	GetRegistrationsByEventID(ctx context.Context, eventID int64) ([]model.Registration, error)
	CancelIfNotConfirmedTx(ctx context.Context, registrationID int64) (bool, error)
	MigrateUp(migrationsDir string) error
	MigrateDown(migrationsDir string) error
}

type repository struct {
	db  *dbpg.DB
	log *zerolog.Logger
}

func NewRepository(db *dbpg.DB, log *zerolog.Logger) (Repository, error) {
	if db == nil {
		return nil, fmt.Errorf("db cannot be nil")
	}
	if err := db.Master.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping DB: %w", err)
	}
	return &repository{db: db, log: log}, nil
}

func (r *repository) MigrateUp(migrationsDir string) error {
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		return fmt.Errorf("failed to read migration files: %w", err)
	}

	for _, file := range files {
		sqlBytes, err := ioutil.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", file, err)
		}

		if _, err := r.db.ExecContext(context.Background(), string(sqlBytes)); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", file, err)
		}
	}

	r.log.Info().Msgf("Migrations applied successfully from %s", migrationsDir)
	return nil
}

func (r *repository) MigrateDown(migrationsDir string) error {
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.down.sql"))
	if err != nil {
		return fmt.Errorf("failed to read rollback files: %w", err)
	}

	for _, file := range files {
		sqlBytes, err := ioutil.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read rollback file %s: %w", file, err)
		}

		if _, err := r.db.ExecContext(context.Background(), string(sqlBytes)); err != nil {
			return fmt.Errorf("failed to rollback migration %s: %w", file, err)
		}
	}

	r.log.Info().Msgf("Migrations rolled back successfully from %s", migrationsDir)
	return nil
}

func (r *repository) CreateEvent(ctx *ginext.Context, e *model.Event) (int64, error) {
	query := `
		INSERT INTO events (name, description, start_time, end_time, location, capacity, payment_timeout_minutes)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`

	row := r.db.QueryRowContext(ctx, query,
		e.Name, e.Description, e.StartTime, e.EndTime, e.Location, e.Capacity, e.PaymentTimeoutMinutes,
	)

	var id int64
	if err := row.Scan(&id); err != nil {
		ctx.Error(fmt.Errorf("failed to insert event: %w", err))
		return 0, err
	}
	return id, nil
}

func (r *repository) GetEventByID(ctx context.Context, id int64) (*model.Event, error) {
	query := `
		SELECT id, name, description, start_time, end_time, location,
		       capacity, payment_timeout_minutes, created_at, updated_at
		FROM events WHERE id = $1
	`
	row := r.db.QueryRowContext(ctx, query, id)

	var e model.Event
	if err := row.Scan(
		&e.ID, &e.Name, &e.Description, &e.StartTime, &e.EndTime, &e.Location,
		&e.Capacity, &e.PaymentTimeoutMinutes, &e.CreatedAt, &e.UpdatedAt,
	); err != nil {
		return nil, ErrEventNotFound
	}
	return &e, nil
}

func (r *repository) GetRegistrationByID(ctx context.Context, id int64) (*model.Registration, error) {
	query := `
		SELECT id, event_id, full_name, email, phone, status, created_at, updated_at
		FROM registrations
		WHERE id = $1
	`
	row := r.db.QueryRowContext(ctx, query, id)

	var reg model.Registration
	if err := row.Scan(
		&reg.ID,
		&reg.EventID,
		&reg.FullName,
		&reg.Email,
		&reg.Phone,
		&reg.Status,
		&reg.CreatedAt,
		&reg.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("registration not found: %w", err)
	}

	return &reg, nil
}

func (r *repository) UpdateRegistrationStatusTx(ctx context.Context, registrationID int64, newStatus string) error {
	tx, err := r.db.Master.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	query := `
		UPDATE registrations
		SET status = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING id
	`

	var id int64
	if err := tx.QueryRowContext(ctx, query, newStatus, registrationID).Scan(&id); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to update registration status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *repository) BookRegistrationTx(ctx context.Context, reg *model.Registration) (int64, int, error) {
	tx, err := r.db.Master.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to start transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	var event model.Event
	err = tx.QueryRowContext(ctx, `
		SELECT id, name, capacity, payment_timeout_minutes
		FROM events
		WHERE id = $1
		FOR UPDATE
	`, reg.EventID).Scan(&event.ID, &event.Name, &event.Capacity, &event.PaymentTimeoutMinutes)
	if err != nil {
		_ = tx.Rollback()
		return 0, 0, ErrEventNotFound
	}
	paymentTimeout := event.PaymentTimeoutMinutes

	var count int
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM registrations
		WHERE event_id = $1 AND status IN ('pending', 'confirmed')
	`, reg.EventID).Scan(&count)
	if err != nil {
		_ = tx.Rollback()
		return 0, 0, fmt.Errorf("failed to count registrations: %w", err)
	}

	if count >= event.Capacity {
		_ = tx.Rollback()
		return 0, 0, ErrEventFull
	}

	var existing int
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM registrations
		WHERE event_id = $1 AND email = $2 AND status != 'canceled'
	`, reg.EventID, reg.Email).Scan(&existing)
	if err != nil {
		_ = tx.Rollback()
		return 0, 0, fmt.Errorf("failed to check duplicate registration: %w", err)
	}
	if existing > 0 {
		_ = tx.Rollback()
		return 0, 0, ErrDuplicateRegistration
	}

	var id int64
	reg.Status = "pending"
	err = tx.QueryRowContext(ctx, `
		INSERT INTO registrations (event_id, full_name, email, phone, status, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		RETURNING id
	`, reg.EventID, reg.FullName, reg.Email, reg.Phone, reg.Status).Scan(&id)
	if err != nil {
		_ = tx.Rollback()
		return 0, 0, fmt.Errorf("failed to create registration: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return id, paymentTimeout, nil
}

func (r *repository) GetRegistrationByRegID(ctx context.Context, regID int64) (*model.Registration, error) {
	query := `
		SELECT id, event_id, full_name, email, phone, status, created_at, updated_at
		FROM registrations
		WHERE id = $1 
	`
	row := r.db.QueryRowContext(ctx, query, regID)

	var reg model.Registration
	if err := row.Scan(
		&reg.ID,
		&reg.EventID,
		&reg.FullName,
		&reg.Email,
		&reg.Phone,
		&reg.Status,
		&reg.CreatedAt,
		&reg.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("registration not found: %w", err)
	}

	return &reg, nil
}

func (r *repository) GetAllEvents(ctx context.Context) ([]model.Event, error) {
	query := `
		SELECT id, name, description, start_time, end_time, location,
		       capacity, payment_timeout_minutes, created_at, updated_at
		FROM events
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get events: %w", err)
	}
	defer rows.Close()

	var events []model.Event
	for rows.Next() {
		var e model.Event
		if err := rows.Scan(
			&e.ID,
			&e.Name,
			&e.Description,
			&e.StartTime,
			&e.EndTime,
			&e.Location,
			&e.Capacity,
			&e.PaymentTimeoutMinutes,
			&e.CreatedAt,
			&e.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}
		events = append(events, e)
	}

	return events, nil
}

func (r *repository) CountRegistrations(ctx context.Context, eventID int64) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM registrations
		WHERE event_id = $1 AND status != 'canceled'
	`

	var count int
	if err := r.db.QueryRowContext(ctx, query, eventID).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count registrations: %w", err)
	}

	return count, nil
}

func (r *repository) GetRegistrationsByEventID(ctx context.Context, eventID int64) ([]model.Registration, error) {
	query := `
		SELECT id, event_id, full_name, email, phone, status, created_at, updated_at
		FROM registrations
		WHERE event_id = $1 AND status != 'canceled'
		ORDER BY created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to get registrations: %w", err)
	}
	defer rows.Close()

	var regs []model.Registration
	for rows.Next() {
		var reg model.Registration
		if err := rows.Scan(
			&reg.ID,
			&reg.EventID,
			&reg.FullName,
			&reg.Email,
			&reg.Phone,
			&reg.Status,
			&reg.CreatedAt,
			&reg.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan registration: %w", err)
		}
		regs = append(regs, reg)
	}

	return regs, nil
}

func (r *repository) CancelIfNotConfirmedTx(ctx context.Context, registrationID int64) (bool, error) {
	tx, err := r.db.Master.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("failed to start transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	var currentStatus string
	querySelect := `
		SELECT status
		FROM registrations
		WHERE id = $1
		FOR UPDATE
	`
	err = tx.QueryRowContext(ctx, querySelect, registrationID).Scan(&currentStatus)
	if err != nil {
		_ = tx.Rollback()
		return false, fmt.Errorf("failed to select registration for cancellation: %w", err)
	}

	if currentStatus == "confirmed" || currentStatus == "canceled" {
		_ = tx.Rollback()
		return false, nil
	}

	queryUpdate := `
		UPDATE registrations
		SET status = 'canceled', updated_at = NOW()
		WHERE id = $1
	`
	if _, err := tx.ExecContext(ctx, queryUpdate, registrationID); err != nil {
		_ = tx.Rollback()
		return false, fmt.Errorf("failed to update registration status to canceled: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("failed to commit cancellation transaction: %w", err)
	}

	return true, nil
}
