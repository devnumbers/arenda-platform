package postgres

// The global read side of the payment rules (ticket #575): the actor-scoped
// cross-property read whose visibility predicate lives in the SQL itself
// (the tasks global feed's rule, ticket #521). The owner→today map arrives
// from the application layer (ADR 0048) and travels into the queries as
// parallel csv lists — uuids and ISO dates hold no commas, the list order is
// the map's sorted keys so the output never depends on map iteration.

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// GlobalPaymentStore implements application.GlobalPaymentReader over the
// generated payments queries. Read-only: none of it ticks.
type GlobalPaymentStore struct {
	db postgres.DBTX
	// Payments reuses the CRUD read for the projection fallback's single
	// rule load (the same nested-path query, pauses attached).
	payments *PaymentStore
}

// NewGlobalPaymentStore builds the global payments read store.
func NewGlobalPaymentStore(db postgres.DBTX) *GlobalPaymentStore {
	return &GlobalPaymentStore{db: db, payments: NewPaymentStore(db)}
}

// Compile-time conformance of the adapter to the consumer-declared port.
var _ application.GlobalPaymentReader = (*GlobalPaymentStore)(nil)

func (s *GlobalPaymentStore) q() *postgres.Queries {
	return postgres.New(s.db)
}

// ListGlobalPaymentOwnerTodays returns the distinct data owners of the
// actor's visible non-archived properties.
func (s *GlobalPaymentStore) ListGlobalPaymentOwnerTodays(
	ctx context.Context, actor uuid.UUID,
) ([]uuid.UUID, error) {
	rows, err := s.q().ListGlobalPaymentOwnerTodays(ctx, pgconv.UUIDToPgtype(actor))
	if err != nil {
		return nil, fmt.Errorf("list global payment owner todays: %w", err)
	}
	owners := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		owners = append(owners, pgconv.UUIDFromPgtype(row))
	}
	return owners, nil
}

// ListGlobalPaymentRules returns the actor's visible merged feed rows under
// the query's search, every row carrying its owner's today.
func (s *GlobalPaymentStore) ListGlobalPaymentRules(
	ctx context.Context, actor uuid.UUID, todays map[uuid.UUID]time.Time, q application.GlobalPaymentRulesQuery,
) ([]application.GlobalPaymentRuleRow, error) {
	ownerIDs, todaysCSV, err := ownerTodaysCSV(todays)
	if err != nil {
		return nil, err
	}
	rows, err := s.q().ListGlobalPaymentRules(ctx, postgres.ListGlobalPaymentRulesParams{
		OwnerIds:      ownerIDs,
		Todays:        todaysCSV,
		Actor:         pgconv.UUIDToPgtype(actor),
		Search:        escapeLikePattern(q.Search),
		CategorySlugs: strings.Join(q.CategorySlugs, ","),
	})
	if err != nil {
		return nil, fmt.Errorf("list global payment rules: %w", err)
	}
	out := make([]application.GlobalPaymentRuleRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, application.GlobalPaymentRuleRow{
			ID:            pgconv.UUIDFromPgtype(row.ID),
			OwnerID:       pgconv.UUIDFromPgtype(row.OwnerID),
			PropertyID:    pgconv.UUIDFromPgtype(row.PropertyID),
			PropertyName:  row.PropertyName,
			Type:          domain.PaymentType(row.Type),
			Title:         row.Title,
			AmountKopecks: row.AmountKopecks,
			AutoPay:       row.AutoPay,
			IsFavorite:    row.IsFavorite,
			Category: domain.CategoryRef{
				Slug:             pgconv.TextToPtrString(row.CategorySlug),
				UserCategoryID:   pgconv.UUIDFromPgtypePtr(row.UserCategoryID),
				UserCategoryName: pgconv.TextToPtrString(row.UserCategoryName),
			},
			Today:             pgconv.DateFromPgtype(row.OwnerToday),
			NextPlannedDate:   pgconv.DatePtrFromPgtype(row.AggNextPlannedDate),
			OverdueCount:      row.OverdueCount,
			OldestOverdueDate: pgconv.DatePtrFromPgtype(row.AggOldestOverdueDate),
		})
	}
	return out, nil
}

// SumGlobalPaymentCounters returns the scope counters over the whole
// visible feed.
func (s *GlobalPaymentStore) SumGlobalPaymentCounters(
	ctx context.Context, actor uuid.UUID, todays map[uuid.UUID]time.Time,
) (application.GlobalPaymentCounters, error) {
	ownerIDs, todaysCSV, err := ownerTodaysCSV(todays)
	if err != nil {
		return application.GlobalPaymentCounters{}, err
	}
	row, err := s.q().SumGlobalPaymentCounters(ctx, postgres.SumGlobalPaymentCountersParams{
		OwnerIds: ownerIDs,
		Todays:   todaysCSV,
		Actor:    pgconv.UUIDToPgtype(actor),
	})
	if err != nil {
		return application.GlobalPaymentCounters{}, fmt.Errorf("sum global payment counters: %w", err)
	}
	return application.GlobalPaymentCounters{
		FavoriteCount:          row.FavoriteCount,
		OverdueOperationsCount: row.OverdueOperationsCount,
	}, nil
}

