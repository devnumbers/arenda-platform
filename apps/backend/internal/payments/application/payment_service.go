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

// MaxAmountKopecks is the create/update amount ceiling: 10⁹ kopecks = 10
// million rubles (ADR 0049 §4).
const MaxAmountKopecks = 1_000_000_000

// MaxTitleLength bounds the rule title, counted in characters — a product
// copy limit, not a byte limit.
const MaxTitleLength = 255

// reminderOffsets are the reminder lead times the product speaks (карта
// #822): 1, 3 or 7 days before an occurrence — the same vocabulary the
// schema CHECK enforces durably.
var reminderOffsets = [3]int{1, 3, 7}

// isValidReminderOffset reports whether the reminder lead time is one of the
// contract's values; nil (no reminders) is always valid.
func isValidReminderOffset(offset *int) bool {
	if offset == nil {
		return true
	}
	for _, valid := range reminderOffsets {
		if *offset == valid {
			return true
		}
	}
	return false
}

// IsValidReminderOffset is the exported read of the reminder contract for
// the sibling contexts that write payment rules through their own seams (the
// rentals' managed payment): nil or 1/3/7 is valid, anything else is not.
func IsValidReminderOffset(offset *int) bool {
	return isValidReminderOffset(offset)
}

// CreatePaymentCommand is the validated create payload of a payment rule.
// The since date is not part of it: the server sets it to the owner's today
// (ADR 0048), and it never changes afterwards.
type CreatePaymentCommand struct {
	Type               domain.PaymentType
	Title              string
	AmountKopecks      int64
	Recurrence         domain.Recurrence
	CategorySlug       string
	EndDate            *time.Time
	AutoPay            bool
	ReminderOffsetDays *int
}

// EndDateUpdate is the PATCH resolution of endDate: Value nil clears the end
// date (the rule becomes open-ended), a date sets it.
type EndDateUpdate struct {
	Value *time.Time
}

// UpdatePaymentCommand is the partial-update payload: a nil field leaves the
// rule unchanged (omit = no change); since is not editable at all.
type UpdatePaymentCommand struct {
	Type          *domain.PaymentType
	Title         *string
	AmountKopecks *int64
	Recurrence    *domain.Recurrence
	CategorySlug  *string
	// EndDate is tri-state: a nil command field keeps the current value; a
	// non-nil one applies the EndDateUpdate (set or clear).
	EndDate *EndDateUpdate
	AutoPay *bool
	// ReminderOffsetDays is tri-state the same way: omit keeps, a set value
	// applies (1/3/7), a clear turns reminders off.
	ReminderOffsetDays *ReminderOffsetUpdate
}

// ReminderOffsetUpdate is the PATCH resolution of reminderOffsetDays: Value
// nil clears the reminders, 1/3/7 sets the lead time.
type ReminderOffsetUpdate struct {
	Value *int
}

// PaymentService orchestrates the payment rule use cases (ticket #457,
// ADR 0049 §4): create, read, list, partial update, delete with keep_overdue,
// pause, resume and favorite. Every mutation runs through the shared
// runMutation conveyor, which owns the ordering invariants structurally: the
// role gate, the property row lock, the owner's today, the change step, its
// audit entry and its action journal row (ADR 0061) in the same transaction,
// and the materialization tick when the step's verdict asks for it. Reads
// never tick and never write.
type PaymentService struct {
	txStoreFactory
	policy    sharedpolicy.Policy
	calendar  OwnerCalendar
	writeGate gateFunc // Full Access+: create/edit/pause/resume/favorite.
	delGate   gateFunc // Owner alone: deletion.
	// Realtime is the late-bound carrier the mutations' frames dispatch
	// through after the commit (карта #714, #716; ADR 0062); nil keeps the
	// pre-#716 silence.
	realtime realtimeapp.Publisher
}

