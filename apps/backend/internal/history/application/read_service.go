// The history feed's reading side (карта #704, тикеты #708/#842, ADR 0061
// §7): the keyset page with the contract's 400/privacy-404, the filter-sheet
// options, and the sanitized search input — the always-OR predicate itself
// (prefix-FTS OR ILIKE-trgm, research #839) lives in the store's SQL. The
// visibility predicate lives there too (actor_can_read_property, 000142) —
// rows outside the reader's scope never leave the database; this service
// proves the property_ids scope before the store is touched so one invisible
// id hides the whole request (the privacy 404, the tasks feed's #547
// discipline) and the tariff gate decides on the already-resolved roles
// (ADR 0064, ревизия #1465).

package application

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// The feed page bounds (канон #597/#633): 50-row default, 100 ceiling.
const (
	feedDefaultLimit = 50
	FeedMaxLimit     = 100
)

// searchMaxBytes is the search input's server-side ceiling (research #839):
// the wire contract caps q at 255 characters, this byte backstop catches
// multibyte and contract-less callers before the predicate chews them.
const searchMaxBytes = 400

// EntryReader reads the journal for the feed and the filter options.
// Implemented by persistence adapters.
type EntryReader interface {
	List(ctx context.Context, actor uuid.UUID, q JournalQuery) ([]domain.FeedEntry, error)
	FilterParticipants(ctx context.Context, actor uuid.UUID, propertyIDs []uuid.UUID) ([]domain.FilterParticipant, error)
	FilterObjects(ctx context.Context, actor uuid.UUID, propertyIDs []uuid.UUID) ([]domain.FilterObject, error)
}

// PaidSectionsGate resolves whether the actor's subscription covers the
// platform's paid sections (ADR 0064). Consumer-side port (ADR 0035): the
// billing module's adapter satisfies it alongside the middleware's and the
// access use cases' ports — the same verdict, consumed here where the
// journal's read scope is already resolved. A user without a subscription
// counts as basic; the gate only blocks, never grants.
type PaidSectionsGate interface {
	CanUsePaidFeatures(ctx context.Context, userID uuid.UUID) (bool, error)
}

// JournalQuery is the store-level page request: every filter already
// validated, the search already sanitized (trimmed; empty — no search), the
// cursors already decoded.
type JournalQuery struct {
	PropertyIDs []uuid.UUID
	ActorIDs    []uuid.UUID
	Kinds       []domain.Kind
	BaseActions []domain.BaseAction
	// DateFrom is inclusive, DateTo exclusive — the screen means «дата+24ч».
	DateFrom *time.Time
	DateTo   *time.Time
	Search   string
	Before   *CursorKey
	After    *CursorKey
	Limit    int
}

// FeedQuery is the GET /history wire request before validation.
type FeedQuery struct {
	BeforeCursor string
	AfterCursor  string
	Limit        int
	DateFrom     *time.Time
	DateTo       *time.Time
	BaseActions  []string
	Kinds        []string
	ActorIDs     []uuid.UUID
	PropertyIDs  []uuid.UUID
	Query        string
}

// FeedPage is one keyset portion of the scope's feed, newest first. The
// cursor is bidirectional: NextCursor continues into the past
// (before_cursor), PrevCursor asks for rows newer than the page's first
// (after_cursor — the prepend of the «новые снизу» лента). An empty page
// carries neither.
type FeedPage struct {
	Items      []domain.FeedEntry
	NextCursor string
	PrevCursor string
}

// FiltersOptions are the GET /history/filters options for the reader's
// scope (ADR 0061 §7).
type FiltersOptions struct {
	Participants []domain.FilterParticipant
	Objects      []domain.FilterObject
}

// HistoryReadService is the journal's reading service.
type HistoryReadService struct {
	reader EntryReader
	policy sharedpolicy.Policy
	gate   PaidSectionsGate
}

// NewHistoryReadService creates the reading service. The policy proves the
// requested property scope before the store is touched; the tariff gate
// decides the paid scope on the already-resolved roles (ADR 0064, ревизия
// #1465). A nil gate disables the check — the package convention for
// optional dependencies (mirrors the access use cases); the composition root
// always wires the billing adapter.
func NewHistoryReadService(reader EntryReader, policy sharedpolicy.Policy, gate PaidSectionsGate) *HistoryReadService {
	return &HistoryReadService{reader: reader, policy: policy, gate: gate}
}

