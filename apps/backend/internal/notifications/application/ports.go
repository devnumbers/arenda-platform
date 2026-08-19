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
	UpdateScheduledAt(ctx context.Context, scope, id uuid.UUID, scheduledAt time.Time) error
	// ReschedulePendingRemindersByOwner recalculates the scheduled_at of all
	// pending reminders for an owner using wall-clock timezone conversion.
	ReschedulePendingRemindersByOwner(ctx context.Context, scope uuid.UUID, oldTZ, newTZ string) error
	GetByID(ctx context.Context, id, scope uuid.UUID) (domain.Reminder, error)
	GetByIDUnscoped(ctx context.Context, id uuid.UUID) (domain.Reminder, error)
	// ListByOwner returns reminders for the scope owner plus, when
	// accessiblePropertyIDs is non-empty, reminders of properties shared with
	// the actor via property membership (issue #157, T3), excluding archived
	// shared properties. When empty, only the owner's own reminders are returned.
	ListByOwner(ctx context.Context, scope uuid.UUID, filter ListFilter, accessiblePropertyIDs []uuid.UUID) ([]domain.Reminder, error)
	ListByOperation(ctx context.Context, scope, operationID uuid.UUID, filter ListFilter) ([]domain.Reminder, error)
	ListByLease(ctx context.Context, scope, leaseID uuid.UUID, filter ListFilter) ([]domain.Reminder, error)
	ListByRecurringOperation(ctx context.Context, scope, recurringOpID uuid.UUID, filter ListFilter) ([]domain.Reminder, error)
	ListDue(ctx context.Context, before time.Time, limit int) ([]domain.Reminder, error)
	ListStaleSendingReminders(ctx context.Context, staleBefore time.Time, limit int) ([]domain.Reminder, error)
	// ListCalendarByOwner returns non-cancelled, non-skipped operation and
	// system reminders for an owner in the half-open time window [from, to),
	// each with its resolved property name (nil for orphans). Ordered by
	// scheduled_at ascending. The accessiblePropertyIDs parameter extends the
	// result with reminders of properties shared with the actor (issue #157,
	// T3), restricted to active/maintenance properties; when empty, only the
	// owner's own reminders are returned.
	ListCalendarByOwner(
		ctx context.Context,
		scope uuid.UUID,
		from, to time.Time,
		accessiblePropertyIDs []uuid.UUID,
	) ([]domain.CalendarReminder, error)
	MarkReminderSending(ctx context.Context, id uuid.UUID) (domain.Reminder, error)
	// MarkSent marks a reminder that is currently sending as sent. It is used
	// after a notification has been dispatched successfully.
	MarkSent(ctx context.Context, id uuid.UUID, at time.Time) error
	// MarkReminderSent marks any pending or sending reminder as sent. It is
	// used when the sent SMS audit row already exists (idempotent success path).
	MarkReminderSent(ctx context.Context, id uuid.UUID, sentAt time.Time) error
	MarkFailed(ctx context.Context, id uuid.UUID, nextAttempt *time.Time, terminal bool) error
	SaveSentSMSReminder(ctx context.Context, id, reminderID, scope uuid.UUID, phone, message, providerResponse string, sentAt time.Time) error
	UpdateSMSProviderResponse(ctx context.Context, reminderID uuid.UUID, response string) error
	IsSMSReminderSent(ctx context.Context, reminderID uuid.UUID) (bool, error)
	SaveSentEmailReminder(ctx context.Context, arg SaveSentEmailReminderParams) error
	IsEmailReminderSent(ctx context.Context, reminderID, recipientID uuid.UUID) (bool, error)
	DeleteSentEmailReminder(ctx context.Context, reminderID, recipientID uuid.UUID) error
	// IsPushReminderSent reports whether a push audit row already exists for the
	// given reminder and recipient (per-recipient deduplication for the push
	// channel).
	IsPushReminderSent(ctx context.Context, reminderID, recipientID uuid.UUID) (bool, error)
	// SaveSentPushReminder records a successfully sent push reminder for audit
	// and deduplication. It returns ErrDuplicatePushReminder when an audit row
	// for the same (reminder, recipient) pair already exists.
	SaveSentPushReminder(ctx context.Context, arg SaveSentPushReminderParams) error
	// DeleteSentPushReminder removes the push audit row for a reminder and
	// recipient. It is used to roll back the audit insert when the external
	// send fails.
	DeleteSentPushReminder(ctx context.Context, reminderID, recipientID uuid.UUID) error
	ResetReminderSending(ctx context.Context, id uuid.UUID) error
	MarkSendingReminderPending(ctx context.Context, id uuid.UUID, nextAttemptAt time.Time) error
	// MarkReminderSkipped marks a pending or sending reminder as skipped. It is
	// used by the worker when the owner revoked permission for the event type.
	MarkReminderSkipped(ctx context.Context, id uuid.UUID) error
	CancelByIDAndOwner(ctx context.Context, scope, reminderID uuid.UUID) (bool, error)
	CancelByTarget(ctx context.Context, scope uuid.UUID, targetType domain.TargetType, targetID uuid.UUID, eventType domain.EventType) error
	CancelByRecurringOperationID(ctx context.Context, scope, recID uuid.UUID) error
	HasReminderForLeaseEvent(ctx context.Context, scope, leaseID uuid.UUID, eventType domain.EventType) (bool, error)
	HasReminderForOperationEvent(ctx context.Context, scope, operationID uuid.UUID, eventType domain.EventType) (bool, error)
	// ListChannelPreferences returns the stored per-channel preference rows of
	// a user (ADR 0030). A missing row means the (event type, channel) pair is
	// allowed.
	ListChannelPreferences(ctx context.Context, userID uuid.UUID) ([]domain.NotificationChannelPreference, error)
	// UpsertChannelPreference inserts or updates one per-channel preference row.
	UpsertChannelPreference(ctx context.Context, userID uuid.UUID, pref domain.NotificationChannelPreference) error
	// IsChannelAllowed reports whether the user permits sending reminders of the
	// given event type over the given channel. A missing row means allowed.
	IsChannelAllowed(ctx context.Context, userID uuid.UUID, eventType domain.EventType, channel domain.NotificationChannel) (bool, error)
	WithTx(tx transaction.Tx) ReminderRepository
}

