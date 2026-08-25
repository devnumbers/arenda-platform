package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// MaxAmountKopecks is the create/update amount ceiling: 10⁹ kopecks = 10
// million rubles (ADR 0049 §4). Shared with the transport validation.
const MaxAmountKopecks = 1_000_000_000

// MaxTitleLength bounds the rule title. Shared with the transport
// validation.
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
// pause and resume. Every mutation locks the property row (the context's
// serialization point), records its audit action in the same transaction and
// reruns the materialization tick over the changed rule before commit — so
// the stored world is consistent when the transaction commits (ADR 0048 №1).
// Reads never tick and never write.
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

// CreatePayment creates a payment rule on the property. The rule acts from
// the owner's today on: the server sets since, no backdated occurrences are
// generated (prototype decision №17). Full Access and Owner may create; a
// viewer gets ErrForbidden, a stranger a privacy-preserving ErrNotFound.
func (s *PaymentService) CreatePayment(
	ctx context.Context, actor, propertyID uuid.UUID, cmd CreatePaymentCommand,
) (domain.Payment, error) {
	role, err := s.writeGate(ctx, actor, propertyID)
	if err != nil {
		return domain.Payment{}, err
	}

	var scope, paymentID uuid.UUID
	err = s.runInTx(ctx, func(stores *txStores) error {
		prop, today, err := s.lockActiveProperty(ctx, stores, propertyID)
		if err != nil {
			return err
		}
		scope = prop.OwnerID
		category := domain.CategoryRef{Slug: &cmd.CategorySlug}
		if err := validateRule(cmd.Title, cmd.AmountKopecks, category, cmd.Recurrence, cmd.EndDate, today); err != nil {
			return err
		}
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("mint payment id: %w", err)
		}
		paymentID = id
		created := domain.Payment{
			ID:            id,
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
		if err := stores.payments.Create(ctx, created); err != nil {
			return fmt.Errorf("create payment: %w", err)
		}
		if err := recordAudit(ctx, stores, actor, role, auditdomain.ActionPaymentCreated, &id, nil); err != nil {
			return err
		}
		// The tick materializes the rule's due occurrences and stands the
		// single future planned (ADR 0049 §3: mutations tick after the
		// change, in their own transaction).
		return stores.tickOwner(ctx, scope, today)
	})
	if err != nil {
		return domain.Payment{}, err
	}
	return s.reread(ctx, scope, propertyID, paymentID)
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
// unchanged, since is not editable. The in-transaction tick rebuilds the
// single future planned from the updated rule; paid operations keep their
// snapshots and are never touched. Full Access and Owner may edit.
func (s *PaymentService) UpdatePayment(
	ctx context.Context, actor, propertyID, paymentID uuid.UUID, cmd UpdatePaymentCommand,
) (domain.Payment, error) {
	role, err := s.writeGate(ctx, actor, propertyID)
	if err != nil {
		return domain.Payment{}, err
	}

	var scope uuid.UUID
	err = s.runInTx(ctx, func(stores *txStores) error {
		prop, today, err := s.lockActiveProperty(ctx, stores, propertyID)
		if err != nil {
			return err
		}
		scope = prop.OwnerID
		payment, err := stores.payments.Get(ctx, paymentID, scope, propertyID)
		if err != nil {
			return err
		}
		applyUpdate(&payment, cmd)
		if err := validateRule(payment.Title, payment.AmountKopecks, payment.Category,
			payment.Recurrence, payment.EndDate, payment.Since); err != nil {
			return err
		}
		if err := stores.payments.Update(ctx, payment); err != nil {
			return fmt.Errorf("update payment: %w", err)
		}
		// Invalidate the strictly future planned: the tick below stands it
		// again from the updated rule with fresh snapshots — the edit rebuilds
		// the future, while the already due occurrences keep their frozen
		// snapshots (user story #32 of the spec).
		if err := stores.payments.DeleteFuturePlanned(ctx, paymentID, today); err != nil {
			return err
		}
		if err := recordAudit(ctx, stores, actor, role, auditdomain.ActionPaymentUpdated, &paymentID,
			map[string]any{"fields": updatedFields(cmd)}); err != nil {
			return err
		}
		return stores.tickOwner(ctx, scope, today)
	})
	if err != nil {
		return domain.Payment{}, err
	}
	return s.reread(ctx, scope, propertyID, paymentID)
}

// DeletePayment removes the rule. Planned operations from today on always go
// with it; the overdue planned debt goes too when keepOverdue is false, and
// paid operations stay in history marked «платёж удалён» (payment_id set to
// NULL by the FK, origin unchanged — ticket #446). Deletion is Owner-only
// (the ADR 0028 matrix: the final decision belongs to the property owner).
func (s *PaymentService) DeletePayment(
	ctx context.Context, actor, propertyID, paymentID uuid.UUID, keepOverdue bool,
) error {
	role, err := s.deleteGate(ctx, actor, propertyID)
	if err != nil {
		return err
	}

	return s.runInTx(ctx, func(stores *txStores) error {
		prop, today, err := s.lockActiveProperty(ctx, stores, propertyID)
		if err != nil {
			return err
		}
		// Existence check inside the lock: a missing or foreign payment is
		// ErrNotFound from the store, before anything is deleted.
		if _, err := stores.payments.Get(ctx, paymentID, prop.OwnerID, propertyID); err != nil {
			return err
		}
		if err := stores.payments.DeletePlannedFrom(ctx, paymentID, today); err != nil {
			return fmt.Errorf("delete future planned of payment %s: %w", paymentID, err)
		}
		if !keepOverdue {
			if err := stores.payments.DeletePlannedBefore(ctx, paymentID, today); err != nil {
				return fmt.Errorf("delete overdue planned of payment %s: %w", paymentID, err)
			}
		}
		if err := stores.payments.Delete(ctx, paymentID, prop.OwnerID); err != nil {
			return fmt.Errorf("delete payment: %w", err)
		}
		if err := recordAudit(ctx, stores, actor, role, auditdomain.ActionPaymentDeleted, &paymentID,
			map[string]any{"keep_overdue": keepOverdue}); err != nil {
			return err
		}
		// No tick rerun: the rule is gone and its plan was resolved
		// explicitly above — an owner-wide snapshot would be a no-op scan.
		return nil
	})
}

// PausePayment opens the open-ended pause from the owner's today on: nothing
// generates inside the pause — no operations, no debt, no auto-pay — and the
// debt accumulated before it stays (CONTEXT.md «Пауза»). The tick inside the
// transaction removes the future planned. A second pause on a rule with an
// open one is ErrAlreadyPaused.
func (s *PaymentService) PausePayment(
	ctx context.Context, actor, propertyID, paymentID uuid.UUID,
) (domain.Payment, error) {
	role, err := s.writeGate(ctx, actor, propertyID)
	if err != nil {
		return domain.Payment{}, err
	}

	var scope uuid.UUID
	err = s.runInTx(ctx, func(stores *txStores) error {
		prop, today, err := s.lockActiveProperty(ctx, stores, propertyID)
		if err != nil {
			return err
		}
		scope = prop.OwnerID
		payment, err := stores.payments.Get(ctx, paymentID, scope, propertyID)
		if err != nil {
			return err
		}
		if _, active := domain.ActivePause(payment.Pauses); active {
			return ErrAlreadyPaused
		}
		if err := stores.payments.InsertPause(ctx, paymentID, today); err != nil {
			return fmt.Errorf("insert pause: %w", err)
		}
		if err := recordAudit(ctx, stores, actor, role, auditdomain.ActionPaymentPaused, &paymentID, nil); err != nil {
			return err
		}
		return stores.tickOwner(ctx, scope, today)
	})
	if err != nil {
		return domain.Payment{}, err
	}
	return s.reread(ctx, scope, propertyID, paymentID)
}

// ResumePayment closes the open pause with the owner's today (the resume day
// is already outside the half-open interval; anchors never shift) and the
// tick inside the transaction stands the future planned back. Resuming a rule
// without an open pause is ErrNotPaused.
func (s *PaymentService) ResumePayment(
	ctx context.Context, actor, propertyID, paymentID uuid.UUID,
) (domain.Payment, error) {
	role, err := s.writeGate(ctx, actor, propertyID)
	if err != nil {
		return domain.Payment{}, err
	}

	var scope uuid.UUID
	err = s.runInTx(ctx, func(stores *txStores) error {
		prop, today, err := s.lockActiveProperty(ctx, stores, propertyID)
		if err != nil {
			return err
		}
		scope = prop.OwnerID
		payment, err := stores.payments.Get(ctx, paymentID, scope, propertyID)
		if err != nil {
			return err
		}
		if _, active := domain.ActivePause(payment.Pauses); !active {
			return ErrNotPaused
		}
		if err := stores.payments.CloseActivePause(ctx, paymentID, today); err != nil {
			return fmt.Errorf("close active pause: %w", err)
		}
		if err := recordAudit(ctx, stores, actor, role, auditdomain.ActionPaymentResumed, &paymentID, nil); err != nil {
			return err
		}
		return stores.tickOwner(ctx, scope, today)
	})
	if err != nil {
		return domain.Payment{}, err
	}
	return s.reread(ctx, scope, propertyID, paymentID)
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
	if !sharedpolicy.CanView(role) {
		return uuid.Nil, ErrNotFound
	}
	prop, err := s.properties.Get(ctx, propertyID)
	if err != nil {
		return uuid.Nil, err
	}
	return prop.OwnerID, nil
}

// writeGate applies the ADR 0028 mutation gate: none/suspended map to
// ErrNotFound (object privacy), a view-only role to ErrForbidden. The
// resolved role returns for the audit entry.
func (s *PaymentService) writeGate(ctx context.Context, actor, propertyID uuid.UUID) (sharedpolicy.Role, error) {
	if s.policy == nil {
		return sharedpolicy.RoleOwner, nil
	}
	role, err := s.policy.RoleForProperty(ctx, actor, propertyID)
	if err != nil {
		return "", fmt.Errorf("resolve role: %w", err)
	}
	if role == sharedpolicy.RoleNone || role == sharedpolicy.RoleSuspended {
		return "", ErrNotFound
	}
	if !sharedpolicy.CanEdit(role) {
		return "", ErrForbidden
	}
	return role, nil
}

// deleteGate applies the ADR 0028 deletion gate: deletion is the owner's
// alone (CanLifecycle); none/suspended still map to ErrNotFound.
func (s *PaymentService) deleteGate(ctx context.Context, actor, propertyID uuid.UUID) (sharedpolicy.Role, error) {
	if s.policy == nil {
		return sharedpolicy.RoleOwner, nil
	}
	role, err := s.policy.RoleForProperty(ctx, actor, propertyID)
	if err != nil {
		return "", fmt.Errorf("resolve role: %w", err)
	}
	if role == sharedpolicy.RoleNone || role == sharedpolicy.RoleSuspended {
		return "", ErrNotFound
	}
	if !sharedpolicy.CanLifecycle(role) {
		return "", ErrForbidden
	}
	return role, nil
}

// reread loads the stored rule after the transaction committed so the
// response carries the persisted timestamps, pauses and category view rather
// than the pre-commit image.
func (s *PaymentService) reread(ctx context.Context, scope, propertyID, paymentID uuid.UUID) (domain.Payment, error) {
	payment, err := s.payments.Get(ctx, paymentID, scope, propertyID)
	if err != nil {
		return domain.Payment{}, err
	}
	return payment, nil
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
		ActorRole:  auditActorRoleFromPolicy(role),
		Action:     action,
		EntityType: auditdomain.EntityPayment,
		EntityID:   entityID,
		Context:    auditCtx,
	}); err != nil {
		return fmt.Errorf("record audit: %w", err)
	}
	return nil
}