// NewPaymentService builds the payment use case service over the shared
// transactional store factory, the owner calendar and the authorization
// policy (ADR 0028). A nil calendar is a wiring mistake; a mutation fails on
// first use rather than writing with a zero date. A nil policy keeps the
// historical owner-only behaviour (the owner acts on their own scope).
func NewPaymentService(factory txStoreFactory, calendar OwnerCalendar, policy sharedpolicy.Policy) *PaymentService {
	return &PaymentService{
		txStoreFactory: factory,
		policy:         policy,
		calendar:       calendar,
		writeGate:      newCapabilityGate(policy, sharedpolicy.CanEdit),
		delGate:        newCapabilityGate(policy, sharedpolicy.CanLifecycle),
	}
}

// SetRealtimePublisher late-binds the realtime carrier (карта #714, #716;
// ADR 0062): the frames of the committed mutations dispatch through it —
// the grace-events canon, best-effort, a broken carrier never fails the
// mutation.
func (s *PaymentService) SetRealtimePublisher(p realtimeapp.Publisher) {
	s.realtime = p
}

// conveyor bundles this service's factory and calendar for the shared
// mutation conveyor.
func (s *PaymentService) conveyor() mutationGates {
	return mutationGates{factory: s.txStoreFactory, calendar: s.calendar, realtime: s.realtime}
}

// readScope applies the read gate and returns the data owner whose scope the
// SQL reads filter by (ADR 0028).
func (s *PaymentService) readScope(ctx context.Context, actor, propertyID uuid.UUID) (uuid.UUID, error) {
	return resolveReadScope(ctx, s.policy, s.properties, actor, propertyID)
}

// CreatePayment creates a payment rule on the property. The rule acts from
// the owner's today on: the server sets since, no backdated occurrences are
// generated (prototype decision №17). Full Access and Owner may create; a
// viewer gets ErrForbidden, a stranger a privacy-preserving ErrNotFound.
func (s *PaymentService) CreatePayment(
	ctx context.Context, actor, propertyID uuid.UUID, cmd CreatePaymentCommand,
) (domain.Payment, error) {
	return runMutation(s.conveyor(), ctx, actor, propertyID, uuid.Nil, s.writeGate,
		func(ctx context.Context, stores *txStores, scope uuid.UUID, _ domain.Payment, today time.Time) (mutationOutcome[domain.Payment], error) {
			draft := domain.Payment{
				OwnerID:            scope,
				PropertyID:         propertyID,
				Type:               cmd.Type,
				Title:              strings.TrimSpace(cmd.Title),
				AmountKopecks:      cmd.AmountKopecks,
				Recurrence:         cmd.Recurrence,
				Since:              today,
				EndDate:            cmd.EndDate,
				AutoPay:            cmd.AutoPay,
				ReminderOffsetDays: cmd.ReminderOffsetDays,
				Category:           domain.CategoryRef{Slug: &cmd.CategorySlug},
			}
			if err := validateRule(draft); err != nil {
				return mutationOutcome[domain.Payment]{}, err
			}
			id, err := uuid.NewV7()
			if err != nil {
				return mutationOutcome[domain.Payment]{}, fmt.Errorf("mint payment id: %w", err)
			}
			draft.ID = id
			if err := stores.payments.Create(ctx, draft); err != nil {
				return mutationOutcome[domain.Payment]{}, fmt.Errorf("create payment: %w", err)
			}
			created := historydomain.PaymentCreated(draft.ID, draft.Title)
			created.Context[historydomain.CtxKeyAmountKopecks] = draft.AmountKopecks
			return mutationOutcome[domain.Payment]{
				Response:      draft,
				Audit:         auditdomain.ActionPaymentCreated,
				AuditEntityID: &draft.ID,
				History:       new(created),
				Changed: []realtimedom.Change{
					realtimedom.On(realtimedom.EntityPayments, propertyID),
					realtimedom.On(realtimedom.EntityOperations, propertyID), // The tick materializes the first planned in the same transaction.
				},
				Tick:            true,
				RereadPaymentID: &draft.ID,
			}, nil
		})
}

// PaymentListItem is the object payments list's read row (ticket #991):
// the rule plus its server-resolved next payment date — the shared
// NearestDateOfPayment seam; the single-rule reads have no place for it.
type PaymentListItem struct {
	Payment     domain.Payment
	NearestDate *time.Time
}