// PushSubscriptionRepository persists Web Push subscriptions keyed by their
// browser-issued endpoint URL.
type PushSubscriptionRepository interface {
	// Upsert inserts a subscription keyed by endpoint, or updates its mutable
	// fields when the endpoint already exists (idempotent re-subscribe).
	Upsert(ctx context.Context, sub domain.PushSubscription) (domain.PushSubscription, error)
	// Delete removes a subscription by endpoint scoped to a user. It returns
	// ErrNotFound when no row matched.
	Delete(ctx context.Context, userID uuid.UUID, endpoint string) error
	// ListByUser returns all stored subscriptions for a user.
	ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.PushSubscription, error)
}

// SharedPropertyIDs returns the ids of properties shared with a user via
// property membership (issue #157). It mirrors
// leases/application.SharedPropertyIDs locally to avoid a cross-context import;
// it is implemented by the access bounded context and injected optionally into
// CalendarService so the agenda can include reminders of the actor's shared
// properties. When nil, only the actor's own reminders are returned (the
// pre-T3 behaviour).
type SharedPropertyIDs interface {
	SharedWith(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
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
	// ScopeID is the user_id of the recipient the email was sent to. Since the
	// per-recipient fan-out (issue #159) it is not necessarily the reminder
	// owner: one audit row is stored per recipient.
	ScopeID   uuid.UUID
	Email     string
	Subject   string
	PlainBody string
	SentAt    time.Time
}

// SaveSentPushReminderParams contains the data recorded when a push reminder is
// sent to a recipient. One row per (reminder, recipient) enables per-recipient
// deduplication across dispatch retries.
type SaveSentPushReminderParams struct {
	ID         uuid.UUID
	ReminderID uuid.UUID
	// RecipientID is the user_id of the recipient the push was sent to. It is
	// not necessarily the reminder owner: one audit row is stored per recipient
	// (mirrors the email per-recipient fan-out, issue #159).
	RecipientID uuid.UUID
	SentAt      time.Time
}

// Notifier dispatches a notification to a recipient.
type Notifier interface {
	Notify(ctx context.Context, n Notification) (providerResponse, renderedPlainBody string, err error)
}

// DirectEmailSender renders and sends a one-off email outside the reminder
// lifecycle (issue #253): the direct-notification service owns subject and
// content, the adapter owns templates and transport.
type DirectEmailSender interface {
	SendDirect(ctx context.Context, to, subject, template string, data map[string]any) error
}

// SMSSender sends an SMS message to a phone number.
type SMSSender interface {
	Send(ctx context.Context, phone, message string) (providerResponse string, err error)
}

// PushSender dispatches a single Web Push message to one browser subscription.
// It encrypts the payload (RFC 8291) and sends it to the push service endpoint
// identified by the subscription. Domain errors signal the outcome:
// ErrSubscriptionGone (404/410) — the subscription is dead and must be deleted;
// ErrRateLimited (429) — the push service throttled the request; a non-nil
// error otherwise means the send failed (the caller may retry on 5xx).
type PushSender interface {
	Send(ctx context.Context, subscription domain.PushSubscription, payload PushPayload) error
}

// ContactResolver resolves the delivery channel and address for an owner.
type ContactResolver interface {
	Resolve(ctx context.Context, scope uuid.UUID) (Contact, error)
}

// PropertyRecipientLister lists users (besides the owner) who actively share
// the property and must receive its reminders (issue #159).
type PropertyRecipientLister interface {
	ListActiveRecipientIDs(ctx context.Context, propertyID uuid.UUID) ([]uuid.UUID, error)
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
	ChannelPush  Channel = "push"
)

// ReminderScheduler is the port used by other bounded contexts to create and cancel reminders transactionally.
type ReminderScheduler interface {
	ScheduleForOperation(ctx context.Context, op OperationInfo, reminderDate time.Time) error
	ScheduleOverdueReminder(ctx context.Context, op OperationInfo, reminderDate time.Time) error
	ScheduleForRecurringOperation(ctx context.Context, rec RecurringOperationInfo, baseReminderDate time.Time, ops []OperationInfo) error
	ScheduleForLease(ctx context.Context, lease LeaseInfo) error
	EnsureRequiresActionReminder(ctx context.Context, lease LeaseInfo) error
	CancelByOperation(ctx context.Context, scope, opID uuid.UUID) error
	CancelOverdueReminderByOperation(ctx context.Context, scope, opID uuid.UUID) error
	CancelByRecurringOperation(ctx context.Context, scope, recID uuid.UUID) error
	CancelByLease(ctx context.Context, scope, leaseID uuid.UUID) error
	HasReminderForOperationEvent(ctx context.Context, scope, operationID uuid.UUID, eventType domain.EventType) (bool, error)
	ListByOperation(ctx context.Context, scope, operationID uuid.UUID, filter ListFilter) ([]domain.Reminder, error)
	ListByLease(ctx context.Context, scope, leaseID uuid.UUID, filter ListFilter) ([]domain.Reminder, error)
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
