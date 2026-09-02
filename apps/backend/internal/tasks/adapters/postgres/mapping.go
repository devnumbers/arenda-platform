// Package postgres holds the tasks persistence adapters: the task rule CRUD
// store with its invalidation queries, the task store (listings, completion
// toggle, journal clear), and the tick store with the owner clock and the
// zone directory (ADR 0051).
package postgres

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/domain"
)

// taskRuleFields is the projection-free row of every rule reader: everything
// any caller needs, read once.
type taskRuleFields struct {
	ID         uuid.UUID
	OwnerID    uuid.UUID
	PropertyID uuid.UUID
	Title      string
	Comment    pgtype.Text
	DueDate    pgtype.Date
	DueTime    pgtype.Time
	Repeat     string
	CreatedAt  pgtype.Timestamptz
	UpdatedAt  pgtype.Timestamptz
}

// taskFields is the same for every task reader.
type taskFields struct {
	ID            uuid.UUID
	OwnerID       uuid.UUID
	PropertyID    uuid.UUID
	RuleID        pgtype.UUID
	DueDate       pgtype.Date
	DueTime       pgtype.Time
	Title         string
	Comment       pgtype.Text
	CompletedDate pgtype.Date
	CreatedAt     pgtype.Timestamptz
	UpdatedAt     pgtype.Timestamptz
}

// mapRuleRow builds the domain rule from a row; an unknown repeat string is a
// data integrity error surfaced by the domain's zero validity (the CHECK
// constraint of migration 000118 is the durable guard).
func mapRuleRow(f taskRuleFields) domain.TaskRule {
	return domain.TaskRule{
		ID:         f.ID,
		OwnerID:    f.OwnerID,
		PropertyID: f.PropertyID,
		Title:      f.Title,
		Comment:    pgconv.TextToPtrString(f.Comment),
		DueDate:    pgconv.DatePtrFromPgtype(f.DueDate),
		DueTime:    timeOfDayFromPgtype(f.DueTime),
		Repeat:     domain.RepeatKind(f.Repeat),
		CreatedAt:  pgconv.TimestamptzToTime(f.CreatedAt),
		UpdatedAt:  pgconv.TimestamptzToTime(f.UpdatedAt),
	}
}

// mapTaskRow builds the domain task from a row.
func mapTaskRow(f taskFields) domain.Task {
	return domain.Task{
		ID:            f.ID,
		OwnerID:       f.OwnerID,
		PropertyID:    f.PropertyID,
		RuleID:        pgconv.UUIDFromPgtypePtr(f.RuleID),
		DueDate:       pgconv.DatePtrFromPgtype(f.DueDate),
		DueTime:       timeOfDayFromPgtype(f.DueTime),
		Title:         f.Title,
		Comment:       pgconv.TextToPtrString(f.Comment),
		CompletedDate: pgconv.DatePtrFromPgtype(f.CompletedDate),
		CreatedAt:     pgconv.TimestamptzToTime(f.CreatedAt),
		UpdatedAt:     pgconv.TimestamptzToTime(f.UpdatedAt),
	}
}

// timeOfDayToPgtype converts the minutes-since-midnight value into the TIME
// column form; nil stays NULL. The whole minute is the contract precision —
// the microseconds are always exact multiples of a minute.
func timeOfDayToPgtype(t *domain.TimeOfDay) pgtype.Time {
	if t == nil {
		return pgtype.Time{}
	}
	return pgtype.Time{Microseconds: int64(*t) * 60 * 1_000_000, Valid: true}
}

// timeOfDayFromPgtype converts a TIME value back; the seconds part (never
// written by this context) is truncated to the minute.
func timeOfDayFromPgtype(t pgtype.Time) *domain.TimeOfDay {
	if !t.Valid {
		return nil
	}
	tod := domain.TimeOfDay(t.Microseconds / (60 * 1_000_000))
	return &tod
}
