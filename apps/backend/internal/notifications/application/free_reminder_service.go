package application

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/tzresolver"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// freeReminderHorizonYears is the requested materialization horizon for
// periodic free reminders, matching the recurring_operations horizon. The
// actual number of materialized rows is bounded by the domain expansion cap
// (expandFreeReminderLimit, 2500 occurrences): monthly and yearly reminders
// cover the full 100 years, while daily (~6.8 years) and weekly (~48 years)
// are capped earlier. Capped periodic reminders continue to fire until their
// materialized rows are exhausted; re-materialization happens on the next
// template update. This matches the recurring_operations approach where the
// horizon is finite by design.
const freeReminderHorizonYears = 100

// freeReminderTitleMaxLen is the maximum allowed length of a free reminder
// title in characters (runes), matching the database CHECK constraint
// `length(title) BETWEEN 1 AND 50` which counts characters, not bytes.
const freeReminderTitleMaxLen = 50

// validateFreeReminderTitle checks that the title is non-empty and within the
// character limit after trimming whitespace. It returns the trimmed title.
func validateFreeReminderTitle(title string) (string, error) {
	t := strings.TrimSpace(title)
	if n := utf8.RuneCountInString(t); n < 1 || n > freeReminderTitleMaxLen {
		return "", fmt.Errorf("%w: title must be 1-%d characters", ErrInvalidFreeReminderInput, freeReminderTitleMaxLen)
	}
	return t, nil
}

// FreeReminderService implements CRUD use cases for free reminder templates.
// Creating or updating a template also (re)materializes concrete reminders in a
// single transaction: existing concrete reminders are cancelled, then new ones
// are generated from the updated template within a 100-year horizon.
type FreeReminderService struct {
	repo       FreeReminderRepository
	db         transaction.Beginner
	clock      clock.Clock
	tzResolver tzresolver.OwnerTimezoneResolver
}

// NewFreeReminderService creates a new free reminder service.
func NewFreeReminderService(
	repo FreeReminderRepository,
	db transaction.Beginner,
	clock clock.Clock,
	tzResolver tzresolver.OwnerTimezoneResolver,
) *FreeReminderService {
	return &FreeReminderService{repo: repo, db: db, clock: clock, tzResolver: tzResolver}
}

// WithTx returns a service bound to the provided transaction. The transaction-
// bound repo is used for template persistence and concrete reminder
// materialization within the caller's transaction.
func (s *FreeReminderService) WithTx(tx transaction.Tx) *FreeReminderService {
	return &FreeReminderService{repo: s.repo.WithTx(tx), db: nil, clock: s.clock, tzResolver: s.tzResolver}
}

