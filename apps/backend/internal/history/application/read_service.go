// The history feed's reading side (карта #704, тикет #708, ADR 0061 §7):
// the keyset page with the contract's 400/privacy-404, the filter-sheet
// options, and the trgm/fts/both search routing (research #705). The
// visibility predicate itself lives in the store's SQL
// (actor_can_read_history, 000137) — rows outside the reader's scope never
// leave the database; this service proves the property_ids scope before the
// store is touched so one invisible id hides the whole request (the privacy
// 404, the tasks feed's #547 discipline).

package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// The contract's read errors: ErrInvalidInput is the 400 (malformed cursor,
// unknown vocabulary, bad page size), ErrNotFound — the privacy 404 (an
// invisible or unknown property id: the request's existence is not revealed).
var (
	ErrInvalidInput = errors.New("history: invalid input")
	ErrNotFound     = errors.New("history: not found")
)

// The feed page bounds (канон #597/#633): 50-row default, 100 ceiling.
const (
	feedDefaultLimit = 50
	FeedMaxLimit     = 100
)

// Search modes of the routed predicate (research #705). The empty mode is
// no search at all.
const (
	searchModeTrgm = "trgm"
	searchModeFTS  = "fts"
	searchModeBoth = "both"
)

// EntryReader reads the journal for the feed and the filter options.
// Implemented by persistence adapters.
type EntryReader interface {
	List(ctx context.Context, actor uuid.UUID, q JournalQuery) ([]domain.FeedEntry, error)
	FilterParticipants(ctx context.Context, actor uuid.UUID, propertyIDs []uuid.UUID) ([]domain.FilterParticipant, error)
	FilterObjects(ctx context.Context, actor uuid.UUID, propertyIDs []uuid.UUID) ([]domain.FilterObject, error)
}

// SearchRoute is the search predicate the store applies: which of the two
// GIN indexes (or both) the query walks, and the trimmed raw input for
// websearch_to_tsquery. The ILIKE pattern for the trgm leg is the store's
// business — the LIKE-escaping canon lives beside the other searches in the
// adapters.
type SearchRoute struct {
	Mode string
	Raw  string
}

// JournalQuery is the store-level page request: every filter already
// validated, the search already routed, the cursors already decoded.
type JournalQuery struct {
	PropertyIDs []uuid.UUID
	ActorIDs    []uuid.UUID
	Kinds       []domain.Kind
	BaseActions []domain.BaseAction
	// DateFrom is inclusive, DateTo exclusive — the screen means «дата+24ч».
	DateFrom *time.Time
	DateTo   *time.Time
	Search   SearchRoute
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
}

// NewHistoryReadService creates the reading service. The policy proves the
// requested property scope before the store is touched.
func NewHistoryReadService(reader EntryReader, policy sharedpolicy.Policy) *HistoryReadService {
	return &HistoryReadService{reader: reader, policy: policy}
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
	if err := s.provePropertiesVisible(ctx, actor, q.PropertyIDs); err != nil {
		return FeedPage{}, err
	}

	rows, err := s.reader.List(ctx, actor, JournalQuery{
		PropertyIDs: q.PropertyIDs,
		ActorIDs:    q.ActorIDs,
		Kinds:       kinds,
		BaseActions: baseActions,
		DateFrom:    q.DateFrom,
		DateTo:      q.DateTo,
		Search:      routeSearch(q.Query),
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
	if err := s.provePropertiesVisible(ctx, actor, propertyIDs); err != nil {
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

// provePropertiesVisible runs every listed property through its view gate
// before the store is touched: one invisible or unknown id is the privacy
// ErrNotFound of the whole request — a mixed list never leaks which ids
// exist (the tasks feed's #547 discipline). Suspended access hides the
// property the same way («suspended — недоступно»).
func (s *HistoryReadService) provePropertiesVisible(ctx context.Context, actor uuid.UUID, propertyIDs []uuid.UUID) error {
	for _, propertyID := range propertyIDs {
		role, err := s.policy.RoleForProperty(ctx, actor, propertyID)
		if err != nil {
			return fmt.Errorf("resolve history read role: %w", err)
		}
		if sharedpolicy.GateFor(role, sharedpolicy.CanView) != sharedpolicy.GateAllow {
			return ErrNotFound
		}
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

// routeSearch decides which search predicate the query walks (research
// #705, the v1 rules): mails and dot-fragments, ≤2-char fragments and
// cut-off words — trgm (only trgm sees substrings); plain words — FTS (the
// morphology trgm is blind to); a word plus a mail token in one query —
// both legs. Refinements (the UI's «искать по фрагменту» switch) — the
// search screen's walkthrough (тикет #710).
func routeSearch(q string) SearchRoute {
	query := strings.TrimSpace(q)
	if query == "" {
		return SearchRoute{}
	}
	mode := searchModeFTS
	switch {
	case hasMailAndWord(query):
		mode = searchModeBoth
	case strings.ContainsRune(query, '@'),
		hasDotInsideWord(query),
		utf8.RuneCountInString(query) <= 2,
		startsOrEndsWithNonLetter(query):
		mode = searchModeTrgm
	}
	return SearchRoute{Mode: mode, Raw: query}
}

// hasMailAndWord reports a mixed query: a mail token plus a standalone ≥3-letter
// word without mail punctuation — the word needs FTS morphology, the mail
// only trgm finds.
func hasMailAndWord(q string) bool {
	if !strings.ContainsRune(q, '@') {
		return false
	}
	for token := range strings.FieldsSeq(q) {
		if !strings.ContainsRune(token, '@') &&
			!strings.ContainsRune(token, '.') &&
			utf8.RuneCountInString(token) >= 3 {
			return true
		}
	}
	return false
}

// hasDotInsideWord reports a dot with alphanumeric neighbours on both sides
// — the email-local-part/fragment shape («ivanov.petr»), which FTS reads as
// one email-ish token and only trgm matches.
func hasDotInsideWord(q string) bool {
	runes := []rune(q)
	for i, r := range runes {
		if r != '.' || i == 0 || i == len(runes)-1 {
			continue
		}
		if isAlnum(runes[i-1]) && isAlnum(runes[i+1]) {
			return true
		}
	}
	return false
}

// startsOrEndsWithNonLetter reports a cut-off fragment («иван.», «125000»,
// «(черновик)») — a substring intent, trgm territory. An empty query never
// reaches here (routeSearch returns early), the guard makes it total anyway.
func startsOrEndsWithNonLetter(q string) bool {
	runes := []rune(q)
	if len(runes) == 0 {
		return false
	}
	return !unicode.IsLetter(runes[0]) || !unicode.IsLetter(runes[len(runes)-1])
}

func isAlnum(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}
