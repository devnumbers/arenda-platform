package application

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// GlobalPaymentItem is one card of the global payment rules feed (ticket
// #575): the rule, its property label, the «Ближайший» schedule date and
// the overdue aggregates — the main screen's row data and the search's row
// composition alike.
type GlobalPaymentItem struct {
	ID            uuid.UUID
	PropertyID    uuid.UUID
	PropertyName  string
	Type          domain.PaymentType
	Title         string
	AmountKopecks int64
	Category      domain.CategoryRef
	AutoPay       bool
	IsFavorite    bool
	// FavoriteOrder is the rule's manual favorite order (ticket #576): the
	// 1-based position the favorites edit mode's save assigned, nil when
	// the rule has never been in a saved order — favorites sort by it with
	// nulls last (the new-favorite-at-the-end rule); the feed's own order
	// is unchanged.
	FavoriteOrder *int64
	// Today is the property owner's calendar date (ADR 0048) the row's
	// schedule math ran against — the client renders «Сегодня»/«Завтра»
	// against it without knowing the owner's timezone.
	Today time.Time
	// NearestDate is «Ближайший»: the earliest stored planned operation on
	// or after today, else the rule's projection — a bare schedule order
	// without a view status. Nil when the schedule has no next occurrence
	// (an open pause or a settled rule).
	NearestDate *time.Time
	// OverdueCount is the rule's overdue operations; OverdueDays the age of
	// the oldest one in days (at least 1) — the «N дней» line; nil when the
	// rule has no overdue operations.
	OverdueCount int64
	OverdueDays  *int
}

// GlobalPaymentFeed is the global «Платежи» feed (ticket #575) with the
// main screen's counters: the sections' rows plus the «Все избранные (N)»
// and «Все просроченные (N)» cards' numbers.
type GlobalPaymentFeed struct {
	Items                  []GlobalPaymentItem
	FavoriteCount          int64
	OverdueOperationsCount int64
}

// GlobalPaymentSearch is the global payment rules search's response (ticket
// #575): the matched rows plus the matched categories — the chips.
type GlobalPaymentSearch struct {
	Items             []GlobalPaymentItem
	MatchedCategories []GlobalPaymentSearchCategory
}

// GlobalPaymentObjectKey is one key of a property's stack (ticket #575):
// the rule behind it and its overdue flag — the red dot.
type GlobalPaymentObjectKey struct {
	PaymentID  uuid.UUID
	HasOverdue bool
}

// GlobalPaymentObjectCard is one visible property with its payment stacks
// (ticket #575): the keys grouped into the auto-pay group and the rest.
type GlobalPaymentObjectCard struct {
	PropertyID uuid.UUID
	Name       string
	Address    string
	// PinnedAt is the property's global pin (ticket #577): nil — not pinned,
	// a moment — pinned since then; the objects read orders the pinned first.
	PinnedAt    *time.Time
	AutoPayKeys []GlobalPaymentObjectKey
	OtherKeys   []GlobalPaymentObjectKey
}

// GlobalPaymentService serves the global payment rules surface (tickets
// #575, #576): the merged «Платежи» feed with the main screen's counters,
// the search with its matched-category chips, the «Объекты» stacks — and
// the favorites manual order, the surface's first cross-property mutation.
// The visibility predicate lives in the SQL of both sides (the actor-scoped
// cross-property read, ticket #521); the owner→today map is resolved here
// (ADR 0048) and threaded into every per-row schedule computation — the
// merged feed mixes owners. The reads never tick; the order save writes
// positions only, so it never ticks either.
type GlobalPaymentService struct {
	reader   GlobalPaymentReader
	calendar OwnerCalendar
	factory  txStoreFactory
}

// NewGlobalPaymentService builds the global reads and the favorites order
// save over the reader port, the owner calendar and the shared store
// factory. A nil calendar is a wiring mistake and fails on first use (the
// shared ownerToday helper's contract).
func NewGlobalPaymentService(
	reader GlobalPaymentReader, calendar OwnerCalendar, factory txStoreFactory,
) *GlobalPaymentService {
	return &GlobalPaymentService{reader: reader, calendar: calendar, factory: factory}
}

