package application

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	realtimeapp "github.com/nambers/arenda-planform/apps/backend/internal/realtime/application"
	realtimedom "github.com/nambers/arenda-planform/apps/backend/internal/realtime/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// Pagination bounds of the operations lists (ticket #461): the wire default
// is a page of 50 with a hard ceiling at 100.
const (
	DefaultOperationsPageSize = 50
	MaxOperationsPageSize     = 100
)

// OperationsListQuery is the operations listing request of both scopes — per
// rule and property-wide. The transport fills everything except Today;
// PrepareOperationsQuery applies the page-size default and the pagination
// bounds, and the service resolves Today from the owner calendar right before
// the store runs the query. Status filters on the server-computed view status
// (nil = no filter); DateFrom/DateTo bound the period inclusively on the
// operation date; sorting is newest-first by contract (a zero Asc), and only
// an explicit Asc=true flips it to oldest-first.
type OperationsListQuery struct {
	Status   *domain.OperationViewStatus
	DateFrom *time.Time
	DateTo   *time.Time
	Limit    int
	Offset   int
	// Search is a case-insensitive substring filter on the title
	// ('' = no filter); the store adapter escapes the ILIKE metacharacters.
	Search string
	// Asc is false by default and by contract: sorting is newest-first unless
	// explicitly requested otherwise.
	Asc bool
	// Type filters on the operation direction (nil = no filter) — the
	// income/expense split the «Доходы/Расходы объекта» screens read.
	Type *domain.PaymentType
	// Categories filters on the operation's category snapshot (nil = no
	// filter); a row without a category snapshot never matches.
	Categories []string
	// Today carries the owner's today the overdue semantics are resolved
	// against; set by the service, never by callers.
	Today time.Time
}

// OperationsSummaryQuery is the summary request of the property scope
// (ticket #473): the same status/period/direction vocabulary as the listing
// minus the pagination — the totals and the category breakdown aggregate in
// SQL, never over a client-side page. Today is resolved by the service.
type OperationsSummaryQuery struct {
	Status   *domain.OperationViewStatus
	Type     *domain.PaymentType
	DateFrom *time.Time
	DateTo   *time.Time
	// Search is the listing's search predicate (OperationsListQuery.Search):
	// the summary of the searched scope feeds the search screen's matched
	// category chips (ticket #476).
	Search string
	// Today carries the owner's today the overdue semantics are resolved
	// against; set by the service, never by callers.
	Today time.Time
}

// CategorySummary is one category's total over the summarized scope: the
// snapshot the chips render (slug for the icon and style, label for the
// text). Rows without a category snapshot never appear in the breakdown —
// their amounts still count in OperationsSummary's totals.
type CategorySummary struct {
	Slug         string
	Label        string
	Type         domain.PaymentType
	TotalKopecks int64
}

// OperationsSummary is the period aggregate behind the «Операции объекта»
// screens (ticket #473): the totals by direction — always both, the summary
// cards read them together — plus the per-category breakdown ordered by
// total, largest first.
type OperationsSummary struct {
	IncomeTotalKopecks  int64
	ExpenseTotalKopecks int64
	Categories          []CategorySummary
}

// PrepareOperationsQuery validates the listing request in place and applies
// the page-size default: a zero limit becomes the default page, anything out
// of range or a negative offset is ErrInvalidInput mapped to the contract's
// 400. The filter vocabulary is already guaranteed by the transport binder's
// enum Valid() check; this pass owns only the numbers.
func PrepareOperationsQuery(q *OperationsListQuery) error {
	if q.Limit == 0 {
		q.Limit = DefaultOperationsPageSize
	}
	if q.Limit < 1 || q.Limit > MaxOperationsPageSize || q.Offset < 0 {
		return ErrInvalidInput
	}
	return nil
}