// Create validates input, resolves the owner's timezone, computes the UTC
// trigger_at, persists the template and materializes concrete reminders in a
// single transaction.
func (s *FreeReminderService) Create(ctx context.Context, actor uuid.UUID, in CreateFreeReminderInput) (domain.FreeReminder, error) {
	title, err := validateFreeReminderTitle(in.Title)
	if err != nil {
		return domain.FreeReminder{}, err
	}
	if !in.Periodicity.Valid() {
		return domain.FreeReminder{}, fmt.Errorf("%w: unknown periodicity %q", ErrInvalidFreeReminderInput, in.Periodicity)
	}

	loc, err := s.tzResolver.Resolve(ctx, actor)
	if err != nil {
		return domain.FreeReminder{}, fmt.Errorf("resolve owner timezone: %w", err)
	}

	now := s.clock.Now()
	triggerAt := localTriggerToUTC(in.TriggerAt, loc)
	if !triggerAt.After(now) {
		return domain.FreeReminder{}, fmt.Errorf("%w: trigger time must be in the future", ErrInvalidReminderDate)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.FreeReminder{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRepo := s.repo.WithTx(tx)

	id, err := uuid.NewV7()
	if err != nil {
		return domain.FreeReminder{}, fmt.Errorf("generate free reminder id: %w", err)
	}

	template := domain.FreeReminder{
		ID:          id,
		OwnerID:     actor,
		PropertyID:  in.PropertyID,
		Title:       title,
		TriggerAt:   triggerAt,
		Periodicity: in.Periodicity,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	created, err := txRepo.Create(ctx, template)
	if err != nil {
		return domain.FreeReminder{}, err
	}

	if err := s.materialize(ctx, txRepo, created, now); err != nil {
		return domain.FreeReminder{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.FreeReminder{}, fmt.Errorf("commit tx: %w", err)
	}
	return created, nil
}

// Get returns a free reminder by ID after verifying ownership.
func (s *FreeReminderService) Get(ctx context.Context, actor, id uuid.UUID) (domain.FreeReminder, error) {
	fr, err := s.repo.GetByID(ctx, id, actor)
	if err != nil {
		return domain.FreeReminder{}, err
	}
	return fr, nil
}

// Update applies a partial update to a free reminder and rematerializes its
// concrete reminders in a single transaction. Existing concrete reminders are
// cancelled, then new ones are generated from the updated template.
func (s *FreeReminderService) Update(ctx context.Context, actor, id uuid.UUID, in UpdateFreeReminderInput) (domain.FreeReminder, error) {
	existing, err := s.repo.GetByID(ctx, id, actor)
	if err != nil {
		return domain.FreeReminder{}, err
	}

	updated := existing
	if in.Title != nil {
		title, err := validateFreeReminderTitle(*in.Title)
		if err != nil {
			return domain.FreeReminder{}, err
		}
		updated.Title = title
	}
	if in.TriggerAt != nil {
		loc, err := s.tzResolver.Resolve(ctx, actor)
		if err != nil {
			return domain.FreeReminder{}, fmt.Errorf("resolve owner timezone: %w", err)
		}
		updated.TriggerAt = localTriggerToUTC(*in.TriggerAt, loc)
	}
	if in.Periodicity != nil {
		if !in.Periodicity.Valid() {
			return domain.FreeReminder{}, fmt.Errorf("%w: unknown periodicity %q", ErrInvalidFreeReminderInput, *in.Periodicity)
		}
		updated.Periodicity = *in.Periodicity
	}

	now := s.clock.Now()
	if !updated.TriggerAt.After(now) {
		return domain.FreeReminder{}, fmt.Errorf("%w: trigger time must be in the future", ErrInvalidReminderDate)
	}
	updated.UpdatedAt = now

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.FreeReminder{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRepo := s.repo.WithTx(tx)

	saved, err := txRepo.Update(ctx, updated)
	if err != nil {
		return domain.FreeReminder{}, err
	}

	if err := txRepo.CancelRemindersByFreeReminderID(ctx, actor, saved.ID); err != nil {
		return domain.FreeReminder{}, err
	}

	if err := s.materialize(ctx, txRepo, saved, now); err != nil {
		return domain.FreeReminder{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.FreeReminder{}, fmt.Errorf("commit tx: %w", err)
	}
	return saved, nil
}

// Delete removes a free reminder template; concrete reminders cascade-delete
// via the ON DELETE CASCADE on reminders.free_reminder_id.
func (s *FreeReminderService) Delete(ctx context.Context, actor, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, actor, id); err != nil {
		return err
	}
	return nil
}

// ListByOwner returns paginated free reminders for an owner.
func (s *FreeReminderService) ListByOwner(ctx context.Context, actor uuid.UUID, limit, offset int) ([]domain.FreeReminder, error) {
	reminders, err := s.repo.ListByOwner(ctx, actor, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list free reminders: %w", err)
	}
	return reminders, nil
}

// ListByProperty returns up to limit free reminders for a property.
func (s *FreeReminderService) ListByProperty(ctx context.Context, actor, propertyID uuid.UUID, limit int) ([]domain.FreeReminder, error) {
	reminders, err := s.repo.ListByProperty(ctx, actor, propertyID, limit)
	if err != nil {
		return nil, fmt.Errorf("list free reminders by property: %w", err)
	}
	return reminders, nil
}

// materialize generates concrete reminders from a free reminder template within
// the 100-year horizon and persists each one via SaveFreeReminder.
func (s *FreeReminderService) materialize(ctx context.Context, repo FreeReminderRepository, fr domain.FreeReminder, now time.Time) error {
	reminders := buildFreeConcreteReminders(fr, now)
	for _, r := range reminders {
		if err := repo.SaveFreeReminder(ctx, r); err != nil {
			return fmt.Errorf("save free reminder concrete: %w", err)
		}
	}
	return nil
}

// buildFreeConcreteReminders expands a free reminder template into concrete
// reminder rows over a 100-year horizon. Each concrete reminder has
// target_type='free', event_type='free_reminder' and is linked back to the
// template via FreeReminderID.
func buildFreeConcreteReminders(fr domain.FreeReminder, now time.Time) []domain.Reminder {
	horizonEnd := now.AddDate(freeReminderHorizonYears, 0, 0)
	occurrences := domain.ExpandFreeReminderOccurrences(fr, now, horizonEnd)

	reminders := make([]domain.Reminder, 0, len(occurrences))
	propertyID := fr.PropertyID
	freeID := fr.ID
	for _, occ := range occurrences {
		id, err := uuid.NewV7()
		if err != nil {
			// uuid.NewV7 effectively never fails; skip the occurrence rather
			// than aborting the whole batch.
			continue
		}
		reminders = append(reminders, domain.Reminder{
			ID:             id,
			OwnerID:        fr.OwnerID,
			TargetType:     domain.TargetFree,
			PropertyID:     &propertyID,
			FreeReminderID: &freeID,
			EventType:      domain.EventFreeReminder,
			Status:         domain.ReminderPending,
			ScheduledAt:    occ,
			MessageTitle:   fr.Title,
			MessageBody:    freeReminderBody(fr.Title),
			CreatedAt:      now,
			UpdatedAt:      now,
		})
	}
	return reminders
}

// freeReminderBody builds the message body for a free reminder. For the MVP the
// body is the reminder title; full localization (property name, local date and
// time) will be handled by the email template or a later iteration.
func freeReminderBody(title string) string {
	return title
}

// localTriggerToUTC interprets trigger as a local date + time-of-day in loc and
// returns the corresponding UTC instant. The wall-clock components (year, month,
// day, hour, minute) are extracted in loc and reassembled so the stored instant
// is independent of whatever zone the incoming time.Time carried.
func localTriggerToUTC(trigger time.Time, loc *time.Location) time.Time {
	t := trigger.In(loc)
	return domain.ScheduledAtForDateTime(t, loc, t.Hour(), t.Minute())
}
