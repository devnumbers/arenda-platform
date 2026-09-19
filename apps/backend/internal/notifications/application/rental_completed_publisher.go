package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
)

// RentalCompletedTarget is one rental the scan fires for: a live rental in
// the needs_attention state (ADR 0053) with the property snapshot the
// publication carries (решение владельца 19.09.2026, #745 — the address line
// travels in the snapshot).
type RentalCompletedTarget struct {
	RentalID uuid.UUID
	// PlannedEndDate is the rental's planned end as a calendar date — the
	// dedup key's date half and the trigger's datum.
	PlannedEndDate  time.Time
	PropertyID      uuid.UUID
	PropertyName    string
	PropertyAddress string
	OwnerID         uuid.UUID
}

// RentalCompletedSource is the scan's window into the rentals context: the
// needs_attention targets of one zone and the property's active members.
// Consumer-declared (CODING_STANDARDS), answered over the owning tables by
// the notifications postgres adapter.
type RentalCompletedSource interface {
	// ListCompletedTargets lists the zone's rentals that sit in the
	// needs_attention state as of the zone's today: not completed, planned
	// end strictly before today — the day after the planned end (решение
	// #737, тип №1).
	ListCompletedTargets(ctx context.Context, zone string, today time.Time) ([]RentalCompletedTarget, error)
	// ListActiveRecipients lists the user ids of the property's active
	// members — the recipients besides the owner, «Просмотр» included
	// (решение #737: получатели объектных событий).
	ListActiveRecipients(ctx context.Context, propertyID uuid.UUID) ([]uuid.UUID, error)
}

// RentalCompletedPublisher is the rental events' publisher (issue #748) on
// the delivery pipeline (карта #734, #740): the hourly zone sweep finds the
// rentals that moved to «Ожидает действия» — the planned end has passed and
// the rental is not completed — and gives each one's «Аренда завершена» row
// to the owner and the active members. The copy, the payload and the dedup
// key are the publisher's (решение #737, тип №1); the «Продлить»/«Завершить»
// buttons are nobody's here — they compute at read time from the live state
// (#743).
//
// The sweep mirrors the materialization ticks (ADR 0048 p.3): one today per
// owner timezone, failures isolated per zone and per rental, and idempotence
// carried by the dedup key instead of sweep state — a repeated pass inserts
// nothing, an extension of the rental moves the planned end and the key with
// it, so the next passing date notifies again (issue #748).
type RentalCompletedPublisher struct {
	pipeline *Publisher
	zones    ScanZoneDirectory
	source   RentalCompletedSource
}

// NewRentalCompletedPublisher builds the rental-completed publisher over the
// pipeline creation service, the zone directory and the scan source.
func NewRentalCompletedPublisher(
	pipeline *Publisher, zones ScanZoneDirectory, source RentalCompletedSource,
) *RentalCompletedPublisher {
	return &RentalCompletedPublisher{pipeline: pipeline, zones: zones, source: source}
}

// RunZoneScans is the hourly sweep: list the zones, compute one today per
// zone from the passed instant, publish every target of the zone. Failures
// are isolated — a broken zone or rental does not stop the rest; the joined
// error reports everything that failed.
func (p *RentalCompletedPublisher) RunZoneScans(ctx context.Context, now time.Time) error {
	if p.zones == nil {
		return errors.New("notifications rental scan: zone directory must be configured")
	}
	zones, err := p.zones.ListScanZones(ctx)
	if err != nil {
		return fmt.Errorf("list rental scan zones: %w", err)
	}
	var errs []error
	for _, zone := range zones {
		today, err := zoneToday(now, zone.Timezone)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		targets, err := p.source.ListCompletedTargets(ctx, zone.Timezone, today)
		if err != nil {
			errs = append(errs, fmt.Errorf("list rental targets of zone %s: %w", zone.Timezone, err))
			continue
		}
		for _, target := range targets {
			if err := p.publish(ctx, target); err != nil {
				errs = append(errs, fmt.Errorf("publish rental %s of zone %s: %w",
					target.RentalID, zone.Timezone, err))
			}
		}
	}
	return errors.Join(errs...)
}

// publish fans the rental's event out to the owner and the active members.
// The dedup key pins the rental and the planned end it fired for — the
// repeat publication of the same rental and date inserts nothing, while the
// extended rental (a new planned end) notifies again. A system scan has no
// initiator: the actor is nobody, nobody is skipped.
func (p *RentalCompletedPublisher) publish(ctx context.Context, target RentalCompletedTarget) error {
	members, err := p.source.ListActiveRecipients(ctx, target.PropertyID)
	if err != nil {
		return fmt.Errorf("list recipients of property %s: %w", target.PropertyID, err)
	}
	recipients := make([]uuid.UUID, 0, len(members)+1)
	recipients = append(recipients, target.OwnerID)
	recipients = append(recipients, members...)

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
			},
			RentalID: &rentalID,
		},
		Recipients: recipients,
	})
}

// rentalCompletedDedupKey builds the scan's dedup key (словарь издателей,
// CONTEXT.md): (event type, rental, planned end date). The date half is the
// planned end itself — a DATE column, no timezone of the instant involved —
// so the key depends only on the facts the trigger reads.
func rentalCompletedDedupKey(rentalID uuid.UUID, plannedEnd time.Time) string {
	return "rental_completed:" + rentalID.String() + ":" + plannedEnd.Format("2006-01-02")
}
