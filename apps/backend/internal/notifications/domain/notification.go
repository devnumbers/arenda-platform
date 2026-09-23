// Package domain defines the notifications vocabulary: the stored feed
// (карта #734, модель — решение #737), the delivery channels and push
// subscriptions.
package domain

import (
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// EventType identifies the kind of a notification: a concrete feed event
// belonging to a category (FeedCategory). The rental-era and direct-channel
// values of the PostgreSQL enum stay as dead rows forever (#438, #277 —
// free_reminder, subscription_grace and friends) — the domain does not know
// them.
type EventType string

// Feed catalog v1 (решение #737, дополненный #741) in catalog order: the
// category of each value is the authoritative FeedCategory mapping — Аренда;
// Платежи и операции; Задачи; Совместный доступ; Тариф; Системные.
const (
	EventRentalCompleted              EventType = "rental_completed"
	EventPaymentDue                   EventType = "payment_due"
	EventPaymentOverdue               EventType = "payment_overdue"
	EventPaymentReminder              EventType = "payment_reminder"
	EventTaskOverdue                  EventType = "task_overdue"
	EventPropertyInvitation           EventType = "property_invitation"
	EventInvitationAccepted           EventType = "invitation_accepted"
	EventAccessRevoked                EventType = "access_revoked"
	EventAccessPaused                 EventType = "access_paused"
	EventAccessResumed                EventType = "access_resumed"
	EventMemberLeft                   EventType = "member_left"
	EventSubscriptionPaymentFailed    EventType = "subscription_payment_failed"
	EventSubscriptionPaymentReminder  EventType = "subscription_payment_reminder"
	EventSubscriptionPaymentSucceeded EventType = "subscription_payment_succeeded"
	EventSubscriptionPlanChanged      EventType = "subscription_plan_changed"
	EventSubscriptionGraceEntered     EventType = "subscription_grace_entered"
	EventSubscriptionGraceExpiring    EventType = "subscription_grace_expiring"
	EventSystemMaintenance            EventType = "system_maintenance"
)

// feedCatalog is the single source of the catalog v1 (решение #737): every
// feed event type with its category and its possible actions. Adding a
// catalog v2 event is one entry here plus the migration's enum value.
var feedCatalog = map[EventType]struct {
	category Category
	actions  []ActionKind
}{
	EventRentalCompleted: {CategoryRental, []ActionKind{ActionRentalExtend, ActionRentalComplete}},
	EventPaymentDue:      {CategoryPaymentsOperations, []ActionKind{ActionOpenPayment}},
	EventPaymentOverdue:  {CategoryPaymentsOperations, []ActionKind{ActionOpenPayment}},
	// «Напоминание о платеже» (карта #822, #824): та же категория и кнопка,
	// что у платёжной пары решения #737 — переход на страницу правила.
	EventPaymentReminder:              {CategoryPaymentsOperations, []ActionKind{ActionOpenPayment}},
	EventTaskOverdue:                  {CategoryTasks, []ActionKind{ActionOpenTask}},
	EventPropertyInvitation:           {CategorySharedAccess, []ActionKind{ActionOpenProperty}},
	EventInvitationAccepted:           {CategorySharedAccess, []ActionKind{ActionOpenPropertyMembers}},
	EventAccessRevoked:                {CategorySharedAccess, nil},
	EventAccessPaused:                 {CategorySharedAccess, nil},
	EventAccessResumed:                {CategorySharedAccess, nil},
	EventMemberLeft:                   {CategorySharedAccess, nil},
	EventSubscriptionPaymentFailed:    {CategoryTariff, []ActionKind{ActionOpenTariffs}},
	EventSubscriptionPaymentReminder:  {CategoryTariff, []ActionKind{ActionOpenTariffs}},
	EventSubscriptionPaymentSucceeded: {CategoryTariff, nil},
	EventSubscriptionPlanChanged:      {CategoryTariff, []ActionKind{ActionOpenTariffs}},
	EventSubscriptionGraceEntered:     {CategoryTariff, []ActionKind{ActionOpenPaymentMethods}},
	EventSubscriptionGraceExpiring:    {CategoryTariff, []ActionKind{ActionOpenPaymentMethods}},
	EventSystemMaintenance:            {CategorySystem, nil},
}

// FeedCategory returns the event type's category per the catalog v1. The
// second return is false for event types outside the feed catalog — the
// dead enum values among them.
func (e EventType) FeedCategory() (Category, bool) {
	entry, ok := feedCatalog[e]
	if !ok {
		return "", false
	}
	return entry.category, true
}

// Category is the notification category (Категория уведомлений, решение
// #737): the group behind the per-channel settings and the feed icon. Four
// categories are user-configurable (rental, payments_operations, tasks,
// shared_access); Tariff and System are service categories — always on,
// outside the settings screen.
type Category string

const (
	CategoryRental             Category = "rental"
	CategoryPaymentsOperations Category = "payments_operations"
	CategoryTasks              Category = "tasks"
	CategorySharedAccess       Category = "shared_access"
	CategoryTariff             Category = "tariff"
	CategorySystem             Category = "system"
)

// ActionKind is a feed action button (Действие, решение #737): a transition
// to an entity's screen, never a mutation. Availability is computed at read
// time from the entity's live state and the reader's role (#743) — the feed
// stores none of it; ActionsForEvent only fixes what is possible per event.
type ActionKind string

const (
	ActionRentalExtend        ActionKind = "rental_extend"
	ActionRentalComplete      ActionKind = "rental_complete"
	ActionOpenPayment         ActionKind = "open_payment"
	ActionOpenTask            ActionKind = "open_task"
	ActionOpenProperty        ActionKind = "open_property"
	ActionOpenPropertyMembers ActionKind = "open_property_members"
	ActionOpenTariffs         ActionKind = "open_tariffs"
	// ActionOpenPaymentMethods opens the payment-methods screen — where the
	// user fixes a failed renewal charge (the grace events' fix screen).
	ActionOpenPaymentMethods ActionKind = "open_payment_methods"
)

// ActionsForEvent returns the action buttons the event type may carry, in
// display order. An empty result means the event renders without buttons.
// What is possible is catalog knowledge; what is available right now is the
// reading side's call (#743).
func ActionsForEvent(e EventType) []ActionKind {
	return slices.Clone(feedCatalog[e].actions)
}

// EntityRef is a payload link with a name snapshot: the id for navigation,
// the name for rendering cards at read time even after the entity is renamed
// or gone. Card lines beyond the name are the card's vocabulary (решение
// владельца 19.09.2026, #745): the property carries its address, the actor —
// their email; both are snapshots of the publication moment, optional, and
// the card simply skips a line the snapshot does not hold.
type EntityRef struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	Address string    `json:"address,omitempty"`
	Email   string    `json:"email,omitempty"`
}

