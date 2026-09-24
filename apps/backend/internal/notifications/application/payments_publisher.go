package application

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
)

// PaymentScheduleTarget is one upcoming boundary the payments scan books
// (issue #776): the rule id, the operation date — the job args' identity —
// and the boundary's instant in the owner's timezone, the job's ScheduledAt.
// The due leg's instant is 00:00 of the operation date, the reminder leg's —
// 00:00 of the operation date minus the rule's lead time (карта #822), the
// overdue leg's — 00:00 of the day after (решение владельца 21.09.2026: все
// уведомления — чётко по времени).
type PaymentScheduleTarget struct {
	PaymentID uuid.UUID
	DueDate   time.Time
	FireAt    time.Time
}

// PaymentScanTarget is one operation the payments scan fires for: a planned
// occurrence (Операция — вхождение Payments) of a live rule on a
// non-archived property, with the property snapshot the publication carries
// (решение владельца 19.09.2026, #745 — the address line travels in the
// snapshot).
type PaymentScanTarget struct {
	// PaymentID is the rule's id (Платёж): the payload link — the
	// «Оплатить» button and the channels' deep link lead to the payment's
	// page — and the dedup key's entity half (решение #737: payment =
	// id правила + дата операции).
	PaymentID uuid.UUID
	// DueDate is the operation's calendar date (DATE): the dedup key's date
	// half and the trigger's datum — the due leg fires on it, the overdue
	// leg strictly after it.
	DueDate         time.Time
	Title           string
	AmountKopecks   int64
	PropertyID      uuid.UUID
	PropertyName    string
	PropertyAddress string
	OwnerID         uuid.UUID
}

// PaymentScanSource is the payments scan's window into the payments context:
// the due, the reminder and the overdue targets of one zone and the
// property's active members. Consumer-declared (CODING_STANDARDS), answered
// over the owning tables by the notifications postgres adapter.
type PaymentScanSource interface {
	// ListDueTargets lists the zone's planned operations dated exactly the
	// zone's today (решение #737, тип №2: в день срока) on rules without the
	// auto-pay mode — an auto-pay rule's due occurrence is extinguished by
	// the tick the same day and never asks to be paid; if the auto charge
	// did not happen, the occurrence becomes overdue and the overdue leg
	// speaks instead.
	ListDueTargets(ctx context.Context, zone string, today time.Time) ([]PaymentScanTarget, error)
	// ListOverdueTargets lists the zone's planned operations dated strictly
	// before the zone's today (решение #737, тип №3: 1-й день просрочки) —
	// auto-pay rules included, the tick never backdates an auto charge
	// (ADR 0049).
	ListOverdueTargets(ctx context.Context, zone string, today time.Time) ([]PaymentScanTarget, error)
	// ListActiveRecipients lists the user ids of the property's active
	// members — the recipients besides the owner, «Просмотр» included
	// (решение #737: получатели объектных событий).
	ListActiveRecipients(ctx context.Context, propertyID uuid.UUID) ([]uuid.UUID, error)
	// ListScheduledDueTargets lists the upcoming operations whose due
	// boundary — 00:00 of the operation date in the owner's timezone —
	// falls in the window (from, until]; the booking list of the due leg
	// (issue #776), auto-pay rules excluded like the sweep's due leg.
	ListScheduledDueTargets(ctx context.Context, from, until time.Time) ([]PaymentScheduleTarget, error)
	// ListScheduledOverdueTargets lists the upcoming operations whose
	// overdue boundary — 00:00 of the day after the operation date in the
	// owner's timezone — falls in the window (from, until]; the booking list
	// of the overdue leg (issue #776), auto-pay rules included.
	ListScheduledOverdueTargets(ctx context.Context, from, until time.Time) ([]PaymentScheduleTarget, error)
	// ListReminderTargets lists the zone's planned operations of rules with
	// a reminder set whose reminder day — the operation date minus the
	// rule's lead time (1/3/7) — is exactly the zone's today (карта #822,
	// #824): auto-pay rules included, the reminder lives independently of
	// auto_pay (решение владельца, #823).
	ListReminderTargets(ctx context.Context, zone string, today time.Time) ([]PaymentScanTarget, error)
	// ListScheduledReminderTargets lists the upcoming operations of rules
	// with a reminder set whose reminder boundary — 00:00 of (operation
	// date − lead time) in the owner's timezone — falls in the window
	// (from, until]; the booking list of the reminder leg (карта #822).
	ListScheduledReminderTargets(ctx context.Context, from, until time.Time) ([]PaymentScheduleTarget, error)
	// GetScheduledDuePayment reloads one operation at its due boundary —
	// the due job's delivery-time resolution (issue #776). The live flag is
	// false when the leg's conditions no longer hold as of now: the
	// operation gone, paid, cancelled, moved to auto-pay, the property
	// archived, or the operation date no longer the zone's today.
	GetScheduledDuePayment(ctx context.Context, paymentID uuid.UUID, date, now time.Time) (PaymentScanTarget, bool, error)
	// GetScheduledOverduePayment reloads one operation at its overdue
	// boundary — the overdue job's delivery-time resolution (issue #776).
	// The live flag is false when the operation is no longer a planned
	// occurrence of the rule on a non-archived property, or the operation
	// date is not strictly before the zone's today as of now.
	GetScheduledOverduePayment(ctx context.Context, paymentID uuid.UUID, date, now time.Time) (PaymentScanTarget, bool, error)
	// GetScheduledReminderPayment reloads one operation at its reminder
	// boundary (карта #822) — the reminder job's delivery-time resolution.
	// The live flag is false when the operation is no longer a planned
	// occurrence of a reminder-carrying rule on a non-archived property, or
	// the rule's reminder day (date − its current lead time) is no longer
	// the zone's today — a lead time changed after the booking sends the
	// stale job away quietly, the new boundary books its own job.
	GetScheduledReminderPayment(ctx context.Context, paymentID uuid.UUID, date, now time.Time) (PaymentScanTarget, bool, error)
}

