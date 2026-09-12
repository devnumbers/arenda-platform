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
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// GlobalPaymentStore implements application.GlobalPaymentReader and the
// favorites order save (application.GlobalPaymentOrderStore, ticket #576)
// over the generated payments queries. The reads never tick; the order save
// writes positions only.
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

// Compile-time conformance of the adapter to the consumer-declared ports.
var (
	_ application.GlobalPaymentReader     = (*GlobalPaymentStore)(nil)
	_ application.GlobalPaymentOrderStore = (*GlobalPaymentStore)(nil)
)

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
		OwnerIds:       ownerIDs,
		Todays:         todaysCSV,
		Actor:          pgconv.UUIDToPgtype(actor),
		Search:         escapeLikePattern(q.Search),
		CategorySlugs:  strings.Join(q.CategorySlugs, ","),
		CategoryFilter: q.Category,
		TypeFilter:     string(q.Type),
		PageLimit:      q.Limit,
		AfterCreatedAt: pgconv.TimePtrToPgtype(q.AfterCreatedAt),
		AfterID:        pgconv.UUIDToPgtypePtr(q.AfterID),
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
			CreatedAt:                pgconv.TimestamptzToTime(row.CreatedAt),
			Today:                    pgconv.DateFromPgtype(row.OwnerToday),
			NextPlannedDate:          pgconv.DatePtrFromPgtype(row.AggNextPlannedDate),
			OverdueCount:             row.OverdueCount,
			OldestOverdueDate:        pgconv.DatePtrFromPgtype(row.AggOldestOverdueDate),
			OldestOverdueOperationID: pgconv.UUIDFromPgtypePtr(row.OldestOverdueOperationID),
			FavoriteOrder:            pgconv.Int8ToPtr(row.FavoriteOrder),
		})
	}
	return out, nil
}

