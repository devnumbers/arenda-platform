package application

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/tzresolver"
)

// CalendarService is the read use case for the reminders calendar. It unifies
// operation and system reminders into a single agenda for a date range, read
// straight from the reminders table.
type CalendarService struct {
	reminders  ReminderRepository
	tzResolver tzresolver.OwnerTimezoneResolver
	// The shared IDs are optionally injected (see SetSharedPropertyIDs); when
	// nil the agenda covers only the actor's own reminders, when set it
	// additionally includes reminders of the properties shared with the actor
	// (issue #157, T3), restricted to active/maintenance properties.
	sharedIDs SharedPropertyIDs
}

// NewCalendarService creates a new calendar service.
func NewCalendarService(
	reminders ReminderRepository,
	tzResolver tzresolver.OwnerTimezoneResolver,
) *CalendarService {
	return &CalendarService{reminders: reminders, tzResolver: tzResolver}
}

// SetSharedPropertyIDs injects the access-context adapter that resolves the
// property ids shared with an actor via property membership (issue #157, T3).
// Optional: when nil, the agenda only shows the actor's own reminders; when
// set, it additionally shows reminders of the shared properties.
func (s *CalendarService) SetSharedPropertyIDs(ids SharedPropertyIDs) {
	s.sharedIDs = ids
}

// ListCalendar returns all reminders (operation, system) for an owner in the
// half-open window [from, to). 'from' and 'to' are calendar dates; they are
// interpreted as local midnights in the owner's timezone before conversion to
// UTC. The result is sorted by scheduled_at ascending.
func (s *CalendarService) ListCalendar(ctx context.Context, actor uuid.UUID, from, to time.Time) ([]domain.CalendarReminder, error) {
	loc, err := s.tzResolver.Resolve(ctx, actor)
	if err != nil {
		return nil, fmt.Errorf("resolve owner timezone: %w", err)
	}

	// The accessiblePropertyIDs slice extends the agenda with reminders of
	// properties shared with the actor via property membership (issue #157, T3).
	// For the owner this is empty and the agenda covers only their own
	// reminders; for a member it additionally includes the shared properties'
	// reminders (matched by property_id, since their owner_id differs),
	// excluding archived ones.
	var accessible []uuid.UUID
	if s.sharedIDs != nil {
		shared, err := s.sharedIDs.SharedWith(ctx, actor)
		if err != nil {
			return nil, fmt.Errorf("list shared property ids: %w", err)
		}
		accessible = shared
	}

	// Interpret the date-only bounds as local midnights in the owner's tz,
	// then convert to UTC instants for the half-open window [from, to).
	fromUTC := midnightIn(from, loc).UTC()
	toUTC := midnightIn(to, loc).UTC()

	out, err := s.reminders.ListCalendarByOwner(ctx, actor, fromUTC, toUTC, accessible)
	if err != nil {
		return nil, fmt.Errorf("list calendar reminders: %w", err)
	}

	slices.SortFunc(out, func(a, b domain.CalendarReminder) int {
		return a.ScheduledAt.Compare(b.ScheduledAt)
	})

	return out, nil
}

// midnightIn returns the start-of-day (00:00) instant of t in the given
// location. It is used to normalize date-only bounds into local midnights.
func midnightIn(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}