// OperationListItem pairs one operation with its server-computed view status
// (CONTEXT.md «Просрочка»): overdue is never stored — it is derived against
// the owner's today before the response leaves the use case.
type OperationListItem struct {
	Operation  domain.Operation
	ViewStatus domain.OperationViewStatus
	// PropertyName is the global listing's row label (ticket #540); empty on
	// the property-scope reads — the property screen resolves the property by
	// its id.
	PropertyName string
}

// CreateOperationCommand is the create payload of a manual operation (ticket
// #569): a one-off income/expense fact. The date is not part of it — the
// server sets it to the owner's today like the rule's since — and no payment
// form exists behind a manual fact: the schema keeps payment_form NULL. The
// category travels as the default-catalog slug and freezes as the snapshot.
type CreateOperationCommand struct {
	Type          domain.PaymentType
	Title         string
	AmountKopecks int64
	CategorySlug  string
}

// OperationService carries the operation use cases of the second contracts
// slice (ticket #461): «Оплатить сейчас» and the two paginated listings (of
// one rule and of the whole property). It runs through the same serialization
// and read-scope discipline as the rule service; only the pay mutation writes.
type OperationService struct {
	txStoreFactory
	policy    sharedpolicy.Policy
	calendar  OwnerCalendar
	writeGate gateFunc // Full Access+: pay.
	// Realtime is the late-bound carrier the mutations' frames dispatch
	// through after the commit (карта #714, #716; ADR 0062); nil keeps the
	// pre-#716 silence.
	realtime realtimeapp.Publisher
}

// SetRealtimePublisher late-binds the realtime carrier (карта #714, #716;
// ADR 0062): the frames of the committed mutations dispatch through it —
// the grace-events canon, best-effort, a broken carrier never fails the
// mutation.
func (s *OperationService) SetRealtimePublisher(p realtimeapp.Publisher) {
	s.realtime = p
}

// NewOperationService builds the operation use cases over the canonical
// payments store factory, the owner calendar and the authorization policy. A
// nil calendar fails on first use rather than computing statuses against a
// zero date.
func NewOperationService(factory txStoreFactory, calendar OwnerCalendar, policy sharedpolicy.Policy) *OperationService {
	return &OperationService{
		txStoreFactory: factory,
		policy:         policy,
		calendar:       calendar,
		writeGate:      newCapabilityGate(policy, sharedpolicy.CanEdit),
	}
}

// conveyor bundles this service's factory and calendar for the shared
// mutation conveyor.
func (s *OperationService) conveyor() mutationGates {
	return mutationGates{factory: s.txStoreFactory, calendar: s.calendar, realtime: s.realtime}
}

// PayOperation implements «Оплатить сейчас» (POST …/operations/{id}/pay): a
// planned operation becomes paid with paid_date = today in the owner's
// timezone; a repeated pay is ErrAlreadyPaid. The schedule does not shift —
// but paying ahead of the date frees the single-future-planned slot, so the
// verdict keeps Tick=true to re-run the tick and stand the next occurrence in
// place; dates are never moved. Full Access and Owner may pay; a viewer gets
// ErrForbidden, a stranger or a foreign operation a privacy-preserving
// ErrNotFound.
func (s *OperationService) PayOperation(
	ctx context.Context, actor, propertyID, operationID uuid.UUID,
) (OperationListItem, error) {
	conveyor := s.conveyor()
	paidOp, err := runMutation(conveyor, ctx, actor, propertyID, uuid.Nil, s.writeGate,
		func(
			ctx context.Context, stores *txStores, scope uuid.UUID, _ domain.Payment, today time.Time,
		) (mutationOutcome[domain.Operation], error) {
			op, err := stores.operations.Get(ctx, operationID, scope, propertyID)
			if err != nil {
				return mutationOutcome[domain.Operation]{}, err
			}
			if op.Status != domain.StatusPlanned {
				return mutationOutcome[domain.Operation]{}, ErrAlreadyPaid
			}
			if err := stores.operations.MarkPaid(ctx, operationID, scope, today); err != nil {
				return mutationOutcome[domain.Operation]{}, err
			}
			op.Status = domain.StatusPaid
			paidDate := today
			op.PaidDate = &paidDate
			history := historydomain.OperationPaid(op.ID, op.Title, op.Date)
			history.Context[historydomain.CtxKeyAmountKopecks] = op.AmountKopecks
			outcome := mutationOutcome[domain.Operation]{
				Response:      op,
				Audit:         auditdomain.ActionOperationPaid,
				AuditEntity:   auditdomain.EntityOperation,
				AuditEntityID: &op.ID,
				History:       new(history),
				Changed:       []realtimedom.Change{realtimedom.On(realtimedom.EntityOperations, propertyID)},
				Tick:          true,
			}
			if op.PaymentID != nil {
				outcome.AuditCtx = map[string]any{"payment_id": *op.PaymentID}
				// The paid fact moves the rule's schedule cursor — the tick
				// stands the next occurrence; the rule's screens are dirty
				// too.
				outcome.Changed = append(outcome.Changed,
					realtimedom.On(realtimedom.EntityPayments, propertyID))
			}
			return outcome, nil
		})
	if err != nil {
		return OperationListItem{}, err
	}
	return OperationListItem{
		Operation:  paidOp,
		ViewStatus: domain.OperationView(paidOp, *paidOp.PaidDate),
	}, nil
}

