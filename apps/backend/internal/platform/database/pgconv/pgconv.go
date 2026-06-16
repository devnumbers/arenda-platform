package pgconv

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// UUIDToPgtype converts a uuid.UUID to pgtype.UUID.
func UUIDToPgtype(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: u, Valid: u != uuid.UUID{}}
}

// UUIDFromPgtype converts a pgtype.UUID to uuid.UUID.
func UUIDFromPgtype(u pgtype.UUID) uuid.UUID {
	if !u.Valid {
		return uuid.UUID{}
	}
	return uuid.UUID(u.Bytes)
}

// UUIDToPgtypePtr converts a *uuid.UUID to pgtype.UUID.
func UUIDToPgtypePtr(u *uuid.UUID) pgtype.UUID {
	if u == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *u, Valid: true}
}

// UUIDFromPgtypePtr converts a pgtype.UUID to *uuid.UUID.
func UUIDFromPgtypePtr(u pgtype.UUID) *uuid.UUID {
	if !u.Valid {
		return nil
	}
	v := uuid.UUID(u.Bytes)
	return &v
}

// TextToString returns the string value of a pgtype.Text, or empty string if invalid.
func TextToString(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

// TextToPtrString converts a pgtype.Text to *string.
// A valid Text (including an empty string) returns a pointer to its value; an invalid Text returns nil.
func TextToPtrString(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}

// StringPtrToPgtype converts a *string to pgtype.Text.
// A nil pointer produces an invalid Text; any non-nil pointer (including empty string) produces a valid Text.
func StringPtrToPgtype(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// DateToPgtype converts a time.Time to pgtype.Date.
func DateToPgtype(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: true}
}

// DatePtrToPgtype converts a *time.Time to pgtype.Date.
func DatePtrToPgtype(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: *t, Valid: true}
}

// DatePtrFromPgtype converts a pgtype.Date to *time.Time.
func DatePtrFromPgtype(d pgtype.Date) *time.Time {
	if !d.Valid {
		return nil
	}
	t := d.Time
	return &t
}

// TimestamptzToTime returns the time.Time value of a pgtype.Timestamptz.
func TimestamptzToTime(t pgtype.Timestamptz) time.Time {
	return t.Time
}

// TimestamptzToPtrTime converts a pgtype.Timestamptz to *time.Time.
func TimestamptzToPtrTime(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	tm := t.Time
	return &tm
}
