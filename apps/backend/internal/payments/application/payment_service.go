package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// MaxAmountKopecks is the create/update amount ceiling: 10⁹ kopecks = 10
// million rubles (ADR 0049 §4).
const MaxAmountKopecks = 1_000_000_000

// MaxTitleLength bounds the rule title, counted in characters — a product
// copy limit, not a byte limit.
const MaxTitleLength = 255

// CreatePaymentCommand is the validated create payload of a payment rule.
// The since date is not part of it: the server sets it to the owner's today
// (ADR 0048), and it never changes afterwards.
type CreatePaymentCommand struct {
	Type          domain.PaymentType
	Title         string
	AmountKopecks int64
	Recurrence    domain.Recurrence
	PaymentForm   domain.PaymentForm
	CategorySlug  string
	EndDate       *time.Time
	AutoPay       bool
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
	PaymentForm   *domain.PaymentForm
	CategorySlug  *string
	// EndDate is tri-state: a nil command field keeps the current value; a
	// non-nil one applies the EndDateUpdate (set or clear).
	EndDate *EndDateUpdate
	AutoPay *bool
}

// PaymentService orchestrates the payment rule use cases (ticket #457,
// ADR 0049 §4): create, read, list, partial update, delete with keep_overdue,
// pause and resume. Every mutation runs through the mutateRule conveyor,
// which owns the ordering invariants structurally: the role gate, the
// property row lock, the owner's today, the change step, its audit entry in
// the same transaction and the materialization tick after the change. Reads
// never tick and never write.
type PaymentService struct {
	txStoreFactory
	policy   sharedpolicy.Policy
	calendar OwnerCalendar
}

// NewPaymentService builds the payment use case service over the shared
// transactional store factory, the owner calendar and the authorization
// policy (ADR 0028). A nil calendar is a wiring mistake; a mutation fails on
// first use rather than writing with a zero date. A nil policy keeps the
// historical owner-only behaviour (the owner acts on their own scope).
func NewPaymentService(factory txStoreFactory, calendar OwnerCalendar, policy sharedpolicy.Policy) *PaymentService {
	return &PaymentService{txStoreFactory: factory, calendar: calendar, policy: policy}
}

// changeStep is the use-case-specific part of the mutation conveyor: it
// receives the loaded rule (a zero rule for creation), applies its change
// inside the open transaction and tells the conveyor the audit entry, the
// tick and the re-read verdicts.
type changeStep func(
	ctx context.Context, stores *txStores, scope uuid.UUID, rule domain.Payment, today time.Time,
) (domain.Payment, mutationOutcome, error)

// mutationOutcome is the change step's verdict for the conveyor: the audit
// entry to record in the same transaction, whether the materialization tick
// runs after the change (deletion resolves its plan explicitly and skips
// it), and whether the stored rule is re-read after commit for the response.
type mutationOutcome struct {
	audit    auditdomain.Action
	auditCtx map[string]any
	tick     bool
	reread   bool
}

// mutateRule is the mutation conveyor shared by every payment use case. It
// runs, in one transaction and in this order: the role gate, the property
// serialization lock (FOR UPDATE — ADR 0049 §3), the owner's today, the load
// of the target rule (skipped for a zero paymentID — creation mints its
// own), the change step, its audit entry, and the materialization tick after
// the change. The change step is the only use-case-specific part; after
// commit the conveyor re-reads the stored rule so the response carries the
// persisted timestamps, pauses and category view.
func (s *PaymentService) mutateRule(
	ctx context.Context, actor, propertyID, paymentID uuid.UUID,
	gate func(context.Context, uuid.UUID, uuid.UUID) (sharedpolicy.Role, error),
	change changeStep,
) (domain.Payment, error) {
	role, err := gate(ctx, actor, propertyID)
	if err != nil {
		return domain.Payment{}, err
	}
	var scope, ruleID uuid.UUID
	var outcome mutationOutcome
	err = s.runInTx(ctx, func(stores *txStores) error {
		prop, today, err := s.lockActiveProperty(ctx, stores, propertyID)
		if err != nil {
			return err
		}
		scope = prop.OwnerID
		rule := domain.Payment{}
		if paymentID != uuid.Nil {
			rule, err = stores.payments.Get(ctx, paymentID, scope, propertyID)
			if err != nil {
				return err
			}
		}
		rule, outcome, err = change(ctx, stores, scope, rule, today)
		if err != nil {
			return err
		}
		ruleID = rule.ID
		if err := recordAudit(ctx, stores, actor, role, outcome.audit, &rule.ID, outcome.auditCtx); err != nil {
			return err
		}
		if outcome.tick {
			return stores.tickOwner(ctx, scope, today)
		}
		return nil
	})
	if err != nil {
		return domain.Payment{}, err
	}
	if !outcome.reread {
		return domain.Payment{}, nil
	}
	return s.payments.Get(ctx, ruleID, scope, propertyID)
}

