# Reminders Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a channel-agnostic reminder subsystem that creates, schedules, dispatches, and cancels reminders for future operations and lease events in the Arenda Platform backend.

**Architecture:** Add a new `internal/notifications` bounded context with domain, application ports/service, and Postgres/SMS adapters. Existing `leases` services create/cancel reminders transactionally through a `ReminderScheduler` port. A background worker polls due reminders and dispatches them through a `Notifier` port, whose SMS implementation reuses the existing `identity.Sender`.

**Tech Stack:** Go 1.26, PostgreSQL 16, pgx/v5, sqlc, oapi-codegen, Chi, OpenTelemetry, slog.

**Required skills:** @go, @postgresql-best-practices.

---

## Task 1: Create notifications migration

**Files:**
- Create: `apps/backend/db/migrations/000008_notifications.up.sql`
- Create: `apps/backend/db/migrations/000008_notifications.down.sql`

**Step 1: Write the migration**

`apps/backend/db/migrations/000008_notifications.up.sql`:

```sql
CREATE TYPE notification_target_type AS ENUM ('operation', 'recurring_operation', 'lease');
CREATE TYPE notification_event_type AS ENUM ('operation_due', 'lease_expiring', 'lease_requires_action');
CREATE TYPE notification_status AS ENUM ('pending', 'sent', 'failed', 'cancelled');

CREATE TABLE reminders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_type notification_target_type NOT NULL,
    operation_id UUID REFERENCES operations(id) ON DELETE CASCADE,
    recurring_operation_id UUID REFERENCES recurring_operations(id) ON DELETE CASCADE,
    lease_id UUID REFERENCES leases(id) ON DELETE CASCADE,
    property_id UUID REFERENCES properties(id) ON DELETE CASCADE,
    event_type notification_event_type NOT NULL,
    status notification_status NOT NULL DEFAULT 'pending',
    scheduled_at TIMESTAMPTZ NOT NULL,
    sent_at TIMESTAMPTZ,
    failed_attempts INT NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ,
    message_title TEXT NOT NULL,
    message_body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT exactly_one_target CHECK (
        (target_type = 'operation' AND operation_id IS NOT NULL AND recurring_operation_id IS NULL AND lease_id IS NULL) OR
        (target_type = 'recurring_operation' AND recurring_operation_id IS NOT NULL AND operation_id IS NULL AND lease_id IS NULL) OR
        (target_type = 'lease' AND lease_id IS NOT NULL AND operation_id IS NULL AND recurring_operation_id IS NULL)
    ),
    CONSTRAINT one_reminder_per_target_event UNIQUE (owner_id, target_type, operation_id, recurring_operation_id, lease_id, event_type)
);

CREATE INDEX idx_reminders_due ON reminders (status, scheduled_at, next_attempt_at) WHERE status = 'pending';
CREATE INDEX idx_reminders_owner ON reminders (owner_id, status, scheduled_at);
CREATE INDEX idx_reminders_operation ON reminders (operation_id);
CREATE INDEX idx_reminders_recurring ON reminders (recurring_operation_id);
CREATE INDEX idx_reminders_lease ON reminders (lease_id);

CREATE TABLE sent_sms_reminders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reminder_id UUID REFERENCES reminders(id) ON DELETE SET NULL,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    phone TEXT NOT NULL,
    message TEXT NOT NULL,
    provider_response TEXT,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

`apps/backend/db/migrations/000008_notifications.down.sql`:

```sql
DROP TABLE IF EXISTS sent_sms_reminders;
DROP TABLE IF EXISTS reminders;
DROP TYPE IF EXISTS notification_status;
DROP TYPE IF EXISTS notification_event_type;
DROP TYPE IF EXISTS notification_target_type;
```

**Step 2: Commit**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add apps/backend/db/migrations/000008_notifications.*.sql
git commit -m "feat(notifications): add reminders and sent_sms_reminders migrations"
```

---

## Task 2: Add sqlc queries for reminders

**Files:**
- Create: `apps/backend/db/queries/notifications.sql`

**Step 1: Write queries**

```sql
-- name: CreateReminder :one
INSERT INTO reminders (
    id, owner_id, target_type, operation_id, recurring_operation_id, lease_id, property_id,
    event_type, status, scheduled_at, message_title, message_body, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
)
RETURNING *;

-- name: GetReminderByID :one
SELECT * FROM reminders WHERE id = $1;

-- name: ListRemindersByOwner :many
SELECT * FROM reminders
WHERE owner_id = $1
  AND ($2::notification_status IS NULL OR status = $2)
ORDER BY scheduled_at ASC
LIMIT $3 OFFSET $4;

-- name: ListDueReminders :many
SELECT * FROM reminders
WHERE status = 'pending'
  AND scheduled_at <= $1
  AND (next_attempt_at IS NULL OR next_attempt_at <= $1)
ORDER BY scheduled_at ASC
LIMIT $2;

-- name: MarkReminderSent :execrows
UPDATE reminders
SET status = 'sent', sent_at = $2, updated_at = NOW()
WHERE id = $1 AND status = 'pending';

-- name: MarkReminderFailed :execrows
UPDATE reminders
SET failed_attempts = failed_attempts + 1,
    next_attempt_at = $2,
    status = CASE WHEN $3 THEN 'failed' ELSE status END,
    updated_at = NOW()
WHERE id = $1 AND status = 'pending';

-- name: CancelReminderByTarget :execrows
UPDATE reminders
SET status = 'cancelled', updated_at = NOW()
WHERE owner_id = $1
  AND target_type = $2
  AND (
      (target_type = 'operation' AND operation_id = $3) OR
      (target_type = 'recurring_operation' AND recurring_operation_id = $3) OR
      (target_type = 'lease' AND lease_id = $3)
  )
  AND event_type = $4
  AND status = 'pending';

-- name: CancelRemindersByOperationIDs :execrows
UPDATE reminders
SET status = 'cancelled', updated_at = NOW()
WHERE owner_id = $1
  AND operation_id = ANY($2::uuid[])
  AND status = 'pending';

-- name: CreateSentSMSReminder :one
INSERT INTO sent_sms_reminders (id, reminder_id, owner_id, phone, message, provider_response, sent_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetUserPhoneByID :one
SELECT phone FROM users WHERE id = $1;
```

**Step 2: Regenerate sqlc**

Run:

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/backend
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
```

Expected: `internal/generated/postgres/` updated with new queries.

**Step 3: Commit**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add apps/backend/db/queries/notifications.sql apps/backend/internal/generated/postgres
git commit -m "feat(notifications): add sqlc queries for reminders"
```

---

## Task 3: Create notifications domain types

**Files:**
- Create: `apps/backend/internal/notifications/domain/reminder.go`

**Step 1: Write the domain file**