// PaymentBoundaryScheduler books a leg's boundary job on the delivery queue
// (issue #776). Booking is idempotent — the queue's unique key keeps one
// in-flight job per (leg, rule, date) — so the hourly passes repeat their
// asks freely.
type PaymentBoundaryScheduler interface {
	// SchedulePaymentDue books the «Оплатите платёж» job at the boundary's
	// instant: 00:00 of the operation date in the owner's timezone.
	SchedulePaymentDue(ctx context.Context, paymentID uuid.UUID, date, fireAt time.Time) error
	// SchedulePaymentOverdue books the «Платёж просрочен» job at the
	// boundary's instant: 00:00 of the day after the operation date.
	SchedulePaymentOverdue(ctx context.Context, paymentID uuid.UUID, date, fireAt time.Time) error
	// SchedulePaymentReminder books the «Напоминание о платеже» job at the
	// boundary's instant: 00:00 of (operation date − lead time) in the
	// owner's timezone (карта #822).
	SchedulePaymentReminder(ctx context.Context, paymentID uuid.UUID, date, fireAt time.Time) error
}

// PaymentBoundaryDeliverer is the boundary jobs' call into the publisher:
// a boundary worker reloads the operation through the source as of its
// wake-up and publishes — or finishes without publishing when the reload
// answers no live operation.
type PaymentBoundaryDeliverer interface {
	DeliverPaymentDue(ctx context.Context, paymentID uuid.UUID, date, now time.Time) error
	DeliverPaymentOverdue(ctx context.Context, paymentID uuid.UUID, date, now time.Time) error
	DeliverPaymentReminder(ctx context.Context, paymentID uuid.UUID, date, now time.Time) error
}

