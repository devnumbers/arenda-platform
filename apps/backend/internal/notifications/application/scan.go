package application

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ScanZone is one sweep target of the notifications scan (карта #734): an
// owner timezone (ADR 0048 p.3) whose owners have entities a scan-driven
// publisher can fire for. Every publisher lists the zones of its own
// population — the rental scan's zones come from its rentals, the payments
// scan's from its operations — so a zone is swept only where it has targets.
type ScanZone struct {
	Timezone string
}

// ScanZoneDirectory lists a scan's sweep targets. Stateless — every run
// re-lists, the scans keep no per-zone state (the dedup key of each
// published row is the only memory).
type ScanZoneDirectory interface {
	ListScanZones(ctx context.Context) ([]ScanZone, error)
}

// ZoneScanner is one scan-driven publisher's hourly sweep: the scheduler's
// worker shell runs one ScanGroup, the group runs the publishers in catalog
// order.
type ZoneScanner interface {
	RunZoneScans(ctx context.Context, now time.Time) error
}

// ScanGroup is the hourly notifications sweep (карта #734): the
// scan-driven publishers in catalog order — the rental-completed sweep
// (#748), the payments due/overdue sweep (#749). One worker shell runs them
// in one pass; a publisher's failure is isolated — the group goes on with
// the rest, the joined error reports everything that failed.
type ScanGroup struct {
	scans []ZoneScanner
}

// NewScanGroup composes the scan publishers into the single sweep the
// scheduler runs.
func NewScanGroup(scans ...ZoneScanner) *ScanGroup {
	return &ScanGroup{scans: scans}
}

// RunZoneScans runs every group member's sweep and joins their failures.
func (g *ScanGroup) RunZoneScans(ctx context.Context, now time.Time) error {
	var errs []error
	for _, scan := range g.scans {
		if err := scan.RunZoneScans(ctx, now); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// zoneToday computes the sweep's "today" for one zone (ADR 0048 p.2, the
// payments/tasks tick convention): the calendar date of the instant in the
// zone's IANA location under the module's UTC-midnight date convention. An
// unknown zone name is a data integrity error, not a fallback case:
// users.timezone is IANA-validated on write.
func zoneToday(now time.Time, timezone string) (time.Time, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("load scan zone %q as location: %w", timezone, err)
	}
	return DateAtUTCMidnight(now, loc), nil
}

// DateAtUTCMidnight is the module's date convention (ADR 0048 p.2): read the
// instant in the given location and return its calendar date as a
// UTC-midnight time.Time, so DATE columns compare without timezone surprises.
// The same convention the payments and tasks ticks run under — each context
// keeps its own copy across the context boundary.
func DateAtUTCMidnight(t time.Time, loc *time.Location) time.Time {
	local := t.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}