```go
package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type TargetType string

const (
	TargetOperation          TargetType = "operation"
	TargetRecurringOperation TargetType = "recurring_operation"
	TargetLease              TargetType = "lease"
)

type EventType string

const (
	EventOperationDue        EventType = "operation_due"
	EventLeaseExpiring       EventType = "lease_expiring"
	EventLeaseRequiresAction EventType = "lease_requires_action"
)

type ReminderStatus string

const (
	ReminderPending   ReminderStatus = "pending"
	ReminderSent      ReminderStatus = "sent"
	ReminderFailed    ReminderStatus = "failed"
	ReminderCancelled ReminderStatus = "cancelled"
)

const fixedDispatchHour = 10
const fixedDispatchTZ   = "Europe/Moscow"

var ErrInvalidReminderDate = errors.New("reminder date must be today or in the future")
var ErrReminderAfterEvent  = errors.New("reminder date must be on or before the event date")

// Reminder is a concrete scheduled notification.
type Reminder struct {
	ID                   uuid.UUID
	OwnerID              uuid.UUID
	TargetType           TargetType
	OperationID          *uuid.UUID
	RecurringOperationID *uuid.UUID
	LeaseID              *uuid.UUID
	PropertyID           *uuid.UUID
	EventType            EventType
	Status               ReminderStatus
	ScheduledAt          time.Time
	SentAt               *time.Time
	FailedAttempts       int
	NextAttemptAt        *time.Time
	MessageTitle         string
	MessageBody          string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// ScheduledAtForDate combines a user-selected date with the fixed dispatch time.
func ScheduledAtForDate(date time.Time) time.Time {
	loc, _ := time.LoadLocation(fixedDispatchTZ)
	if loc == nil {
		loc = time.UTC
	}
	d := time.Date(date.Year(), date.Month(), date.Day(), fixedDispatchHour, 0, 0, 0, loc)
	return d.UTC()
}

// ReminderOffset returns the number of days between an operation date and a reminder date.
func ReminderOffset(operationDate, reminderDate time.Time) int {
	op := operationDate.UTC().Truncate(24 * time.Hour)
	rm := reminderDate.UTC().Truncate(24 * time.Hour)
	return int(op.Sub(rm).Hours() / 24)
}

// ValidatePendingDate checks that a reminder date is valid relative to an event date.
func ValidatePendingDate(eventDate, reminderDate time.Time, now time.Time) error {
	rm := reminderDate.UTC().Truncate(24 * time.Hour)
	today := now.UTC().Truncate(24 * time.Hour)
	if rm.Before(today) {
		return fmt.Errorf("%w: %s", ErrInvalidReminderDate, reminderDate)
	}
	if eventDate.IsZero() {
		return nil
	}
	ev := eventDate.UTC().Truncate(24 * time.Hour)
	if rm.After(ev) {
		return fmt.Errorf("%w: reminder %s after event %s", ErrReminderAfterEvent, reminderDate, eventDate)
	}
	return nil
}

// NewOperationReminder creates a pending reminder for a future operation.
func NewOperationReminder(ownerID, operationID, propertyID uuid.UUID, eventDate, reminderDate time.Time, title, body string, now time.Time) (Reminder, error) {
	if err := ValidatePendingDate(eventDate, reminderDate, now); err != nil {
		return Reminder{}, err
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return Reminder{}, fmt.Errorf("generate reminder id: %w", err)
	}
	return Reminder{
		ID:          id,
		OwnerID:     ownerID,
		TargetType:  TargetOperation,
		OperationID: &operationID,
		PropertyID:  &propertyID,
		EventType:   EventOperationDue,
		Status:      ReminderPending,
		ScheduledAt: ScheduledAtForDate(reminderDate),
		MessageTitle: title,
		MessageBody: body,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}
```

**Step 2: Write the failing test**

`apps/backend/internal/notifications/domain/reminder_test.go`:

```go
package domain

import (
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
```

**Step 3: Run tests to verify failure**

Run:

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/backend
go test ./internal/notifications/domain/... -v
```

Expected: FAIL because package not yet compiled (domain file exists, tests should pass after writing).

**Step 4: Run tests to verify pass**

After writing code, run the same command. Expected: PASS.

**Step 5: Commit**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add apps/backend/internal/notifications/domain
git commit -m "feat(notifications): add reminder domain model and rules"
```

---

## Task 4: Create notifications application ports

**Files:**
- Create: `apps/backend/internal/notifications/application/ports.go`

**Step 1: Write ports**

```go
package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

type ReminderRepository interface {
	Save(ctx context.Context, r domain.Reminder) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.Reminder, error)
	ListByOwner(ctx context.Context, ownerID uuid.UUID, filter ListFilter) ([]domain.Reminder, error)
	ListDue(ctx context.Context, before time.Time, limit int) ([]domain.Reminder, error)
	MarkSent(ctx context.Context, id uuid.UUID, at time.Time) error
	MarkFailed(ctx context.Context, id uuid.UUID, nextAttempt *time.Time, terminal bool) error
	CancelByTarget(ctx context.Context, ownerID uuid.UUID, targetType domain.TargetType, targetID uuid.UUID, eventType domain.EventType) error
	CancelByOperationIDs(ctx context.Context, ownerID uuid.UUID, operationIDs []uuid.UUID) error
	WithTx(tx transaction.Tx) ReminderRepository
}

type ListFilter struct {
	Status *domain.ReminderStatus
	Limit  int
	Offset int
}

type Notifier interface {
	Notify(ctx context.Context, n Notification) error
}

type ContactResolver interface {
	Resolve(ctx context.Context, ownerID uuid.UUID) (Contact, error)
}

type Notification struct {
	RecipientID uuid.UUID
	EventType   domain.EventType
	Title       string
	Body        string
}

type Contact struct {
	Channel Channel
	Address string
}

type Channel string

const (
	ChannelSMS Channel = "sms"
)

type ReminderScheduler interface {
	ScheduleForOperation(ctx context.Context, op OperationInfo, reminderDate time.Time) error
	ScheduleForRecurringOperation(ctx context.Context, rec RecurringOperationInfo, baseReminderDate time.Time, ops []OperationInfo) error
	ScheduleForLease(ctx context.Context, lease LeaseInfo) error
	CancelByOperation(ctx context.Context, ownerID, opID uuid.UUID) error
	CancelByRecurringOperation(ctx context.Context, ownerID, recID uuid.UUID) error
	CancelByLease(ctx context.Context, ownerID, leaseID uuid.UUID) error
	WithTx(tx transaction.Tx) ReminderScheduler
}

type OperationInfo struct {
	ID            uuid.UUID
	OwnerID       uuid.UUID
	PropertyID    uuid.UUID
	LeaseID       *uuid.UUID
	RecurringOperationID *uuid.UUID
	OperationDate time.Time
	Type          string
	Category      string
	AmountKopecks int64
}

type RecurringOperationInfo struct {
	ID            uuid.UUID
	OwnerID       uuid.UUID
	PropertyID    uuid.UUID
	LeaseID       *uuid.UUID
}

type LeaseInfo struct {
	ID       uuid.UUID
	OwnerID  uuid.UUID
	PropertyID uuid.UUID
	EndDate  *time.Time
}
```

