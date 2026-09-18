package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
)

// FeedLiveState is the reading side's window into the entities' live state
// (Действие, решение #737): the action buttons are computed at read time
// from the live state and the reader's rights — the feed stores none of it.
// The port is consumer-declared and answers exactly the questions the
// catalog's action kinds need; the notifications postgres adapter implements
// it over the owning tables directly.
type FeedLiveState interface {
	// RentalActionState reports the live rental facts behind the
	// «Продлить»/«Завершить» buttons: whether the rental still awaits action
	// (not completed, planned end already passed — the needs_attention state,
	// ADR 0053) and which property it belongs to. Exists=false when the
	// rental is gone.
	RentalActionState(ctx context.Context, rentalID uuid.UUID) (RentalActionState, error)
	// PaymentOpen reports whether the notification's operation (Операция —
	// вхождение Payments) still awaits payment: it exists and its status is
	// planned. Paid or cancelled — the state has moved on.
	PaymentOpen(ctx context.Context, paymentID uuid.UUID) (bool, error)
	// TaskOpen reports whether the notification's task still awaits action:
	// it exists and is not completed.
	TaskOpen(ctx context.Context, taskID uuid.UUID) (bool, error)
	// PropertyAccess resolves the reader's live role on the property
	// (ADR 0028): Exists = the property stands and the reader is its owner
	// or an active member; Manageable = owner or full_access — the mutation
	// buttons' gate («Просмотр» читает, но не действует).
	PropertyAccess(ctx context.Context, propertyID, readerID uuid.UUID) (PropertyAccess, error)
}

// RentalActionState is the live rental answer for the action computation.
type RentalActionState struct {
	Exists bool
	// AwaitingAction is the needs_attention state: not completed and the
	// planned end has already passed.
	AwaitingAction bool
	// PropertyID is the live rental's property — rights resolve against it,
	// not against the payload snapshot.
	PropertyID uuid.UUID
}

// PropertyAccess is the reader's live standing on a property.
type PropertyAccess struct {
	Exists     bool
	Manageable bool
}

// NotificationDetail is the single-notification view (GET /notifications/{id}):
// the stored row plus the action buttons computed for this reader right now.
type NotificationDetail struct {
	Notification domain.Notification
	// Actions is the live subset of the event type's catalog actions, in
	// display order; empty — the page renders without buttons.
	Actions []domain.ActionKind
}

// Get returns the user's view of one feed row with its live actions. A row
// that is not the reader's, or already deleted, does not exist for them.
func (s *FeedService) Get(ctx context.Context, userID, id uuid.UUID) (NotificationDetail, error) {
	n, err := s.feed.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return NotificationDetail{}, ErrNotFound
		}
		return NotificationDetail{}, fmt.Errorf("get notification: %w", err)
	}
	if n.UserID != userID || n.DeletedAt != nil {
		return NotificationDetail{}, ErrNotFound
	}

	actions, err := availableActions(ctx, s.live, userID, n)
	if err != nil {
		return NotificationDetail{}, fmt.Errorf("compute notification actions: %w", err)
	}
	return NotificationDetail{Notification: n, Actions: actions}, nil
}

// availableActions computes which of the event's catalog actions still stand
// for this reader: the state-gone and entity-gone rules of решение #737.
// The service categories' navigation actions (Тариф — open_tariffs,
// open_payment_methods) are global screens and always stand.
func availableActions(
	ctx context.Context, live FeedLiveState, reader uuid.UUID, n domain.Notification,
) ([]domain.ActionKind, error) {
	candidates := domain.ActionsForEvent(n.EventType)
	if len(candidates) == 0 {
		return nil, nil
	}
	out := make([]domain.ActionKind, 0, len(candidates))
	for _, kind := range candidates {
		stands, err := actionStands(ctx, live, reader, kind, n)
		if err != nil {
			return nil, err
		}
		if stands {
			out = append(out, kind)
		}
	}
	return out, nil
}

// actionStands answers one candidate action's availability against the live
// state. The global screens (open_tariffs, open_payment_methods) have no
// entity to live-check — their buttons always stand.
func actionStands(
	ctx context.Context, live FeedLiveState, reader uuid.UUID, kind domain.ActionKind, n domain.Notification,
) (bool, error) {
	switch kind {
	case domain.ActionRentalExtend, domain.ActionRentalComplete:
		if n.Payload.RentalID == nil {
			return false, nil
		}
		state, err := live.RentalActionState(ctx, *n.Payload.RentalID)
		if err != nil {
			return false, err
		}
		if !state.AwaitingAction {
			return false, nil
		}
		access, err := live.PropertyAccess(ctx, state.PropertyID, reader)
		if err != nil {
			return false, err
		}
		return access.Manageable, nil
	case domain.ActionOpenPayment:
		if n.Payload.PaymentID == nil {
			return false, nil
		}
		return live.PaymentOpen(ctx, *n.Payload.PaymentID)
	case domain.ActionOpenTask:
		if n.Payload.TaskID == nil {
			return false, nil
		}
		return live.TaskOpen(ctx, *n.Payload.TaskID)
	case domain.ActionOpenProperty, domain.ActionOpenPropertyMembers:
		if n.Payload.Property == nil {
			return false, nil
		}
		access, err := live.PropertyAccess(ctx, n.Payload.Property.ID, reader)
		if err != nil {
			return false, err
		}
		return access.Exists, nil
	default:
		return true, nil
	}
}