// CreatePayment creates a payment rule on the property. The rule acts from
// the owner's today on: the server sets since, no backdated occurrences are
// generated (prototype decision №17). Full Access and Owner may create; a
// viewer gets ErrForbidden, a stranger a privacy-preserving ErrNotFound.
func (s *PaymentService) CreatePayment(
	ctx context.Context, actor, propertyID uuid.UUID, cmd CreatePaymentCommand,
) (domain.Payment, error) {
	return s.mutateRule(ctx, actor, propertyID, uuid.Nil, s.writeGate,
		func(ctx context.Context, stores *txStores, scope uuid.UUID, _ domain.Payment, today time.Time) (domain.Payment, mutationOutcome, error) {
			draft := domain.Payment{
				OwnerID:       scope,
				PropertyID:    propertyID,
				Type:          cmd.Type,
				Title:         strings.TrimSpace(cmd.Title),
				AmountKopecks: cmd.AmountKopecks,
				Recurrence:    cmd.Recurrence,
				Since:         today,
				EndDate:       cmd.EndDate,
				AutoPay:       cmd.AutoPay,
				PaymentForm:   cmd.PaymentForm,
				Category:      domain.CategoryRef{Slug: &cmd.CategorySlug},
			}
			if err := validateRule(draft); err != nil {
				return domain.Payment{}, mutationOutcome{}, err
			}
			id, err := uuid.NewV7()
			if err != nil {
				return domain.Payment{}, mutationOutcome{}, fmt.Errorf("mint payment id: %w", err)
			}
			draft.ID = id
			if err := stores.payments.Create(ctx, draft); err != nil {
				return domain.Payment{}, mutationOutcome{}, fmt.Errorf("create payment: %w", err)
			}
			return draft, mutationOutcome{audit: auditdomain.ActionPaymentCreated, tick: true, reread: true}, nil
		})
}