**Step 2: Commit**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add apps/backend/internal/notifications/application/ports.go
git commit -m "feat(notifications): add application ports"
```

---

## Task 5: Create notification application service

**Files:**
- Create: `apps/backend/internal/notifications/application/service.go`

**Step 1: Write the service**

```go
package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

const leaseExpiringOffsetDays = 30

type ReminderService struct {
	repo     ReminderRepository
	clock    clock.Clock
}

func NewReminderService(repo ReminderRepository, clock clock.Clock) *ReminderService {
	return &ReminderService{repo: repo, clock: clock}
}

func (s *ReminderService) CreateForOperation(ctx context.Context, op OperationInfo, reminderDate time.Time) (domain.Reminder, error) {
	title := fmt.Sprintf("Напоминание об операции")
	body := fmt.Sprintf("%s %d коп. запланировано на %s", op.Category, op.AmountKopecks, op.OperationDate.Format("02.01.2006"))
	r, err := domain.NewOperationReminder(op.OwnerID, op.ID, op.PropertyID, op.OperationDate, reminderDate, title, body, s.clock.Now())
	if err != nil {
		return domain.Reminder{}, err
	}
	if err := s.repo.Save(ctx, r); err != nil {
		return domain.Reminder{}, fmt.Errorf("save reminder: %w", err)
	}
	return r, nil
}

func (s *ReminderService) CreateForRecurringOperation(ctx context.Context, rec RecurringOperationInfo, baseReminderDate time.Time, ops []OperationInfo) error {
	if len(ops) == 0 {
		return nil
	}
	nextOp := ops[0]
	for _, op := range ops {
		if op.OperationDate.Before(nextOp.OperationDate) {
			nextOp = op
		}
	}
	offset := domain.ReminderOffset(nextOp.OperationDate, baseReminderDate)
	if offset < 0 {
		offset = 0
	}
	for _, op := range ops {
		reminderDate := op.OperationDate.AddDate(0, 0, -offset)
		if _, err := s.CreateForOperation(ctx, op, reminderDate); err != nil {
			return fmt.Errorf("create reminder for operation %s: %w", op.ID, err)
		}
	}
	return nil
}

func (s *ReminderService) CreateForLease(ctx context.Context, lease LeaseInfo) error {
	if lease.EndDate == nil || lease.EndDate.IsZero() {
		return nil
	}
	ownerID := lease.OwnerID
	propertyID := lease.PropertyID

	expiringTitle := "Аренда скоро заканчивается"
	expiringBody := fmt.Sprintf("Аренда по объекту заканчивается %s", lease.EndDate.Format("02.01.2006"))
	expiringDate := lease.EndDate.AddDate(0, 0, -leaseExpiringOffsetDays)
	expiring, err := domain.NewOperationReminder(ownerID, lease.ID, propertyID, *lease.EndDate, expiringDate, expiringTitle, expiringBody, s.clock.Now())
	if err != nil {
		return fmt.Errorf("create lease expiring reminder: %w", err)
	}
	expiring.TargetType = domain.TargetLease
	expiring.OperationID = nil
	expiring.LeaseID = &lease.ID
	expiring.EventType = domain.EventLeaseExpiring
	if err := s.repo.Save(ctx, expiring); err != nil {
		return fmt.Errorf("save lease expiring reminder: %w", err)
	}

	requiresActionTitle := "Аренда требует действия"
	requiresActionBody := "Срок аренды закончился. Подтвердите продление или завершение аренды."
	requiresActionDate := lease.EndDate.AddDate(0, 0, 1)
	requiresAction, err := domain.NewOperationReminder(ownerID, lease.ID, propertyID, *lease.EndDate, requiresActionDate, requiresActionTitle, requiresActionBody, s.clock.Now())
	if err != nil {
		return fmt.Errorf("create lease requires_action reminder: %w", err)
	}
	requiresAction.TargetType = domain.TargetLease
	requiresAction.OperationID = nil
	requiresAction.LeaseID = &lease.ID
	requiresAction.EventType = domain.EventLeaseRequiresAction
	if err := s.repo.Save(ctx, requiresAction); err != nil {
		return fmt.Errorf("save lease requires_action reminder: %w", err)
	}
	return nil
}

func (s *ReminderService) ListByOwner(ctx context.Context, ownerID uuid.UUID, filter ListFilter) ([]domain.Reminder, error) {
	return s.repo.ListByOwner(ctx, ownerID, filter)
}

func (s *ReminderService) GetByID(ctx context.Context, ownerID, id uuid.UUID) (domain.Reminder, error) {
	r, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return domain.Reminder{}, err
	}
	if r.OwnerID != ownerID {
		return domain.Reminder{}, fmt.Errorf("reminder not found")
	}
	return r, nil
}

func (s *ReminderService) Reschedule(ctx context.Context, ownerID, id uuid.UUID, newDate time.Time) (domain.Reminder, error) {
	r, err := s.GetByID(ctx, ownerID, id)
	if err != nil {
		return domain.Reminder{}, err
	}
	if r.Status != domain.ReminderPending {
		return domain.Reminder{}, fmt.Errorf("cannot reschedule non-pending reminder")
	}
	r.ScheduledAt = domain.ScheduledAtForDate(newDate)
	r.UpdatedAt = s.clock.Now()
	if err := s.repo.Save(ctx, r); err != nil {
		return domain.Reminder{}, fmt.Errorf("save rescheduled reminder: %w", err)
	}
	return r, nil
}

func (s *ReminderService) Cancel(ctx context.Context, ownerID, id uuid.UUID) error {
	r, err := s.GetByID(ctx, ownerID, id)
	if err != nil {
		return err
	}
	if r.Status != domain.ReminderPending {
		return fmt.Errorf("cannot cancel non-pending reminder")
	}
	return s.repo.CancelByTarget(ctx, ownerID, r.TargetType, targetIDFor(r), r.EventType)
}

func targetIDFor(r domain.Reminder) uuid.UUID {
	if r.OperationID != nil {
		return *r.OperationID
	}
	if r.RecurringOperationID != nil {
		return *r.RecurringOperationID
	}
	return *r.LeaseID
}

// scheduler implements ReminderScheduler and runs inside transactions.
type scheduler struct {
	service *ReminderService
	repo    ReminderRepository
}

func NewReminderScheduler(service *ReminderService, repo ReminderRepository) ReminderScheduler {
	return &scheduler{service: service, repo: repo}
}

func (s *scheduler) WithTx(tx transaction.Tx) ReminderScheduler {
	return &scheduler{service: s.service, repo: s.repo.WithTx(tx)}
}

