---
title: Reminders / Notifications Subsystem Design
date: 2026-06-17
---

# Reminders / Notifications Subsystem Design

## Status

Approved for implementation.

## Context

MVP Arenda Platform needs user-facing reminders for:
- future operations (one-time and recurring);
- lease end date (30 days before);
- lease transitioning to `requires_action`.

The delivery channel is not fixed: SMS today, possibly email/push later. Business logic must not depend on a concrete channel.

## Goals

- Channel-agnostic reminder scheduling and dispatch.
- Separate REST resource for reminders.
- Eager generation of concrete reminder records.
- Automatic cancellation when the underlying operation/lease/template changes.
- At-least-once delivery with retries and idempotency for SMS (ADR 0006).

## Non-goals

- Multi-channel preference per user in MVP.
- Rich templating or user-edited message text.
- Push/email implementations.

## Decisions

| Topic | Decision |
|-------|----------|
| Trigger sources | Future `Operation`, `RecurringOperation` template, `Lease`. |
| Recurring operation reminder | Attached to the template; user picks a date for the nearest generated operation, system stores the offset and creates concrete reminders for all generated operations. |
| Lease reminders | Hard-coded 30-day pre-expiration reminder and a `requires_action` reminder. |
| Dispatch time | 10:00 Europe/Moscow on the chosen date. |
| Channel abstraction | `Notifier` port; SMS adapter wraps existing `identity.Sender`. |
| Lifecycle coupling | Reminders are created/cancelled inside the same transaction as operations/leases. |
| Failure handling | Exponential backoff, terminal `failed` status after max attempts. |
| Opt-out | Not supported in MVP; reminders are always enabled. |

## Architecture

```
┌─────────────────────────────────────┐
│  transport / openapi handlers       │
└──────────────┬──────────────────────┘
               │
┌──────────────▼──────────────────────┐
│  internal/notifications/application │
│  - ReminderService (CRUD + worker)  │
│  - ReminderScheduler port           │
│  - Notifier / ContactResolver ports │
└──────────────┬──────────────────────┘
               │
┌──────────────▼──────────────────────┐
│  internal/notifications/domain      │
│  - Reminder entity & rules          │
└──────────────┬──────────────────────┘
               │
┌──────────────▼──────────────────────┐
│  adapters                           │
│  - postgres.ReminderRepository      │
│  - sms.Notifier (uses identity.Sender)
└─────────────────────────────────────┘
```

Other bounded contexts (`leases`, `operations`) call `ReminderScheduler` through its port to create/cancel reminders inside their transactions.

## Domain model

```go
type Reminder struct {
    ID                   uuid.UUID
    OwnerID              uuid.UUID
    TargetType           TargetType    // operation | recurring_operation | lease
    OperationID          *uuid.UUID
    RecurringOperationID *uuid.UUID
    LeaseID              *uuid.UUID
    PropertyID           *uuid.UUID
    EventType            EventType     // operation_due | lease_expiring | lease_requires_action
    Status               ReminderStatus // pending | sending (internal) | sent | failed | cancelled
    ScheduledAt          time.Time
    SentAt               *time.Time
    FailedAttempts       int
    NextAttemptAt        *time.Time
    MessageTitle         string
    MessageBody          string
    CreatedAt            time.Time
    UpdatedAt            time.Time
}
```

### Key rules

- `ScheduledAt` = user date at 10:00 Europe/Moscow.
- For recurring operations: offset = `operation_date - reminder_date` for the first operation, applied to all future operations.
- One reminder per concrete target + event.
- Cancelled/failed reminders are not dispatched again.

## Application ports

