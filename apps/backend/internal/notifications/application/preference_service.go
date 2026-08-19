package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
)

// PreferenceService implements the notification preference use cases: reading
// the effective per-channel permissions and replacing them atomically.
type PreferenceService struct {
	txStoreFactory
}

// NewPreferenceService creates a new notification preference service bound to
// the shared notifications txStoreFactory, so ReplaceChannelPreferences runs
// through runInTx (ADR 0033).
func NewPreferenceService(factory txStoreFactory) *PreferenceService {
	return &PreferenceService{txStoreFactory: factory}
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
func (s *PreferenceService) ReplaceChannelPreferences(ctx context.Context, userID uuid.UUID, prefs []domain.NotificationChannelPreference) ([]domain.NotificationChannelPreference, error) {
	if err := validateChannelPreferences(prefs); err != nil {
		return nil, err
	}

	if err := s.runInTx(ctx, func(stores *txStores) error {
		stored, err := stores.repo.ListChannelPreferences(ctx, userID)
		if err != nil {
			return fmt.Errorf("list notification channel preferences: %w", err)
		}
		current := mergeChannelPreferences(stored)

		for _, p := range prefs {
			if err := stores.repo.UpsertChannelPreference(ctx, userID, p); err != nil {
				return fmt.Errorf("upsert notification channel preference: %w", err)
			}
		}

		if changes := channelPreferenceChanges(current, prefs); len(changes) > 0 {
			if err := stores.audit.Record(ctx, auditdomain.Entry{
				ActorID:    &userID,
				ActorRole:  auditdomain.ActorRoleOwner,
				Action:     auditdomain.ActionNotificationPreferencesUpdated,
				EntityType: auditdomain.EntityUser,
				EntityID:   &userID,
				Context:    map[string]any{"changes": changes},
			}); err != nil {
				return fmt.Errorf("record audit: %w", err)
			}
		}
		return nil
	}); err != nil {
		return nil, err
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