func (s *scheduler) ScheduleForOperation(ctx context.Context, op OperationInfo, reminderDate time.Time) error {
	// Ensure any pending reminder for this operation is cancelled first.
	if err := s.repo.CancelByTarget(ctx, op.OwnerID, domain.TargetOperation, op.ID, domain.EventOperationDue); err != nil {
		return err
	}
	_, err := s.service.CreateForOperation(ctx, op, reminderDate)
	return err
}

func (s *scheduler) ScheduleForRecurringOperation(ctx context.Context, rec RecurringOperationInfo, baseReminderDate time.Time, ops []OperationInfo) error {
	if err := s.repo.CancelByTarget(ctx, rec.OwnerID, domain.TargetRecurringOperation, rec.ID, domain.EventOperationDue); err != nil {
		return err
	}
	return s.service.CreateForRecurringOperation(ctx, rec, baseReminderDate, ops)
}

func (s *scheduler) ScheduleForLease(ctx context.Context, lease LeaseInfo) error {
	if lease.EndDate == nil {
		return nil
	}
	_ = s.repo.CancelByTarget(ctx, lease.OwnerID, domain.TargetLease, lease.ID, domain.EventLeaseExpiring)
	_ = s.repo.CancelByTarget(ctx, lease.OwnerID, domain.TargetLease, lease.ID, domain.EventLeaseRequiresAction)
	return s.service.CreateForLease(ctx, lease)
}

func (s *scheduler) CancelByOperation(ctx context.Context, ownerID, opID uuid.UUID) error {
	return s.repo.CancelByTarget(ctx, ownerID, domain.TargetOperation, opID, domain.EventOperationDue)
}

func (s *scheduler) CancelByRecurringOperation(ctx context.Context, ownerID, recID uuid.UUID) error {
	return s.repo.CancelByTarget(ctx, ownerID, domain.TargetRecurringOperation, recID, domain.EventOperationDue)
}

func (s *scheduler) CancelByLease(ctx context.Context, ownerID, leaseID uuid.UUID) error {
	_ = s.repo.CancelByTarget(ctx, ownerID, domain.TargetLease, leaseID, domain.EventLeaseExpiring)
	return s.repo.CancelByTarget(ctx, ownerID, domain.TargetLease, leaseID, domain.EventLeaseRequiresAction)
}
```

**Step 2: Commit**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add apps/backend/internal/notifications/application/service.go
git commit -m "feat(notifications): add reminder application service and scheduler"
```

---

## Task 6: Create Postgres reminder repository

**Files:**
- Create: `apps/backend/internal/notifications/adapters/postgres/repository.go`

**Step 1: Write the repository**

```go
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nambers/arenda-planform/apps/backend/internal/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

type ReminderRepository struct {
	queries *postgres.Queries
}

func NewReminderRepository(db postgres.DBTX) *ReminderRepository {
	return &ReminderRepository{queries: postgres.New(db)}
}

func (r *ReminderRepository) WithTx(tx transaction.Tx) application.ReminderRepository {
	return &ReminderRepository{queries: r.queries.WithTx(tx.(pgx.Tx))}
}

func (r *ReminderRepository) Save(ctx context.Context, rm domain.Reminder) error {
	operationID := uuid.NullUUID{}
	recurringID := uuid.NullUUID{}
	leaseID := uuid.NullUUID{}
	propertyID := uuid.NullUUID{}
	if rm.OperationID != nil {
		operationID = uuid.NullUUID{UUID: *rm.OperationID, Valid: true}
	}
	if rm.RecurringOperationID != nil {
		recurringID = uuid.NullUUID{UUID: *rm.RecurringOperationID, Valid: true}
	}
	if rm.LeaseID != nil {
		leaseID = uuid.NullUUID{UUID: *rm.LeaseID, Valid: true}
	}
	if rm.PropertyID != nil {
		propertyID = uuid.NullUUID{UUID: *rm.PropertyID, Valid: true}
	}

	_, err := r.queries.CreateReminder(ctx, postgres.CreateReminderParams{
		ID:                   rm.ID,
		OwnerID:              rm.OwnerID,
		TargetType:           postgres.NotificationTargetType(rm.TargetType),
		OperationID:          operationID,
		RecurringOperationID: recurringID,
		LeaseID:              leaseID,
		PropertyID:           propertyID,
		EventType:            postgres.NotificationEventType(rm.EventType),
		Status:               postgres.NotificationStatus(rm.Status),
		ScheduledAt:          rm.ScheduledAt,
		MessageTitle:         rm.MessageTitle,
		MessageBody:          rm.MessageBody,
		CreatedAt:            rm.CreatedAt,
		UpdatedAt:            rm.UpdatedAt,
	})
	if err != nil {
		return fmt.Errorf("create reminder: %w", err)
	}
	return nil
}

func (r *ReminderRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Reminder, error) {
	row, err := r.queries.GetReminderByID(ctx, id)
	if err != nil {
		return domain.Reminder{}, fmt.Errorf("get reminder: %w", err)
	}
	return toDomain(row), nil
}

func (r *ReminderRepository) ListByOwner(ctx context.Context, ownerID uuid.UUID, filter application.ListFilter) ([]domain.Reminder, error) {
	var status *postgres.NotificationStatus
	if filter.Status != nil {
		s := postgres.NotificationStatus(*filter.Status)
		status = &s
	}
	rows, err := r.queries.ListRemindersByOwner(ctx, postgres.ListRemindersByOwnerParams{
		OwnerID: ownerID,
		Status:  status,
		Limit:   int32(filter.Limit),
		Offset:  int32(filter.Offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list reminders: %w", err)
	}
	out := make([]domain.Reminder, len(rows))
	for i, row := range rows {
		out[i] = toDomain(row)
	}
	return out, nil
}

func (r *ReminderRepository) ListDue(ctx context.Context, before time.Time, limit int) ([]domain.Reminder, error) {
	rows, err := r.queries.ListDueReminders(ctx, postgres.ListDueRemindersParams{
		ScheduledAt: before,
		Limit:       int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list due reminders: %w", err)
	}
	out := make([]domain.Reminder, len(rows))
	for i, row := range rows {
		out[i] = toDomain(row)
	}
	return out, nil
}

func (r *ReminderRepository) MarkSent(ctx context.Context, id uuid.UUID, at time.Time) error {
	_, err := r.queries.MarkReminderSent(ctx, postgres.MarkReminderSentParams{ID: id, SentAt: at})
	if err != nil {
		return fmt.Errorf("mark reminder sent: %w", err)
	}
	return nil
}

func (r *ReminderRepository) MarkFailed(ctx context.Context, id uuid.UUID, nextAttempt *time.Time, terminal bool) error {
	_, err := r.queries.MarkReminderFailed(ctx, postgres.MarkReminderFailedParams{
		ID:            id,
		NextAttemptAt: nextAttempt,
		Column3:       terminal,
	})
	if err != nil {
		return fmt.Errorf("mark reminder failed: %w", err)
	}
	return nil
}

func (r *ReminderRepository) CancelByTarget(ctx context.Context, ownerID uuid.UUID, targetType domain.TargetType, targetID uuid.UUID, eventType domain.EventType) error {
	_, err := r.queries.CancelReminderByTarget(ctx, postgres.CancelReminderByTargetParams{
		OwnerID:    ownerID,
		TargetType: postgres.NotificationTargetType(targetType),
		Column3:    targetID,
		EventType:  postgres.NotificationEventType(eventType),
	})
	if err != nil {
		return fmt.Errorf("cancel reminder by target: %w", err)
	}
	return nil
}

func (r *ReminderRepository) CancelByOperationIDs(ctx context.Context, ownerID uuid.UUID, operationIDs []uuid.UUID) error {
	_, err := r.queries.CancelRemindersByOperationIDs(ctx, postgres.CancelRemindersByOperationIDsParams{
		OwnerID:     ownerID,
		OperationID: operationIDs,
	})
	if err != nil {
		return fmt.Errorf("cancel reminders by operation ids: %w", err)
	}
	return nil
}

func toDomain(row postgres.Reminder) domain.Reminder {
	var opID, recID, leaseID, propID *uuid.UUID
	if row.OperationID.Valid {
		opID = &row.OperationID.UUID
	}
	if row.RecurringOperationID.Valid {
		recID = &row.RecurringOperationID.UUID
	}
	if row.LeaseID.Valid {
		leaseID = &row.LeaseID.UUID
	}
	if row.PropertyID.Valid {
		propID = &row.PropertyID.UUID
	}
	return domain.Reminder{
		ID:                   row.ID,
		OwnerID:              row.OwnerID,
		TargetType:           domain.TargetType(row.TargetType),
		OperationID:          opID,
		RecurringOperationID: recID,
		LeaseID:              leaseID,
		PropertyID:           propID,
		EventType:            domain.EventType(row.EventType),
		Status:               domain.ReminderStatus(row.Status),
		ScheduledAt:          row.ScheduledAt,
		SentAt:               nullableTime(row.SentAt),
		FailedAttempts:       int(row.FailedAttempts),
		NextAttemptAt:        nullableTime(row.NextAttemptAt),
		MessageTitle:         row.MessageTitle,
		MessageBody:          row.MessageBody,
		CreatedAt:            row.CreatedAt,
		UpdatedAt:            row.UpdatedAt,
	}
}

func nullableTime(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}
```

