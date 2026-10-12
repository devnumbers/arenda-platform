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
	// IsFavorite is the reading actor's personal star (the cutover #1308,
	// D12): the rule's row carries no favorite state — the actor's own row
	// in user_payment_favorites answers it.
	IsFavorite bool
	// FavoriteOrder is the rule's position in the actor's personal favorite
	// order (ticket #576): the 1-based position the favorites edit mode's
	// save assigned, nil when the rule has never been in the actor's saved
	// order — favorites sort by it with nulls last (the
	// new-favorite-at-the-end rule); the feed's own order is unchanged.
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
	// IsCompleted is the display-complete flag of the row (ticket #1341):
	// the computed settlement completeness (domain.IsCompleted — the
	// glossary's «Завершённый платёж») with the open-pause exception
	// resolved on the server — the pause's cut occurrences make the raw
	// flag vacuously true, and the feed row carries no pauses for the
	// client to repeat the resolution the payment screen does by the end
	// date. The hub card shows «Полностью оплачен» by it; the payment
	// detail read carries the raw flag (PaymentResponse.isCompleted).
	IsCompleted bool
	// OverdueCount is the rule's overdue operations; OverdueDays the age of
	// the oldest one in days (at least 1) — the «N дней» line; nil when the
	// rule has no overdue operations.
	OverdueCount int64
	OverdueDays  *int
	// OldestOverdueOperationID is the oldest overdue operation's id — the
	// overdue card's link target on the main screen (ticket #578); nil
	// when the rule has no overdue operations.
	OldestOverdueOperationID *uuid.UUID
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
// #575): the matched rows plus the matched categories — the chips, one
// category identity each (ticket #602). NextCursor is the keyset
// continuation (ticket #597) — ” when the matches are exhausted. Total is
// the whole scope's match count under the query and the chip filter
// (ticket #599) — «найдено N», the same on every walked page.
type GlobalPaymentSearch struct {
	Items             []GlobalPaymentItem
	MatchedCategories []domain.CategoryRef
	NextCursor        string
	Total             int64
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
	// PinnedAt is the reading actor's personal pin on the object (ticket
	// #577, the cutover #1307): nil — not pinned, a moment — pinned since
	// then; the objects read orders the pinned first.
	PinnedAt *time.Time
	// PhotoURL is the card avatar's photo — the object's first (oldest) one
	// (ticket #582); nil when the object has no photos.
	PhotoURL *string
	// Type is the property's type — the avatar placeholder glyph's key
	// (карта #1217, #1244).
	Type        string
	AutoPayKeys []GlobalPaymentObjectKey
	OtherKeys   []GlobalPaymentObjectKey
}

// GlobalPaymentService serves the global payment rules surface (tickets
// #575, #576): the merged «Платежи» feed with the main screen's counters,
// the search with its matched-category chips, the «Объекты» stacks — and
// the favorites manual order, the surface's first cross-property mutation,
// a personal one since the cutover #1308 (D12). The visibility predicate
// lives in the SQL of both sides (the actor-scoped cross-property read,
// ticket #521); the owner→today map is resolved here (ADR 0048) and
// threaded into every per-row schedule computation — the merged feed mixes
// owners. The reads never tick; the order save writes positions only, so it
// never ticks either. Personal states publish no realtime frames (D18) —
// the order save and the star are frame-less, other tabs of the actor
// re-read on their next load.
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
	items, err := s.feedItems(ctx, actor, todays, GlobalPaymentRulesQuery{
		Search:        "",
		CategorySlugs: defaultCategorySlugsMatching(""),
	})
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

// GlobalPaymentSearchPage is the search list's page request (map #573
// rework): the server-side chip filter plus the 50-per-page window the
// search screen's infinite scroll walks. The zero limit means the
// contract's default page; Cursor is the previous page's opaque continuation
// (” = from the beginning). The filter narrows the rules list only — the
// matched categories always describe the query's whole matched scope.
type GlobalPaymentSearchPage struct {
	Category string
	Type     domain.PaymentType
	Limit    int32
	Cursor   string
}

const (
	DefaultPaymentRulesPageSize = 50
	MaxPaymentRulesPageSize     = 100
)

