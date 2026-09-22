package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
)

// SettingsService implements the notification settings use cases (карта
// #734, решение #738, ADR 0058): the email matrix lives on the account —
// one configuration for every device; the push matrix lives on the device —
// the master toggle and the four category flags on the push subscription.
// The service categories (Тариф, Системные) are outside the contract: always
// on, nothing reads or stores them.
type SettingsService struct {
	email EmailPreferencesRepository
	push  PushSubscriptionRepository
}

// NewSettingsService creates the notification settings service.
func NewSettingsService(email EmailPreferencesRepository, push PushSubscriptionRepository) *SettingsService {
	return &SettingsService{email: email, push: push}
}

// EmailPreferences returns the account's email matrix; a user without a
// settings row reads as the all-on default.
func (s *SettingsService) EmailPreferences(ctx context.Context, userID uuid.UUID) (domain.CategoryPrefs, error) {
	prefs, err := s.email.Get(ctx, userID)
	if err != nil {
		return domain.CategoryPrefs{}, fmt.Errorf("get email preferences: %w", err)
	}
	return prefs, nil
}

// SetEmailPreferences replaces the account's email matrix (full replacement).
func (s *SettingsService) SetEmailPreferences(ctx context.Context, userID uuid.UUID, prefs domain.CategoryPrefs) error {
	if err := s.email.Set(ctx, userID, prefs); err != nil {
		return fmt.Errorf("set email preferences: %w", err)
	}
	return nil
}

// PushPreferences returns the device's stored settings state (master +
// category flags). ErrNotFound when the endpoint is not this user's.
func (s *SettingsService) PushPreferences(ctx context.Context, userID uuid.UUID, endpoint string) (domain.PushSubscription, error) {
	if endpoint == "" {
		return domain.PushSubscription{}, errors.Join(ErrInvalidPushSubscription, errors.New("endpoint is required"))
	}
	sub, err := s.push.GetByEndpoint(ctx, userID, endpoint)
	if err != nil {
		return domain.PushSubscription{}, fmt.Errorf("get push preferences: %w", err)
	}
	return sub, nil
}

// SetPushPreferences replaces the device's delivery state (the upsert of
// решение #738: выключение — флаг, подписка и категории сохраняются).
// ErrNotFound when the endpoint is not this user's.
func (s *SettingsService) SetPushPreferences(
	ctx context.Context, userID uuid.UUID, endpoint string, enabled bool, prefs domain.CategoryPrefs,
) error {
	if endpoint == "" {
		return errors.Join(ErrInvalidPushSubscription, errors.New("endpoint is required"))
	}
	ok, err := s.push.UpdatePreferences(ctx, userID, endpoint, enabled, prefs)
	if err != nil {
		return fmt.Errorf("set push preferences: %w", err)
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}
