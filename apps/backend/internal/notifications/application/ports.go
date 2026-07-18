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
	SaveOrReplaceOperationReminder(ctx context.Context, r domain.Reminder) error
	UpdateScheduledAt(ctx context.Context, ownerID, id uuid.UUID, scheduledAt time.Time) error
	GetByID(ctx context.Context, id, ownerID uuid.UUID) (domain.Reminder, error)
	GetByIDUnscoped(ctx context.Context, id uuid.UUID) (domain.Reminder, error)
	ListByOwner(ctx context.Context, ownerID uuid.UUID, filter ListFilter) ([]domain.Reminder, error)
	ListByOperation(ctx context.Context, ownerID, operationID uuid.UUID, filter ListFilter) ([]domain.Reminder, error)
	ListByLease(ctx context.Context, ownerID, leaseID uuid.UUID, filter ListFilter) ([]domain.Reminder, error)
	ListByRecurringOperation(ctx context.Context, ownerID, recurringOpID uuid.UUID, filter ListFilter) ([]domain.Reminder, error)
	ListDue(ctx context.Context, before time.Time, limit int) ([]domain.Reminder, error)
	ListStaleSendingReminders(ctx context.Context, staleBefore time.Time, limit int) ([]domain.Reminder, error)
	MarkReminderSending(ctx context.Context, id uuid.UUID) (domain.Reminder, error)
	// MarkSent marks a reminder that is currently sending as sent. It is used
	// after a notification has been dispatched successfully.
	MarkSent(ctx context.Context, id uuid.UUID, at time.Time) error
	// MarkReminderSent marks any pending or sending reminder as sent. It is
	// used when the sent SMS audit row already exists (idempotent success path).
	MarkReminderSent(ctx context.Context, id uuid.UUID, sentAt time.Time) error
	MarkFailed(ctx context.Context, id uuid.UUID, nextAttempt *time.Time, terminal bool) error
	SaveSentSMSReminder(ctx context.Context, id, reminderID, ownerID uuid.UUID, phone, message, providerResponse string, sentAt time.Time) error
	UpdateSMSProviderResponse(ctx context.Context, reminderID uuid.UUID, response string) error
	IsSMSReminderSent(ctx context.Context, reminderID uuid.UUID) (bool, error)
	SaveSentEmailReminder(ctx context.Context, arg SaveSentEmailReminderParams) error
	IsEmailReminderSent(ctx context.Context, reminderID uuid.UUID) (bool, error)
	DeleteSentEmailReminder(ctx context.Context, reminderID uuid.UUID) error
	ResetReminderSending(ctx context.Context, id uuid.UUID) error
	MarkSendingReminderPending(ctx context.Context, id uuid.UUID, nextAttemptAt time.Time) error
	// MarkReminderSkipped marks a pending or sending reminder as skipped. It is
	// used by the worker when the owner revoked permission for the event type.
	MarkReminderSkipped(ctx context.Context, id uuid.UUID) error
	CancelByIDAndOwner(ctx context.Context, ownerID, reminderID uuid.UUID) (bool, error)
	CancelByTarget(ctx context.Context, ownerID uuid.UUID, targetType domain.TargetType, targetID uuid.UUID, eventType domain.EventType) error
	CancelByRecurringOperationID(ctx context.Context, ownerID, recID uuid.UUID) error
	HasReminderForLeaseEvent(ctx context.Context, ownerID, leaseID uuid.UUID, eventType domain.EventType) (bool, error)
	HasReminderForOperationEvent(ctx context.Context, ownerID, operationID uuid.UUID, eventType domain.EventType) (bool, error)
	// ListPreferences returns the stored notification preference rows of a
	// user. A missing row means the event type is allowed (opt-out model).
	ListPreferences(ctx context.Context, userID uuid.UUID) ([]domain.NotificationPreference, error)
	// UpsertPreference inserts or updates one notification preference row.
	UpsertPreference(ctx context.Context, userID uuid.UUID, pref domain.NotificationPreference) error
	// IsEventAllowed reports whether the user permits sending reminders of the
	// given event type. A missing row means allowed.
	IsEventAllowed(ctx context.Context, userID uuid.UUID, eventType domain.EventType) (bool, error)
	WithTx(tx transaction.Tx) ReminderRepository
}

// ListFilter controls pagination and optional status filtering for ListByOwner.
type ListFilter struct {
	Status *domain.ReminderStatus
	Limit  int
	Offset int
}

// SaveSentEmailReminderParams contains the data recorded when an email reminder is sent.
type SaveSentEmailReminderParams struct {
	ID         uuid.UUID
	ReminderID uuid.UUID
	OwnerID    uuid.UUID
	Email      string
	Subject    string
	PlainBody  string
	SentAt     time.Time
}

// Notifier dispatches a notification to a recipient.
type Notifier interface {
	Notify(ctx context.Context, n Notification) (providerResponse, renderedPlainBody string, err error)
}

// SMSSender sends an SMS message to a phone number.
type SMSSender interface {
	Send(ctx context.Context, phone string, message string) (providerResponse string, err error)
}

// ContactResolver resolves the delivery channel and address for an owner.
type ContactResolver interface {
	Resolve(ctx context.Context, ownerID uuid.UUID) (Contact, error)
}

// Notification is a channel-agnostic outbound message.
type Notification struct {
	RecipientID uuid.UUID
	ReminderID  uuid.UUID
	EventType   domain.EventType
	Title       string
	Body        string
	Contact     *Contact
}

// Contact is a resolved delivery endpoint.
type Contact struct {
	Channel Channel
	Phone   string
	Email   string
}

// Channel identifies a delivery channel.
type Channel string

const (
	ChannelSMS   Channel = "sms"
	ChannelEmail Channel = "email"
)

// ReminderScheduler is the port used by other bounded contexts to create and cancel reminders transactionally.
type ReminderScheduler interface {
	ScheduleForOperation(ctx context.Context, op OperationInfo, reminderDate time.Time) error
	ScheduleOverdueReminder(ctx context.Context, op OperationInfo, reminderDate time.Time) error
	ScheduleForRecurringOperation(ctx context.Context, rec RecurringOperationInfo, baseReminderDate time.Time, ops []OperationInfo) error
	ScheduleForLease(ctx context.Context, lease LeaseInfo) error
	EnsureRequiresActionReminder(ctx context.Context, lease LeaseInfo) error
	CancelByOperation(ctx context.Context, ownerID, opID uuid.UUID) error
	CancelOverdueReminderByOperation(ctx context.Context, ownerID, opID uuid.UUID) error
	CancelByRecurringOperation(ctx context.Context, ownerID, recID uuid.UUID) error
	CancelByLease(ctx context.Context, ownerID, leaseID uuid.UUID) error
	HasReminderForOperationEvent(ctx context.Context, ownerID, operationID uuid.UUID, eventType domain.EventType) (bool, error)
	ListByOperation(ctx context.Context, ownerID, operationID uuid.UUID, filter ListFilter) ([]domain.Reminder, error)
	ListByLease(ctx context.Context, ownerID, leaseID uuid.UUID, filter ListFilter) ([]domain.Reminder, error)
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
	CategoryName         string
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