// ListGlobalPayments returns the actor's visible merged feed of payment
// rules with the scope counters (ticket #575). The rules of archived
// properties are not in the feed; no reminders, no «На оплату» (out of
// scope of map #573). Reads never tick.
func (s *GlobalPaymentService) ListGlobalPayments(ctx context.Context, actor uuid.UUID) (GlobalPaymentFeed, error) {
	todays, err := s.ownerTodays(ctx, actor)
	if err != nil {
		return GlobalPaymentFeed{}, err
	}
	items, err := s.feedItems(ctx, actor, todays, "")
	if err != nil {
		return GlobalPaymentFeed{}, err
	}
	counters, err := s.reader.SumGlobalPaymentCounters(ctx, actor, todays)
	if err != nil {
		return GlobalPaymentFeed{}, fmt.Errorf("sum global payment counters: %w", err)
	}
	return GlobalPaymentFeed{
		Items:                  items,
		FavoriteCount:          counters.FavoriteCount,
		OverdueOperationsCount: counters.OverdueOperationsCount,
	}, nil
}

// SearchGlobalPayments narrows the feed by the search query — a
// case-insensitive substring over the title and the category (the default
// catalog label of the rule's slug or the user category's name) — and
// returns the matched categories of the matched rules: the chips. The
// counters are not part of the search contract.
func (s *GlobalPaymentService) SearchGlobalPayments(
	ctx context.Context, actor uuid.UUID, search string,
) (GlobalPaymentSearch, error) {
	todays, err := s.ownerTodays(ctx, actor)
	if err != nil {
		return GlobalPaymentSearch{}, err
	}
	items, err := s.feedItems(ctx, actor, todays, search)
	if err != nil {
		return GlobalPaymentSearch{}, err
	}
	categories, err := s.reader.SumGlobalPaymentSearchCategories(ctx, actor, GlobalPaymentRulesQuery{
		Search:        search,
		CategorySlugs: defaultCategorySlugsMatching(search),
	})
	if err != nil {
		return GlobalPaymentSearch{}, fmt.Errorf("sum global payment search categories: %w", err)
	}
	return GlobalPaymentSearch{Items: items, MatchedCategories: categories}, nil
}

// ListGlobalPaymentObjects returns the actor's visible non-archived
// properties with their payment stacks (ticket #575): the rules grouped
// into the auto-pay group and the rest, every key carrying its overdue
// flag. The search filters the objects by name or address; the stacks
// always carry the object's every rule whatever the query matched.
func (s *GlobalPaymentService) ListGlobalPaymentObjects(
	ctx context.Context, actor uuid.UUID, search string,
) ([]GlobalPaymentObjectCard, error) {
	todays, err := s.ownerTodays(ctx, actor)
	if err != nil {
		return nil, err
	}
	if len(todays) == 0 {
		return []GlobalPaymentObjectCard{}, nil
	}
	// The stacks are the object's every rule: the rows run unsearched even
	// when the query filters the objects.
	rows, err := s.reader.ListGlobalPaymentRules(ctx, actor, todays, GlobalPaymentRulesQuery{})
	if err != nil {
		return nil, fmt.Errorf("list global payment rules: %w", err)
	}
	keysByProperty := make(map[uuid.UUID][]GlobalPaymentRuleRow, len(rows))
	for _, row := range rows {
		keysByProperty[row.PropertyID] = append(keysByProperty[row.PropertyID], row)
	}
	objects, err := s.reader.ListGlobalPaymentObjects(ctx, actor, search)
	if err != nil {
		return nil, fmt.Errorf("list global payment objects: %w", err)
	}
	cards := make([]GlobalPaymentObjectCard, 0, len(objects))
	for _, object := range objects {
		card := GlobalPaymentObjectCard{
			PropertyID:  object.PropertyID,
			Name:        object.Name,
			Address:     object.Address,
			PinnedAt:    object.PinnedAt,
			AutoPayKeys: []GlobalPaymentObjectKey{},
			OtherKeys:   []GlobalPaymentObjectKey{},
		}
		for _, row := range keysByProperty[object.PropertyID] {
			key := GlobalPaymentObjectKey{PaymentID: row.ID, HasOverdue: row.OverdueCount > 0}
			if row.AutoPay {
				card.AutoPayKeys = append(card.AutoPayKeys, key)
			} else {
				card.OtherKeys = append(card.OtherKeys, key)
			}
		}
		cards = append(cards, card)
	}
	return cards, nil
}

