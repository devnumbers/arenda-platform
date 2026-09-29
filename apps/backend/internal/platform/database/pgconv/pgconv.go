// Package pgconv converts between Go domain values (UUIDs, timestamps) and pgx wire types,
// and shapes SQL-facing strings (ILIKE-pattern escaping via EscapeLikePattern).
package pgconv

import (
	"math"
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

// UUIDSliceToPgtype converts a slice of uuid.UUID to a slice of pgtype.UUID.
func UUIDSliceToPgtype(ids []uuid.UUID) []pgtype.UUID {
	out := make([]pgtype.UUID, len(ids))
	for i, id := range ids {
		out[i] = UUIDToPgtype(id)
	}
	return out
}

// UUIDSliceFromPgtype converts a slice of pgtype.UUID (a uuid[] column scan)
// back to uuid.UUID values; nil stays nil.
func UUIDSliceFromPgtype(ids []pgtype.UUID) []uuid.UUID {
	if ids == nil {
		return nil
	}
	out := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		out[i] = UUIDFromPgtype(id)
	}
	return out
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
	return new(uuid.UUID(u.Bytes))
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
	return new(t.String)
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
	return new(d.Time)
}

// DateFromPgtype converts a pgtype.Date to time.Time.
// An invalid Date returns the zero time.
func DateFromPgtype(d pgtype.Date) time.Time {
	if !d.Valid {
		return time.Time{}
	}
	return d.Time
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
	return new(t.Time)
}

// TimePtrToPgtype converts a *time.Time to pgtype.Timestamptz.
func TimePtrToPgtype(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{Valid: false}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// Int8PtrToPgtype converts a *int64 to pgtype.Int8.
// A nil pointer produces an invalid Int8; any non-nil pointer produces a valid Int8 holding its value.
func Int8PtrToPgtype(n *int64) pgtype.Int8 {
	if n == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *n, Valid: true}
}

// Int8ToPtr converts a pgtype.Int8 to *int64.
// An invalid Int8 returns nil; a valid Int8 returns a pointer to its value.
func Int8ToPtr(n pgtype.Int8) *int64 {
	if !n.Valid {
		return nil
	}
	return new(n.Int64)
}

// Int4PtrToPgtype converts a *int to pgtype.Int4.
// A nil pointer produces an invalid Int4; any non-nil pointer produces a valid Int4 holding its value.
// A value outside the int32 range produces an invalid Int4 — a lossy write must not look like a NULL-free success.
func Int4PtrToPgtype(n *int) pgtype.Int4 {
	if n == nil || *n < math.MinInt32 || *n > math.MaxInt32 {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*n), Valid: true}
}

// Int4ToPtr converts a pgtype.Int4 to *int.
// An invalid Int4 returns nil; a valid Int4 returns a pointer to its value.
func Int4ToPtr(n pgtype.Int4) *int {
	if !n.Valid {
		return nil
	}
	return new(int(n.Int32))
}
