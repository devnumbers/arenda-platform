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
// (решение #737) over the owning tables directly: the rentals, operations,
// tasks, properties and membership rows decide what still stands — the feed
// stores none of it.
type FeedLiveState struct {
	db       postgres.DBTX
	calendar ownerTodayCalendar
}

// NewFeedLiveState creates the live-state adapter. The calendar resolves the
// owner's today for the rental's needs_attention state.
func NewFeedLiveState(db postgres.DBTX, calendar ownerTodayCalendar) *FeedLiveState {
	return &FeedLiveState{db: db, calendar: calendar}
}

func (r *FeedLiveState) q() *postgres.Queries {
	return postgres.New(r.db)
}

// RentalActionState reads the live rental: gone when the row is absent;
// awaiting action in the needs_attention state (not completed, planned end
// already passed against the owner's today, ADR 0053).
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

// PaymentOpen reports whether the operation still awaits payment.
func (r *FeedLiveState) PaymentOpen(ctx context.Context, paymentID uuid.UUID) (bool, error) {
	open, err := r.q().GetOperationOpenState(ctx, pgconv.UUIDToPgtype(paymentID))
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

// PropertyAccess resolves the reader's live role on the property: ” reads
// as a stranger, the missing row as a gone property.
func (r *FeedLiveState) PropertyAccess(ctx context.Context, propertyID, readerID uuid.UUID) (application.PropertyAccess, error) {
	role, err := r.q().GetPropertyAccessFor(ctx, postgres.GetPropertyAccessForParams{
		ID:      pgconv.UUIDToPgtype(propertyID),
		OwnerID: pgconv.UUIDToPgtype(readerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.PropertyAccess{}, nil
		}
		return application.PropertyAccess{}, fmt.Errorf("property access: %w", err)
	}
	if role == "" {
		return application.PropertyAccess{}, nil
	}
	return application.PropertyAccess{
		Exists:     true,
		Manageable: role == "owner" || role == "full_access",
	}, nil
}
