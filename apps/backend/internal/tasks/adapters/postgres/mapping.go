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
// any caller needs, read once. PropertyID is nullable since 000120 (ADR 0052
// — the property-less slice).
type taskRuleFields struct {
	ID         uuid.UUID
	OwnerID    uuid.UUID
	PropertyID pgtype.UUID
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
	PropertyID    pgtype.UUID
	RuleID        pgtype.UUID
	DueDate       pgtype.Date
	DueTime       pgtype.Time
	Title         string
	Comment       pgtype.Text
	CompletedDate pgtype.Date
	CreatedAt     pgtype.Timestamptz
	UpdatedAt     pgtype.Timestamptz
	// RuleRepeat is the live rule's repeat read through the LEFT JOIN of the
	// task readers; NULL once the rule is deleted. A read projection for the
	// wire, never persisted on the task.
	RuleRepeat pgtype.Text
}

// mapRuleRow builds the domain rule from a row; an unknown repeat string is a
// data integrity error surfaced by the domain's zero validity (the CHECK
// constraint of migration 000118 is the durable guard).
func mapRuleRow(f taskRuleFields) domain.TaskRule {
	return domain.TaskRule{
		ID:         f.ID,
		OwnerID:    f.OwnerID,
		PropertyID: pgconv.UUIDFromPgtypePtr(f.PropertyID),
		Title:      f.Title,
		Comment:    pgconv.TextToPtrString(f.Comment),
		DueDate:    pgconv.DatePtrFromPgtype(f.DueDate),
		DueTime:    timeOfDayFromPgtype(f.DueTime),
		Repeat:     domain.RepeatKind(f.Repeat),
		CreatedAt:  pgconv.TimestamptzToTime(f.CreatedAt),
		UpdatedAt:  pgconv.TimestamptzToTime(f.UpdatedAt),
	}
}

// mapTaskRow builds the domain task from a row; the joined rule repeat goes
// along as the read projection — any non-null string maps through, the
// CHECK constraint of migration 000118 is the durable guard, same as in
// mapRuleRow.
func mapTaskRow(f taskFields) domain.Task {
	task := domain.Task{
		ID:            f.ID,
		OwnerID:       f.OwnerID,
		PropertyID:    pgconv.UUIDFromPgtypePtr(f.PropertyID),
		RuleID:        pgconv.UUIDFromPgtypePtr(f.RuleID),
		DueDate:       pgconv.DatePtrFromPgtype(f.DueDate),
		DueTime:       timeOfDayFromPgtype(f.DueTime),
		Title:         f.Title,
		Comment:       pgconv.TextToPtrString(f.Comment),
		CompletedDate: pgconv.DatePtrFromPgtype(f.CompletedDate),
		CreatedAt:     pgconv.TimestamptzToTime(f.CreatedAt),
		UpdatedAt:     pgconv.TimestamptzToTime(f.UpdatedAt),
	}
	if f.RuleRepeat.Valid {
		repeat := domain.RepeatKind(f.RuleRepeat.String)
		task.Repeat = &repeat
	}
	return task
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