// SaveFavoriteOrder rebuilds the manual order of the actor's favorites
// (PUT /payments/favorites/order, ticket #576): the submitted list is the
// actor's visible favorite rules in their new order, and one transaction
// locks the rows (the sorted ids being the deadlock-safe lock order) and
// assigns dense 1-based positions in the list order. The save is a full
// replacement: visible favorites OUTSIDE the list lose their positions and
// fall to the end of the reading order — duplicate positions never survive
// a save, an empty list resets the whole order. The visibility predicate
// and the per-row role live in the lock's SQL: an unknown, foreign or
// otherwise invisible id is the privacy-preserving ErrNotFound, a visible
// non-favorite or a duplicate id is ErrInvalidInput, and the write
// capability is the favorite star's (#461, Full Access+ via CanEdit) — a
// viewer's ErrForbidden. A rule without a position (new and legacy
// favorites, and whatever this save did not place) sorts last — the
// new-favorite-at-the-end rule — and PUT favorite keeps that
// null-when-unstarred invariant atomic. A pure order write: nothing
// materializes, nothing ticks.
func (s *GlobalPaymentService) SaveFavoriteOrder(ctx context.Context, actor uuid.UUID, orderedIDs []uuid.UUID) error {
	seen := make(map[uuid.UUID]struct{}, len(orderedIDs))
	for _, id := range orderedIDs {
		if _, dup := seen[id]; dup {
			return ErrInvalidInput
		}
		seen[id] = struct{}{}
	}
	sorted := slices.Clone(orderedIDs)
	slices.SortFunc(sorted, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })

	return s.factory.runInTx(ctx, func(stores *txStores) error {
		locked, err := stores.favoriteOrders.LockVisibleFavorites(ctx, actor, sorted)
		if err != nil {
			return fmt.Errorf("lock visible favorites: %w", err)
		}
		if len(locked) < len(sorted) {
			return ErrNotFound
		}
		allOwner := true
		for _, row := range locked {
			if !row.IsFavorite {
				return ErrInvalidInput
			}
			if !sharedpolicy.CanEdit(row.Role) {
				return ErrForbidden
			}
			allOwner = allOwner && row.Role == sharedpolicy.RoleOwner
		}
		if err := stores.favoriteOrders.ClearFavoriteOrdersOutside(ctx, actor, sorted); err != nil {
			return fmt.Errorf("clear favorite orders outside the save: %w", err)
		}
		for position, id := range orderedIDs {
			if err := stores.favoriteOrders.SaveFavoritePosition(ctx, id, int64(position+1)); err != nil {
				return fmt.Errorf("save favorite position of %s: %w", id, err)
			}
		}
		// The save's audit role is the actor's own scope when every placed
		// rule is theirs; any shared rule makes the acting role the
		// membership's (Full Access — a viewer cannot reach here).
		auditRole := sharedpolicy.RoleOwner
		if !allOwner {
			auditRole = sharedpolicy.RoleFullAccess
		}
		return recordAudit(ctx, stores, actor, auditRole, auditdomain.ActionPaymentUpdated,
			auditdomain.EntityPayment, nil, map[string]any{
				auditFieldsKey: []string{"favorite_order"},
				"count":        len(orderedIDs),
			})
	})
}

// ownerTodays resolves the calendar date per distinct data owner of the
// actor's visible non-archived properties (ADR 0048): the merged feed's
// schedule math runs against each row's own owner's today.
func (s *GlobalPaymentService) ownerTodays(ctx context.Context, actor uuid.UUID) (map[uuid.UUID]time.Time, error) {
	owners, err := s.reader.ListGlobalPaymentOwnerTodays(ctx, actor)
	if err != nil {
		return nil, fmt.Errorf("list global payment owners: %w", err)
	}
	todays := make(map[uuid.UUID]time.Time, len(owners))
	for _, owner := range owners {
		today, err := ownerToday(s.calendar, ctx, owner)
		if err != nil {
			return nil, err
		}
		todays[owner] = today
	}
	return todays, nil
}

// feedItems runs the feed query and enriches the rows: the nearest-date
// fallback and the overdue age are the application layer's business.
func (s *GlobalPaymentService) feedItems(
	ctx context.Context, actor uuid.UUID, todays map[uuid.UUID]time.Time, search string,
) ([]GlobalPaymentItem, error) {
	if len(todays) == 0 {
		return []GlobalPaymentItem{}, nil
	}
	rows, err := s.reader.ListGlobalPaymentRules(ctx, actor, todays, GlobalPaymentRulesQuery{
		Search:        search,
		CategorySlugs: defaultCategorySlugsMatching(search),
	})
	if err != nil {
		return nil, fmt.Errorf("list global payment rules: %w", err)
	}
	return s.enrichItems(ctx, rows)
}

