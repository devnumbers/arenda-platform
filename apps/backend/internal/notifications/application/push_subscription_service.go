package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// PushSubscriptionService implements the Web Push subscription use cases:
// storing, deleting and listing per-user browser push subscriptions. These are
// personal to the authenticated user (not owner-scoped data like reminders), so
// there is no policy gate: the actor owns their subscriptions.
type PushSubscriptionService struct {
	repo  PushSubscriptionRepository
	clock clock.Clock
}

// NewPushSubscriptionService creates a new push subscription service.
func NewPushSubscriptionService(repo PushSubscriptionRepository, clk clock.Clock) *PushSubscriptionService {
	return &PushSubscriptionService{repo: repo, clock: clk}
}

// UpsertPushSubscriptionInput is the user-supplied data for registering a push
// subscription. Categories are optional: a nil pointer keeps the all-on
// default (решение #738; спека #1028 §5 — POST несёт категории явно, дефолт
// для чужих клиентов — все ВКЛ).
type UpsertPushSubscriptionInput struct {
	Endpoint       string
	P256dh         string
	Auth           string
	ExpirationTime *time.Time
	Categories     *domain.CategoryPrefs
}

// Upsert validates the subscription fields and stores them keyed by endpoint.
// A repeated call with the same endpoint updates the mutable fields (idempotent
// re-subscribe), settings included — the body carries the device's desired
// state. It returns the stored subscription.
func (s *PushSubscriptionService) Upsert(
	ctx context.Context,
	userID uuid.UUID,
	in UpsertPushSubscriptionInput,
) (domain.PushSubscription, error) {
	if err := domain.ValidatePushSubscription(in.Endpoint, in.P256dh, in.Auth); err != nil {
		return domain.PushSubscription{}, fmt.Errorf("%w: %w", ErrInvalidPushSubscription, err)
	}

	categories := domain.DefaultCategoryPrefs()
	if in.Categories != nil {
		categories = *in.Categories
	}

	now := s.clock.Now()
	id, err := uuid.NewV7()
	if err != nil {
		return domain.PushSubscription{}, fmt.Errorf("generate push subscription id: %w", err)
	}
	sub := domain.PushSubscription{
		ID:             id,
		UserID:         userID,
		Endpoint:       in.Endpoint,
		P256dh:         in.P256dh,
		Auth:           in.Auth,
		ExpirationTime: in.ExpirationTime,
		Categories:     categories,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	stored, err := s.repo.Upsert(ctx, sub)
	if err != nil {
		return domain.PushSubscription{}, fmt.Errorf("upsert push subscription: %w", err)
	}
	return stored, nil
}

// Delete removes the subscription with the given endpoint for the user.
// Idempotent (спека #1028 §5): the «выключено» intent is fulfilled whether
// the row existed or not — there is no not-found outcome.
func (s *PushSubscriptionService) Delete(ctx context.Context, userID uuid.UUID, endpoint string) error {
	if endpoint == "" {
		return errors.Join(ErrInvalidPushSubscription, errors.New("endpoint is required"))
	}
	if err := s.repo.Delete(ctx, userID, endpoint); err != nil {
		return fmt.Errorf("delete push subscription: %w", err)
	}
	return nil
}

// ListByUser returns all stored subscriptions for a user.
func (s *PushSubscriptionService) ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.PushSubscription, error) {
	subs, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list push subscriptions: %w", err)
	}
	return subs, nil
}