**Step 2: Fix imports if needed**

The generated `postgres` package includes `pgtype`. Use `postgres` import for `pgtype.Timestamptz` or import `github.com/jackc/pgx/v5/pgtype`. Adjust based on generated code.

**Step 3: Commit**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add apps/backend/internal/notifications/adapters/postgres/repository.go
git commit -m "feat(notifications): add postgres reminder repository"
```

---

## Task 7: Create SMS notifier adapter

**Files:**
- Create: `apps/backend/internal/notifications/adapters/sms/notifier.go`

**Step 1: Write the adapter**

```go
package sms

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	identitydomain "github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
)

type Notifier struct {
	sender identityapp.Sender
	logger *slog.Logger
}

func NewNotifier(sender identityapp.Sender, logger *slog.Logger) *Notifier {
	if logger == nil {
		logger = slog.Default()
	}
	return &Notifier{sender: sender, logger: logger}
}

func (n *Notifier) Notify(ctx context.Context, notification application.Notification) error {
	contact, err := n.resolveContact(ctx, notification.RecipientID)
	if err != nil {
		return fmt.Errorf("resolve contact: %w", err)
	}
	if contact.Channel != application.ChannelSMS {
		return fmt.Errorf("unsupported channel: %s", contact.Channel)
	}

	phone := identitydomain.Phone(contact.Address)
	if err := n.sender.Send(ctx, phone, notification.Body); err != nil {
		return fmt.Errorf("send sms: %w", err)
	}
	n.logger.InfoContext(ctx, "sms reminder sent", "recipient_id", notification.RecipientID, "event_type", notification.EventType)
	return nil
}

func (n *Notifier) resolveContact(ctx context.Context, ownerID uuid.UUID) (application.Contact, error) {
	// Placeholder: will be replaced by ContactResolver in Task 8.
	return application.Contact{}, fmt.Errorf("not implemented")
}
```

**Step 2: Commit**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add apps/backend/internal/notifications/adapters/sms/notifier.go
git commit -m "feat(notifications): add sms notifier adapter skeleton"
```

---

## Task 8: Create user contact resolver

**Files:**
- Create: `apps/backend/internal/notifications/adapters/postgres/contact_resolver.go`

**Step 1: Write the resolver**

```go
package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
)

type ContactResolver struct {
	queries *postgres.Queries
}

func NewContactResolver(db postgres.DBTX) *ContactResolver {
	return &ContactResolver{queries: postgres.New(db)}
}

func (r *ContactResolver) Resolve(ctx context.Context, ownerID uuid.UUID) (application.Contact, error) {
	phone, err := r.queries.GetUserPhoneByID(ctx, ownerID)
	if err != nil {
		return application.Contact{}, fmt.Errorf("get user phone: %w", err)
	}
	return application.Contact{
		Channel: application.ChannelSMS,
		Address: phone,
	}, nil
}
```

**Step 2: Update SMS notifier to use ContactResolver**

Modify `apps/backend/internal/notifications/adapters/sms/notifier.go`:

```go
type Notifier struct {
	resolver application.ContactResolver
	sender   identityapp.Sender
	logger   *slog.Logger
}

func NewNotifier(resolver application.ContactResolver, sender identityapp.Sender, logger *slog.Logger) *Notifier {
	...
}

func (n *Notifier) Notify(ctx context.Context, notification application.Notification) error {
	contact, err := n.resolver.Resolve(ctx, notification.RecipientID)
	...
}

// remove resolveContact method
```

**Step 3: Commit**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add apps/backend/internal/notifications/adapters/postgres/contact_resolver.go apps/backend/internal/notifications/adapters/sms/notifier.go
git commit -m "feat(notifications): add contact resolver and wire into sms notifier"
```

---

## Task 9: Create reminder dispatch worker

**Files:**
- Create: `apps/backend/internal/platform/scheduler/reminder_worker.go`

**Step 1: Write the worker**

```go
package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

type Backoff interface {
	Next(attempt int) time.Duration
}

type ReminderWorker struct {
	repo        application.ReminderRepository
	resolver    application.ContactResolver
	notifier    application.Notifier
	clock       clock.Clock
	backoff     Backoff
	maxAttempts int
	interval    time.Duration
	batchSize   int
	logger      *slog.Logger
}

