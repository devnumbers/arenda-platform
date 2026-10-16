package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// SubscriptionTransitionRepository appends to and reads the immutable
// subscription transition log. UPDATE is rejected at the database level
// (ADR 0037), so the repository has no mutation methods besides Append — and
// ShiftGraceEntryTimes, the stand-only time travel of the dunning anchor
// (issue #665), which moves rows as a delete-and-reinsert because the trigger
// leaves DELETE deliberately unguarded.
type SubscriptionTransitionRepository struct {
	db postgres.DBTX
}

// NewSubscriptionTransitionRepository creates a new transition repository.
func NewSubscriptionTransitionRepository(db postgres.DBTX) *SubscriptionTransitionRepository {
	return &SubscriptionTransitionRepository{db: db}
}

func (r *SubscriptionTransitionRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *SubscriptionTransitionRepository) WithTx(tx transaction.Tx) (application.SubscriptionTransitionRepository, error) {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return nil, fmt.Errorf("billing.SubscriptionTransitionRepository.WithTx: %T is not a postgres.DBTX", tx)
	}
	return NewSubscriptionTransitionRepository(dbtx), nil
}

// Append records a transition. Callers append inside the same transaction that
// mutates the subscription, so a transition can never exist for a state that
// was rolled back.
func (r *SubscriptionTransitionRepository) Append(ctx context.Context, transition domain.Transition) error {
	params := postgres.AppendSubscriptionTransitionParams{
		ID:             pgtype.UUID{Bytes: transition.ID, Valid: true},
		SubscriptionID: pgtype.UUID{Bytes: transition.SubscriptionID, Valid: true},
		ToStatus:       string(transition.ToStatus),
		ToTariffID:     pgtype.UUID{Bytes: transition.ToTariffID, Valid: true},
		Reason:         string(transition.Reason),
		InitiatorType:  string(transition.Initiator),
	}
	if transition.FromStatus != nil {
		params.FromStatus = pgtype.Text{String: string(*transition.FromStatus), Valid: true}
	}
	if transition.FromTariffID != nil {
		params.FromTariffID = pgtype.UUID{Bytes: *transition.FromTariffID, Valid: true}
	}
	if transition.InitiatorID != nil {
		params.InitiatorID = pgtype.UUID{Bytes: *transition.InitiatorID, Valid: true}
	}
	if transition.PaymentID != nil {
		params.PaymentID = pgtype.UUID{Bytes: *transition.PaymentID, Valid: true}
	}
	if err := r.q().AppendSubscriptionTransition(ctx, params); err != nil {
		return fmt.Errorf("append subscription transition: %w", err)
	}
	return nil
}

// ListBySubscriptionID returns the subscription's transitions, newest first.
func (r *SubscriptionTransitionRepository) ListBySubscriptionID(
	ctx context.Context, subscriptionID uuid.UUID,
) ([]domain.Transition, error) {
	rows, err := r.q().ListSubscriptionTransitionsBySubscription(ctx, pgtype.UUID{Bytes: subscriptionID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list subscription transitions: %w", err)
	}
	transitions := make([]domain.Transition, 0, len(rows))
	for _, row := range rows {
		transition, err := mapTransition(row)
		if err != nil {
			return nil, fmt.Errorf("map transition row: %w", err)
		}
		transitions = append(transitions, transition)
	}
	return transitions, nil
}

// ShiftGraceEntryTimes moves the created_at of every grace-entry transition by
// the signed delta (issue #665) — the stand-only time travel of the dunning
// anchor. The immutability trigger rejects UPDATE (ADR 0037), so each row
// moves as a delete-and-reinsert of the same values with the shifted
// timestamp; the row data is copied verbatim from what was read, so nothing
// but created_at can change. Runs inside the caller's transaction.
func (r *SubscriptionTransitionRepository) ShiftGraceEntryTimes(
	ctx context.Context, subscriptionID uuid.UUID, delta time.Duration,
) (int, error) {
	rows, err := r.q().ListSubscriptionTransitionsBySubscription(ctx, pgtype.UUID{Bytes: subscriptionID, Valid: true})
	if err != nil {
		return 0, fmt.Errorf("list subscription transitions for the time shift: %w", err)
	}
	entries := make([]postgres.SubscriptionTransition, 0, len(rows))
	for _, row := range rows {
		if domain.TransitionReason(row.Reason) == domain.TransitionReasonGraceEntered {
			entries = append(entries, row)
		}
	}
	if len(entries) == 0 {
		return 0, nil
	}
	if err := r.q().DeleteSubscriptionGraceEntryTransitions(ctx, pgtype.UUID{Bytes: subscriptionID, Valid: true}); err != nil {
		return 0, fmt.Errorf("delete grace-entry transitions for the time shift: %w", err)
	}
	for _, row := range entries {
		params := postgres.AppendSubscriptionTransitionWithCreatedAtParams{
			ID:             row.ID,
			SubscriptionID: row.SubscriptionID,
			FromStatus:     row.FromStatus,
			ToStatus:       row.ToStatus,
			FromTariffID:   row.FromTariffID,
			ToTariffID:     row.ToTariffID,
			Reason:         row.Reason,
			InitiatorType:  row.InitiatorType,
			InitiatorID:    row.InitiatorID,
			PaymentID:      row.PaymentID,
			CreatedAt:      pgtype.Timestamptz{Time: row.CreatedAt.Time.Add(delta), Valid: true},
		}
		if err := r.q().AppendSubscriptionTransitionWithCreatedAt(ctx, params); err != nil {
			return 0, fmt.Errorf("reinsert shifted grace-entry transition: %w", err)
		}
	}
	return len(entries), nil
}

func mapTransition(row postgres.SubscriptionTransition) (domain.Transition, error) {
	return domain.ReconstituteTransition(domain.Transition{
		ID:             pgconv.UUIDFromPgtype(row.ID),
		SubscriptionID: pgconv.UUIDFromPgtype(row.SubscriptionID),
		FromStatus:     statusPtrFromPgtype(row.FromStatus),
		ToStatus:       domain.SubscriptionStatus(row.ToStatus),
		FromTariffID:   pgconv.UUIDFromPgtypePtr(row.FromTariffID),
		ToTariffID:     pgconv.UUIDFromPgtype(row.ToTariffID),
		Reason:         domain.TransitionReason(row.Reason),
		Initiator:      domain.TransitionInitiator(row.InitiatorType),
		InitiatorID:    pgconv.UUIDFromPgtypePtr(row.InitiatorID),
		PaymentID:      pgconv.UUIDFromPgtypePtr(row.PaymentID),
		CreatedAt:      row.CreatedAt.Time,
	})
}

func statusPtrFromPgtype(s pgtype.Text) *domain.SubscriptionStatus {
	if !s.Valid {
		return nil
	}
	status := domain.SubscriptionStatus(s.String)
	return &status
}
