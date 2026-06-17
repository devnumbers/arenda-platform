package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// ReminderRepository persists and queries reminders.
type ReminderRepository interface {
	Save(ctx context.Context, r domain.Reminder) error
	Update(ctx context.Context, r domain.Reminder) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.Reminder, error)
	ListByOwner(ctx context.Context, ownerID uuid.UUID, filter ListFilter) ([]domain.Reminder, error)
	ListDue(ctx context.Context, before time.Time, limit int) ([]domain.Reminder, error)
	MarkSent(ctx context.Context, id uuid.UUID, at time.Time) error
	MarkFailed(ctx context.Context, id uuid.UUID, nextAttempt *time.Time, terminal bool) error
	CancelByTarget(ctx context.Context, ownerID uuid.UUID, targetType domain.TargetType, targetID uuid.UUID, eventType domain.EventType) error
	CancelByRecurringOperationID(ctx context.Context, ownerID, recID uuid.UUID) error
	WithTx(tx transaction.Tx) ReminderRepository
}

// SentSMSReminderRepository tracks successfully sent SMS reminders for audit and deduplication.
type SentSMSReminderRepository interface {
	Save(ctx context.Context, reminderID *uuid.UUID, ownerID uuid.UUID, phone, message, providerResponse string, sentAt time.Time) error
}

// ListFilter controls pagination and optional status filtering for ListByOwner.
type ListFilter struct {
	Status *domain.ReminderStatus
	Limit  int
	Offset int
}

// Notifier dispatches a notification to a recipient.
type Notifier interface {
	Notify(ctx context.Context, n Notification) error
}

// SMSSender sends an SMS message to a phone number.
type SMSSender interface {
	Send(ctx context.Context, phone string, message string) error
}

// ContactResolver resolves the delivery channel and address for an owner.
type ContactResolver interface {
	Resolve(ctx context.Context, ownerID uuid.UUID) (Contact, error)
}

// UserContactProvider returns contact information for a user by ID.
type UserContactProvider interface {
	PhoneByID(ctx context.Context, userID uuid.UUID) (string, error)
}

// Notification is a channel-agnostic outbound message.
type Notification struct {
	RecipientID uuid.UUID
	ReminderID  uuid.UUID
	EventType   domain.EventType
	Title       string
	Body        string
}

// Contact is a resolved delivery endpoint.
type Contact struct {
	Channel Channel
	Address string
}

// Channel identifies a delivery channel.
type Channel string

const (
	ChannelSMS Channel = "sms"
)

// ReminderScheduler is the port used by other bounded contexts to create and cancel reminders transactionally.
type ReminderScheduler interface {
	ScheduleForOperation(ctx context.Context, op OperationInfo, reminderDate time.Time) error
	ScheduleForRecurringOperation(ctx context.Context, rec RecurringOperationInfo, baseReminderDate time.Time, ops []OperationInfo) error
	ScheduleForLease(ctx context.Context, lease LeaseInfo) error
	EnsureRequiresActionReminder(ctx context.Context, lease LeaseInfo) error
	CancelByOperation(ctx context.Context, ownerID, opID uuid.UUID) error
	CancelByRecurringOperation(ctx context.Context, ownerID, recID uuid.UUID) error
	CancelByLease(ctx context.Context, ownerID, leaseID uuid.UUID) error
	WithTx(tx transaction.Tx) ReminderScheduler
}

// OperationInfo describes a concrete operation for reminder scheduling.
type OperationInfo struct {
	ID                   uuid.UUID
	OwnerID              uuid.UUID
	PropertyID           uuid.UUID
	LeaseID              *uuid.UUID
	RecurringOperationID *uuid.UUID
	OperationDate        time.Time
	Type                 string
	Category             string
	AmountKopecks        int64
}

// RecurringOperationInfo describes a recurring operation template for reminder scheduling.
type RecurringOperationInfo struct {
	ID         uuid.UUID
	OwnerID    uuid.UUID
	PropertyID uuid.UUID
	LeaseID    *uuid.UUID
}

// LeaseInfo describes a lease for reminder scheduling.
type LeaseInfo struct {
	ID         uuid.UUID
	OwnerID    uuid.UUID
	PropertyID uuid.UUID
	EndDate    *time.Time
}