// ListPayments returns the property's payment rules in creation order with
// their pause intervals and next payment dates (ticket #991). Search is a
// case-insensitive substring filter on the title (” = no filter). Any actor
// with the view capability may read; a stranger gets ErrNotFound. Reads
// never tick.
func (s *PaymentService) ListPayments(
	ctx context.Context, actor, propertyID uuid.UUID, search string,
) ([]PaymentListItem, error) {
	scope, err := s.readScope(ctx, actor, propertyID)
	if err != nil {
		return nil, err
	}
	payments, err := s.payments.ListByProperty(ctx, scope, propertyID, search)
	if err != nil {
		return nil, fmt.Errorf("list payments: %w", err)
	}
	items := make([]PaymentListItem, len(payments))
	if len(payments) == 0 {
		return items, nil
	}
	today, err := ownerToday(s.calendar, ctx, scope)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(payments))
	for i, payment := range payments {
		ids[i] = payment.ID
	}
	inputs, err := s.operations.NearestDateInputsOfPayments(ctx, scope, propertyID, today, ids)
	if err != nil {
		return nil, err
	}
	for i, payment := range payments {
		items[i] = PaymentListItem{
			Payment:     payment,
			NearestDate: NearestDateOfPayment(payment, today, inputs[payment.ID]),
		}
	}
	return items, nil
}

// GetPayment returns one rule with its pause intervals. Any actor with the
// view capability may read; a missing or foreign payment is ErrNotFound.
// Reads never tick.
func (s *PaymentService) GetPayment(
	ctx context.Context, actor, propertyID, paymentID uuid.UUID,
) (domain.Payment, error) {
	scope, err := s.readScope(ctx, actor, propertyID)
	if err != nil {
		return domain.Payment{}, err
	}
	payment, err := s.payments.Get(ctx, paymentID, scope, propertyID)
	if err != nil {
		return domain.Payment{}, err
	}
	return payment, nil
}

// UpdatePayment applies a diff-patch to the rule: omitted fields are left
// unchanged, since is not editable. The change invalidates the strictly
// future planned and the verdict keeps Tick=true so the tick stands it again
// with fresh snapshots; the already due occurrences keep their frozen
// snapshots (user story #32), and paid operations are never touched. Full
// Access and Owner may edit.
func (s *PaymentService) UpdatePayment(
	ctx context.Context, actor, propertyID, paymentID uuid.UUID, cmd UpdatePaymentCommand,
) (domain.Payment, error) {
	return runMutation(s.conveyor(), ctx, actor, propertyID, paymentID, s.writeGate,
		func(
			ctx context.Context, stores *txStores, scope uuid.UUID, rule domain.Payment, today time.Time,
		) (mutationOutcome[domain.Payment], error) {
			if err := ensureNotRentalManaged(ctx, stores, scope, rule.ID); err != nil {
				return mutationOutcome[domain.Payment]{}, err
			}
			applyUpdate(&rule, cmd)
			if err := validateRule(rule); err != nil {
				return mutationOutcome[domain.Payment]{}, err
			}
			if err := stores.payments.Update(ctx, rule); err != nil {
				return mutationOutcome[domain.Payment]{}, fmt.Errorf("update payment: %w", err)
			}
			if err := stores.payments.DeleteFuturePlanned(ctx, rule.ID, today); err != nil {
				return mutationOutcome[domain.Payment]{}, err
			}
			updated := historydomain.PaymentUpdated(rule.ID, rule.Title)
			updated.Context[historydomain.CtxKeyAmountKopecks] = rule.AmountKopecks
			return mutationOutcome[domain.Payment]{
				Response:      rule,
				Audit:         auditdomain.ActionPaymentUpdated,
				AuditEntityID: &rule.ID,
				AuditCtx:      map[string]any{auditFieldsKey: updatedFields(cmd)},
				History:       new(updated),
				Changed: []realtimedom.Change{
					realtimedom.On(realtimedom.EntityPayments, propertyID),
					realtimedom.On(realtimedom.EntityOperations, propertyID), // The tick stands the strictly future planned again in the same transaction.
				},
				Tick:            true,
				RereadPaymentID: &rule.ID,
			}, nil
		})
}

