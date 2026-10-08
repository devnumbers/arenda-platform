package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
)

// RentalScheduleTarget is one upcoming boundary the scan books (issue
// #777): the rental id and the planned end date — the job args' identity —
// and the boundary's instant in the owner's timezone, the job's
// ScheduledAt. The boundary's instant is 00:00 of the day after the planned
// end (решение владельца 21.09.2026: все уведомления — чётко по времени).
type RentalScheduleTarget struct {
	RentalID       uuid.UUID
	PlannedEndDate time.Time
	FireAt         time.Time
}

// RentalCompletedTarget is one rental the scan fires for: a live rental in
// the needs_attention state (ADR 0053) with the property snapshot the
// publication carries (решение владельца 19.09.2026, #745 — the address line
// travels in the snapshot; the type — the card glyph's key, карта #1217,
// #1244).
type RentalCompletedTarget struct {
	RentalID uuid.UUID
	// PlannedEndDate is the rental's planned end as a calendar date — the
	// dedup key's date half and the trigger's datum.
	PlannedEndDate  time.Time
	PropertyID      uuid.UUID
	PropertyName    string
	PropertyAddress string
	PropertyType    string
	OwnerID         uuid.UUID
}

// RentalCompletedSource is the scan's window into the rentals context: the
// needs_attention targets of one zone, the upcoming boundaries' booking
// list, the boundary job's reload and the property's active members.
// Consumer-declared (CODING_STANDARDS), answered over the owning tables by
// the notifications postgres adapter.
type RentalCompletedSource interface {
	// ListCompletedTargets lists the zone's rentals that sit in the
	// needs_attention state as of the zone's today: not completed, planned
	// end strictly before today — the day after the planned end (решение
	// #737, тип №1).
	ListCompletedTargets(ctx context.Context, zone string, today time.Time) ([]RentalCompletedTarget, error)
	// ListScheduledCompletedTargets lists the unfinished rentals whose
	// boundary — 00:00 of the day after the planned end in the owner's
	// timezone — falls in the window (from, until]; the booking list of the
	// boundary leg (issue #777).
	ListScheduledCompletedTargets(ctx context.Context, from, until time.Time) ([]RentalScheduleTarget, error)
	// GetScheduledCompletedRental reloads one rental at its boundary — the
	// boundary job's delivery-time resolution (issue #777). The live flag
	// is false when the needs_attention conditions no longer hold as of
	// now: the rental completed, the planned end moved off the booked date
	// (an extension), the property archived, or the planned end no longer
	// strictly before the zone's today.
	GetScheduledCompletedRental(ctx context.Context, rentalID uuid.UUID, plannedEnd, now time.Time) (RentalCompletedTarget, bool, error)
	// ListActiveRecipients lists the user ids of the property's active
	// members — the recipients besides the owner, «Просмотр» included
	// (решение #737: получатели объектных событий).
	ListActiveRecipients(ctx context.Context, propertyID uuid.UUID) ([]uuid.UUID, error)
}

// RentalBoundaryScheduler books a rental's boundary job on the delivery
// queue (issue #777). Booking is idempotent — the queue's unique key keeps
// one in-flight job per (rental, planned end) — so the hourly passes repeat
// their asks freely.
type RentalBoundaryScheduler interface {
	// ScheduleRentalCompleted books the «Аренда завершена» job at the
	// boundary's instant: 00:00 of the day after the planned end in the
	// owner's timezone.
	ScheduleRentalCompleted(ctx context.Context, rentalID uuid.UUID, plannedEnd, fireAt time.Time) error
}

// RentalBoundaryDeliverer is the boundary job's call into the publisher:
// the boundary worker reloads the rental through the source as of its
// wake-up and publishes — or finishes without publishing when the reload
// answers no live rental.
type RentalBoundaryDeliverer interface {
	DeliverRentalCompleted(ctx context.Context, rentalID uuid.UUID, plannedEnd, now time.Time) error
}

// RentalCompletedPublisher is the rental events' publisher (issue #748) on
// the delivery pipeline (карта #734, #740): the rentals that moved to
// «Ожидает действия» — the planned end has passed and the rental is not
// completed — give each one's «Аренда завершена» row to the owner and the
// active members. The copy, the payload and the dedup key are the
// publisher's (решение #737, тип №1); the «Продлить»/«Завершить» buttons
// are nobody's here — they compute at read time from the live state (#743).
//
// The trigger has two mechanisms (issue #777, the payments scan's canon of
// #776): the hourly pass books the boundary jobs — each at 00:00 of the day
// after its planned end, in the owner's timezone, re-asking while the rental
// sits in the future — and the zone sweep stays the backstop for everything
// the jobs missed (a pass delayed past a boundary, a booking window not
// reached yet, the retrospective after a downtime). In the norm the job
// wakes exactly at the boundary and the sweep's publication is silent — the
// dedup key carries the idempotence, no sweep state does.
//
// The sweep mirrors the materialization ticks (ADR 0048 p.3): one today per
// owner timezone, failures isolated per zone and per rental, and idempotence
// carried by the dedup key instead of sweep state — a repeated pass inserts
// nothing, an extension of the rental moves the planned end and the key with
// it, so the next passing date notifies again (issue #748).
type RentalCompletedPublisher struct {
	pipeline  *Publisher
	zones     ScanZoneDirectory
	source    RentalCompletedSource
	scheduler RentalBoundaryScheduler
}