func NewReminderWorker(
	repo application.ReminderRepository,
	resolver application.ContactResolver,
	notifier application.Notifier,
	clock clock.Clock,
	backoff Backoff,
	maxAttempts int,
	interval time.Duration,
	batchSize int,
	logger *slog.Logger,
) *ReminderWorker {
	if logger == nil {
		logger = slog.Default()
	}
	return &ReminderWorker{
		repo: repo, resolver: resolver, notifier: notifier, clock: clock,
		backoff: backoff, maxAttempts: maxAttempts,
		interval: interval, batchSize: batchSize, logger: logger,
	}
}

func (w *ReminderWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.tick(ctx); err != nil {
				w.logger.ErrorContext(ctx, "reminder worker tick failed", "error", err)
			}
		}
	}
}

func (w *ReminderWorker) tick(ctx context.Context) error {
	now := w.clock.Now()
	reminders, err := w.repo.ListDue(ctx, now, w.batchSize)
	if err != nil {
		return err
	}
	for _, r := range reminders {
		if err := w.dispatch(ctx, r, now); err != nil {
			w.logger.ErrorContext(ctx, "dispatch reminder failed", "reminder_id", r.ID, "error", err)
		}
	}
	return nil
}

func (w *ReminderWorker) dispatch(ctx context.Context, r application.Reminder, now time.Time) error {
	// Idempotency guard: re-check status inside repo methods.
	if err := w.notifier.Notify(ctx, application.Notification{
		RecipientID: r.OwnerID,
		EventType:   r.EventType,
		Title:       r.MessageTitle,
		Body:        r.MessageBody,
	}); err != nil {
		attempts := r.FailedAttempts + 1
		terminal := attempts >= w.maxAttempts
		var next *time.Time
		if !terminal {
			n := now.Add(w.backoff.Next(attempts))
			next = &n
		}
		if markErr := w.repo.MarkFailed(ctx, r.ID, next, terminal); markErr != nil {
			w.logger.ErrorContext(ctx, "mark reminder failed", "reminder_id", r.ID, "error", markErr)
		}
		return err
	}
	if err := w.repo.MarkSent(ctx, r.ID, now); err != nil {
		return err
	}
	return nil
}
```

**Step 2: Commit**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add apps/backend/internal/platform/scheduler/reminder_worker.go
git commit -m "feat(notifications): add reminder dispatch worker"
```

---

## Task 10: Create lease reconciliation job

**Files:**
- Create: `apps/backend/internal/platform/scheduler/lease_reconciliation.go`

**Step 1: Write the job**

```go
package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/timeutil"
)

type LeaseReconciliationWorker struct {
	leases   application.LeaseRepository
	scheduler application.ReminderScheduler
	clock    clock.Clock
	interval time.Duration
	logger   *slog.Logger
}

func NewLeaseReconciliationWorker(
	leases application.LeaseRepository,
	scheduler application.ReminderScheduler,
	clock clock.Clock,
	interval time.Duration,
	logger *slog.Logger,
) *LeaseReconciliationWorker {
	if logger == nil {
		logger = slog.Default()
	}
	return &LeaseReconciliationWorker{leases: leases, scheduler: scheduler, clock: clock, interval: interval, logger: logger}
}

func (w *LeaseReconciliationWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.tick(ctx); err != nil {
				w.logger.ErrorContext(ctx, "lease reconciliation failed", "error", err)
			}
		}
	}
}

func (w *LeaseReconciliationWorker) tick(ctx context.Context) error {
	today := timeutil.Date(w.clock.Now())
	leases, err := w.leases.ListByOwner(ctx, uuid.Nil) // needs real query; placeholder
	if err != nil {
		return err
	}
	for _, lease := range leases {
		if lease.EndDate == nil || !lease.Status.IsOpen() {
			continue
		}
		end := timeutil.Date(*lease.EndDate)
		if today.After(end) && lease.Status != domain.LeaseStatusRequiresAction {
			// Trigger status update and reminder creation.
			// Actual status update should be delegated to LeaseService.
		}
	}
	return nil
}
```

> Note: This is a placeholder. The real reconciliation should reuse `LeaseService.recalculateStatus` or add a repository query for open leases with `end_date < today`.

**Step 2: Commit**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add apps/backend/internal/platform/scheduler/lease_reconciliation.go
git commit -m "feat(notifications): add lease reconciliation worker skeleton"
```

---

## Task 11: Add ReminderScheduler port to leases application

**Files:**
- Modify: `apps/backend/internal/leases/application/ports.go`

**Step 1: Add the port import and interface**

Add at the top:

```go
import "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
```

Append to the file:

```go
// ReminderScheduler is the port used by leases services to schedule/cancel reminders.
type ReminderScheduler = application.ReminderScheduler
```

**Step 2: Commit**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add apps/backend/internal/leases/application/ports.go
git commit -m "feat(leasess): expose ReminderScheduler port"
```

---

## Task 12: Wire scheduler into OperationService

**Files:**
- Modify: `apps/backend/internal/leases/application/operation_service.go`

**Step 1: Add field and constructor param**

Change `OperationService` struct:

```go
type OperationService struct {
	operations OperationRepository
	properties PropertyRepository
	leases     LeaseRepository
	scheduler  ReminderScheduler
	clock      clock.Clock
	logger     *slog.Logger
}
```

Update constructor signature and body to accept `scheduler ReminderScheduler`.

**Step 2: Call scheduler on create/update/delete**

In `CreateOperation`, after `created, err := s.operations.Create(ctx, op)`:

```go
if s.scheduler != nil && cmd.ReminderDate != nil {
    if err := s.scheduler.ScheduleForOperation(ctx, toOperationInfo(created), *cmd.ReminderDate); err != nil {
        return domain.Operation{}, fmt.Errorf("schedule reminder: %w", err)
    }
}
```

Add `ReminderDate *time.Time` to `CreateOperationCommand` and `UpdateOperationCommand`.

In `UpdateOperation`, after successful update, if `cmd.OperationDate` changed or `cmd.ReminderDate` provided:

```go
if s.scheduler != nil {
    _ = s.scheduler.CancelByOperation(ctx, ownerID, updated.ID)
    if cmd.ReminderDate != nil {
        if err := s.scheduler.ScheduleForOperation(ctx, toOperationInfo(updated), *cmd.ReminderDate); err != nil {
            return domain.Operation{}, fmt.Errorf("schedule reminder: %w", err)
        }
    }
}
```

In `DeleteOperation`:

```go
if s.scheduler != nil {
    _ = s.scheduler.CancelByOperation(ctx, ownerID, id)
}
```

**Step 3: Add helper toOperationInfo**