// DeletePayment removes the rule. Planned operations from today on always go
// with it; the overdue planned debt goes too when keepOverdue is false, and
// paid operations stay in history marked «платёж удалён» (payment_id set to
// NULL by the FK, origin unchanged — ticket #446). Deletion is Owner-only
// (the ADR 0028 matrix) and its verdict states Tick=false explicitly: the
// rule is gone and its plan was resolved by the change step itself.
func (s *PaymentService) DeletePayment(
	ctx context.Context, actor, propertyID, paymentID uuid.UUID, keepOverdue bool,
) error {
	_, err := runMutation(s.conveyor(), ctx, actor, propertyID, paymentID, s.delGate,
		func(
			ctx context.Context, stores *txStores, scope uuid.UUID, rule domain.Payment, today time.Time,
		) (mutationOutcome[domain.Payment], error) {
			// The rentals RESTRICT FK forbids the direct delete for any
			// rental state — the gate states the same in the domain's own
			// words (ticket #818): delete the rental, the payment follows.
			if err := ensureNotRentalManaged(ctx, stores, scope, rule.ID); err != nil {
				return mutationOutcome[domain.Payment]{}, err
			}
			if err := stores.payments.DeletePlannedFrom(ctx, rule.ID, today); err != nil {
				return mutationOutcome[domain.Payment]{}, err
			}
			if !keepOverdue {
				if err := stores.payments.DeletePlannedBefore(ctx, rule.ID, today); err != nil {
					return mutationOutcome[domain.Payment]{}, err
				}
			}
			if err := stores.payments.Delete(ctx, rule.ID, scope); err != nil {
				return mutationOutcome[domain.Payment]{}, fmt.Errorf("delete payment: %w", err)
			}
			return mutationOutcome[domain.Payment]{
				Response:      rule, // The pre-delete state; nothing re-reads it.
				Audit:         auditdomain.ActionPaymentDeleted,
				AuditEntityID: &rule.ID,
				AuditCtx:      map[string]any{"keep_overdue": keepOverdue},
				History:       new(historydomain.PaymentDeleted(rule.ID, rule.Title)),
				Changed: []realtimedom.Change{
					realtimedom.On(realtimedom.EntityPayments, propertyID),
					// The change step itself deletes the planned in the same transaction — no tick needed.
					realtimedom.On(realtimedom.EntityOperations, propertyID),
				},
			}, nil
		})
	return err
}

// PausePayment opens the open-ended pause from the owner's today on: nothing
// generates inside the pause — no operations, no debt, no auto-pay — and the
// debt accumulated before it stays (CONTEXT.md «Пауза»). The verdict carries
// Tick=true: the tick removes the future planned. A second pause on a rule
// with an open one is ErrAlreadyPaused.
func (s *PaymentService) PausePayment(
	ctx context.Context, actor, propertyID, paymentID uuid.UUID,
) (domain.Payment, error) {
	return runMutation(s.conveyor(), ctx, actor, propertyID, paymentID, s.writeGate,
		func(
			ctx context.Context, stores *txStores, scope uuid.UUID, rule domain.Payment, today time.Time,
		) (mutationOutcome[domain.Payment], error) {
			// The gate precedes the rule's own state: pausing a rent payment
			// would silently stop the rental's materialization (#818).
			if err := ensureNotRentalManaged(ctx, stores, scope, rule.ID); err != nil {
				return mutationOutcome[domain.Payment]{}, err
			}
			if _, active := domain.ActivePause(rule.Pauses); active {
				return mutationOutcome[domain.Payment]{}, ErrAlreadyPaused
			}
			if err := stores.payments.InsertPause(ctx, rule.ID, today); err != nil {
				return mutationOutcome[domain.Payment]{}, fmt.Errorf("insert pause: %w", err)
			}
			return mutationOutcome[domain.Payment]{
				Response:      rule,
				Audit:         auditdomain.ActionPaymentPaused,
				AuditEntityID: &rule.ID,
				History:       new(historydomain.PaymentPaused(rule.ID, rule.Title)),
				Changed: []realtimedom.Change{
					realtimedom.On(realtimedom.EntityPayments, propertyID),
					realtimedom.On(realtimedom.EntityOperations, propertyID), // The tick removes the future planned in the same transaction.
				},
				Tick:            true,
				RereadPaymentID: &rule.ID,
			}, nil
		})
}