// Feed returns one page of the reader's journal feed.
func (s *HistoryReadService) Feed(ctx context.Context, actor uuid.UUID, q FeedQuery) (FeedPage, error) {
	limit, err := foldLimit(q.Limit)
	if err != nil {
		return FeedPage{}, err
	}
	before, after, err := foldCursors(q)
	if err != nil {
		return FeedPage{}, err
	}
	kinds, err := parseKinds(q.Kinds)
	if err != nil {
		return FeedPage{}, err
	}
	baseActions, err := parseBaseActions(q.BaseActions)
	if err != nil {
		return FeedPage{}, err
	}
	search, err := sanitizeSearch(q.Query)
	if err != nil {
		return FeedPage{}, err
	}
	if err := s.proveReadScope(ctx, actor, q.PropertyIDs); err != nil {
		return FeedPage{}, err
	}

	rows, err := s.reader.List(ctx, actor, JournalQuery{
		PropertyIDs: q.PropertyIDs,
		ActorIDs:    q.ActorIDs,
		Kinds:       kinds,
		BaseActions: baseActions,
		DateFrom:    q.DateFrom,
		DateTo:      q.DateTo,
		Search:      search,
		Before:      before,
		After:       after,
		Limit:       limit,
	})
	if err != nil {
		return FeedPage{}, fmt.Errorf("list journal page: %w", err)
	}

	page := FeedPage{Items: rows}
	if n := len(rows); n > 0 {
		if n == limit {
			page.NextCursor = encodeFeedCursor(CursorKey{CreatedAt: rows[n-1].CreatedAt, ID: rows[n-1].ID})
		}
		page.PrevCursor = encodeFeedCursor(CursorKey{CreatedAt: rows[0].CreatedAt, ID: rows[0].ID})
	}
	return page, nil
}

// foldLimit applies the page-size default and ceiling (канон #597/#633).
func foldLimit(limit int) (int, error) {
	if limit == 0 {
		return feedDefaultLimit, nil
	}
	if limit < 0 || limit > FeedMaxLimit {
		return 0, fmt.Errorf("limit %d: %w", limit, ErrInvalidInput)
	}
	return limit, nil
}

// foldCursors decodes the wire cursors; the pair is mutually exclusive and
// either malformed blob is the contract's 400.
func foldCursors(q FeedQuery) (before, after *CursorKey, err error) {
	if q.BeforeCursor != "" && q.AfterCursor != "" {
		return nil, nil, fmt.Errorf("before_cursor and after_cursor are mutually exclusive: %w", ErrInvalidInput)
	}
	if q.BeforeCursor != "" {
		key, err := decodeFeedCursor(q.BeforeCursor)
		if err != nil {
			return nil, nil, err
		}
		before = &key
	}
	if q.AfterCursor != "" {
		key, err := decodeFeedCursor(q.AfterCursor)
		if err != nil {
			return nil, nil, err
		}
		after = &key
	}
	return before, after, nil
}

// Filters returns the filter-sheet options for the reader's scope.
func (s *HistoryReadService) Filters(ctx context.Context, actor uuid.UUID, propertyIDs []uuid.UUID) (FiltersOptions, error) {
	if err := s.proveReadScope(ctx, actor, propertyIDs); err != nil {
		return FiltersOptions{}, err
	}
	participants, err := s.reader.FilterParticipants(ctx, actor, propertyIDs)
	if err != nil {
		return FiltersOptions{}, fmt.Errorf("list history filter participants: %w", err)
	}
	objects, err := s.reader.FilterObjects(ctx, actor, propertyIDs)
	if err != nil {
		return FiltersOptions{}, fmt.Errorf("list history filter objects: %w", err)
	}
	return FiltersOptions{Participants: participants, Objects: objects}, nil
}

