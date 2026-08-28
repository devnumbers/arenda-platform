package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
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
	// Today carries the owner's today the overdue semantics are resolved
	// against; set by the service, never by callers.
	Today time.Time
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
			outcome := mutationOutcome[domain.Operation]{
				Response:      op,
				Audit:         auditdomain.ActionOperationPaid,
				AuditEntity:   auditdomain.EntityOperation,
				AuditEntityID: &op.ID,
				Tick:          true,
			}
			if op.PaymentID != nil {
				outcome.AuditCtx = map[string]any{"payment_id": *op.PaymentID}
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

// conveyor bundles this service's factory and calendar for the shared
// mutation conveyor.
func (s *OperationService) conveyor() mutationGates {
	return mutationGates{factory: s.txStoreFactory, calendar: s.calendar}
}