// prepareGlobalPaymentSearchPage applies the page-size default, rejects the
// out-of-range numbers with ErrInvalidInput (the operations' canon) and
// decodes the page's continuation cursor (ticket #597); the empty cursor is
// the list's beginning, no keyset key.
func prepareGlobalPaymentSearchPage(page *GlobalPaymentSearchPage) (time.Time, uuid.UUID, bool, error) {
	if page.Limit == 0 {
		page.Limit = DefaultPaymentRulesPageSize
	}
	if page.Limit < 1 || page.Limit > MaxPaymentRulesPageSize {
		return time.Time{}, uuid.Nil, false, ErrInvalidInput
	}
	if page.Cursor == "" {
		return time.Time{}, uuid.Nil, false, nil
	}
	afterCreatedAt, afterID, err := decodeRuleCursor(page.Cursor)
	if err != nil {
		return time.Time{}, uuid.Nil, false, err
	}
	return afterCreatedAt, afterID, true, nil
}

// SearchGlobalPayments narrows the feed by the search query — a
// case-insensitive substring over the title and the category (the default
// catalog label of the rule's slug or the user category's name) — and
// returns the matched categories of the matched rules: the chips. The list
// runs in pages of 50 under the chip's category/type filter (the search
// screen's infinite scroll); the counters are not part of the search
// contract. The window is the feed's keyset walk (ticket #597): the page
// resumes strictly after the cursor's (created_at, id) and answers with the
// next page's cursor once it came back full.
func (s *GlobalPaymentService) SearchGlobalPayments(
	ctx context.Context, actor uuid.UUID, search string, page GlobalPaymentSearchPage,
) (GlobalPaymentSearch, error) {
	afterCreatedAt, afterID, hasCursor, err := prepareGlobalPaymentSearchPage(&page)
	if err != nil {
		return GlobalPaymentSearch{}, err
	}
	todays, err := s.ownerTodays(ctx, actor)
	if err != nil {
		return GlobalPaymentSearch{}, err
	}
	// The rows and the enriched items stay index-aligned (enrichItems keeps
	// the rows' order), so the page's last row carries the continuation's
	// keyset key (ticket #597).
	query := GlobalPaymentRulesQuery{
		Search:        search,
		CategorySlugs: defaultCategorySlugsMatching(search),
		Category:      page.Category,
		Type:          page.Type,
		Limit:         page.Limit,
	}
	if hasCursor {
		query.AfterCreatedAt = &afterCreatedAt
		query.AfterID = &afterID
	}
	rows := []GlobalPaymentRuleRow{}
	if len(todays) > 0 {
		rows, err = s.reader.ListGlobalPaymentRules(ctx, actor, todays, query)
		if err != nil {
			return GlobalPaymentSearch{}, fmt.Errorf("list global payment rules: %w", err)
		}
	}
	items, err := s.enrichItems(ctx, rows)
	if err != nil {
		return GlobalPaymentSearch{}, err
	}
	// The chips are the whole scope's: the chip filter narrows the list,
	// never the matched categories (the operations' canon).
	categories, err := s.reader.SumGlobalPaymentSearchCategories(ctx, actor, GlobalPaymentRulesQuery{
		Search:        search,
		CategorySlugs: defaultCategorySlugsMatching(search),
	})
	if err != nil {
		return GlobalPaymentSearch{}, fmt.Errorf("sum global payment search categories: %w", err)
	}
	// A full page answers with the last row's continuation; a short one has
	// walked the matches to the end — the '' cursor stops the scroll.
	nextCursor := ""
	if len(rows) == int(page.Limit) {
		last := rows[len(rows)-1]
		nextCursor = encodeRuleCursor(last.CreatedAt, last.ID)
	}
	// The total counts the whole matched scope (ticket #599): the list's
	// search and chip filters, no keyset key — the cursor only positions
	// the window, the count is the same on every walked page. An empty
	// visible scope counts nothing, the store untouched.
	total := int64(0)
	if len(todays) > 0 {
		total, err = s.reader.CountGlobalPaymentRules(ctx, actor, todays, GlobalPaymentRulesQuery{
			Search:        search,
			CategorySlugs: defaultCategorySlugsMatching(search),
			Category:      page.Category,
			Type:          page.Type,
		})
		if err != nil {
			return GlobalPaymentSearch{}, fmt.Errorf("count global payment rules: %w", err)
		}
	}
	return GlobalPaymentSearch{
		Items:             items,
		MatchedCategories: categories,
		NextCursor:        nextCursor,
		Total:             total,
	}, nil
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
			Type:        object.Type,
			PinnedAt:    object.PinnedAt,
			PhotoURL:    object.PhotoURL,
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
// (PUT /payments/favorites/order, ticket #576; the personal cutover #1308,
// D12): the submitted list is the actor's starred rules in their new order,
// and one transaction locks the actor's rows (the sorted ids being the
// deadlock-safe lock order) and assigns dense 1-based positions in the list
// order. The save is a full replacement within the actor's personal set:
// the actor's placed-outside favorites lose their positions and fall to the
// end of the reading order — duplicate positions never survive a save, an
// empty list resets the actor's whole visible order, and a favorite on an
// archived property keeps its position through the save (D16). The lock
// pass keeps the visibility and archive cuts (the feed's predicate): an id
// that does not
// come back — unknown, foreign, invisible, archived or simply not a
// favorite of this actor, the old «visible non-favorite» case folded in —
// is the privacy-preserving ErrNotFound, and a duplicate id is
// ErrInvalidInput. The right to order the personal set is the star's own,
// the view one (D12 — a viewer orders their favorites on par with the
// owner and Full Access); the per-row role is resolved beside the data and
// doubles as the audit role's raw material. A rule without a position (a
// fresh favorite, and whatever this save did not place) sorts last — the
// new-favorite-at-the-end rule. A pure order write: nothing materializes,
// nothing ticks, and — D18 — no realtime frame is published.
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

	return s.saveFavoriteOrderTx(ctx, actor, sorted, orderedIDs)
}

