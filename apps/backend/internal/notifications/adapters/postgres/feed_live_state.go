package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// The adapter satisfies the consumer-declared port (CODING_STANDARDS).
var _ application.FeedLiveState = (*FeedLiveState)(nil)

// ownerTodayCalendar is the adapter's need behind the rental state: the data
// owner's calendar date (ADR 0048) — the same semantics the rentals context
// computes its statuses against (*paymentspg.OwnerCalendar in the wiring).
type ownerTodayCalendar interface {
	Today(ctx context.Context, ownerID uuid.UUID) (time.Time, error)
}

// FeedLiveState answers the action computation's live-state questions
// (решение #737) over the owning tables directly: the rentals, operations
// and tasks rows decide what still stands — the feed stores none of it. The
// reader's standing on a property resolves through the policy port (ADR
// 0028, the single actor-to-role point), never in SQL.
type FeedLiveState struct {
	db       postgres.DBTX
	calendar ownerTodayCalendar
	policy   policy.Policy
}

// NewFeedLiveState creates the live-state adapter: the calendar resolves the
// owner's today for the rental's needs_attention state, the policy resolves
// the reader's role on the property.
func NewFeedLiveState(db postgres.DBTX, calendar ownerTodayCalendar, pol policy.Policy) *FeedLiveState {
	return &FeedLiveState{db: db, calendar: calendar, policy: pol}
}

func (r *FeedLiveState) q() *postgres.Queries {
	return postgres.New(r.db)
}

// RentalActionState reads the live rental: gone when the row is absent;
// awaiting action in the needs_attention state (not completed, planned end
// already passed against the owner's today — the StatusOf semantics of
// ADR 0053).
func (r *FeedLiveState) RentalActionState(ctx context.Context, rentalID uuid.UUID) (application.RentalActionState, error) {
	row, err := r.q().GetRentalActionState(ctx, pgconv.UUIDToPgtype(rentalID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.RentalActionState{}, nil
		}
		return application.RentalActionState{}, fmt.Errorf("rental action state: %w", err)
	}
	today, err := r.calendar.Today(ctx, pgconv.UUIDFromPgtype(row.OwnerID))
	if err != nil {
		return application.RentalActionState{}, fmt.Errorf("rental action state: %w", err)
	}
	awaiting := !row.CompletedDate.Valid &&
		!row.StartDate.Time.After(today) &&
		row.PlannedEndDate.Valid && row.PlannedEndDate.Time.Before(today)
	return application.RentalActionState{
		Exists:         true,
		AwaitingAction: awaiting,
		PropertyID:     pgconv.UUIDFromPgtype(row.PropertyID),
	}, nil
}

// PaymentOpen reports whether the notification's own occurrence still
// awaits payment: a planned operation of the rule dated the operation date
// (решение #737: payment = правило + дата операции). The date pins the
// check to the notified occurrence — the rule's other, later occurrences
// say nothing about it.
func (r *FeedLiveState) PaymentOpen(ctx context.Context, paymentID uuid.UUID, dueDate time.Time) (bool, error) {
	open, err := r.q().GetPaymentOpenState(ctx, postgres.GetPaymentOpenStateParams{
		PaymentID: pgconv.UUIDToPgtype(paymentID),
		Date:      pgconv.DateToPgtype(dueDate),
	})
	if err != nil {
		return false, fmt.Errorf("payment open state: %w", err)
	}
	return open, nil
}

// TaskOpen reports whether the task exists and is not completed.
func (r *FeedLiveState) TaskOpen(ctx context.Context, taskID uuid.UUID) (bool, error) {
	open, err := r.q().GetTaskOpenState(ctx, pgconv.UUIDToPgtype(taskID))
	if err != nil {
		return false, fmt.Errorf("task open state: %w", err)
	}
	return open, nil
}

// PropertyAccess resolves the reader's standing on the property through the
// policy port: owner or full_access manage, a viewer reads, and everyone
// else — including a suspended membership and a deleted property — sees
// nothing (RoleNone is also the not-found mapping of ADR 0028).
func (r *FeedLiveState) PropertyAccess(ctx context.Context, propertyID, readerID uuid.UUID) (application.PropertyAccess, error) {
	role, err := r.policy.RoleForProperty(ctx, readerID, propertyID)
	if err != nil {
		return application.PropertyAccess{}, fmt.Errorf("property access: %w", err)
	}
	switch role {
	case policy.RoleOwner, policy.RoleFullAccess:
		return application.PropertyAccess{Exists: true, Manageable: true}, nil
	case policy.RoleViewer:
		return application.PropertyAccess{Exists: true, Manageable: false}, nil
	default: // RoleNone (a stranger or a gone property), RoleSuspended.
		return application.PropertyAccess{}, nil
	}
}