// CreateOperation implements the manual «+ операция» (POST
// …/operations, ticket #569): the one-off fact is born paid with
// date = paid_date = today in the owner's timezone (ADR 0048), origin
// manual, no rule behind it — payment_id and payment_form stay NULL. The
// category freezes as the snapshot right here: the label resolves from the
// catalog (the validator guarantees the slug resolves), the slug travels.
// Full Access and Owner may create; a viewer gets ErrForbidden, a stranger
// the privacy ErrNotFound, an archived property ErrArchivedProperty. The
// verdict keeps Tick=true — the mutation canon of the context.
func (s *OperationService) CreateOperation(
	ctx context.Context, actor, propertyID uuid.UUID, cmd CreateOperationCommand,
) (OperationListItem, error) {
	conveyor := s.conveyor()
	op, err := runMutation(conveyor, ctx, actor, propertyID, uuid.Nil, s.writeGate,
		func(
			ctx context.Context, stores *txStores, scope uuid.UUID, _ domain.Payment, today time.Time,
		) (mutationOutcome[domain.Operation], error) {
			slug := cmd.CategorySlug
			draft := domain.Operation{
				OwnerID:       scope,
				PropertyID:    propertyID,
				Origin:        domain.OriginManual,
				Date:          today,
				PaidDate:      &today,
				Status:        domain.StatusPaid,
				Type:          cmd.Type,
				Title:         strings.TrimSpace(cmd.Title),
				AmountKopecks: cmd.AmountKopecks,
				CategoryLabel: domain.CategoryRef{Slug: &slug}.SnapshotLabel(),
				CategorySlug:  &slug,
			}
			if err := validateManualOperation(draft); err != nil {
				return mutationOutcome[domain.Operation]{}, err
			}
			id, err := uuid.NewV7()
			if err != nil {
				return mutationOutcome[domain.Operation]{}, fmt.Errorf("mint operation id: %w", err)
			}
			draft.ID = id
			if err := stores.operations.Create(ctx, draft); err != nil {
				return mutationOutcome[domain.Operation]{}, fmt.Errorf("create manual operation: %w", err)
			}
			history := historydomain.OperationCreated(draft.ID, draft.Title, draft.Date)
			history.Context[historydomain.CtxKeyAmountKopecks] = draft.AmountKopecks
			return mutationOutcome[domain.Operation]{
				Response:      draft,
				Audit:         auditdomain.ActionOperationCreated,
				AuditEntity:   auditdomain.EntityOperation,
				AuditEntityID: &draft.ID,
				History:       new(history),
				Changed:       []realtimedom.Change{realtimedom.On(realtimedom.EntityOperations, propertyID)},
				Tick:          true,
			}, nil
		})
	if err != nil {
		return OperationListItem{}, err
	}
	return OperationListItem{
		Operation:  op,
		ViewStatus: domain.OperationView(op, *op.PaidDate),
	}, nil
}