// proveReadScope runs every listed property through its view gate and the
// tariff gate before the store is touched, one role resolution per id (ADR
// 0028 — no second resolve). Privacy answers first: one invisible or unknown
// id is the privacy ErrNotFound of the whole request — a mixed list never
// leaks which ids exist (the tasks feed's #547 discipline), suspended access
// hides the property the same way («suspended — недоступно»). Only then does
// ensureTariffAllowsScope decide the tariff verdict (ревизия #1465).
func (s *HistoryReadService) proveReadScope(ctx context.Context, actor uuid.UUID, propertyIDs []uuid.UUID) error {
	roles := make([]sharedpolicy.Role, 0, len(propertyIDs))
	for _, propertyID := range propertyIDs {
		role, err := s.policy.RoleForProperty(ctx, actor, propertyID)
		if err != nil {
			return fmt.Errorf("resolve history read role: %w", err)
		}
		roles = append(roles, role)
		if sharedpolicy.GateFor(role, sharedpolicy.CanView) != sharedpolicy.GateAllow {
			return ErrNotFound
		}
	}
	return s.ensureTariffAllowsScope(ctx, actor, roles)
}

// ensureTariffAllowsScope is the role-aware tariff gate of the journal reads
// (ADR 0064, ревизия #1465): on a non-paid tariff a read passes only in the
// narrowed membership scope — every listed property must be one the reader
// participates in as a member. The owner's own object refuses («свой объект
// не в истории» буквально, любая поверхность), and so does the un-narrowed
// request: the global feed is owner-scope by construction and stays paid for
// everyone. By the time this runs the privacy pass has discarded the
// suspended and foreign ids, so a visible role here is the owner or an
// active member — the strict rule reduces to «no owned id in the list». A
// gate storage error propagates for the HTTP layer's 500 — the gate never
// fails open.
func (s *HistoryReadService) ensureTariffAllowsScope(ctx context.Context, actor uuid.UUID, roles []sharedpolicy.Role) error {
	if s.gate == nil {
		return nil
	}
	paid, err := s.gate.CanUsePaidFeatures(ctx, actor)
	if err != nil {
		return fmt.Errorf("paid sections gate: %w", err)
	}
	if paid {
		return nil
	}
	if len(roles) == 0 {
		return ErrTariffRequired
	}
	if slices.Contains(roles, sharedpolicy.RoleOwner) {
		return ErrTariffRequired
	}
	return nil
}

// parseKinds validates the wire's kind list against the dictionary (ADR 0061
// §4); anything unknown is a contract 400, not a silent empty filter.
func parseKinds(kinds []string) ([]domain.Kind, error) {
	if len(kinds) == 0 {
		return nil, nil
	}
	out := make([]domain.Kind, 0, len(kinds))
	known := map[string]bool{}
	for _, k := range domain.AllKinds {
		known[string(k)] = true
	}
	for _, k := range kinds {
		if !known[k] {
			return nil, fmt.Errorf("unknown kind %q: %w", k, ErrInvalidInput)
		}
		out = append(out, domain.Kind(k))
	}
	return out, nil
}

// parseBaseActions validates the wire's base-action list the same way.
func parseBaseActions(actions []string) ([]domain.BaseAction, error) {
	if len(actions) == 0 {
		return nil, nil
	}
	out := make([]domain.BaseAction, 0, len(actions))
	known := map[string]bool{}
	for _, a := range domain.AllBaseActions {
		known[string(a)] = true
	}
	for _, a := range actions {
		if !known[a] {
			return nil, fmt.Errorf("unknown base action %q: %w", a, ErrInvalidInput)
		}
		out = append(out, domain.BaseAction(a))
	}
	return out, nil
}

// sanitizeSearch prepares the wire's q for the store's always-OR search
// predicate (research #839): trims the edges, folds whitespace-only input
// into «no search» and rejects oversized input with the contract's 400.
// There is no routing here on purpose — guessing the intent from the
// input's shape was the v1 mistake the research retired: the SQL predicate
// applies prefix-FTS and ILIKE-trgm to any input.
func sanitizeSearch(q string) (string, error) {
	query := strings.TrimSpace(q)
	if len(query) >= searchMaxBytes {
		return "", fmt.Errorf("query %d bytes: %w", len(query), ErrInvalidInput)
	}
	return query, nil
}
