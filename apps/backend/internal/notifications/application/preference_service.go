package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// PreferenceService implements the notification preference use cases: reading
// the effective per-event-type permissions and replacing them atomically.
type PreferenceService struct {
	repo  ReminderRepository
	db    transaction.Beginner
	audit auditapp.Recorder
}

// NewPreferenceService creates a new notification preference service.
func NewPreferenceService(repo ReminderRepository, db transaction.Beginner, audit auditapp.Recorder) *PreferenceService {
	if audit == nil {
		audit = auditapp.Noop{}
	}
	return &PreferenceService{repo: repo, db: db, audit: audit}
}

// ListPreferences returns the effective permission for every event type in
// stable order: a stored row wins, a missing row means allowed.
func (s *PreferenceService) ListPreferences(ctx context.Context, userID uuid.UUID) ([]domain.NotificationPreference, error) {
	stored, err := s.repo.ListPreferences(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list notification preferences: %w", err)
	}
	return mergePreferences(stored), nil
}

// ReplacePreferences validates the full preference set, stores it in a single
// transaction, and audits the event types whose permission actually changed.
// It returns the effective preferences in stable order.
//
//nolint:dupl // legacy per-event-type path; frozen until the contract phase removes it alongside ReplaceChannelPreferences (ADR 0030).
func (s *PreferenceService) ReplacePreferences(ctx context.Context, userID uuid.UUID, prefs []domain.NotificationPreference) ([]domain.NotificationPreference, error) {
	if err := validatePreferences(prefs); err != nil {
		return nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRepo := s.repo.WithTx(tx)

	stored, err := txRepo.ListPreferences(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list notification preferences: %w", err)
	}
	current := mergePreferences(stored)

	for _, p := range prefs {
		if err := txRepo.UpsertPreference(ctx, userID, p); err != nil {
			return nil, fmt.Errorf("upsert notification preference: %w", err)
		}
	}

	if changes := preferenceChanges(current, prefs); len(changes) > 0 {
		if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
			ActorID:    &userID,
			ActorRole:  auditdomain.ActorRoleOwner,
			Action:     auditdomain.ActionNotificationPreferencesUpdated,
			EntityType: auditdomain.EntityUser,
			EntityID:   &userID,
			Context:    map[string]any{"changes": changes},
		}); err != nil {
			return nil, fmt.Errorf("record audit: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return mergePreferences(prefs), nil
}

// mergePreferences overlays stored rows on the default allow-all set and
// returns the effective preference for every event type in stable order.
func mergePreferences(stored []domain.NotificationPreference) []domain.NotificationPreference {
	byType := make(map[domain.EventType]bool, len(stored))
	for _, p := range stored {
		byType[p.EventType] = p.Allowed
	}
	effective := domain.DefaultNotificationPreferences()
	for i, p := range effective {
		if allowed, ok := byType[p.EventType]; ok {
			effective[i].Allowed = allowed
		}
	}
	return effective
}

// validatePreferences checks that the set carries exactly one entry for every
// known event type.
func validatePreferences(prefs []domain.NotificationPreference) error {
	if len(prefs) != len(domain.AllEventTypes()) {
		return fmt.Errorf("%w: expected %d preferences, got %d", ErrInvalidPreferences, len(domain.AllEventTypes()), len(prefs))
	}
	seen := make(map[domain.EventType]bool, len(prefs))
	for _, p := range prefs {
		if !p.EventType.IsValid() {
			return fmt.Errorf("%w: unknown event_type %q", ErrInvalidPreferences, p.EventType)
		}
		if seen[p.EventType] {
			return fmt.Errorf("%w: duplicate event_type %q", ErrInvalidPreferences, p.EventType)
		}
		seen[p.EventType] = true
	}
	return nil
}

// preferenceChange describes one changed permission for the audit journal.
type preferenceChange struct {
	EventType domain.EventType `json:"event_type"`
	Old       bool             `json:"old"`
	New       bool             `json:"new"`
}

// preferenceChanges lists the event types whose effective permission differs
// between the current and the requested set.
func preferenceChanges(current, next []domain.NotificationPreference) []preferenceChange {
	oldByType := make(map[domain.EventType]bool, len(current))
	for _, p := range current {
		oldByType[p.EventType] = p.Allowed
	}
	var changes []preferenceChange
	for _, p := range next {
		if old, ok := oldByType[p.EventType]; !ok || old != p.Allowed {
			changes = append(changes, preferenceChange{EventType: p.EventType, Old: old, New: p.Allowed})
		}
	}
	return changes
}

// ListChannelPreferences returns the effective per-channel permission for
// every (event type, channel) pair in stable order: a stored row wins, a
// missing row means allowed.
func (s *PreferenceService) ListChannelPreferences(ctx context.Context, userID uuid.UUID) ([]domain.NotificationChannelPreference, error) {
	stored, err := s.repo.ListChannelPreferences(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list notification channel preferences: %w", err)
	}
	return mergeChannelPreferences(stored), nil
}

// ReplaceChannelPreferences validates the full per-channel preference set,
// stores it in a single transaction, and audits the pairs whose permission
// actually changed. It returns the effective preferences in stable order.
//
//nolint:dupl // expand-phase companion to ReplacePreferences; removed together with the legacy path in the contract phase (ADR 0030).
func (s *PreferenceService) ReplaceChannelPreferences(ctx context.Context, userID uuid.UUID, prefs []domain.NotificationChannelPreference) ([]domain.NotificationChannelPreference, error) {
	if err := validateChannelPreferences(prefs); err != nil {
		return nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRepo := s.repo.WithTx(tx)

	stored, err := txRepo.ListChannelPreferences(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list notification channel preferences: %w", err)
	}
	current := mergeChannelPreferences(stored)

	for _, p := range prefs {
		if err := txRepo.UpsertChannelPreference(ctx, userID, p); err != nil {
			return nil, fmt.Errorf("upsert notification channel preference: %w", err)
		}
	}

	if changes := channelPreferenceChanges(current, prefs); len(changes) > 0 {
		if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
			ActorID:    &userID,
			ActorRole:  auditdomain.ActorRoleOwner,
			Action:     auditdomain.ActionNotificationPreferencesUpdated,
			EntityType: auditdomain.EntityUser,
			EntityID:   &userID,
			Context:    map[string]any{"changes": changes},
		}); err != nil {
			return nil, fmt.Errorf("record audit: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return mergeChannelPreferences(prefs), nil
}

// mergeChannelPreferences overlays stored rows on the default allow-all set
// and returns the effective preference for every (event type, channel) pair in
// stable order.
func mergeChannelPreferences(stored []domain.NotificationChannelPreference) []domain.NotificationChannelPreference {
	byKey := make(map[channelPrefKey]bool, len(stored))
	for _, p := range stored {
		byKey[channelPrefKey{p.EventType, p.Channel}] = p.Allowed
	}
	effective := domain.DefaultNotificationChannelPreferences()
	for i, p := range effective {
		if allowed, ok := byKey[channelPrefKey{p.EventType, p.Channel}]; ok {
			effective[i].Allowed = allowed
		}
	}
	return effective
}

// validateChannelPreferences checks that the set carries exactly one entry for
// every (event type, channel) pair.
func validateChannelPreferences(prefs []domain.NotificationChannelPreference) error {
	expected := len(domain.AllEventTypes()) * len(domain.AllNotificationChannels())
	if len(prefs) != expected {
		return fmt.Errorf("%w: expected %d channel preferences, got %d", ErrInvalidPreferences, expected, len(prefs))
	}
	seen := make(map[channelPrefKey]bool, len(prefs))
	for _, p := range prefs {
		if !p.EventType.IsValid() {
			return fmt.Errorf("%w: unknown event_type %q", ErrInvalidPreferences, p.EventType)
		}
		if !p.Channel.IsValid() {
			return fmt.Errorf("%w: unknown channel %q", ErrInvalidPreferences, p.Channel)
		}
		if seen[channelPrefKey{p.EventType, p.Channel}] {
			return fmt.Errorf("%w: duplicate event_type %q channel %q", ErrInvalidPreferences, p.EventType, p.Channel)
		}
		seen[channelPrefKey{p.EventType, p.Channel}] = true
	}
	return nil
}

// channelPreferenceChange describes one changed per-channel permission for the
// audit journal.
type channelPreferenceChange struct {
	EventType domain.EventType           `json:"event_type"`
	Channel   domain.NotificationChannel `json:"channel"`
	Old       bool                       `json:"old"`
	New       bool                       `json:"new"`
}

// channelPrefKey is the composite map key for (event type, channel) lookups.
type channelPrefKey struct {
	eventType domain.EventType
	channel   domain.NotificationChannel
}

// channelPreferenceChanges lists the (event type, channel) pairs whose
// effective permission differs between the current and the requested set.
func channelPreferenceChanges(current, next []domain.NotificationChannelPreference) []channelPreferenceChange {
	oldByKey := make(map[channelPrefKey]bool, len(current))
	for _, p := range current {
		oldByKey[channelPrefKey{p.EventType, p.Channel}] = p.Allowed
	}
	var changes []channelPreferenceChange
	for _, p := range next {
		k := channelPrefKey{p.EventType, p.Channel}
		if old, ok := oldByKey[k]; !ok || old != p.Allowed {
			changes = append(changes, channelPreferenceChange{EventType: p.EventType, Channel: p.Channel, Old: old, New: p.Allowed})
		}
	}
	return changes
}