// auditActorRoleFromPolicy maps a policy role to the audit actor role. The
// mapping lives in the calling module by design (audit must not depend on
// shared/policy); suspended/none never reach a write path.
func auditActorRoleFromPolicy(role sharedpolicy.Role) auditdomain.ActorRole {
	switch role {
	case sharedpolicy.RoleFullAccess:
		return auditdomain.ActorRoleFullAccess
	case sharedpolicy.RoleViewer:
		return auditdomain.ActorRoleViewer
	default:
		return auditdomain.ActorRoleOwner
	}
}

// validateRule checks the create/update contract invariants the schema and
// the catalog enforce: non-empty bounded title, kopecks within 1..10⁹, a
// resolvable category reference (a default-catalog slug or a user category —
// the XOR is a durable schema invariant), a constructed (non-zero) recurrence
// and an end date no earlier than the rule's lower bound.
func validateRule(
	title string, amount int64, category domain.CategoryRef, recurrence domain.Recurrence,
	endDate *time.Time, since time.Time,
) error {
	if t := strings.TrimSpace(title); t == "" || len(t) > MaxTitleLength {
		return ErrInvalidInput
	}
	if amount < 1 || amount > MaxAmountKopecks {
		return ErrInvalidInput
	}
	switch {
	case category.Slug != nil:
		if !domain.IsValidDefaultCategorySlug(*category.Slug) {
			return ErrInvalidInput
		}
	case category.UserCategoryID != nil:
		// The user-category reference is carried unchanged; its endpoints
		// arrive with the categories slice.
	default:
		return ErrInvalidInput
	}
	if recurrence.Kind() == "" {
		return ErrInvalidInput
	}
	if endDate != nil && endDate.Before(since) {
		return ErrInvalidInput
	}
	return nil
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
