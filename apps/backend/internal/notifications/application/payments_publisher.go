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
// the due and the overdue targets of one zone and the property's active
// members. Consumer-declared (CODING_STANDARDS), answered over the owning
// tables by the notifications postgres adapter.
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
}

// PaymentsPublisher is the payments events' publisher (issue #749) on the
// delivery pipeline (карта #734, #740): the hourly zone sweep finds the
// operations that came due today — «Оплатите платёж» — and the ones whose
// due date has passed unpaid — «Платёж просрочен» — and gives each one's row
// to the owner and the active members. The copy, the payload and the dedup
// keys are the publisher's (решение #737, типы №2–№3); the «Оплатить» button
// is nobody's here — it computes at read time from the live state (#743).
//
// The sweep mirrors the materialization ticks (ADR 0048 p.3): one today per
// owner timezone, failures isolated per zone and per operation, and
// idempotence carried by the dedup key instead of sweep state — the due leg
// fires once per (rule, date) because the key outlives the day, and an
// overdue operation sweeps daily without ever producing a second row.
type PaymentsPublisher struct {
	pipeline *Publisher
	zones    ScanZoneDirectory
	source   PaymentScanSource
}

// NewPaymentsPublisher builds the payments publisher over the pipeline
// creation service, the zone directory and the scan source.
func NewPaymentsPublisher(
	pipeline *Publisher, zones ScanZoneDirectory, source PaymentScanSource,
) *PaymentsPublisher {
	return &PaymentsPublisher{pipeline: pipeline, zones: zones, source: source}
}

// RunZoneScans is the hourly sweep: list the zones, compute one today per
// zone from the passed instant, publish the zone's due targets then its
// overdue ones. Failures are isolated — a broken zone or operation does not
// stop the rest; the joined error reports everything that failed.
func (p *PaymentsPublisher) RunZoneScans(ctx context.Context, now time.Time) error {
	if p.zones == nil {
		return errors.New("notifications payments scan: zone directory must be configured")
	}
	zones, err := p.zones.ListScanZones(ctx)
	if err != nil {
		return fmt.Errorf("list payments scan zones: %w", err)
	}
	var errs []error
	for _, zone := range zones {
		today, err := zoneToday(now, zone.Timezone)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		due, err := p.source.ListDueTargets(ctx, zone.Timezone, today)
		if err != nil {
			errs = append(errs, fmt.Errorf("list due payments of zone %s: %w", zone.Timezone, err))
		}
		for _, target := range due {
			if err := p.publish(ctx, domain.EventPaymentDue, target); err != nil {
				errs = append(errs, fmt.Errorf("publish due payment %s of zone %s: %w",
					target.PaymentID, zone.Timezone, err))
			}
		}
		overdue, err := p.source.ListOverdueTargets(ctx, zone.Timezone, today)
		if err != nil {
			errs = append(errs, fmt.Errorf("list overdue payments of zone %s: %w", zone.Timezone, err))
		}
		for _, target := range overdue {
			if err := p.publish(ctx, domain.EventPaymentOverdue, target); err != nil {
				errs = append(errs, fmt.Errorf("publish overdue payment %s of zone %s: %w",
					target.PaymentID, zone.Timezone, err))
			}
		}
	}
	return errors.Join(errs...)
}

// publish fans one operation's event out to the owner and the active
// members. The dedup key pins the rule and the operation date it fired for —
// the repeat publication of the same rule and date inserts nothing. A system
// scan has no initiator: the actor is nobody, nobody is skipped.
func (p *PaymentsPublisher) publish(ctx context.Context, eventType domain.EventType, target PaymentScanTarget) error {
	members, err := p.source.ListActiveRecipients(ctx, target.PropertyID)
	if err != nil {
		return fmt.Errorf("list recipients of property %s: %w", target.PropertyID, err)
	}
	recipients := make([]uuid.UUID, 0, len(members)+1)
	recipients = append(recipients, target.OwnerID)
	recipients = append(recipients, members...)

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
// типы №2–№3): the titles verbatim.
var paymentsTitles = map[domain.EventType]string{
	domain.EventPaymentDue:     "Оплатите платёж",
	domain.EventPaymentOverdue: "Платёж просрочен",
}

// paymentsBody renders the catalog's body template (решение #737, verbatim):
// the due leg asks for the payment on its day, the overdue one — the day
// after. The name is the rule's, the amount and the date are the
// operation's.
func paymentsBody(eventType domain.EventType, target PaymentScanTarget) string {
	name := fmt.Sprintf("Платёж «%s» по объекту «%s»", target.Title, target.PropertyName)
	amount := formatAmountKopecks(target.AmountKopecks)
	date := formatPaymentDueDate(target.DueDate)
	if eventType == domain.EventPaymentDue {
		return fmt.Sprintf("%s: %s. Срок оплаты: %s", name, amount, date)
	}
	return fmt.Sprintf("%s просрочен: %s. Срок оплаты был %s", name, amount, date)
}

// paymentsDedupKey builds the scan's dedup key (словарь издателей,
// CONTEXT.md): (event type, rule, operation date). The date half is the
// operation's DATE itself — no timezone of the instant involved — so the key
// depends only on the facts the trigger reads; an operation unpaid past its
// date keeps the overdue key for its whole overdue life and never repeats.
func paymentsDedupKey(eventType domain.EventType, ruleID uuid.UUID, dueDate time.Time) string {
	return string(eventType) + ":" + ruleID.String() + ":" + dueDate.Format("2006-01-02")
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
	return fmt.Sprintf("%d %s", t.Day(), graceMonths[int(t.Month())-1])
}
