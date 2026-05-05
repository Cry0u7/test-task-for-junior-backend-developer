package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	const query = `
		INSERT INTO tasks (
			title,
			description,
			status,
			created_at,
			updated_at,
			recurrence_type,
			recurrence_every_n_days,
			recurrence_month_days,
			recurrence_dates,
			recurrence_parity,
			recurrence_start_date
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING
			id,
			title,
			description,
			status,
			created_at,
			updated_at,
			recurrence_type,
			recurrence_every_n_days,
			recurrence_month_days,
			recurrence_dates,
			recurrence_parity,
			recurrence_start_date
	`

	recurrenceType, everyNDays, monthDays, dates, parity, startDate := recurrenceToDB(task.Recurrence)

	row := r.pool.QueryRow(
		ctx,
		query,
		task.Title,
		task.Description,
		string(task.Status),
		task.CreatedAt,
		task.UpdatedAt,
		recurrenceType,
		everyNDays,
		monthDays,
		dates,
		parity,
		startDate,
	)

	created, err := scanTask(row)
	if err != nil {
		return nil, err
	}

	return created, nil
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	const query = `
		SELECT
			id,
			title,
			description,
			status,
			created_at,
			updated_at,
			recurrence_type,
			recurrence_every_n_days,
			recurrence_month_days,
			recurrence_dates,
			recurrence_parity,
			recurrence_start_date
		FROM tasks
		WHERE id = $1
	`

	row := r.pool.QueryRow(ctx, query, id)
	found, err := scanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}

		return nil, err
	}

	return found, nil
}

func (r *Repository) Update(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	const query = `
		UPDATE tasks
		SET
			title = $1,
			description = $2,
			status = $3,
			updated_at = $4,
			recurrence_type = $5,
			recurrence_every_n_days = $6,
			recurrence_month_days = $7,
			recurrence_dates = $8,
			recurrence_parity = $9,
			recurrence_start_date = $10
		WHERE id = $11
		RETURNING
			id,
			title,
			description,
			status,
			created_at,
			updated_at,
			recurrence_type,
			recurrence_every_n_days,
			recurrence_month_days,
			recurrence_dates,
			recurrence_parity,
			recurrence_start_date
	`

	recurrenceType, everyNDays, monthDays, dates, parity, startDate := recurrenceToDB(task.Recurrence)

	row := r.pool.QueryRow(
		ctx,
		query,
		task.Title,
		task.Description,
		string(task.Status),
		task.UpdatedAt,
		recurrenceType,
		everyNDays,
		monthDays,
		dates,
		parity,
		startDate,
		task.ID,
	)

	updated, err := scanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}

		return nil, err
	}

	return updated, nil
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	const query = `DELETE FROM tasks WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return taskdomain.ErrNotFound
	}

	return nil
}

func (r *Repository) List(ctx context.Context) ([]taskdomain.Task, error) {
	const query = `
		SELECT
			id,
			title,
			description,
			status,
			created_at,
			updated_at,
			recurrence_type,
			recurrence_every_n_days,
			recurrence_month_days,
			recurrence_dates,
			recurrence_parity,
			recurrence_start_date
		FROM tasks
		ORDER BY id DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]taskdomain.Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}

		tasks = append(tasks, *task)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}

type taskScanner interface {
	Scan(dest ...any) error
}

func scanTask(scanner taskScanner) (*taskdomain.Task, error) {
	var (
		task   taskdomain.Task
		status string

		recurrenceType      sql.NullString
		recurrenceEveryNDays sql.NullInt32
		recurrenceMonthDays []int32
		recurrenceDates     []time.Time
		recurrenceParity    sql.NullString
		recurrenceStartDate sql.NullTime
	)

	if err := scanner.Scan(
		&task.ID,
		&task.Title,
		&task.Description,
		&status,
		&task.CreatedAt,
		&task.UpdatedAt,
		&recurrenceType,
		&recurrenceEveryNDays,
		&recurrenceMonthDays,
		&recurrenceDates,
		&recurrenceParity,
		&recurrenceStartDate,
	); err != nil {
		return nil, err
	}

	task.Status = taskdomain.Status(status)
	task.Recurrence = recurrenceFromDB(
		recurrenceType,
		recurrenceEveryNDays,
		recurrenceMonthDays,
		recurrenceDates,
		recurrenceParity,
		recurrenceStartDate,
	)

	return &task, nil
}

func recurrenceToDB(r *taskdomain.Recurrence) (
	recurrenceType any,
	everyNDays any,
	monthDays any,
	dates any,
	parity any,
	startDate any,
) {
	if r == nil {
		return nil, nil, nil, nil, nil, nil
	}

	r.Normalize()

	recurrenceType = string(r.Type)

	if !r.StartDate.Time.IsZero() {
		startDate = r.StartDate.Time
	}

	switch r.Type {
	case taskdomain.RecurrenceTypeDaily:
		if r.EveryNDays > 0 {
			everyNDays = r.EveryNDays
		}

	case taskdomain.RecurrenceTypeMonthlyDays:
		if len(r.MonthDays) > 0 {
			values := make([]int32, 0, len(r.MonthDays))
			for _, d := range r.MonthDays {
				values = append(values, int32(d))
			}
			monthDays = values
		}

	case taskdomain.RecurrenceTypeDates:
		if len(r.Dates) > 0 {
			values := make([]time.Time, 0, len(r.Dates))
			for _, d := range r.Dates {
				values = append(values, d.Time)
			}
			dates = values
		}

	case taskdomain.RecurrenceTypeParity:
		if r.Parity != "" {
			parity = string(r.Parity)
		}
	}

	return recurrenceType, everyNDays, monthDays, dates, parity, startDate
}

func recurrenceFromDB(
	recurrenceType sql.NullString,
	everyNDays sql.NullInt32,
	monthDays []int32,
	dates []time.Time,
	parity sql.NullString,
	startDate sql.NullTime,
) *taskdomain.Recurrence {
	if !recurrenceType.Valid {
		return nil
	}

	r := &taskdomain.Recurrence{
		Type: taskdomain.RecurrenceType(recurrenceType.String),
	}

	if startDate.Valid {
		r.StartDate = taskdomain.NewDate(startDate.Time)
	}

	switch r.Type {
	case taskdomain.RecurrenceTypeDaily:
		if everyNDays.Valid {
			r.EveryNDays = int(everyNDays.Int32)
		}

	case taskdomain.RecurrenceTypeMonthlyDays:
		if len(monthDays) > 0 {
			r.MonthDays = make([]int, 0, len(monthDays))
			for _, d := range monthDays {
				r.MonthDays = append(r.MonthDays, int(d))
			}
		}

	case taskdomain.RecurrenceTypeDates:
		if len(dates) > 0 {
			r.Dates = make([]taskdomain.Date, 0, len(dates))
			for _, d := range dates {
				r.Dates = append(r.Dates, taskdomain.NewDate(d))
			}
		}

	case taskdomain.RecurrenceTypeParity:
		if parity.Valid {
			r.Parity = taskdomain.DayParity(parity.String)
		}
	}

	r.Normalize()
	return r
}