// saveFavoriteOrderTx is the save's transactional body: the lock pass, the
// full-replacement order write and the audit entry in one transaction.
func (s *GlobalPaymentService) saveFavoriteOrderTx(
	ctx context.Context, actor uuid.UUID, sorted, orderedIDs []uuid.UUID,
) error {
	return s.factory.runInTx(ctx, func(stores *txStores) error {
		locked, err := stores.favoriteOrders.LockVisibleFavorites(ctx, actor, sorted)
		if err != nil {
			return fmt.Errorf("lock visible favorites: %w", err)
		}
		if len(locked) < len(sorted) {
			return ErrNotFound
		}
		// The right is the star's own, the view one (D12): every row back
		// from the lock must pass it — the SQL's visibility predicate makes
		// the check unreachable today, the structure keeps the gate beside
		// the data it guards.
		for _, row := range locked {
			if !sharedpolicy.CanView(row.Role) {
				return ErrForbidden
			}
		}
		if err := stores.favoriteOrders.ClearFavoriteOrdersOutside(ctx, actor, sorted); err != nil {
			return fmt.Errorf("clear favorite orders outside the save: %w", err)
		}
		for position, id := range orderedIDs {
			if err := stores.favoriteOrders.SaveFavoritePosition(ctx, actor, id, int64(position+1)); err != nil {
				return fmt.Errorf("save favorite position of %s: %w", id, err)
			}
		}
		// The save's audit role is the actor's uniform acting role when every
		// placed rule carries the same one; a mixed submission — the actor
		// holds different roles on the touched properties — acts in the
		// shared leg's Full Access bucket, the order save's historical
		// convention (a viewer actor can only produce a uniform viewer set:
		// their lock returns their viewer rows alone). An empty reset acts
		// in the actor's own scope.
		auditRole := sharedpolicy.RoleOwner
		if len(locked) > 0 {
			auditRole = locked[0].Role
			for _, row := range locked[1:] {
				if row.Role != auditRole {
					auditRole = sharedpolicy.RoleFullAccess
					break
				}
			}
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

// feedItems runs the rules query and enriches the rows: the nearest-date
// fallback and the overdue age are the application layer's business.
func (s *GlobalPaymentService) feedItems(
	ctx context.Context, actor uuid.UUID, todays map[uuid.UUID]time.Time, q GlobalPaymentRulesQuery,
) ([]GlobalPaymentItem, error) {
	if len(todays) == 0 {
		return []GlobalPaymentItem{}, nil
	}
	rows, err := s.reader.ListGlobalPaymentRules(ctx, actor, todays, q)
	if err != nil {
		return nil, fmt.Errorf("list global payment rules: %w", err)
	}
	return s.enrichItems(ctx, rows)
}

// enrichItems turns the raw rows into response items. Rows without a
// stored next planned fall back to the pure projection through the shared
// NearestDateOfPayment seam (ticket #991) — the open pause and the settled
// rule being the true nulls; the overdue age is the owner today minus the
// oldest overdue date; the display-complete flag rides the same rule-body
// loads (ticket #1341).
func (s *GlobalPaymentService) enrichItems(
	ctx context.Context, rows []GlobalPaymentRuleRow,
) ([]GlobalPaymentItem, error) {
	fallback := make([]int, 0)
	for i, row := range rows {
		if row.NextPlannedDate == nil {
			fallback = append(fallback, i)
		}
	}
	projections, err := s.projectedDates(ctx, rows, fallback)
	if err != nil {
		return nil, err
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
		default:
			projection := projections[row.ID]
			item.NearestDate = projection.next
			// A stored next planned or any unpaid occurrence keeps the rule
			// unfinished; the settled/paused resolution lives where the rule
			// body is in hand.
			item.IsCompleted = projection.completed
		}
		if row.OldestOverdueDate != nil && row.OverdueCount > 0 {
			days := int(row.Today.Sub(*row.OldestOverdueDate).Hours() / 24)
			item.OverdueDays = &days
			item.OldestOverdueOperationID = row.OldestOverdueOperationID
		}
		items[i] = item
	}
	return items, nil
}

// feedProjection is the rule-body resolution of a fallback row: the pure
// projection's next occurrence and the display-complete flag (ticket
// #1341).
type feedProjection struct {
	next      *time.Time
	completed bool
}

// projectedDates resolves the projection fallback for the rows without a
// stored next planned (ticket #991): the shared NearestDateOfPayment seam
// over each rule's fresh body — the newest materialized date is the cursor,
// absent facts fall to it as nils. A rule gone mid-read projects nothing —
// its row shows no next. The same bodies answer the display-complete flag:
// the true null (nothing projected) is an open pause or a settled rule, and
// the pause wins — the raw domain.IsCompleted answers «завершён» behind an
// open pause's cut cursor (the glossary's exception), the feed row carries
// no pauses, so the flag ships pause-free (ticket #1341).
func (s *GlobalPaymentService) projectedDates(
	ctx context.Context, rows []GlobalPaymentRuleRow, fallback []int,
) (map[uuid.UUID]feedProjection, error) {
	projected := make(map[uuid.UUID]feedProjection, len(fallback))
	if len(fallback) == 0 {
		return projected, nil
	}
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
		rule, found, err := s.reader.GetGlobalPaymentRule(ctx, row.OwnerID, row.PropertyID, row.ID)
		if err != nil {
			return nil, fmt.Errorf("load payment rule for projection: %w", err)
		}
		if !found {
			continue // The rule is gone mid-read; its row shows no next.
		}
		var lastDate *time.Time
		if d, ok := lastDates[row.ID]; ok {
			lastDate = &d
		}
		next := NearestDateOfPayment(rule, row.Today, NearestDateInputs{LastMaterialized: lastDate})
		entry := feedProjection{next: next}
		if next == nil {
			if _, active := domain.ActivePause(rule.Pauses); !active {
				// Overdue operations are planned too — an unsettled debt
				// keeps the rule unfinished (the feed's fallback rows have
				// no stored next planned, so the overdue count is the whole
				// hasPlanned story here).
				entry.completed = domain.IsCompleted(rule, row.Today, lastDate, row.OverdueCount > 0)
			}
		}
		projected[row.ID] = entry
	}
	return projected, nil
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