// DeleteOperation implements «Удалить операцию» (DELETE …/operations/{id}):
// a planned (the overdue debt) or paid operation gets the cancelled
// tombstone — paid_date is cleared with the payment fact, the debt and the
// history entry disappear, and the row keeps its (payment_id, date) key so
// the tick never re-materializes the cancelled occurrence (решение владельца;
// ближайшая будущая плановая и проекции не удаляются — расписание правится
// на уровне правила). Full Access and Owner may delete; a viewer gets
// ErrForbidden, an archived property ErrArchivedProperty, and a foreign or
// already-cancelled operation the privacy ErrNotFound.
func (s *OperationService) DeleteOperation(
	ctx context.Context, actor, propertyID, operationID uuid.UUID,
) error {
	conveyor := s.conveyor()
	_, err := runMutation(conveyor, ctx, actor, propertyID, uuid.Nil, s.writeGate,
		func(
			ctx context.Context, stores *txStores, scope uuid.UUID, _ domain.Payment, today time.Time,
		) (mutationOutcome[domain.Operation], error) {
			// Get исключает отменённые: повторное удаление — тот же 404.
			op, err := stores.operations.Get(ctx, operationID, scope, propertyID)
			if err != nil {
				return mutationOutcome[domain.Operation]{}, err
			}
			if err := stores.operations.Cancel(ctx, operationID, scope, propertyID); err != nil {
				return mutationOutcome[domain.Operation]{}, err
			}
			history := historydomain.OperationDeleted(op.ID, op.Title, op.Date)
			outcome := mutationOutcome[domain.Operation]{
				Audit:         auditdomain.ActionOperationDeleted,
				AuditEntity:   auditdomain.EntityOperation,
				AuditEntityID: &op.ID,
				History:       new(history),
				Changed:       []realtimedom.Change{realtimedom.On(realtimedom.EntityOperations, propertyID)},
			}
			if op.PaymentID != nil {
				outcome.AuditCtx = map[string]any{"payment_id": *op.PaymentID}
				// The canceled payment fact dirties the rule's derived view —
				// paid vs unpaid, planned counts — with no tick involvement:
				// the row keeps its (payment_id, date) key, so the tick never
				// re-materializes the canceled occurrence (see DeleteOperation's
				// doc).
				outcome.Changed = append(outcome.Changed,
					realtimedom.On(realtimedom.EntityPayments, propertyID))
			}
			return outcome, nil
		})
	return err
}

// GetOperation returns one operation with its server-computed view status —
// the read behind the operation page. It is a pure read: a viewer reads it
// like the listings, an unknown or foreign row is the privacy ErrNotFound,
// and overdue is derived against the owner's today (CONTEXT.md «Просрочка»).
func (s *OperationService) GetOperation(
	ctx context.Context, actor, propertyID, operationID uuid.UUID,
) (OperationListItem, error) {
	scope, err := resolveReadScope(ctx, s.policy, s.properties, actor, propertyID)
	if err != nil {
		return OperationListItem{}, err
	}
	op, err := s.operations.Get(ctx, operationID, scope, propertyID)
	if err != nil {
		return OperationListItem{}, err
	}
	today, err := ownerToday(s.calendar, ctx, scope)
	if err != nil {
		return OperationListItem{}, err
	}
	return OperationListItem{Operation: op, ViewStatus: domain.OperationView(op, today)}, nil
}