// NewRentalCompletedPublisher builds the rental-completed publisher over the
// pipeline creation service, the zone directory, the scan source and the
// boundary scheduler.
func NewRentalCompletedPublisher(
	pipeline *Publisher, zones ScanZoneDirectory, source RentalCompletedSource, scheduler RentalBoundaryScheduler,
) *RentalCompletedPublisher {
	return &RentalCompletedPublisher{pipeline: pipeline, zones: zones, source: source, scheduler: scheduler}
}

// RunZoneScans is the hourly sweep: book the upcoming boundaries' jobs first
// — every hour counts down to their midnight — then sweep the zones,
// publishing every needs_attention rental. Failures are isolated — a broken
// booking, zone or rental does not stop the rest; the joined error reports
// everything that failed.
func (p *RentalCompletedPublisher) RunZoneScans(ctx context.Context, now time.Time) error {
	if p.zones == nil {
		return errors.New("notifications rental scan: zone directory must be configured")
	}
	errs := append([]error{}, p.scheduleBoundaries(ctx, now)...)
	if err := p.sweepZones(ctx, now); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// scheduleBoundaries books the boundary jobs of the unfinished rentals whose
// boundaries — 00:00 of the day after the planned end — fall into the
// horizon window. A broken booking is isolated — the rest of the window
// books on.
func (p *RentalCompletedPublisher) scheduleBoundaries(ctx context.Context, now time.Time) []error {
	targets, err := p.source.ListScheduledCompletedTargets(ctx, now, now.Add(scheduledHorizon))
	if err != nil {
		return []error{fmt.Errorf("list completed rental boundaries: %w", err)}
	}
	var errs []error
	for _, target := range targets {
		if err := p.scheduler.ScheduleRentalCompleted(ctx, target.RentalID, target.PlannedEndDate, target.FireAt); err != nil {
			errs = append(errs, fmt.Errorf("schedule boundary of rental %s: %w", target.RentalID, err))
		}
	}
	return errs
}

// sweepZones is the zone sweep (ADR 0048 p.3): one today per owner timezone,
// every needs_attention rental published.
func (p *RentalCompletedPublisher) sweepZones(ctx context.Context, now time.Time) error {
	return forEachZone(ctx, p.zones, now, "rental", func(zone string, today time.Time) error {
		targets, err := p.source.ListCompletedTargets(ctx, zone, today)
		if err != nil {
			return fmt.Errorf("list rental targets of zone %s: %w", zone, err)
		}
		var errs []error
		for _, target := range targets {
			if err := p.publish(ctx, target); err != nil {
				errs = append(errs, fmt.Errorf("publish rental %s of zone %s: %w",
					target.RentalID, zone, err))
			}
		}
		return errors.Join(errs...)
	})
}

// DeliverRentalCompleted is the boundary job's half (issue #777): reload the
// rental through the source as of the wake-up and publish if it still sits
// in the needs_attention state. A false live flag — completed, extended (the
// planned end moved off the booked date), the property archived — finishes
// the job without publishing; an error is returned to River for its retry
// ladder.
func (p *RentalCompletedPublisher) DeliverRentalCompleted(ctx context.Context, rentalID uuid.UUID, plannedEnd, now time.Time) error {
	target, live, err := p.source.GetScheduledCompletedRental(ctx, rentalID, plannedEnd, now)
	if err != nil {
		return fmt.Errorf("load completed rental %s: %w", rentalID, err)
	}
	if !live {
		return nil
	}
	return p.publish(ctx, target)
}

// publish fans the rental's event out to the owner and the active members.
// The dedup key pins the rental and the planned end it fired for — the
// repeat publication of the same rental and date inserts nothing, while the
// extended rental (a new planned end) notifies again. A system scan has no
// initiator: the actor is nobody, nobody is skipped.
func (p *RentalCompletedPublisher) publish(ctx context.Context, target RentalCompletedTarget) error {
	recipients, err := ownerPlusMembers(ctx, p.source.ListActiveRecipients, target.OwnerID, target.PropertyID)
	if err != nil {
		return err
	}

	rentalID := target.RentalID
	return p.pipeline.Publish(ctx, Publication{
		EventType: domain.EventRentalCompleted,
		DedupKey:  domain.DedupKey(rentalCompletedDedupKey(target.RentalID, target.PlannedEndDate)),
		Title:     "Аренда завершена",
		Body: fmt.Sprintf(
			"Договор аренды по объекту «%s» завершён. Продлите договор или завершите аренду",
			target.PropertyName),
		ContextLabel: target.PropertyName,
		Payload: domain.Payload{
			Property: &domain.EntityRef{
				ID:      target.PropertyID,
				Name:    target.PropertyName,
				Address: target.PropertyAddress,
				Type:    target.PropertyType,
			},
			RentalID: &rentalID,
		},
		Recipients: recipients,
	})
}

// rentalCompletedDedupKey builds the scan's dedup key (словарь издателей,
// CONTEXT.md): (event type, rental, planned end date) — the extended rental
// (a new planned end) notifies again under its new key.
func rentalCompletedDedupKey(rentalID uuid.UUID, plannedEnd time.Time) string {
	return entityDateDedupKey("rental_completed", rentalID, plannedEnd)
}