```go
func toOperationInfo(op domain.Operation) notificationsapp.OperationInfo {
	return notificationsapp.OperationInfo{
		ID:            op.ID,
		OwnerID:       op.OwnerID,
		PropertyID:    op.PropertyID,
		LeaseID:       op.LeaseID,
		RecurringOperationID: op.RecurringOperationID,
		OperationDate: op.OperationDate,
		Type:          string(op.Type),
		Category:      string(op.Category),
		AmountKopecks: op.AmountKopecks,
	}
}
```

**Step 4: Commit**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add apps/backend/internal/leases/application/operation_service.go
git commit -m "feat(leasess): wire reminders into OperationService"
```

---

## Task 13: Wire scheduler into RecurringOperationService

**Files:**
- Modify: `apps/backend/internal/leases/application/recurring_operation_service.go`

**Step 1: Add scheduler field and param**

Add to struct:

```go
scheduler ReminderScheduler
```

Update constructor.

**Step 2: Schedule after generate in Create/Update/Resume**

In `CreateRecurringOperation`, after `generateOperations`:

```go
if s.scheduler != nil && cmd.ReminderDate != nil {
    recInfo := notificationsapp.RecurringOperationInfo{ID: created.ID, OwnerID: created.OwnerID, PropertyID: created.PropertyID, LeaseID: created.LeaseID}
    generated, _ := txOps.ListByRecurringOperation(ctx, created.ID) // or reuse toCreate from generateOperations
    _ = s.scheduler.ScheduleForRecurringOperation(ctx, recInfo, *cmd.ReminderDate, toOperationInfoSlice(generated))
}
```

> Note: `ListByRecurringOperation` may need to be added to `OperationRepository` or reuse `buildOperations` result. Adapt to existing code.

In `UpdateRecurringOperation`, after regenerating operations:

```go
if s.scheduler != nil && cmd.ReminderDate != nil {
    recInfo := ...
    generated, _ := txOps.ListByRecurringOperation(ctx, updated.ID)
    _ = s.scheduler.ScheduleForRecurringOperation(ctx, recInfo, *cmd.ReminderDate, toOperationInfoSlice(generated))
}
```

In `PauseRecurringOperation`, wrap in transaction and cancel reminders:

```go
if s.scheduler != nil {
    tx, err := s.db.Begin(ctx)
    ...
    txScheduler := s.scheduler.WithTx(tx)
    if err := txScheduler.CancelByRecurringOperation(ctx, ownerID, id); err != nil { ... }
    rec, err := s.recurringOps.WithTx(tx).UpdateStatus(...)
    ...
}
```

In `ResumeRecurringOperation`, after generating operations, schedule reminders if template has stored reminder offset. This requires storing the offset; for MVP, reschedule only if a new `ReminderDate` is provided in command or store offset in recurring operation row.

**Step 3: Commit**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add apps/backend/internal/leases/application/recurring_operation_service.go
git commit -m "feat(leasess): wire reminders into RecurringOperationService"
```

---

## Task 14: Wire scheduler into LeaseService and RentService

**Files:**
- Modify: `apps/backend/internal/leases/application/service.go`
- Modify: `apps/backend/internal/leases/application/rent_service.go`

**Step 1: LeaseService**

Add `scheduler ReminderScheduler` to `LeaseService` and constructor.

In `CreateLease`, after commit:

```go
if s.scheduler != nil {
    _ = s.scheduler.ScheduleForLease(ctx, toLeaseInfo(created))
}
```

Place after `tx.Commit` so the lease row is visible. Alternatively do it before commit using transactional scheduler.

In `UpdateLease`, after schedule changes and commit:

```go
if s.scheduler != nil {
    txScheduler := s.scheduler.WithTx(tx)
    _ = txScheduler.CancelByLease(ctx, ownerID, updated.ID)
    _ = txScheduler.ScheduleForLease(ctx, toLeaseInfo(updated))
}
```

In `CompleteLease`, before commit:

```go
if s.scheduler != nil {
    _ = s.scheduler.WithTx(tx).CancelByLease(ctx, ownerID, id)
}
```

**Step 2: RentService**

No direct reminder changes unless you want to centralize recurring op reminders here. Keep minimal.

**Step 3: Commit**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add apps/backend/internal/leases/application/service.go apps/backend/internal/leases/application/rent_service.go
git commit -m "feat(leasess): wire reminders into LeaseService"
```

---

## Task 15: Wire scheduler into PropertyBillingLifecycle

**Files:**
- Modify: `apps/backend/internal/leases/adapters/postgres/property_billing_lifecycle.go`

**Step 1: Add scheduler field**

```go
type PropertyBillingLifecycle struct {
	ops          *OperationRepository
	recurringOps *RecurringOperationRepository
	scheduler    leasesapp.ReminderScheduler
	clock        clock.Clock
}
```

Update constructor to accept scheduler and `WithTx` to pass scheduler.WithTx.

**Step 2: Cancel/schedule on suspend/resume**

In `Suspend`, after deleting operations and pausing recurring ops, cancel reminders for the property's operations. For MVP, cancel by querying recurring op IDs and operation IDs. Simpler: add `CancelByProperty` to scheduler or cancel by each recurring op ID after listing.

In `Resume`, after generating operations, schedule reminders for each resumed recurring op. This requires stored reminder offset per recurring op; if not stored, skip.

**Step 3: Commit**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add apps/backend/internal/leases/adapters/postgres/property_billing_lifecycle.go
git commit -m "feat(leasess): handle reminders on property archive/unarchive"
```

---

## Task 16: Add HTTP handlers for reminders

**Files:**
- Create: `apps/backend/internal/platform/httpapi/reminder_handlers.go`

**Step 1: Write handlers**

```go
package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
)

type ReminderHandlers struct {
	service ReminderService
	logger  *slog.Logger
}

type ReminderService interface {
	CreateForOperation(ctx context.Context, ownerID, operationID uuid.UUID, reminderDate time.Time) (notificationsapp.Reminder, error)
	// ... other methods
}

func NewReminderHandlers(service ReminderService, logger *slog.Logger) *ReminderHandlers {
	return &ReminderHandlers{service: service, logger: logger}
}

func (h *ReminderHandlers) CreateOperationReminder(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := ownerIDFromContext(r.Context())
	if !ok { ... }
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil { ... }
	var req struct {
		ReminderDate string `json:"reminder_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { ... }
	date, err := time.Parse("2006-01-02", req.ReminderDate)
	if err != nil { ... }
	reminder, err := h.service.CreateForOperation(r.Context(), ownerID, id, date)
	if err != nil { ... }
	writeJSON(w, http.StatusCreated, reminder)
}

// Implement remaining handlers (GET, PATCH, DELETE, recurring, lease, list) similarly.
```

**Step 2: Commit**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add apps/backend/internal/platform/httpapi/reminder_handlers.go
git commit -m "feat(http): add reminder handlers skeleton"
```

---