// ResumePayment closes the open pause with the owner's today (the resume day
// is already outside the half-open interval; anchors never shift) and the
// tick stands the future planned back. Resuming a rule without an open pause
// is ErrNotPaused.
func (s *PaymentService) ResumePayment(
	ctx context.Context, actor, propertyID, paymentID uuid.UUID,
) (domain.Payment, error) {
	return runMutation(s.conveyor(), ctx, actor, propertyID, paymentID, s.writeGate,
		func(
			ctx context.Context, stores *txStores, scope uuid.UUID, rule domain.Payment, today time.Time,
		) (mutationOutcome[domain.Payment], error) {
			if err := ensureNotRentalManaged(ctx, stores, scope, rule.ID); err != nil {
				return mutationOutcome[domain.Payment]{}, err
			}
			if _, active := domain.ActivePause(rule.Pauses); !active {
				return mutationOutcome[domain.Payment]{}, ErrNotPaused
			}
			if err := stores.payments.CloseActivePause(ctx, rule.ID, today); err != nil {
				return mutationOutcome[domain.Payment]{}, fmt.Errorf("close active pause: %w", err)
			}
			return mutationOutcome[domain.Payment]{
				Response:      rule,
				Audit:         auditdomain.ActionPaymentResumed,
				AuditEntityID: &rule.ID,
				History:       new(historydomain.PaymentResumed(rule.ID, rule.Title)),
				Changed: []realtimedom.Change{
					realtimedom.On(realtimedom.EntityPayments, propertyID),
					realtimedom.On(realtimedom.EntityOperations, propertyID), // The tick stands the future planned back in the same transaction.
				},
				Tick:            true,
				RereadPaymentID: &rule.ID,
			}, nil
		})
}

// SetPaymentFavorite writes the rule's favorite star atomically (PUT
// favorite, ticket #461): one UPDATE inside the conveyor's transaction — never
// a read-modify-write through the full PATCH. The star is a pure read-side
// flag: generation is unaffected, so the verdict states Tick=false explicitly
// and asks for no re-read — the rule resolved under the property lock travels
// out through Response. Full Access and Owner may favorite.
func (s *PaymentService) SetPaymentFavorite(
	ctx context.Context, actor, propertyID, paymentID uuid.UUID, favorite bool,
) (domain.Payment, error) {
	return runMutation(s.conveyor(), ctx, actor, propertyID, paymentID, s.writeGate,
		func(ctx context.Context, stores *txStores, scope uuid.UUID, rule domain.Payment, _ time.Time) (mutationOutcome[domain.Payment], error) {
			if err := stores.payments.SetFavorite(ctx, rule.ID, scope, favorite); err != nil {
				return mutationOutcome[domain.Payment]{}, fmt.Errorf("set payment favorite: %w", err)
			}
			rule.IsFavorite = favorite
			return mutationOutcome[domain.Payment]{
				Response:      rule,
				Audit:         auditdomain.ActionPaymentUpdated,
				AuditEntityID: &rule.ID,
				AuditCtx:      map[string]any{auditFieldsKey: []string{"favorite"}},
				Changed:       []realtimedom.Change{realtimedom.On(realtimedom.EntityPayments, propertyID)},
			}, nil
		})
}

// auditFieldsKey is the audit context key listing the mutated fields.
const auditFieldsKey = "fields"

// validateRule is the single validator of the create/update contract
// invariants — the transport decodes and delegates here, so the rules cannot
// drift between layers. It enforces: the direction enum, a
// non-empty title within MaxTitleLength characters (counted in runes: the
// limit is a product copy limit, not a byte limit), kopecks within 1..10⁹,
// a resolvable category reference (a default-catalog slug or a user category
// — the XOR is a durable schema invariant), a constructed non-zero
// recurrence and an end date no earlier than the rule's lower bound.
func validateRule(rule domain.Payment) error {
	if rule.Type != domain.TypeIncome && rule.Type != domain.TypeExpense {
		return ErrInvalidInput
	}
	if title := strings.TrimSpace(rule.Title); title == "" || utf8.RuneCountInString(title) > MaxTitleLength {
		return ErrInvalidInput
	}
	if rule.AmountKopecks < 1 || rule.AmountKopecks > MaxAmountKopecks {
		return ErrInvalidInput
	}
	if err := validateCategory(rule.Category); err != nil {
		return err
	}
	if rule.Recurrence.Kind() == "" {
		return ErrInvalidInput
	}
	if rule.EndDate != nil && rule.EndDate.Before(rule.Since) {
		return ErrInvalidInput
	}
	if !isValidReminderOffset(rule.ReminderOffsetDays) {
		return ErrInvalidInput
	}
	return nil
}

