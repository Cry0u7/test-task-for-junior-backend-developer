package task

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type Status string

const (
	StatusNew        Status = "new"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
)

type RecurrenceType string

const (
	RecurrenceTypeDaily       RecurrenceType = "daily"
	RecurrenceTypeMonthlyDays RecurrenceType = "monthly_days"
	RecurrenceTypeDates       RecurrenceType = "dates"
	RecurrenceTypeParity      RecurrenceType = "parity"
)

type DayParity string

const (
	DayParityOdd  DayParity = "odd"
	DayParityEven DayParity = "even"
)

type Task struct {
	ID          int64       `json:"id"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Status      Status      `json:"status"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	Recurrence  *Recurrence `json:"recurrence,omitempty"`
}

type Recurrence struct {
	Type RecurrenceType `json:"type"`

	EveryNDays int   `json:"every_n_days,omitempty"`
	MonthDays  []int `json:"month_days,omitempty"`
	Dates      []Date `json:"dates,omitempty"`
	Parity     DayParity `json:"parity,omitempty"`

	StartDate Date `json:"start_date"`
}

type Date struct {
	time.Time
}

func NewDate(t time.Time) Date {
	return Date{Time: normalizeDate(t)}
}

func (d Date) MarshalJSON() ([]byte, error) {
	if d.Time.IsZero() {
		return []byte(`""`), nil
	}
	return []byte(`"` + d.Time.Format("2006-01-02") + `"`), nil
}

func (d *Date) UnmarshalJSON(data []byte) error {
	raw := string(data)
	if raw == `""` || raw == `null` {
		d.Time = time.Time{}
		return nil
	}

	t, err := time.Parse(`"2006-01-02"`, raw)
	if err != nil {
		return fmt.Errorf("invalid date format for recurrence date: expected YYYY-MM-DD")
	}

	d.Time = normalizeDate(t)
	return nil
}

func (s Status) Valid() bool {
	switch s {
	case StatusNew, StatusInProgress, StatusDone:
		return true
	default:
		return false
	}
}

func (t Task) Validate() error {
	if strings.TrimSpace(t.Title) == "" {
		return fmt.Errorf("title is required")
	}

	if !t.Status.Valid() {
		return fmt.Errorf("status is invalid")
	}

	if t.Recurrence != nil {
		if err := t.Recurrence.Validate(); err != nil {
			// без wrap ErrInvalidInput здесь, это делает usecase
			return err
		}
	}

	return nil
}

func (t Task) Valid() bool {
	return t.Validate() == nil
}

func (r Recurrence) Validate() error {
	if r.StartDate.Time.IsZero() {
		return fmt.Errorf("recurrence.start_date is required")
	}

	switch r.Type {
	case RecurrenceTypeDaily:
		if !r.validDaily() {
			return fmt.Errorf("recurrence.every_n_days must be greater than 0 for type 'daily'")
		}
		return nil

	case RecurrenceTypeMonthlyDays:
		if !r.validMonthlyDays() {
			return fmt.Errorf("recurrence.month_days must contain values between 1 and 30 for type 'monthly_days'")
		}
		return nil

	case RecurrenceTypeDates:
		if !r.validDates() {
			return fmt.Errorf("recurrence.dates must contain at least one valid date")
		}
		return nil

	case RecurrenceTypeParity:
		if !r.validParity() {
			return fmt.Errorf("recurrence.parity must be 'odd' or 'even' for type 'parity'")
		}
		return nil

	default:
		return fmt.Errorf("recurrence.type is invalid")
	}
}

func (r Recurrence) Valid() bool {
	return r.Validate() == nil
}

func (r Recurrence) OccursOn(date time.Time) bool {
	if !r.Valid() {
		return false
	}

	target := normalizeDate(date)
	start := normalizeDate(r.StartDate.Time)

	if target.Before(start) {
		return false
	}

	switch r.Type {
	case RecurrenceTypeDaily:
		diffDays := int(target.Sub(start).Hours() / 24)
		return diffDays%r.EveryNDays == 0

	case RecurrenceTypeMonthlyDays:
		day := target.Day()
		for _, d := range r.MonthDays {
			if d == day {
				return true
			}
		}
		return false

	case RecurrenceTypeDates:
		for _, d := range r.Dates {
			if sameDate(d.Time, target) {
				return true
			}
		}
		return false

	case RecurrenceTypeParity:
		day := target.Day()
		if r.Parity == DayParityEven {
			return day%2 == 0
		}
		return day%2 != 0

	default:
		return false
	}
}

func (r *Recurrence) Normalize() {
	if r == nil {
		return
	}

	r.StartDate = NewDate(r.StartDate.Time)

	switch r.Type {
	case RecurrenceTypeMonthlyDays:
		r.MonthDays = uniqueSortedDays(r.MonthDays)
	case RecurrenceTypeDates:
		r.Dates = uniqueSortedDates(r.Dates)
	}
}

func (r Recurrence) validDaily() bool {
	return r.EveryNDays > 0
}

func (r Recurrence) validMonthlyDays() bool {
	if len(r.MonthDays) == 0 {
		return false
	}

	for _, day := range r.MonthDays {
		if day < 1 || day > 30 {
			return false
		}
	}

	return true
}

func (r Recurrence) validDates() bool {
	if len(r.Dates) == 0 {
		return false
	}

	for _, d := range r.Dates {
		if d.Time.IsZero() {
			return false
		}
	}

	return true
}

func (r Recurrence) validParity() bool {
	return r.Parity == DayParityOdd || r.Parity == DayParityEven
}

func normalizeDate(t time.Time) time.Time {
	if t.IsZero() {
		return time.Time{}
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func sameDate(a, b time.Time) bool {
	a = normalizeDate(a)
	b = normalizeDate(b)
	return a.Equal(b)
}

func uniqueSortedDays(days []int) []int {
	seen := make(map[int]struct{}, len(days))
	result := make([]int, 0, len(days))

	for _, day := range days {
		if _, ok := seen[day]; ok {
			continue
		}
		seen[day] = struct{}{}
		result = append(result, day)
	}

	sort.Ints(result)
	return result
}

func uniqueSortedDates(dates []Date) []Date {
	seen := make(map[string]struct{}, len(dates))
	result := make([]Date, 0, len(dates))

	for _, d := range dates {
		n := normalizeDate(d.Time)
		key := n.Format("2006-01-02")

		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, NewDate(n))
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Time.Before(result[j].Time)
	})

	return result
}