// TariffRef is the tariff snapshot of the billing events (слаг, период,
// сумма, дата до). The amount is BIGINT kopecks (ADR 0008).
type TariffRef struct {
	Slug          string     `json:"slug"`
	Period        string     `json:"period"`
	AmountKopecks int64      `json:"amount_kopecks"`
	ActiveUntil   *time.Time `json:"active_until,omitempty"`
}

// Payload carries the feed row's links and name snapshots (решение #737):
// property (id + имя-снимок), actor, tariff, and the bare entity ids of the
// transition targets. All fields are optional — the text snapshot in
// title/body/context_label is self-sufficient, the payload only adds
// navigation and card rendering.
type Payload struct {
	Property  *EntityRef `json:"property,omitempty"`
	Actor     *EntityRef `json:"actor,omitempty"`
	RentalID  *uuid.UUID `json:"rental_id,omitempty"`
	PaymentID *uuid.UUID `json:"payment_id,omitempty"`
	// PaymentDate is the payment events' operation date (решение #737:
	// payload payment = id правила + дата операции) — the calendar date the
	// «Оплатить» button's live check pins the occurrence to (#743).
	PaymentDate *time.Time `json:"payment_date,omitempty"`
	TaskID      *uuid.UUID `json:"task_id,omitempty"`
	// TaskRuleID is the task's producing rule (the tasks events, #750): an
	// active task's screens are its rule's screens, so the «переход к
	// задаче» navigation needs the rule id alongside the task id.
	TaskRuleID   *uuid.UUID `json:"task_rule_id,omitempty"`
	MembershipID *uuid.UUID `json:"membership_id,omitempty"`
	Tariff       *TariffRef `json:"tariff,omitempty"`
}

// DedupKey is the publication dedup key (Дедуп-ключ, решение #737): the
// (event type, entity, phase/date) half of the pair whose recipient half is
// the row's own user. A repeat publication with the same key never creates a
// second row — the unique index (user_id, dedup_key) makes the invariant
// durable. Key formats are the publishers' vocabulary (#740, #748–#752).
type DedupKey string

// NewDedupKey validates and returns a dedup key: a blank key would collapse
// all of a recipient's publications into one row.
func NewDedupKey(raw string) (DedupKey, error) {
	if strings.TrimSpace(raw) == "" {
		return "", ErrInvalidNotification
	}
	return DedupKey(raw), nil
}

// Notification is one recipient's feed row (Уведомление, решение #737): the
// text snapshot (title, body, context label), the payload links, and the
// personal flags (read, deleted). One event = one row per recipient (fan-out);
// rewriting a template never touches stored rows.
type Notification struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Category  Category
	EventType EventType
	Title     string
	Body      string
	// ContextLabel is the optional line above the title (имя объекта/тарифа,
	// «Системные уведомления»); the empty string means no label (SQL NULL).
	ContextLabel string
	Payload      Payload
	DedupKey     DedupKey
	ReadAt       *time.Time
	DeletedAt    *time.Time
	CreatedAt    time.Time
}

// NewNotification creates a feed row for one recipient. The id is the
// caller's app-generated UUIDv7 (ADR 0019); the category is derived from the
// event type — a row cannot disagree with the catalog.
// ReadAt/DeletedAt/CreatedAt start empty: they belong to the database and
// the reading side, not to publication.
func NewNotification(
	id, userID uuid.UUID,
	eventType EventType,
	title, body, contextLabel string,
	payload Payload,
	dedupKey DedupKey,
) (*Notification, error) {
	if id == uuid.Nil || userID == uuid.Nil {
		return nil, ErrInvalidNotification
	}
	category, ok := eventType.FeedCategory()
	if !ok {
		return nil, ErrInvalidNotification
	}
	if _, err := NewDedupKey(string(dedupKey)); err != nil {
		return nil, ErrInvalidNotification
	}
	if title == "" || body == "" {
		return nil, ErrInvalidNotification
	}
	return &Notification{
		ID:           id,
		UserID:       userID,
		Category:     category,
		EventType:    eventType,
		Title:        title,
		Body:         body,
		ContextLabel: contextLabel,
		Payload:      payload,
		DedupKey:     dedupKey,
	}, nil
}