// validateCategory checks the rule's category reference: a default-catalog
// slug must be in the catalog, a user-category reference is carried unchanged
// (its endpoints arrive with the categories slice), and neither set is
// invalid — the XOR is a durable schema invariant.
func validateCategory(category domain.CategoryRef) error {
	switch {
	case category.Slug != nil:
		if !domain.IsValidDefaultCategorySlug(*category.Slug) {
			return ErrInvalidInput
		}
		return nil
	case category.UserCategoryID != nil:
		return nil
	default:
		return ErrInvalidInput
	}
}

// applyUpdate folds the command's diff into the rule; only provided fields
// change, each validated as at creation by the caller's validateRule pass.
func applyUpdate(payment *domain.Payment, cmd UpdatePaymentCommand) {
	if cmd.Type != nil {
		payment.Type = *cmd.Type
	}
	if cmd.Title != nil {
		payment.Title = strings.TrimSpace(*cmd.Title)
	}
	if cmd.AmountKopecks != nil {
		payment.AmountKopecks = *cmd.AmountKopecks
	}
	if cmd.Recurrence != nil {
		payment.Recurrence = *cmd.Recurrence
	}
	if cmd.CategorySlug != nil {
		payment.Category = domain.CategoryRef{Slug: cmd.CategorySlug}
	}
	if cmd.EndDate != nil {
		payment.EndDate = cmd.EndDate.Value
	}
	if cmd.AutoPay != nil {
		payment.AutoPay = *cmd.AutoPay
	}
	if cmd.ReminderOffsetDays != nil {
		payment.ReminderOffsetDays = cmd.ReminderOffsetDays.Value
	}
}

// updatedFields lists the field names a command changes; the audit context
// carries names only, never values (the title and amount are user data).
func updatedFields(cmd UpdatePaymentCommand) []string {
	fields := make([]string, 0, 8)
	if cmd.Type != nil {
		fields = append(fields, "type")
	}
	if cmd.Title != nil {
		fields = append(fields, "title")
	}
	if cmd.AmountKopecks != nil {
		fields = append(fields, "amount_kopecks")
	}
	if cmd.Recurrence != nil {
		fields = append(fields, "recurrence")
	}
	if cmd.CategorySlug != nil {
		fields = append(fields, "category_slug")
	}
	if cmd.EndDate != nil {
		fields = append(fields, "end_date")
	}
	if cmd.AutoPay != nil {
		fields = append(fields, "auto_pay")
	}
	if cmd.ReminderOffsetDays != nil {
		fields = append(fields, "reminder_offset_days")
	}
	return fields
}

// CompletedStatus reports the computed settlement status of one rule
// (CONTEXT.md «Завершённый платёж»): no planned operations — overdue
// included — and no occurrence beyond the last materialized date of any
// status. A read-side view like overdue (domain.IsCompleted); reads never
// tick, the aggregate queries are bounded to one row each.
func (s *PaymentService) CompletedStatus(
	ctx context.Context, actor, propertyID, paymentID uuid.UUID,
) (bool, error) {
	scope, err := s.readScope(ctx, actor, propertyID)
	if err != nil {
		return false, err
	}
	payment, err := s.payments.Get(ctx, paymentID, scope, propertyID)
	if err != nil {
		return false, err
	}
	return s.ruleCompleted(ctx, scope, payment)
}