// ListPaymentOperations returns one rule's operations paginated, filtered and
// sorted per the query, each with its computed view status. A missing or
// foreign rule is ErrNotFound — the operations never reveal it either.
func (s *OperationService) ListPaymentOperations(
	ctx context.Context, actor, propertyID, paymentID uuid.UUID, q OperationsListQuery,
) ([]OperationListItem, error) {
	scope, err := resolveReadScope(ctx, s.policy, s.properties, actor, propertyID)
	if err != nil {
		return nil, err
	}
	// The nested path surfaces as the same privacy 404: no rule — no listing.
	if _, err := s.payments.Get(ctx, paymentID, scope, propertyID); err != nil {
		return nil, err
	}
	return s.listScoped(ctx, scope, q, func(prepared OperationsListQuery) ([]domain.Operation, error) {
		return s.operations.ListByPayment(ctx, scope, propertyID, paymentID, prepared)
	})
}

// ListPropertyOperations returns the property's operations across its rules
// with the same pagination, filters and direction. Serves the «Просроченные»
// section (status=overdue) and the property's full lists.
func (s *OperationService) ListPropertyOperations(
	ctx context.Context, actor, propertyID uuid.UUID, q OperationsListQuery,
) ([]OperationListItem, error) {
	scope, err := resolveReadScope(ctx, s.policy, s.properties, actor, propertyID)
	if err != nil {
		return nil, err
	}
	return s.listScoped(ctx, scope, q, func(prepared OperationsListQuery) ([]domain.Operation, error) {
		return s.operations.ListByProperty(ctx, scope, propertyID, prepared)
	})
}

// SummarizePropertyOperations returns the period aggregate of the property's
// operations (ticket #473): the totals by direction and the per-category
// breakdown. A pure read with the listing's scope discipline — a viewer
// reads it, a stranger gets the privacy ErrNotFound — and the same today
// semantics: the service resolves the owner's today once, before the store
// aggregates.
func (s *OperationService) SummarizePropertyOperations(
	ctx context.Context, actor, propertyID uuid.UUID, q OperationsSummaryQuery,
) (OperationsSummary, error) {
	scope, err := resolveReadScope(ctx, s.policy, s.properties, actor, propertyID)
	if err != nil {
		return OperationsSummary{}, err
	}
	today, err := ownerToday(s.calendar, ctx, scope)
	if err != nil {
		return OperationsSummary{}, err
	}
	q.Today = today
	return s.operations.SummarizeByProperty(ctx, scope, propertyID, q)
}

// GlobalOperationsListQuery is the global feed's listing request (ticket
// #540): the property listing's filter vocabulary minus the status filter —
// paid is the feed's only view status — plus the propertyIds multi-select.
// The transport fills everything; PrepareGlobalOperationsQuery applies the
// page-size default and the pagination bounds. There is no Today: paid rows
// never split into planned/overdue, so no owner calendar is consulted.
type GlobalOperationsListQuery struct {
	// PropertyIDs narrows the feed to the listed properties (the «Объект»
	// multi-select); the service resolves every entry through the view gate,
	// an unknown or non-visible one being the privacy ErrNotFound.
	PropertyIDs []uuid.UUID
	DateFrom    *time.Time
	DateTo      *time.Time
	Limit       int
	// Cursor is the previous page's opaque continuation ('' = from the
	// beginning); PrepareGlobalOperationsQuery decodes it into AfterDate and
	// AfterID — the keyset key the page resumes strictly after (ticket
	// #597). The pair travels together or not at all.
	Cursor    string
	AfterDate *time.Time
	AfterID   *uuid.UUID
	// Search is the listing's search predicate (OperationsListQuery.Search).
	Search string
	// Asc is false by default and by contract: sorting is newest-first unless
	// explicitly requested otherwise.
	Asc bool
	// Type filters on the operation direction (nil = no filter).
	Type *domain.PaymentType
	// Categories filters on the operation's category snapshot (nil = no
	// filter); a row without a category snapshot never matches.
	Categories []string
	// IncludeArchived lifts the feed's archive cut (#549): the archived
	// properties' paid rows rejoin under the same visibility predicate.
	// False — the contract default — keeps the archive excluded.
	IncludeArchived bool
}