// enrichItems turns the raw rows into response items. Rows without a
// stored next planned fall back to the pure projection (the pause and the
// settled rule being the true nulls); the overdue age is the owner today
// minus the oldest overdue date.
func (s *GlobalPaymentService) enrichItems(
	ctx context.Context, rows []GlobalPaymentRuleRow,
) ([]GlobalPaymentItem, error) {
	fallback := make([]int, 0)
	for i, row := range rows {
		if row.NextPlannedDate == nil {
			fallback = append(fallback, i)
		}
	}
	projected := map[uuid.UUID]*time.Time{}
	if len(fallback) > 0 {
		ids := make([]uuid.UUID, 0, len(fallback))
		for _, i := range fallback {
			ids = append(ids, rows[i].ID)
		}
		lastDates, err := s.reader.LastOperationDatesOfPayments(ctx, ids)
		if err != nil {
			return nil, fmt.Errorf("last operation dates of payments: %w", err)
		}
		for _, i := range fallback {
			row := rows[i]
			next, ok, err := s.projectedNext(ctx, row, lastDates[row.ID])
			if err != nil {
				return nil, err
			}
			if ok {
				projected[row.ID] = &next
			}
		}
	}
	items := make([]GlobalPaymentItem, len(rows))
	for i, row := range rows {
		item := GlobalPaymentItem{
			ID:            row.ID,
			PropertyID:    row.PropertyID,
			PropertyName:  row.PropertyName,
			Type:          row.Type,
			Title:         row.Title,
			AmountKopecks: row.AmountKopecks,
			Category:      row.Category,
			AutoPay:       row.AutoPay,
			IsFavorite:    row.IsFavorite,
			FavoriteOrder: row.FavoriteOrder,
			Today:         row.Today,
			OverdueCount:  row.OverdueCount,
		}
		switch {
		case row.NextPlannedDate != nil:
			item.NearestDate = row.NextPlannedDate
		case projected[row.ID] != nil:
			item.NearestDate = projected[row.ID]
		}
		if row.OldestOverdueDate != nil && row.OverdueCount > 0 {
			days := int(row.Today.Sub(*row.OldestOverdueDate).Hours() / 24)
			item.OverdueDays = &days
		}
		items[i] = item
	}
	return items, nil
}

// projectedNext computes the rule's first occurrence after the projection
// cursor — the newest materialized date, or yesterday when nothing has
// been materialized (the IsCompleted cursor rule). The boolean is false
// when the schedule has nothing to project: an open pause cuts the
// occurrences, so a paused rule projects nothing — the true null — and a
// rule gone mid-read projects nothing either, its row showing no next.
func (s *GlobalPaymentService) projectedNext(
	ctx context.Context, row GlobalPaymentRuleRow, lastDate time.Time,
) (time.Time, bool, error) {
	rule, found, err := s.reader.GetGlobalPaymentRule(ctx, row.OwnerID, row.PropertyID, row.ID)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("load payment rule for projection: %w", err)
	}
	if !found {
		return time.Time{}, false, nil // The rule is gone mid-read; its row shows no next.
	}
	cursor := row.Today.AddDate(0, 0, -1)
	if lastDate.After(cursor) {
		cursor = lastDate
	}
	next, ok := domain.NextOccurrenceAfter(rule, cursor)
	return next, ok, nil
}

// defaultCategorySlugsMatching expands the search query into the default
// catalog slugs whose label contains it — the database stores the slug,
// not the label, so the label match is the application layer's business.
// The matching mirrors the SQL's ILIKE substring: a case-insensitive
// contains, no trimming. An empty query expands to nothing (no filter).
func defaultCategorySlugsMatching(query string) []string {
	if query == "" {
		return nil
	}
	needle := strings.ToLower(query)
	var slugs []string
	for _, entry := range domain.CategoryCatalogEntries() {
		if strings.Contains(strings.ToLower(entry.Label), needle) {
			slugs = append(slugs, entry.Slug)
		}
	}
	return slugs
}