// PaymentFlags computes both list-response flags in a single pass over the
// property's rule list (#815, #818, ADR 0053): one ListByProperty read fills
// isCompleted per rule (the settlement view) and isRentalManaged (the rental
// gate's read flag) — the list rows themselves hold no operation data to
// derive the flags client-side, and the unmanaged rules travel out as false.
func (s *PaymentService) PaymentFlags(
	ctx context.Context, actor, propertyID uuid.UUID,
) (completed, managed map[uuid.UUID]bool, err error) {
	scope, err := s.readScope(ctx, actor, propertyID)
	if err != nil {
		return nil, nil, err
	}
	payments, err := s.payments.ListByProperty(ctx, scope, propertyID, "")
	if err != nil {
		return nil, nil, err
	}
	today, err := ownerToday(s.calendar, ctx, scope)
	if err != nil {
		return nil, nil, err
	}
	ids := make([]uuid.UUID, 0, len(payments))
	completed = make(map[uuid.UUID]bool, len(payments))
	for _, payment := range payments {
		flag, err := s.ruleCompletedToday(ctx, scope, today, payment)
		if err != nil {
			return nil, nil, err
		}
		completed[payment.ID] = flag
		ids = append(ids, payment.ID)
	}
	// The reader answers the managed subset; every listed rule travels out
	// of this read, the unmanaged ones as false.
	managedSub, err := s.rentalManaged.ManagedPaymentIDs(ctx, scope, ids)
	if err != nil {
		return nil, nil, fmt.Errorf("rental-managed statuses: %w", err)
	}
	managed = make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		managed[id] = managedSub[id]
	}
	return completed, managed, nil
}

// RentalManagedStatus reports whether the rule is managed by a rental
// (ADR 0053, ticket #818): the read the payment screen uses to hide the
// rule mutations the gate rejects. Reads never tick.
func (s *PaymentService) RentalManagedStatus(
	ctx context.Context, actor, propertyID, paymentID uuid.UUID,
) (bool, error) {
	scope, err := s.readScope(ctx, actor, propertyID)
	if err != nil {
		return false, err
	}
	if _, err := s.payments.Get(ctx, paymentID, scope, propertyID); err != nil {
		return false, err
	}
	managed, err := s.rentalManaged.ManagedPaymentIDs(ctx, scope, []uuid.UUID{paymentID})
	if err != nil {
		return false, fmt.Errorf("rental-managed status: %w", err)
	}
	return managed[paymentID], nil
}

// ruleCompleted resolves the data owner's today and computes the flag for a
// loaded rule.
func (s *PaymentService) ruleCompleted(
	ctx context.Context, scope uuid.UUID, payment domain.Payment,
) (bool, error) {
	today, err := ownerToday(s.calendar, ctx, scope)
	if err != nil {
		return false, err
	}
	return s.ruleCompletedToday(ctx, scope, today, payment)
}

// ruleCompletedToday gathers the operation aggregates the pure domain check
// needs: any planned operation (the overdue view status included — an
// unsettled debt keeps the rule unfinished) and the newest materialized date
// across both stored statuses. Three bounded queries over the rule's own
// rows; with planned present the rest is unnecessary.
func (s *PaymentService) ruleCompletedToday(
	ctx context.Context, scope uuid.UUID, today time.Time, payment domain.Payment,
) (bool, error) {
	overdue := domain.ViewStatusOverdue
	planned := domain.ViewStatusPlanned
	for _, status := range []*domain.OperationViewStatus{&overdue, &planned} {
		ops, err := s.operations.ListByPayment(ctx, scope, payment.PropertyID, payment.ID, OperationsListQuery{
			Status: status,
			Limit:  1,
			Today:  today,
		})
		if err != nil {
			return false, fmt.Errorf("completion status check: %w", err)
		}
		if len(ops) > 0 {
			return false, nil
		}
	}
	newest, err := s.operations.ListByPayment(ctx, scope, payment.PropertyID, payment.ID, OperationsListQuery{
		Limit: 1,
		Today: today,
	})
	if err != nil {
		return false, fmt.Errorf("completion last operation: %w", err)
	}
	var lastMaterialized *time.Time
	if len(newest) > 0 {
		date := newest[0].Date
		lastMaterialized = &date
	}
	return domain.IsCompleted(payment, today, lastMaterialized, false), nil
}
