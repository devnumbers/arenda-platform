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
// free, operation, and system reminders into a single agenda for a date range.
// Free reminders are expanded from their templates on read (independent of the
// materialization horizon); operation/system reminders are read from the
// reminders table.
type CalendarService struct {
	reminders     ReminderRepository
	freeReminders FreeReminderRepository
	tzResolver    tzresolver.OwnerTimezoneResolver
}

// NewCalendarService creates a new calendar service.
func NewCalendarService(
	reminders ReminderRepository,
	freeReminders FreeReminderRepository,
	tzResolver tzresolver.OwnerTimezoneResolver,
) *CalendarService {
	return &CalendarService{reminders: reminders, freeReminders: freeReminders, tzResolver: tzResolver}
}

// ListCalendar returns all reminders (free, operation, system) for an owner in
// the half-open window [from, to). 'from' and 'to' are calendar dates; they
// are interpreted as local midnights in the owner's timezone before conversion
// to UTC. The result is sorted by scheduled_at ascending.
func (s *CalendarService) ListCalendar(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]domain.CalendarReminder, error) {
	loc, err := s.tzResolver.Resolve(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("resolve owner timezone: %w", err)
	}

	// Interpret the date-only bounds as local midnights in the owner's tz,
	// then convert to UTC instants for the half-open window [from, to).
	fromUTC := midnightIn(from, loc).UTC()
	toUTC := midnightIn(to, loc).UTC()

	// Operation and system reminders come straight from the reminders table
	// with a LEFT JOIN for the property name.
	opSys, err := s.reminders.ListCalendarByOwner(ctx, ownerID, fromUTC, toUTC)
	if err != nil {
		return nil, fmt.Errorf("list calendar operation/system reminders: %w", err)
	}

	// Free reminders are expanded from templates in the requested window,
	// independent of the materialization horizon.
	templates, err := s.freeReminders.ListTemplatesByOwner(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list free reminder templates: %w", err)
	}

	out := make([]domain.CalendarReminder, 0, len(opSys))
	out = append(out, opSys...)

	for _, tpl := range templates {
		propertyID := tpl.PropertyID
		for _, occ := range domain.ExpandFreeReminderOccurrences(tpl.FreeReminder, fromUTC, toUTC) {
			periodicity := tpl.Periodicity
			freeReminderID := tpl.ID
			out = append(out, domain.CalendarReminder{
				ID:             tpl.ID,
				Type:           domain.CalendarTypeFree,
				ScheduledAt:    occ,
				Title:          tpl.Title,
				PropertyID:     &propertyID,
				PropertyName:   tpl.PropertyName,
				HasProperty:    tpl.PropertyName != nil,
				FreeReminderID: &freeReminderID,
				Periodicity:    &periodicity,
			})
		}
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