```go
type ReminderRepository interface {
    Save(ctx context.Context, r Reminder) error
    GetByID(ctx context.Context, id uuid.UUID) (Reminder, error)
    ListByOwner(ctx context.Context, ownerID uuid.UUID, filter ListFilter) ([]Reminder, error)
    ListDue(ctx context.Context, before time.Time, limit int) ([]Reminder, error)
    MarkSent(ctx context.Context, id uuid.UUID, at time.Time) error
    MarkFailed(ctx context.Context, id uuid.UUID, nextAttempt *time.Time) error
    CancelByTarget(ctx context.Context, ownerID uuid.UUID, targetType TargetType, targetID uuid.UUID, eventType EventType) error
    CancelByOperationIDs(ctx context.Context, ownerID uuid.UUID, operationIDs []uuid.UUID) error
    WithTx(tx transaction.Tx) ReminderRepository
}

type Notifier interface {
    Notify(ctx context.Context, n Notification) error
}

type ContactResolver interface {
    Resolve(ctx context.Context, ownerID uuid.UUID) (Contact, error)
}

type ReminderScheduler interface {
    ScheduleForOperation(ctx context.Context, op Operation, reminderDate time.Time) error
    ScheduleForRecurringOperation(ctx context.Context, rec RecurringOperation, baseReminderDate time.Time, ops []Operation) error
    ScheduleForLease(ctx context.Context, lease Lease) error
    CancelByOperation(ctx context.Context, opID uuid.UUID) error
    CancelByRecurringOperation(ctx context.Context, recID uuid.UUID) error
    CancelByLease(ctx context.Context, leaseID uuid.UUID) error
    WithTx(tx transaction.Tx) ReminderScheduler
}
```

## Database schema

```sql
CREATE TYPE notification_target_type AS ENUM ('operation', 'recurring_operation', 'lease');
CREATE TYPE notification_event_type AS ENUM ('operation_due', 'lease_expiring', 'lease_requires_action');
CREATE TYPE notification_status AS ENUM ('pending', 'sending', 'sent', 'failed', 'cancelled');

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

`sent_sms_reminders` satisfies ADR 0006's requirement to track sent messages for deduplication and audit.

## Lifecycle integration

### Operations

- `CreateOperation` → `ScheduleForOperation` (if reminder requested).
- `UpdateOperation` (date change) → cancel old, create new.
- `DeleteOperation` → `CancelByOperation`.

### Recurring operations

- `CreateRecurringOperation` / `UpdateRecurringOperation` / `ResumeRecurringOperation` → after regenerating operations, call `ScheduleForRecurringOperation` to create concrete reminders for all generated operations using the stored offset.
- `PauseRecurringOperation` → `CancelByRecurringOperation`.
- Property archive/unarchive → `Suspend`/`Resume` cancels/recreates reminders alongside operations.

### Leases

- `CreateLease` / `UpdateLease` → `ScheduleForLease` creates:
  - `lease_expiring` on `end_date - 30 days`;
  - `lease_requires_action` on `end_date + 1 day`.
- `CompleteLease` / archive → cancel lease reminders.

Because `requires_action` is computed query-time, a daily reconciliation job scans open leases and creates the reminder when the transition actually happens.

## Worker

A background goroutine polls `ReminderRepository.ListDue` on a short interval (e.g. 1 minute), resolves the owner's contact, calls `Notifier.Notify`, and marks the reminder as sent or failed with a retry schedule.

## API

OpenAPI-first additions. Paths are nested under `/properties/{propertyId}` for operations and recurring operations, while lease reminders remain under `/leases`:

```
POST   /properties/{propertyId}/operations/{operationId}/reminders
GET    /properties/{propertyId}/operations/{operationId}/reminders
POST   /properties/{propertyId}/recurring-operations/{recurringOperationId}/reminders
GET    /properties/{propertyId}/recurring-operations/{recurringOperationId}/reminders
GET    /leases/{leaseId}/reminders
GET    /reminders?limit={limit}&offset={offset}
PATCH  /reminders/{reminderId}
DELETE /reminders/{reminderId}
```

`GET /reminders` supports optional `limit` (default 100, maximum 1000) and `offset` (default 0) query parameters for pagination.

The `sending` status is an internal worker state and is never exposed through the API; external clients see `pending` instead.

Request body for creation:

```json
{
  "reminder_date": "2026-06-15"
}
```

## Error handling

- Retry transient `Notifier` errors with exponential backoff.
- Terminal `failed` status after max attempts.
- Never log full phone numbers or SMS codes in production.

## Testing

- Unit tests for `Reminder` domain rules (`ScheduledAt`, offset, status transitions).
- Application service tests with fake repository, clock, notifier, and contact resolver.
- Integration tests for `ListDue`, cancellation, and unique constraints.
- Worker tests with deterministic clock and fake notifier.

## Open questions

- Keep `identity.Sender` in place and wrap it, or move it to `internal/notifications` per ADR 0006? The design recommends wrapping to minimize churn.