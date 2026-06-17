package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestScheduledAtForDate_UsesFixedMoscowTime(t *testing.T) {
	date := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	got := ScheduledAtForDate(date)
	want := time.Date(2026, 6, 15, 7, 0, 0, 0, time.UTC) // 10:00 MSK = 07:00 UTC
	if !got.Equal(want) {
		t.Fatalf("scheduled_at = %v, want %v", got, want)
	}
}

func TestReminderOffset(t *testing.T) {
	cases := []struct {
		name         string
		operationDate time.Time
		reminderDate  time.Time
		want         int
	}{
		{
			name:         "reminder three days before",
			operationDate: time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
			reminderDate:  time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC),
			want:         3,
		},
		{
			name:         "reminder on event date",
			operationDate: time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
			reminderDate:  time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
			want:         0,
		},
		{
			name:         "reminder after event",
			operationDate: time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
			reminderDate:  time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC),
			want:         -1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ReminderOffset(tc.operationDate, tc.reminderDate)
			if got != tc.want {
				t.Fatalf("offset = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestValidatePendingDate(t *testing.T) {
	cases := []struct {
		name        string
		eventDate   time.Time
		reminderDate time.Time
		now         time.Time
		wantErr     error
	}{
		{
			name:        "valid future reminder",
			eventDate:   time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
			reminderDate: time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC),
			now:         time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
			wantErr:     nil,
		},
		{
			name:        "reminder today is valid",
			eventDate:   time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
			reminderDate: time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
			now:         time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
			wantErr:     nil,
		},
		{
			name:        "past reminder rejected",
			eventDate:   time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
			reminderDate: time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC),
			now:         time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
			wantErr:     ErrInvalidReminderDate,
		},
		{
			name:        "reminder after event rejected",
			eventDate:   time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
			reminderDate: time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC),
			now:         time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
			wantErr:     ErrReminderAfterEvent,
		},
		{
			name:        "zero event date is valid",
			eventDate:   time.Time{},
			reminderDate: time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC),
			now:         time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
			wantErr:     nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePendingDate(tc.eventDate, tc.reminderDate, tc.now)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ValidatePendingDate error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestNewOperationReminder(t *testing.T) {
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	owner := uuid.New()
	op := uuid.New()
	prop := uuid.New()
	eventDate := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	reminderDate := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)

	r, err := NewOperationReminder(owner, op, prop, eventDate, reminderDate, "title", "body", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if r.OwnerID != owner {
		t.Fatalf("owner_id = %v, want %v", r.OwnerID, owner)
	}
	if r.OperationID == nil || *r.OperationID != op {
		t.Fatalf("operation_id mismatch")
	}
	if r.PropertyID == nil || *r.PropertyID != prop {
		t.Fatalf("property_id mismatch")
	}
	if r.TargetType != TargetOperation {
		t.Fatalf("target_type = %v, want %v", r.TargetType, TargetOperation)
	}
	if r.EventType != EventOperationDue {
		t.Fatalf("event_type = %v, want %v", r.EventType, EventOperationDue)
	}
	if r.Status != ReminderPending {
		t.Fatalf("status = %v, want %v", r.Status, ReminderPending)
	}
	wantScheduled := time.Date(2026, 6, 12, 7, 0, 0, 0, time.UTC)
	if !r.ScheduledAt.Equal(wantScheduled) {
		t.Fatalf("scheduled_at = %v, want %v", r.ScheduledAt, wantScheduled)
	}
	if r.MessageTitle != "title" {
		t.Fatalf("message_title = %v, want title", r.MessageTitle)
	}
	if r.MessageBody != "body" {
		t.Fatalf("message_body = %v, want body", r.MessageBody)
	}
	if !r.CreatedAt.Equal(now) {
		t.Fatalf("created_at = %v, want %v", r.CreatedAt, now)
	}
}

func TestNewOperationReminder_RejectReminderInThePast(t *testing.T) {
	now := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	eventDate := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	reminderDate := time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)
	owner := uuid.New()
	op := uuid.New()
	prop := uuid.New()
	_, err := NewOperationReminder(owner, op, prop, eventDate, reminderDate, "title", "body", now)
	if err == nil {
		t.Fatal("expected error for past reminder date")
	}
}
