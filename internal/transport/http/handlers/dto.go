package handlers

import (
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
	usetask "example.com/taskservice/internal/usecase/task"
)

// DTO для части, которую клиент может изменить/передать
type taskMutationDTO struct {
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Status      taskdomain.Status `json:"status"`
	Recurrence  *recurrenceDTO   `json:"recurrence,omitempty"`
}

// DTO для ответа
type taskDTO struct {
	ID          int64            `json:"id"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Status      taskdomain.Status `json:"status"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	Recurrence  *recurrenceDTO   `json:"recurrence,omitempty"`
}

// DTO для Recurrence в JSON
type recurrenceDTO struct {
	Type       taskdomain.RecurrenceType `json:"type"`
	EveryNDays int                       `json:"every_n_days,omitempty"`
	MonthDays  []int                     `json:"month_days,omitempty"`
	Dates      []taskdomain.Date         `json:"dates,omitempty"`
	Parity     taskdomain.DayParity      `json:"parity,omitempty"`
	StartDate  taskdomain.Date           `json:"start_date"`
}

// domain.Task -> taskDTO
func newTaskDTO(task *taskdomain.Task) taskDTO {
	dto := taskDTO{
		ID:          task.ID,
		Title:       task.Title,
		Description: task.Description,
		Status:      task.Status,
		CreatedAt:   task.CreatedAt,
		UpdatedAt:   task.UpdatedAt,
	}

	if task.Recurrence != nil {
		dto.Recurrence = &recurrenceDTO{
			Type:       task.Recurrence.Type,
			EveryNDays: task.Recurrence.EveryNDays,
			MonthDays:  append([]int(nil), task.Recurrence.MonthDays...),
			Dates:      append([]taskdomain.Date(nil), task.Recurrence.Dates...),
			Parity:     task.Recurrence.Parity,
			StartDate:  task.Recurrence.StartDate,
		}
	}

	return dto
}

// taskMutationDTO -> usecase.CreateInput
func (d *taskMutationDTO) toCreateInput() usetask.CreateInput {
	var rec *taskdomain.Recurrence
	if d.Recurrence != nil {
		rec = &taskdomain.Recurrence{
			Type:       d.Recurrence.Type,
			EveryNDays: d.Recurrence.EveryNDays,
			MonthDays:  append([]int(nil), d.Recurrence.MonthDays...),
			Dates:      append([]taskdomain.Date(nil), d.Recurrence.Dates...),
			Parity:     d.Recurrence.Parity,
			StartDate:  d.Recurrence.StartDate,
		}
		rec.Normalize()
	}

	return usetask.CreateInput{
		Title:       d.Title,
		Description: d.Description,
		Status:      d.Status,
		Recurrence:  rec,
	}
}

// taskMutationDTO -> usecase.UpdateInput
func (d *taskMutationDTO) toUpdateInput() usetask.UpdateInput {
	var rec *taskdomain.Recurrence
	if d.Recurrence != nil {
		rec = &taskdomain.Recurrence{
			Type:       d.Recurrence.Type,
			EveryNDays: d.Recurrence.EveryNDays,
			MonthDays:  append([]int(nil), d.Recurrence.MonthDays...),
			Dates:      append([]taskdomain.Date(nil), d.Recurrence.Dates...),
			Parity:     d.Recurrence.Parity,
			StartDate:  d.Recurrence.StartDate,
		}
		rec.Normalize()
	}

	return usetask.UpdateInput{
		Title:       d.Title,
		Description: d.Description,
		Status:      d.Status,
		Recurrence:  rec,
	}
}