// PaymentsPublisher is the payments events' publisher (issue #749) on the
// delivery pipeline (карта #734, #740): the operations that came due today —
// «Оплатите платёж» — and the ones whose due date has passed unpaid —
// «Платёж просрочен» — reach the owner and the active members with each
// event's row. The copy, the payload and the dedup keys are the publisher's
// (решение #737, типы №2–№3); the «Оплатить» button is nobody's here — it
// computes at read time from the live state (#743).
//
// The trigger has two mechanisms (issue #776, the tasks scan's canon of
// #750): the hourly pass books the boundary jobs — the due leg's at 00:00 of
// the operation date, the reminder leg's at 00:00 of the operation date
// minus the rule's lead time (карта #822), the overdue leg's at 00:00 of the
// day after, each in the owner's timezone — and the zone sweep stays the
// backstop for everything the jobs missed (a pass delayed past a boundary, a
// booking window not reached yet, the retrospective after a downtime). In
// the norm the job wakes exactly at the boundary and the sweep's publication
// is silent — the dedup key carries the idempotence, no sweep state does.
//
// The sweep mirrors the materialization ticks (ADR 0048 p.3): one today per
// owner timezone, failures isolated per zone and per operation, and
// idempotence carried by the dedup key instead of sweep state — the due leg
// fires once per (rule, date) because the key outlives the day, and an
// overdue operation sweeps daily without ever producing a second row.
type PaymentsPublisher struct {
	pipeline  *Publisher
	zones     ScanZoneDirectory
	source    PaymentScanSource
	scheduler PaymentBoundaryScheduler
}

// NewPaymentsPublisher builds the payments publisher over the pipeline
// creation service, the zone directory, the scan source and the boundary
// scheduler.
func NewPaymentsPublisher(
	pipeline *Publisher, zones ScanZoneDirectory, source PaymentScanSource, scheduler PaymentBoundaryScheduler,
) *PaymentsPublisher {
	return &PaymentsPublisher{pipeline: pipeline, zones: zones, source: source, scheduler: scheduler}
}