// CountGlobalPaymentRules counts the search's whole-scope matches (ticket
// #599): the list's predicate — the search, the chip filter, the visibility
// — without the per-row aggregates and the keyset window.
func (s *GlobalPaymentStore) CountGlobalPaymentRules(
	ctx context.Context, actor uuid.UUID, todays map[uuid.UUID]time.Time, q application.GlobalPaymentRulesQuery,
) (int64, error) {
	ownerIDs, todaysCSV, err := ownerTodaysCSV(todays)
	if err != nil {
		return 0, err
	}
	count, err := s.q().CountGlobalPaymentRules(ctx, postgres.CountGlobalPaymentRulesParams{
		OwnerIds:       ownerIDs,
		Todays:         todaysCSV,
		Actor:          pgconv.UUIDToPgtype(actor),
		Search:         escapeLikePattern(q.Search),
		CategorySlugs:  strings.Join(q.CategorySlugs, ","),
		CategoryFilter: q.Category,
		TypeFilter:     string(q.Type),
	})
	if err != nil {
		return 0, fmt.Errorf("count global payment rules: %w", err)
	}
	return count, nil
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
// search — the chips' category identities, one per category (ticket #602),
// the largest match count first.
func (s *GlobalPaymentStore) SumGlobalPaymentSearchCategories(
	ctx context.Context, actor uuid.UUID, q application.GlobalPaymentRulesQuery,
) ([]domain.CategoryRef, error) {
	rows, err := s.q().SumGlobalPaymentSearchCategories(ctx, postgres.SumGlobalPaymentSearchCategoriesParams{
		Actor:         pgconv.UUIDToPgtype(actor),
		Search:        escapeLikePattern(q.Search),
		CategorySlugs: strings.Join(q.CategorySlugs, ","),
	})
	if err != nil {
		return nil, fmt.Errorf("sum global payment search categories: %w", err)
	}
	out := make([]domain.CategoryRef, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.CategoryRef{
			Slug:             pgconv.TextToPtrString(row.CategorySlug),
			UserCategoryID:   pgconv.UUIDFromPgtypePtr(row.UserCategoryID),
			UserCategoryName: pgconv.TextToPtrString(row.UserCategoryLabel),
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
		// The SQL hands '' for «no photo» (the lateral join's COALESCE — an
		// object without photos must not scan-fail), the port speaks nil.
		var photoURL *string
		if row.PhotoUrl != "" {
			url := row.PhotoUrl
			photoURL = &url
		}
		out = append(out, application.GlobalPaymentObject{
			PropertyID: pgconv.UUIDFromPgtype(row.ID),
			Name:       row.Name,
			Address:    row.Address,
			PinnedAt:   pgconv.TimestamptzToPtrTime(row.PinnedAt),
			PhotoURL:   photoURL,
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

// LockVisibleFavorites takes the favorites order save's FOR UPDATE row
// locks — the caller's ids arrive sorted and the SQL keeps that order, the
// deadlock-safety — and returns the rows visible to the actor on
// non-archived properties with the actor's per-row role (the SQL resolves
// owner vs the active membership's role).
func (s *GlobalPaymentStore) LockVisibleFavorites(
	ctx context.Context, actor uuid.UUID, ids []uuid.UUID,
) ([]application.GlobalPaymentFavoriteLock, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.q().LockGlobalPaymentFavorites(ctx, postgres.LockGlobalPaymentFavoritesParams{
		Ids:   joinPropertyIDs(ids),
		Actor: pgconv.UUIDToPgtype(actor),
	})
	if err != nil {
		return nil, fmt.Errorf("lock global payment favorites: %w", err)
	}
	out := make([]application.GlobalPaymentFavoriteLock, 0, len(rows))
	for _, row := range rows {
		out = append(out, application.GlobalPaymentFavoriteLock{
			ID:         pgconv.UUIDFromPgtype(row.ID),
			IsFavorite: row.IsFavorite,
			Role:       sharedpolicy.Role(row.ActorRole),
		})
	}
	return out, nil
}

// SaveFavoritePosition writes one rule's 1-based favorite position and
// requires exactly one affected row — the lock pass has already proven
// existence and visibility inside the same transaction, so a mismatch is a
// wiring defect, not a runtime condition.
func (s *GlobalPaymentStore) SaveFavoritePosition(ctx context.Context, id uuid.UUID, position int64) error {
	rows, err := s.q().SetPaymentFavoriteOrder(ctx, postgres.SetPaymentFavoriteOrderParams{
		ID:            pgconv.UUIDToPgtype(id),
		FavoriteOrder: pgconv.Int8PtrToPgtype(&position),
	})
	if err != nil {
		return fmt.Errorf("set favorite order of payment %s: %w", id, err)
	}
	if rows != 1 {
		return fmt.Errorf("set favorite order of payment %s: %d rows affected, want 1", id, rows)
	}
	return nil
}

// ClearFavoriteOrdersOutside drops the positions of the actor's visible
// favorites outside the submitted list — the save's full-replacement pass;
// an empty keep list clears the whole visible order.
func (s *GlobalPaymentStore) ClearFavoriteOrdersOutside(ctx context.Context, actor uuid.UUID, keepIDs []uuid.UUID) error {
	if _, err := s.q().ClearGlobalPaymentFavoriteOrders(ctx, postgres.ClearGlobalPaymentFavoriteOrdersParams{
		KeepIds: joinPropertyIDs(keepIDs),
		Actor:   pgconv.UUIDToPgtype(actor),
	}); err != nil {
		return fmt.Errorf("clear favorite orders outside the save: %w", err)
	}
	return nil
}

// WithTx binds the store to the caller's transaction — the favorites order
// save's writes ride the same Unit-of-Work as every payments mutation.
func (s *GlobalPaymentStore) WithTx(tx transaction.Tx) (application.GlobalPaymentOrderStore, error) {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return nil, fmt.Errorf("payments.GlobalPaymentStore.WithTx: %T is not a postgres.DBTX", tx)
	}
	return NewGlobalPaymentStore(dbtx), nil
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