// GlobalOperationsPage is one walked window of the global feed (ticket
// #597): the listed rows plus the keyset continuation — the next page's
// opaque cursor, ” when the feed is exhausted. Total is the whole scope's
// paid count under the query's filters (ticket #599) — «найдено N», the
// same on every walked page.
type GlobalOperationsPage struct {
	Items      []OperationListItem
	NextCursor string
	Total      int64
}

// GlobalOperationsSummaryQuery is the global summary's request (ticket
// #540): the property summary's vocabulary plus the propertyIds multi-select
// and the category filter. The store applies propertyIds, the period and the
// search to the whole scope — totals and the breakdown alike — and the type
// and category filters to the category breakdown only: the totals always
// report both directions.
type GlobalOperationsSummaryQuery struct {
	PropertyIDs []uuid.UUID
	DateFrom    *time.Time
	DateTo      *time.Time
	Search      string
	Type        *domain.PaymentType
	Categories  []string
	// IncludeArchived lifts the archive cut (#549): the archived
	// properties' paid rows count in the totals and the breakdown.
	// False — the contract default — keeps the archive excluded.
	IncludeArchived bool
}

// PrepareGlobalOperationsQuery validates the global listing request in place:
// a zero limit becomes the default page, anything out of range is
// ErrInvalidInput mapped to the contract's 400; the page's continuation
// cursor (ticket #597) decodes into the AfterDate/AfterID keyset key.
func PrepareGlobalOperationsQuery(q *GlobalOperationsListQuery) error {
	if q.Limit == 0 {
		q.Limit = DefaultOperationsPageSize
	}
	if q.Limit < 1 || q.Limit > MaxOperationsPageSize {
		return ErrInvalidInput
	}
	if q.Cursor == "" {
		return nil
	}
	afterDate, afterID, err := decodeOperationCursor(q.Cursor)
	if err != nil {
		return err
	}
	q.AfterDate = &afterDate
	q.AfterID = &afterID
	return nil
}

// ListGlobalOperations returns one page of the actor's visible paid
// operations — the global «Операции» screen's feed (ticket #540): the paid
// facts of the actor's own properties plus the properties they can view
// (ADR 0028), the archived ones excluded, cancelled rows never existing for
// reads. The visibility predicate lives in the store's SQL — the
// actor-scoped cross-property read (the tasks global feed precedent,
// ticket #521); the propertyIds entries however are resolved through
// the view gate first, an unknown or non-visible one being the privacy
// ErrNotFound. The page walks the feed's own (date, id) order by keyset
// (ticket #597): a full page answers with the last row's continuation, a
// short one has reached the feed's end. A pure read: never ticks, and no
// owner calendar is consulted.
func (s *OperationService) ListGlobalOperations(
	ctx context.Context, actor uuid.UUID, q GlobalOperationsListQuery,
) (GlobalOperationsPage, error) {
	if err := PrepareGlobalOperationsQuery(&q); err != nil {
		return GlobalOperationsPage{}, err
	}
	if err := resolveGlobalPropertyFilter(ctx, s.policy, s.properties, actor, q.PropertyIDs); err != nil {
		return GlobalOperationsPage{}, err
	}
	rows, err := s.operations.ListGlobal(ctx, actor, q)
	if err != nil {
		return GlobalOperationsPage{}, fmt.Errorf("list global operations: %w", err)
	}
	items := make([]OperationListItem, len(rows))
	for i, row := range rows {
		items[i] = OperationListItem{
			Operation: row.Operation,
			// The store's contract: the feed carries paid rows only, so the
			// view status is the stored one — no calendar in the equation.
			ViewStatus:   domain.OperationViewStatus(row.Operation.Status),
			PropertyName: row.PropertyName,
		}
	}
	nextCursor := ""
	if len(items) == q.Limit {
		last := rows[len(rows)-1]
		nextCursor = encodeOperationCursor(last.Operation.Date, last.Operation.ID)
	}
	// The total counts the whole scope under the query's filters (ticket
	// #599): the same predicate as the rows with the keyset key aside —
	// the cursor only positions the window, the count is the same on every
	// walked page. The count query has no limit arg, the copy's window
	// fields are dead weight for it.
	countQuery := q
	countQuery.Cursor = ""
	countQuery.AfterDate = nil
	countQuery.AfterID = nil
	total, err := s.operations.CountGlobal(ctx, actor, countQuery)
	if err != nil {
		return GlobalOperationsPage{}, fmt.Errorf("count global operations: %w", err)
	}
	return GlobalOperationsPage{Items: items, NextCursor: nextCursor, Total: total}, nil
}

