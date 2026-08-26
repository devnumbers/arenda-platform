package application

import (
	"context"
	"errors"
	"fmt"
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

// ListOperationsCommand is the transport-shaped operations listing request.
// Status filters on the server-computed view status (overdue included);
// DateFrom/DateTo bound the period inclusively on the operation date; Desc is
// the sort direction over the date (the contract default is desc, newest
// first).
type ListOperationsCommand struct {
	Status   *domain.OperationViewStatus
	DateFrom *time.Time
	DateTo   *time.Time
	Limit    int
	Offset   int
	Desc     bool
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
	policy   sharedpolicy.Policy
	calendar OwnerCalendar
}

// NewOperationService builds the operation use cases over the canonical
// payments store factory, the owner calendar and the authorization policy. A
// nil calendar fails on first use rather than computing statuses against a
// zero date.
func NewOperationService(factory txStoreFactory, calendar OwnerCalendar, policy sharedpolicy.Policy) *OperationService {
	return &OperationService{txStoreFactory: factory, calendar: calendar, policy: policy}
}

// PayOperation implements «Оплатить сейчас» (POST …/operations/{id}/pay): a
// planned operation becomes paid with paid_date = today in the owner's
// timezone; a repeated pay is ErrAlreadyPaid. The schedule does not shift —
// but paying ahead of the date frees the single-future-planned slot, so the
// conveyor's tick re-runs to stand the following occurrence in place; dates
// are never moved. Full Access and Owner may pay; a viewer gets ErrForbidden,
// a stranger or a foreign operation a privacy-preserving ErrNotFound.
func (s *OperationService) PayOperation(
	ctx context.Context, actor, propertyID, operationID uuid.UUID,
) (domain.Operation, error) {
	gates := mutationGates{factory: s.txStoreFactory, calendar: s.calendar}
	var paid domain.Operation
	_, err := gates.runMutation(ctx, actor, propertyID, uuid.Nil, s.writeGate,
		func(
			ctx context.Context, stores *txStores, scope uuid.UUID, _ domain.Payment, today time.Time,
		) (domain.Payment, mutationOutcome, error) {
			op, err := stores.operations.Get(ctx, operationID, scope, propertyID)
			if err != nil {
				return domain.Payment{}, mutationOutcome{}, err
			}
			if op.Status != domain.StatusPlanned {
				return domain.Payment{}, mutationOutcome{}, ErrAlreadyPaid
			}
			if err := stores.operations.MarkPaid(ctx, operationID, scope, today); err != nil {
				return domain.Payment{}, mutationOutcome{}, err
			}
			op.Status = domain.StatusPaid
			paidDate := today
			op.PaidDate = &paidDate
			paid = op
			outcome := mutationOutcome{
				audit:         auditdomain.ActionOperationPaid,
				tick:          true,
				auditEntity:   auditdomain.EntityOperation,
				auditEntityID: &op.ID,
			}
			if op.PaymentID != nil {
				outcome.auditCtx = map[string]any{"payment_id": *op.PaymentID}
			}
			return domain.Payment{}, outcome, nil
		})
	if err != nil {
		return domain.Operation{}, err
	}
	return paid, nil
}

// ListPaymentOperations returns one rule's operations paginated, filtered and
// sorted per the command, each with its computed view status. A missing or
// foreign rule is ErrNotFound — the operations never reveal it either.
func (s *OperationService) ListPaymentOperations(
	ctx context.Context, actor, propertyID, paymentID uuid.UUID, cmd ListOperationsCommand,
) ([]OperationListItem, error) {
	scope, err := resolveReadScope(ctx, s.policy, s.properties, actor, propertyID)
	if err != nil {
		return nil, err
	}
	// The nested path surfaces as the same privacy 404: no rule — no listing.
	if _, err := s.payments.Get(ctx, paymentID, scope, propertyID); err != nil {
		return nil, err
	}
	return s.listScoped(ctx, scope, cmd, func(q OperationsListQuery) ([]domain.Operation, error) {
		return s.operations.ListByPayment(ctx, scope, propertyID, paymentID, q)
	})
}

// ListPropertyOperations returns the property's operations across its rules
// with the same pagination, filters and direction. Serves the «Просроченные»
// section (status=overdue) and the property's full lists.
func (s *OperationService) ListPropertyOperations(
	ctx context.Context, actor, propertyID uuid.UUID, cmd ListOperationsCommand,
) ([]OperationListItem, error) {
	scope, err := resolveReadScope(ctx, s.policy, s.properties, actor, propertyID)
	if err != nil {
		return nil, err
	}
	return s.listScoped(ctx, scope, cmd, func(q OperationsListQuery) ([]domain.Operation, error) {
		return s.operations.ListByProperty(ctx, scope, propertyID, q)
	})
}

// listScoped normalizes the command, resolves the owner's today once and maps
// every returned operation onto its computed view status.
func (s *OperationService) listScoped(
	ctx context.Context, scope uuid.UUID, cmd ListOperationsCommand,
	fetch func(OperationsListQuery) ([]domain.Operation, error),
) ([]OperationListItem, error) {
	query, err := NormalizeOperationsCommand(cmd)
	if err != nil {
		return nil, err
	}
	if s.calendar == nil {
		return nil, errors.New("payments: owner calendar must be configured")
	}
	today, err := s.calendar.Today(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("resolve owner today: %w", err)
	}
	query.Today = today
	ops, err := fetch(query)
	if err != nil {
		return nil, err
	}
	items := make([]OperationListItem, len(ops))
	for i, op := range ops {
		items[i] = OperationListItem{Operation: op, ViewStatus: domain.OperationView(op, today)}
	}
	return items, nil
}

// NormalizeOperationsCommand folds the transport command into the store query:
// it applies the page-size default, enforces the pagination bounds and checks
// the filter vocabulary — everything out of range is ErrInvalidInput mapped to
// the contract's 400.
func NormalizeOperationsCommand(cmd ListOperationsCommand) (OperationsListQuery, error) {
	limit := cmd.Limit
	if limit == 0 {
		limit = DefaultOperationsPageSize
	}
	if limit < 1 || limit > MaxOperationsPageSize || cmd.Offset < 0 {
		return OperationsListQuery{}, ErrInvalidInput
	}
	var status domain.OperationViewStatus
	if cmd.Status != nil {
		switch *cmd.Status {
		case domain.ViewStatusPlanned, domain.ViewStatusPaid, domain.ViewStatusOverdue:
			status = *cmd.Status
		default:
			return OperationsListQuery{}, ErrInvalidInput
		}
	}
	return OperationsListQuery{
		Status:   status,
		DateFrom: cmd.DateFrom,
		DateTo:   cmd.DateTo,
		Limit:    limit,
		Offset:   cmd.Offset,
		Desc:     cmd.Desc,
	}, nil
}

// writeGate mirrors the rule mutations' gate: paying is a Full Access+ action
// (CanEdit), a viewer is ErrForbidden.
func (s *OperationService) writeGate(ctx context.Context, actor, propertyID uuid.UUID) (sharedpolicy.Role, error) {
	return newCapabilityGate(s.policy, sharedpolicy.CanEdit)(ctx, actor, propertyID)
}