## Task 17: Update httpapi Deps and wiring

**Files:**
- Modify: `apps/backend/internal/platform/httpapi/server.go`
- Modify: `apps/backend/cmd/api/main.go`

**Step 1: server.go**

Add to `Deps`:

```go
Reminders *notificationsapp.ReminderService
```

Add `*ReminderHandlers` to `composedHandler` and instantiate in `New`.

**Step 2: main.go**

Instantiate repositories, service, adapters, and worker:

```go
reminderRepo := notificationspg.NewReminderRepository(pool)
contactResolver := notificationspg.NewContactResolver(pool)
smsNotifier := notificationsms.NewNotifier(contactResolver, smsSender, logger)
reminderService := notificationsapp.NewReminderService(reminderRepo, realClock{})
reminderScheduler := notificationsapp.NewReminderScheduler(reminderService, reminderRepo)

// Workers
reminderWorker := scheduler.NewReminderWorker(reminderRepo, contactResolver, smsNotifier, realClock{}, backoff, 5, 1*time.Minute, 100, logger)
go reminderWorker.Run(ctx)
```

Pass `ReminderService` into `httpapi.Deps`.

Update service constructors to accept scheduler:

```go
operationService := leasesapp.NewOperationService(operationRepo, leasePropertyRepo, leaseRepo, reminderScheduler, realClock{}, logger)
recurringOperationService := leasesapp.NewRecurringOperationService(..., reminderScheduler, ...)
leaseService := leasesapp.NewLeaseService(..., reminderScheduler, ...)
propertyBillingLifecycle := leasespg.NewPropertyBillingLifecycle(operationRepo, recurringOpRepo, reminderScheduler, realClock{})
```

**Step 3: Commit**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add apps/backend/internal/platform/httpapi/server.go apps/backend/cmd/api/main.go
git commit -m "feat(http): wire reminder service, scheduler and worker"
```

---

## Task 18: Update OpenAPI spec

**Files:**
- Modify: `apps/backend/api/openapi/openapi.yaml`

**Step 1: Add reminder schemas and endpoints**

Add components:

```yaml
Reminder:
  type: object
  required: [id, owner_id, target_type, event_type, status, scheduled_at, message_title, message_body]
  properties:
    id: { type: string, format: uuid }
    owner_id: { type: string, format: uuid }
    target_type: { type: string, enum: [operation, recurring_operation, lease] }
    operation_id: { type: string, format: uuid, nullable: true }
    recurring_operation_id: { type: string, format: uuid, nullable: true }
    lease_id: { type: string, format: uuid, nullable: true }
    event_type: { type: string, enum: [operation_due, lease_expiring, lease_requires_action] }
    status: { type: string, enum: [pending, sent, failed, cancelled] }
    scheduled_at: { type: string, format: date-time }
    sent_at: { type: string, format: date-time, nullable: true }
    message_title: { type: string }
    message_body: { type: string }

CreateReminderRequest:
  type: object
  required: [reminder_date]
  properties:
    reminder_date: { type: string, format: date }
```

Add paths for `/operations/{id}/reminders`, `/recurring-operations/{id}/reminders`, `/reminders`, `/reminders/{id}`.

**Step 2: Commit**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add apps/backend/api/openapi/openapi.yaml
git commit -m "feat(api): add reminder endpoints to openapi spec"
```

---

## Task 19: Regenerate generated code

**Step 1: Run generators**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/backend
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.0 -config api/openapi/oapi-codegen.yaml api/openapi/openapi.yaml
```

Expected: `internal/generated/postgres/` and `internal/platform/openapi/` updated.

**Step 2: Commit**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add apps/backend/internal/generated/postgres apps/backend/internal/platform/openapi
git commit -m "chore: regenerate sqlc and openapi for reminders"
```

---

## Task 20: Domain unit tests

**Files:**
- Create: `apps/backend/internal/notifications/domain/reminder_test.go`

**Step 1: Add tests**

Already added in Task 3. Expand with offset and validation tests.

**Step 2: Run tests**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/backend
go test ./internal/notifications/domain/... -v
```

Expected: PASS.

**Step 3: Commit**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add apps/backend/internal/notifications/domain/reminder_test.go
git commit -m "test(notifications): add domain reminder tests"
```

---

## Task 21: Application service tests

**Files:**
- Create: `apps/backend/internal/notifications/application/service_test.go`

**Step 1: Write tests**

Use fake repository and clock. Test:
- creating operation reminder;
- creating recurring operation reminders computes offset correctly;
- creating lease reminders;
- rescheduling and cancelling.

**Step 2: Run tests**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/backend
go test ./internal/notifications/application/... -v
```

Expected: PASS.

**Step 3: Commit**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add apps/backend/internal/notifications/application/service_test.go
git commit -m "test(notifications): add reminder application service tests"
```

---

## Task 22: Repository integration tests

**Files:**
- Create: `apps/backend/internal/notifications/adapters/postgres/repository_test.go`

**Step 1: Write tests**

Use existing test DB setup. Test:
- Save + GetByID roundtrip;
- ListDue ordering and status filter;
- MarkSent/MarkFailed;
- CancelByTarget;
- unique constraint.

**Step 2: Run tests**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/backend
go test ./internal/notifications/adapters/postgres/... -v
```

Expected: PASS.

**Step 3: Commit**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add apps/backend/internal/notifications/adapters/postgres/repository_test.go
git commit -m "test(notifications): add reminder repository integration tests"
```

---

## Task 23: Worker tests

**Files:**
- Create: `apps/backend/internal/platform/scheduler/reminder_worker_test.go`

**Step 1: Write tests**

Use fake repository, notifier, resolver, clock. Test:
- dispatch marks sent;
- failure increments attempts and schedules retry;
- terminal failed after max attempts.

**Step 2: Run tests**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/backend
go test ./internal/platform/scheduler/... -v
```

Expected: PASS.

**Step 3: Commit**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add apps/backend/internal/platform/scheduler/reminder_worker_test.go
git commit -m "test(notifications): add reminder worker tests"
```

---

## Task 24: Lint and full test run

**Step 1: Run lint**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
make backend-lint
```

Expected: no errors.

**Step 2: Run tests**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/backend
go test ./...
go vet ./...
```

Expected: PASS.

**Step 3: Commit any fixes**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add -A
git commit -m "chore: lint and test fixes for reminders"
```

---

## Known open questions

1. Should `identity.Sender` be moved to `internal/notifications/application` per ADR 0006, or wrapped as proposed? Proposed plan wraps it to minimize churn.
2. How to store recurring operation reminder offset? The plan currently reschedules on Create/Update/Resume only when a new `ReminderDate` is supplied. To fully support property archive/unarchive, consider adding a `reminder_offset_days` column to `recurring_operations`.
3. Lease reconciliation job needs a repository query to list open leases with `end_date < today` to avoid loading all leases.