// SummarizeGlobalOperations returns the period aggregate of the actor's
// visible paid operations (ticket #540) — the global twin of
// SummarizePropertyOperations with the same summary shape. A pure read with
// the feed's scope discipline: the propertyIds entries resolve through the
// view gate, a stranger gets the privacy ErrNotFound.
func (s *OperationService) SummarizeGlobalOperations(
	ctx context.Context, actor uuid.UUID, q GlobalOperationsSummaryQuery,
) (OperationsSummary, error) {
	if err := resolveGlobalPropertyFilter(ctx, s.policy, s.properties, actor, q.PropertyIDs); err != nil {
		return OperationsSummary{}, err
	}
	return s.operations.SummarizeGlobal(ctx, actor, q)
}

// resolveGlobalPropertyFilter runs every propertyIds entry through the read
// gate (ADR 0028): a viewer reads a listed property's slice, an unknown or
// non-visible id is the privacy ErrNotFound — the filter never widens the
// feed beyond the unfiltered read's visibility.
func resolveGlobalPropertyFilter(
	ctx context.Context, policy sharedpolicy.Policy, properties PropertyStore,
	actor uuid.UUID, propertyIDs []uuid.UUID,
) error {
	for _, propertyID := range propertyIDs {
		if _, err := resolveReadScope(ctx, policy, properties, actor, propertyID); err != nil {
			return err
		}
	}
	return nil
}

// listScoped prepares the query, resolves the owner's today once and maps
// every returned operation onto its computed view status.
func (s *OperationService) listScoped(
	ctx context.Context, scope uuid.UUID, q OperationsListQuery,
	fetch func(OperationsListQuery) ([]domain.Operation, error),
) ([]OperationListItem, error) {
	if err := PrepareOperationsQuery(&q); err != nil {
		return nil, err
	}
	today, err := ownerToday(s.calendar, ctx, scope)
	if err != nil {
		return nil, err
	}
	q.Today = today
	ops, err := fetch(q)
	if err != nil {
		return nil, err
	}
	items := make([]OperationListItem, len(ops))
	for i, op := range ops {
		items[i] = OperationListItem{Operation: op, ViewStatus: domain.OperationView(op, today)}
	}
	return items, nil
}

// validateManualOperation is the single validator of the manual-operation
// create contract (ticket #569): the rule's create vocabulary (validateRule)
// minus the rule-only parts — a manual fact carries no recurrence and no
// payment form, and its category reference is the default-catalog slug only.
// The transport decodes and delegates here, so the rules cannot drift
// between layers: the direction enum, a non-empty title within
// MaxTitleLength characters (counted in runes), kopecks within 1..10⁹ and a
// resolvable catalog slug.
func validateManualOperation(op domain.Operation) error {
	if op.Type != domain.TypeIncome && op.Type != domain.TypeExpense {
		return ErrInvalidInput
	}
	if title := strings.TrimSpace(op.Title); title == "" || utf8.RuneCountInString(title) > MaxTitleLength {
		return ErrInvalidInput
	}
	if op.AmountKopecks < 1 || op.AmountKopecks > MaxAmountKopecks {
		return ErrInvalidInput
	}
	return validateCategory(domain.CategoryRef{Slug: op.CategorySlug})
}