// SumGlobalPaymentSearchCategories returns the matched categories of the
// search — the chips' identities and their rule counts.
func (s *GlobalPaymentStore) SumGlobalPaymentSearchCategories(
	ctx context.Context, actor uuid.UUID, q application.GlobalPaymentRulesQuery,
) ([]application.GlobalPaymentSearchCategory, error) {
	rows, err := s.q().SumGlobalPaymentSearchCategories(ctx, postgres.SumGlobalPaymentSearchCategoriesParams{
		Actor:         pgconv.UUIDToPgtype(actor),
		Search:        escapeLikePattern(q.Search),
		CategorySlugs: strings.Join(q.CategorySlugs, ","),
	})
	if err != nil {
		return nil, fmt.Errorf("sum global payment search categories: %w", err)
	}
	out := make([]application.GlobalPaymentSearchCategory, 0, len(rows))
	for _, row := range rows {
		out = append(out, application.GlobalPaymentSearchCategory{
			Category: domain.CategoryRef{
				Slug:             pgconv.TextToPtrString(row.CategorySlug),
				UserCategoryID:   pgconv.UUIDFromPgtypePtr(row.UserCategoryID),
				UserCategoryName: pgconv.TextToPtrString(row.UserCategoryLabel),
			},
			Type:      domain.PaymentType(row.Type),
			RuleCount: row.RuleCount,
		})
	}
	return out, nil
}

// ListGlobalPaymentObjects returns the actor's visible non-archived
// properties under the object search.
func (s *GlobalPaymentStore) ListGlobalPaymentObjects(
	ctx context.Context, actor uuid.UUID, search string,
) ([]application.GlobalPaymentObject, error) {
	rows, err := s.q().ListGlobalPaymentObjects(ctx, postgres.ListGlobalPaymentObjectsParams{
		Actor:  pgconv.UUIDToPgtype(actor),
		Search: escapeLikePattern(search),
	})
	if err != nil {
		return nil, fmt.Errorf("list global payment objects: %w", err)
	}
	out := make([]application.GlobalPaymentObject, 0, len(rows))
	for _, row := range rows {
		out = append(out, application.GlobalPaymentObject{
			PropertyID: pgconv.UUIDFromPgtype(row.ID),
			Name:       row.Name,
			Address:    row.Address,
		})
	}
	return out, nil
}

// LastOperationDatesOfPayments returns the newest materialized date across
// planned and paid per listed rule.
func (s *GlobalPaymentStore) LastOperationDatesOfPayments(
	ctx context.Context, paymentIDs []uuid.UUID,
) (map[uuid.UUID]time.Time, error) {
	if len(paymentIDs) == 0 {
		return map[uuid.UUID]time.Time{}, nil
	}
	rows, err := s.q().LastOperationDatesOfPayments(ctx, joinPropertyIDs(paymentIDs))
	if err != nil {
		return nil, fmt.Errorf("last operation dates of payments: %w", err)
	}
	out := make(map[uuid.UUID]time.Time, len(rows))
	for _, row := range rows {
		if !row.PaymentID.Valid {
			continue
		}
		out[pgconv.UUIDFromPgtype(row.PaymentID)] = pgconv.DateFromPgtype(row.LastDate)
	}
	return out, nil
}

// GetGlobalPaymentRule loads one rule with its pause intervals — the
// projection fallback's input; false when the rule is gone. The CRUD read
// carries the same nested path payment→property, its ErrNotFound folding
// into the flag here at the adapter edge.
func (s *GlobalPaymentStore) GetGlobalPaymentRule(
	ctx context.Context, ownerID, propertyID, paymentID uuid.UUID,
) (domain.Payment, bool, error) {
	rule, err := s.payments.Get(ctx, paymentID, ownerID, propertyID)
	if err != nil {
		if errors.Is(err, application.ErrNotFound) {
			return domain.Payment{}, false, nil
		}
		return domain.Payment{}, false, fmt.Errorf("get payment %s for projection: %w", paymentID, err)
	}
	return rule, true, nil
}

// ownerTodaysCSV folds the owner→today map into the queries' parallel csv
// lists, keyed in sorted order so the output never depends on map
// iteration.
func ownerTodaysCSV(todays map[uuid.UUID]time.Time) (ownerIDsCSV, todaysCSV string, err error) {
	if len(todays) == 0 {
		return "", "", nil
	}
	owners := slices.Collect(maps.Keys(todays))
	slices.SortFunc(owners, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
	ids := make([]string, 0, len(owners))
	dates := make([]string, 0, len(owners))
	for _, owner := range owners {
		today := todays[owner]
		if today.IsZero() {
			return "", "", fmt.Errorf("global payments: no today resolved for owner %s", owner)
		}
		ids = append(ids, owner.String())
		dates = append(dates, today.Format(time.DateOnly))
	}
	return strings.Join(ids, ","), strings.Join(dates, ","), nil
}