// ListPayments returns the property's payment rules in creation order with
// their pause intervals. Any actor with the view capability may read; a
// stranger gets ErrNotFound. Reads never tick.
func (s *PaymentService) ListPayments(ctx context.Context, actor, propertyID uuid.UUID) ([]domain.Payment, error) {
	scope, err := s.readScope(ctx, actor, propertyID)
	if err != nil {
		return nil, err
	}
	payments, err := s.payments.ListByProperty(ctx, scope, propertyID)
	if err != nil {
		return nil, fmt.Errorf("list payments: %w", err)
	}
	return payments, nil
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
// future planned and the conveyor's tick stands it again with fresh
// snapshots; the already due occurrences keep their frozen snapshots (user
// story #32), and paid operations are never touched. Full Access and Owner
// may edit.
func (s *PaymentService) UpdatePayment(
	ctx context.Context, actor, propertyID, paymentID uuid.UUID, cmd UpdatePaymentCommand,
) (domain.Payment, error) {
	return s.mutateRule(ctx, actor, propertyID, paymentID, s.writeGate,
		func(ctx context.Context, stores *txStores, _ uuid.UUID, rule domain.Payment, today time.Time) (domain.Payment, mutationOutcome, error) {
			applyUpdate(&rule, cmd)
			if err := validateRule(rule); err != nil {
				return domain.Payment{}, mutationOutcome{}, err
			}
			if err := stores.payments.Update(ctx, rule); err != nil {
				return domain.Payment{}, mutationOutcome{}, fmt.Errorf("update payment: %w", err)
			}
			if err := stores.payments.DeleteFuturePlanned(ctx, rule.ID, today); err != nil {
				return domain.Payment{}, mutationOutcome{}, err
			}
			return rule, mutationOutcome{
				audit:    auditdomain.ActionPaymentUpdated,
				auditCtx: map[string]any{"fields": updatedFields(cmd)},
				tick:     true,
				reread:   true,
			}, nil
		})
}

// DeletePayment removes the rule. Planned operations from today on always go
// with it; the overdue planned debt goes too when keepOverdue is false, and
// paid operations stay in history marked «платёж удалён» (payment_id set to
// NULL by the FK, origin unchanged — ticket #446). Deletion is Owner-only
// (the ADR 0028 matrix) and is the one mutation without a tick: the rule is
// gone and its plan was resolved explicitly by the change step.
func (s *PaymentService) DeletePayment(
	ctx context.Context, actor, propertyID, paymentID uuid.UUID, keepOverdue bool,
) error {
	_, err := s.mutateRule(ctx, actor, propertyID, paymentID, s.deleteGate,
		func(
			ctx context.Context, stores *txStores, scope uuid.UUID, rule domain.Payment, today time.Time,
		) (domain.Payment, mutationOutcome, error) {
			if err := stores.payments.DeletePlannedFrom(ctx, rule.ID, today); err != nil {
				return domain.Payment{}, mutationOutcome{}, err
			}
			if !keepOverdue {
				if err := stores.payments.DeletePlannedBefore(ctx, rule.ID, today); err != nil {
					return domain.Payment{}, mutationOutcome{}, err
				}
			}
			if err := stores.payments.Delete(ctx, rule.ID, scope); err != nil {
				return domain.Payment{}, mutationOutcome{}, fmt.Errorf("delete payment: %w", err)
			}
			return rule, mutationOutcome{
				audit:    auditdomain.ActionPaymentDeleted,
				auditCtx: map[string]any{"keep_overdue": keepOverdue},
			}, nil
		})
	return err
}

// PausePayment opens the open-ended pause from the owner's today on: nothing
// generates inside the pause — no operations, no debt, no auto-pay — and the
// debt accumulated before it stays (CONTEXT.md «Пауза»). The conveyor's tick
// removes the future planned. A second pause on a rule with an open one is
// ErrAlreadyPaused.
func (s *PaymentService) PausePayment(
	ctx context.Context, actor, propertyID, paymentID uuid.UUID,
) (domain.Payment, error) {
	return s.mutateRule(ctx, actor, propertyID, paymentID, s.writeGate,
		func(ctx context.Context, stores *txStores, _ uuid.UUID, rule domain.Payment, today time.Time) (domain.Payment, mutationOutcome, error) {
			if _, active := domain.ActivePause(rule.Pauses); active {
				return domain.Payment{}, mutationOutcome{}, ErrAlreadyPaused
			}
			if err := stores.payments.InsertPause(ctx, rule.ID, today); err != nil {
				return domain.Payment{}, mutationOutcome{}, fmt.Errorf("insert pause: %w", err)
			}
			return rule, mutationOutcome{audit: auditdomain.ActionPaymentPaused, tick: true, reread: true}, nil
		})
}

// ResumePayment closes the open pause with the owner's today (the resume day
// is already outside the half-open interval; anchors never shift) and the
// conveyor's tick stands the future planned back. Resuming a rule without an
// open pause is ErrNotPaused.
func (s *PaymentService) ResumePayment(
	ctx context.Context, actor, propertyID, paymentID uuid.UUID,
) (domain.Payment, error) {
	return s.mutateRule(ctx, actor, propertyID, paymentID, s.writeGate,
		func(ctx context.Context, stores *txStores, _ uuid.UUID, rule domain.Payment, today time.Time) (domain.Payment, mutationOutcome, error) {
			if _, active := domain.ActivePause(rule.Pauses); !active {
				return domain.Payment{}, mutationOutcome{}, ErrNotPaused
			}
			if err := stores.payments.CloseActivePause(ctx, rule.ID, today); err != nil {
				return domain.Payment{}, mutationOutcome{}, fmt.Errorf("close active pause: %w", err)
			}
			return rule, mutationOutcome{audit: auditdomain.ActionPaymentResumed, tick: true, reread: true}, nil
		})
}

// lockActiveProperty loads the property with its row locked — the mutation's
// serialization point (ADR 0049 §3) — resolves the owner's today and rejects
// the financial read-only archived state.
func (s *PaymentService) lockActiveProperty(
	ctx context.Context, stores *txStores, propertyID uuid.UUID,
) (PropertyRef, time.Time, error) {
	prop, err := stores.properties.GetForUpdate(ctx, propertyID)
	if err != nil {
		return PropertyRef{}, time.Time{}, err
	}
	if prop.Archived {
		return PropertyRef{}, time.Time{}, ErrArchivedProperty
	}
	if s.calendar == nil {
		return PropertyRef{}, time.Time{}, errors.New("payments: owner calendar must be configured")
	}
	today, err := s.calendar.Today(ctx, prop.OwnerID)
	if err != nil {
		return PropertyRef{}, time.Time{}, fmt.Errorf("resolve owner today: %w", err)
	}
	return prop, today, nil
}

// readScope applies the read gate and returns the data owner whose scope the
// SQL reads filter by (ADR 0028). A role without the view capability maps to
// ErrNotFound so the existence of a payment is never revealed.
func (s *PaymentService) readScope(ctx context.Context, actor, propertyID uuid.UUID) (uuid.UUID, error) {
	if s.policy == nil {
		prop, err := s.properties.Get(ctx, propertyID)
		if err != nil {
			return uuid.Nil, err
		}
		if prop.OwnerID != actor {
			return uuid.Nil, ErrNotFound
		}
		return actor, nil
	}
	role, err := s.policy.RoleForProperty(ctx, actor, propertyID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("resolve role: %w", err)
	}
	// Every non-allow outcome hides the payment's existence: none and
	// suspended are the privacy 404, forbidden (unreachable for view today)
	// would mean the same for reads.
	if sharedpolicy.GateFor(role, sharedpolicy.CanView) != sharedpolicy.GateAllow {
		return uuid.Nil, ErrNotFound
	}
	prop, err := s.properties.Get(ctx, propertyID)
	if err != nil {
		return uuid.Nil, err
	}
	return prop.OwnerID, nil
}

// writeGate applies the ADR 0028 mutation gate (CanEdit) and returns the
// resolved role for the audit entry. The decision mapping is the payments
// error vocabulary over the shared gate skeleton (policy.GateFor).
func (s *PaymentService) writeGate(ctx context.Context, actor, propertyID uuid.UUID) (sharedpolicy.Role, error) {
	return s.capabilityGate(ctx, actor, propertyID, sharedpolicy.CanEdit)
}

// deleteGate applies the ADR 0028 deletion gate: deletion is the owner's
// alone (CanLifecycle).
func (s *PaymentService) deleteGate(ctx context.Context, actor, propertyID uuid.UUID) (sharedpolicy.Role, error) {
	return s.capabilityGate(ctx, actor, propertyID, sharedpolicy.CanLifecycle)
}

// capabilityGate resolves the actor's role and maps the shared gate decision
// for the capability onto the payments errors: none/suspended stay
// privacy-preserving (ErrNotFound — the existence of a payment is never
// revealed), a role without the capability is a straight ErrForbidden.
func (s *PaymentService) capabilityGate(
	ctx context.Context, actor, propertyID uuid.UUID, can func(sharedpolicy.Role) bool,
) (sharedpolicy.Role, error) {
	if s.policy == nil {
		return sharedpolicy.RoleOwner, nil
	}
	role, err := s.policy.RoleForProperty(ctx, actor, propertyID)
	if err != nil {
		return "", fmt.Errorf("resolve role: %w", err)
	}
	switch sharedpolicy.GateFor(role, can) {
	case sharedpolicy.GateAllow:
		return role, nil
	case sharedpolicy.GateForbidden:
		return "", ErrForbidden
	default: // GateNone, GateSuspended.
		return "", ErrNotFound
	}
}

// recordAudit writes the mutation's audit entry inside the transaction
// (fail-safe: an insert error rolls the mutation back, ADR 0020). The role is
// the one the gate resolved before the transaction opened. Context carries
// whitelisted keys only — never the rule's title or amount.
func recordAudit(
	ctx context.Context, stores *txStores, actor uuid.UUID, role sharedpolicy.Role,
	action auditdomain.Action, entityID *uuid.UUID, auditCtx map[string]any,
) error {
	if err := stores.audit.Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  sharedpolicy.AuditActorRole(role),
		Action:     action,
		EntityType: auditdomain.EntityPayment,
		EntityID:   entityID,
		Context:    auditCtx,
	}); err != nil {
		return fmt.Errorf("record audit: %w", err)
	}
	return nil
}

// validateRule is the single validator of the create/update contract
// invariants — the transport decodes and delegates here, so the rules cannot
// drift between layers. It enforces: the direction and payment form enums, a
// non-empty title within MaxTitleLength characters (counted in runes: the
// limit is a product copy limit, not a byte limit), kopecks within 1..10⁹,
// a resolvable category reference (a default-catalog slug or a user category
// — the XOR is a durable schema invariant), a constructed non-zero
// recurrence and an end date no earlier than the rule's lower bound.
func validateRule(rule domain.Payment) error {
	if rule.Type != domain.TypeIncome && rule.Type != domain.TypeExpense {
		return ErrInvalidInput
	}
	if rule.PaymentForm != domain.FormTransfer && rule.PaymentForm != domain.FormCash {
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
	if cmd.PaymentForm != nil {
		payment.PaymentForm = *cmd.PaymentForm
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
	if cmd.PaymentForm != nil {
		fields = append(fields, "payment_form")
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
	return fields
}