// RunZoneScans is the hourly sweep: book the upcoming boundaries' jobs
// first — every hour counts down to their midnight — then sweep the zones,
// publishing the zone's due, reminder and overdue targets. Failures are
// isolated — a broken booking, zone or operation does not stop the rest;
// the joined error reports everything that failed.
func (p *PaymentsPublisher) RunZoneScans(ctx context.Context, now time.Time) error {
	if p.zones == nil {
		return errors.New("notifications payments scan: zone directory must be configured")
	}
	errs := append([]error{}, p.scheduleBoundaries(ctx, now)...)
	if err := p.sweepZones(ctx, now); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// scheduleBoundaries books the boundary jobs of the operations whose
// boundaries fall into the horizon window — the due leg's ahead of the
// operation date's midnight, the reminder leg's ahead of the operation date
// minus the rule's lead time (карта #822), the overdue leg's ahead of the
// day after's.
// A broken booking is isolated — the rest of the window books on.
func (p *PaymentsPublisher) scheduleBoundaries(ctx context.Context, now time.Time) []error {
	var errs []error
	for _, leg := range []struct {
		list    func(context.Context, time.Time, time.Time) ([]PaymentScheduleTarget, error)
		book    func(context.Context, uuid.UUID, time.Time, time.Time) error
		diagnos string
	}{
		{p.source.ListScheduledDueTargets, p.scheduler.SchedulePaymentDue, "due"},
		{p.source.ListScheduledReminderTargets, p.scheduler.SchedulePaymentReminder, "reminder"},
		{p.source.ListScheduledOverdueTargets, p.scheduler.SchedulePaymentOverdue, "overdue"},
	} {
		targets, err := leg.list(ctx, now, now.Add(scheduledHorizon))
		if err != nil {
			errs = append(errs, fmt.Errorf("list %s payment boundaries: %w", leg.diagnos, err))
			continue
		}
		for _, target := range targets {
			if err := leg.book(ctx, target.PaymentID, target.DueDate, target.FireAt); err != nil {
				errs = append(errs, fmt.Errorf("schedule %s boundary of payment %s: %w",
					leg.diagnos, target.PaymentID, err))
			}
		}
	}
	return errs
}

// sweepZones is the zone sweep (ADR 0048 p.3): one today per owner timezone,
// every leg's targets published, in catalog order — due, reminder (карта
// #822), overdue.
func (p *PaymentsPublisher) sweepZones(ctx context.Context, now time.Time) error {
	return forEachZone(ctx, p.zones, now, "payments", func(zone string, today time.Time) error {
		// A broken leg's list is isolated — the other legs still run.
		var errs []error
		for _, leg := range []struct {
			list    func(context.Context, string, time.Time) ([]PaymentScanTarget, error)
			event   domain.EventType
			diagnos string
		}{
			{p.source.ListDueTargets, domain.EventPaymentDue, "due"},
			{p.source.ListReminderTargets, domain.EventPaymentReminder, "reminder"},
			{p.source.ListOverdueTargets, domain.EventPaymentOverdue, "overdue"},
		} {
			targets, err := leg.list(ctx, zone, today)
			if err != nil {
				errs = append(errs, fmt.Errorf("list %s payments of zone %s: %w", leg.diagnos, zone, err))
				continue
			}
			for _, target := range targets {
				if err := p.publish(ctx, leg.event, target); err != nil {
					errs = append(errs, fmt.Errorf("publish %s payment %s of zone %s: %w",
						leg.diagnos, target.PaymentID, zone, err))
				}
			}
		}
		return errors.Join(errs...)
	})
}

// DeliverPaymentDue is the due boundary job's half (issue #776): reload the
// operation through the source as of the wake-up and publish if it still
// lives due. A false live flag — paid, cancelled, the rule edited or
// deleted, the property archived, or the operation date no longer the
// zone's today — finishes the job without publishing; an error is returned
// to River for its retry ladder.
func (p *PaymentsPublisher) DeliverPaymentDue(ctx context.Context, paymentID uuid.UUID, date, now time.Time) error {
	target, live, err := p.source.GetScheduledDuePayment(ctx, paymentID, date, now)
	if err != nil {
		return fmt.Errorf("load due payment %s: %w", paymentID, err)
	}
	if !live {
		return nil
	}
	return p.publish(ctx, domain.EventPaymentDue, target)
}

// DeliverPaymentOverdue is the overdue boundary job's half (issue #776):
// reload the operation as of the wake-up and publish if it still lives
// overdue — planned, on a non-archived property, its date strictly before
// the zone's today.
func (p *PaymentsPublisher) DeliverPaymentOverdue(ctx context.Context, paymentID uuid.UUID, date, now time.Time) error {
	target, live, err := p.source.GetScheduledOverduePayment(ctx, paymentID, date, now)
	if err != nil {
		return fmt.Errorf("load overdue payment %s: %w", paymentID, err)
	}
	if !live {
		return nil
	}
	return p.publish(ctx, domain.EventPaymentOverdue, target)
}

// DeliverPaymentReminder is the reminder boundary job's half (карта #822,
// #824): reload the operation as of the wake-up and publish if it still
// lives on its reminder day — planned, on a non-archived property, the
// rule's current lead time still landing the reminder day on today. A
// lead time changed since the booking sends the stale job away quietly.
func (p *PaymentsPublisher) DeliverPaymentReminder(ctx context.Context, paymentID uuid.UUID, date, now time.Time) error {
	target, live, err := p.source.GetScheduledReminderPayment(ctx, paymentID, date, now)
	if err != nil {
		return fmt.Errorf("load reminder payment %s: %w", paymentID, err)
	}
	if !live {
		return nil
	}
	return p.publish(ctx, domain.EventPaymentReminder, target)
}

// publish fans one operation's event out to the owner and the active
// members. The dedup key pins the rule and the operation date it fired for —
// the repeat publication of the same rule and date inserts nothing. A system
// scan has no initiator: the actor is nobody, nobody is skipped.
func (p *PaymentsPublisher) publish(ctx context.Context, eventType domain.EventType, target PaymentScanTarget) error {
	recipients, err := ownerPlusMembers(ctx, p.source.ListActiveRecipients, target.OwnerID, target.PropertyID)
	if err != nil {
		return err
	}

	dueDate := target.DueDate
	return p.pipeline.Publish(ctx, Publication{
		EventType: eventType,
		DedupKey:  domain.DedupKey(paymentsDedupKey(eventType, target.PaymentID, target.DueDate)),
		Title:     paymentsTitles[eventType],
		Body:      paymentsBody(eventType, target),
		// The line above the title is the property the payment hangs on —
		// the feed's context label vocabulary (решение #737).
		ContextLabel: target.PropertyName,
		Payload: domain.Payload{
			Property: &domain.EntityRef{
				ID:      target.PropertyID,
				Name:    target.PropertyName,
				Address: target.PropertyAddress,
			},
			PaymentID:   &target.PaymentID,
			PaymentDate: &dueDate,
		},
		Recipients: recipients,
	})
}

// paymentsTitles is the catalog's copy per event type (решение #737,
// типы №2–№3; напоминание — карта #822): the titles verbatim.
var paymentsTitles = map[domain.EventType]string{
	domain.EventPaymentDue:      "Оплатите платёж",
	domain.EventPaymentOverdue:  "Платёж просрочен",
	domain.EventPaymentReminder: "Напоминание о платеже",
}

// paymentsBody renders the catalog's body template (решение #737, verbatim):
// the due and reminder legs ask for the payment ahead of its day, the
// overdue one — the day after. The name is the rule's, the amount and the
// date are the operation's.
func paymentsBody(eventType domain.EventType, target PaymentScanTarget) string {
	name := fmt.Sprintf("Платёж «%s» по объекту «%s»", target.Title, target.PropertyName)
	amount := formatAmountKopecks(target.AmountKopecks)
	date := formatPaymentDueDate(target.DueDate)
	if eventType == domain.EventPaymentOverdue {
		return fmt.Sprintf("%s просрочен: %s. Срок оплаты был %s", name, amount, date)
	}
	return fmt.Sprintf("%s: %s. Срок оплаты: %s", name, amount, date)
}

// paymentsDedupKey builds the scan's dedup key (словарь издателей,
// CONTEXT.md): (event type, rule, operation date) — an operation unpaid past
// its date keeps the overdue key for its whole overdue life and never
// repeats.
func paymentsDedupKey(eventType domain.EventType, ruleID uuid.UUID, dueDate time.Time) string {
	return entityDateDedupKey(string(eventType), ruleID, dueDate)
}

// formatAmountKopecks renders a kopecks amount as rubles for the
// notification copy (ADR 0008, integer-only): thousands separated with the
// non-breaking space the frontend's ru-RU rendering uses, the kopeck tail —
// only when the amount has one ("2 500 ₽", "2 500,50 ₽").
func formatAmountKopecks(kopecks int64) string {
	rubles := strconv.FormatInt(kopecks/100, 10)
	remainder := kopecks % 100
	grouped := groupDigits(rubles)
	if remainder == 0 {
		return grouped + " ₽"
	}
	return grouped + fmt.Sprintf(",%02d ₽", remainder)
}

// groupDigits inserts a non-breaking space between thousands groups
// ("2500" → "2 500").
func groupDigits(digits string) string {
	if len(digits) <= 3 {
		return digits
	}
	head := len(digits) % 3
	if head == 0 {
		head = 3
	}
	parts := []string{digits[:head]}
	for i := head; i < len(digits); i += 3 {
		parts = append(parts, digits[i:i+3])
	}
	return strings.Join(parts, "\u00a0")
}

// formatPaymentDueDate renders an operation date as a Russian day-month
// string ("20 сентября") — the copy names the day, not the year, the same
// rendering the grace deadline uses (formatGraceDeadline).
func formatPaymentDueDate(t time.Time) string {
	return formatDayMonth(t)
}
