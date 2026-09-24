package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/rentals/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// MaxAmountKopecks is the create/update amount ceiling: 10⁹ kopecks = 10
// million rubles (ADR 0053 §4, the payments bound).
const MaxAmountKopecks = 1_000_000_000

// MaxCommentLength bounds the rental comment, counted in characters — a
// product copy limit, not a byte limit (the schema CHECK counts the same
// number: char_length is character-based for TEXT).
const MaxCommentLength = 2000

// DateUpdate is the tri-state PATCH resolution of a nullable date (the
// planned end): a nil command field keeps the current value, Value nil clears
// it, a date sets it.
type DateUpdate struct {
	Value *time.Time
}

// AmountUpdate is the tri-state PATCH resolution of a nullable amount (the
// deposit, the commission): a nil command field keeps the current value,
// Value nil clears it, an amount sets it.
type AmountUpdate struct {
	Value *int64
}

// ContactUpdate is the tri-state PATCH resolution of the tenant reference: a
// nil command field keeps the current contact, Value nil clears it («Контакта
// нет»), a contact id sets it.
type ContactUpdate struct {
	Value *uuid.UUID
}

// StringUpdate is the tri-state PATCH resolution of the comment: a nil
// command field keeps the current text, Value nil clears it, a string sets it.
type StringUpdate struct {
	Value *string
}

// CreateRentalCommand is the validated create payload of a rental (ADR 0053
// §4). The managed payment is not part of it: the service derives it —
// income, «Арендная плата», transfer form, the rent category, the recurrence
// from the payment day, since = the start date, end_date = the planned end.
type CreateRentalCommand struct {
	AmountKopecks     int64
	PaymentDay        domain.PaymentDay
	StartDate         time.Time
	PlannedEndDate    *time.Time
	Utilities         domain.Utilities
	DepositKopecks    *int64
	CommissionKopecks *int64
	ContactID         *uuid.UUID
	Comment           string
	AutoPay           bool
	// ReminderOffsetDays is the managed payment's reminder lead time
	// (карта #822): 1/3/7, nil = без напоминаний.
	ReminderOffsetDays *int
}

// UpdateRentalCommand is the partial-update payload: a nil field leaves the
// term unchanged (omit = no change). The start date is not editable at all
// (ADR 0053 §3); the nullable terms — planned end, deposit, commission,
// tenant, comment — are tri-state (явный null = очистить).
type UpdateRentalCommand struct {
	AmountKopecks     *int64
	PaymentDay        *domain.PaymentDay
	AutoPay           *bool
	PlannedEndDate    *DateUpdate
	Utilities         *domain.Utilities
	DepositKopecks    *AmountUpdate
	CommissionKopecks *AmountUpdate
	ContactID         *ContactUpdate
	Comment           *StringUpdate
}

// DepositReturn is the optional deposit-return record of the completion: how
// much of the deposit went back to the tenant (a zero sum is valid — «не
// вернул») and the optional comment (only with an amount, the schema CHECK).
type DepositReturn struct {
	AmountKopecks int64
	Comment       *string
}

// CompleteRentalCommand is the completion payload: the factual completion
// date (the start ≤ date ≤ today; «По плану» is the UI's substitution) and
// the optional deposit return.
type CompleteRentalCommand struct {
	CompletedDate time.Time
	DepositReturn *DepositReturn
}

// RentalProgress is the «N из M месяцев» read model (ADR 0053 §2): paidMonths
// counts the managed payment's paid operations; totalMonths and
// monthsRemaining exist for a term rental only (nil for an open-ended one).
// OverdueMonths is the managed payment's overdue occurrences — «Просрочено
// N месяцев» (#817); nil when nothing is overdue (no red line on the card).
type RentalProgress struct {
	PaidMonths      int
	TotalMonths     *int
	MonthsRemaining *int
	OverdueMonths   *int
}

// RentalView is the assembled rental response model (ADR 0053 §4): the
// stored row plus everything the TZ-blind client renders from — the computed
// status, the payment's render state (the day of payment lives in the
// payment, решение №5), the single future planned operation, the progress and
// the owner's today.
type RentalView struct {
	Rental      domain.Rental
	Status      domain.Status
	Payment     RentPaymentState
	NextPayment *PlannedOccurrence
	Progress    RentalProgress
	Today       time.Time
}

// RentalSummary is the rental's period totals (решение №13): all paid
// operations of the property — of any payment and manual ones — with the
// operation date inside [from, until]. Profit = income − expense and may be
// negative.
type RentalSummary struct {
	From           time.Time
	Until          time.Time
	IncomeKopecks  int64
	ExpenseKopecks int64
	ProfitKopecks  int64
}

// RentalService orchestrates the rental use cases (ADR 0053, ticket #529):
// create (with the atomic payment), read, list, partial update, complete,
// delete and the period summary. Every mutation runs through the shared
// runRentalMutation conveyor, which owns the ordering invariants
// structurally: the role gate, the property row lock, the owner's today, the
// change step, its audit entry in the same transaction and the payments tick
// when the managed payment changed. Reads never tick and never write.
type RentalService struct {
	txStoreFactory
	policy    sharedpolicy.Policy
	calendar  paymentsapp.OwnerCalendar
	viewGate  gateFunc // Viewer reads.
	writeGate gateFunc // Full Access+: create/edit/complete.
	delGate   gateFunc // Owner alone: deletion.
}

// NewRentalService builds the rental use case service over the composite
// transactional store factory, the owner calendar and the authorization
// policy (ADR 0028). A nil calendar is a wiring mistake; a mutation fails on
// first use rather than writing with a zero date. A nil policy keeps the
// historical owner-only behaviour (the owner acts on their own scope).
func NewRentalService(
	factory txStoreFactory, calendar paymentsapp.OwnerCalendar, policy sharedpolicy.Policy,
) *RentalService {
	return &RentalService{
		txStoreFactory: factory,
		policy:         policy,
		calendar:       calendar,
		viewGate:       newCapabilityGate(policy, sharedpolicy.CanView),
		writeGate:      newCapabilityGate(policy, sharedpolicy.CanEdit),
		delGate:        newCapabilityGate(policy, sharedpolicy.CanLifecycle),
	}
}

// conveyor bundles this service's factory and calendar for the shared
// mutation conveyor.
func (s *RentalService) conveyor() mutationGates {
	return mutationGates{factory: s.txStoreFactory, calendar: s.calendar}
}

// readScope applies the read gate and returns the data owner whose SQL reads
// filter by (ADR 0028).
func (s *RentalService) readScope(ctx context.Context, actor, propertyID uuid.UUID) (uuid.UUID, error) {
	return resolveReadScope(ctx, s.policy, s.properties, actor, propertyID)
}

// CreateRental creates the rental and its managed payment atomically (ADR 0053
// §3): the payment rule (income, «Арендная плата», transfer, the rent
// category, the recurrence from the payment day, since = the start date,
// end_date = the planned end) inside the same transaction as the rentals row.
// The start date is today or later — no backdated rentals; a second
// unfinished rental on the property is ErrPropertyOccupied (the 409 under the
// lock; the partial unique index backstops the race). Full Access and Owner
// may create.
func (s *RentalService) CreateRental(
	ctx context.Context, actor, propertyID uuid.UUID, cmd CreateRentalCommand,
) (RentalView, error) {
	rentalID, err := runRentalMutation(s.conveyor(), ctx, actor, propertyID, uuid.Nil, s.writeGate,
		func(
			ctx context.Context, stores *txStores, scope uuid.UUID, _ domain.Rental, today time.Time,
		) (mutationOutcome, error) {
			if err := validateCreate(cmd, today); err != nil {
				return mutationOutcome{}, err
			}
			if cmd.ContactID != nil {
				ok, err := s.tenantExists(ctx, scope, *cmd.ContactID)
				if err != nil {
					return mutationOutcome{}, err
				}
				if !ok {
					return mutationOutcome{}, ErrInvalidInput
				}
			}
			occupied, err := stores.rentals.HasUnfinished(ctx, scope, propertyID)
			if err != nil {
				return mutationOutcome{}, fmt.Errorf("check unfinished rental: %w", err)
			}
			if occupied {
				return mutationOutcome{}, ErrPropertyOccupied
			}
			paymentID, err := stores.pay.Create(ctx, RentPaymentSeed{
				OwnerID:            scope,
				PropertyID:         propertyID,
				AmountKopecks:      cmd.AmountKopecks,
				PaymentDay:         cmd.PaymentDay,
				StartDate:          cmd.StartDate,
				PlannedEndDate:     cmd.PlannedEndDate,
				AutoPay:            cmd.AutoPay,
				ReminderOffsetDays: cmd.ReminderOffsetDays,
			})
			if err != nil {
				return mutationOutcome{}, fmt.Errorf("create rent payment: %w", err)
			}
			rentalID, err := uuid.NewV7()
			if err != nil {
				return mutationOutcome{}, fmt.Errorf("mint rental id: %w", err)
			}
			draft := domain.Rental{
				ID:                rentalID,
				OwnerID:           scope,
				PropertyID:        propertyID,
				PaymentID:         paymentID,
				ContactID:         cmd.ContactID,
				StartDate:         cmd.StartDate,
				PlannedEndDate:    cmd.PlannedEndDate,
				Utilities:         cmd.Utilities,
				DepositKopecks:    cmd.DepositKopecks,
				CommissionKopecks: cmd.CommissionKopecks,
				Comment:           strings.TrimSpace(cmd.Comment),
			}
			if err := stores.rentals.Create(ctx, draft); err != nil {
				return mutationOutcome{}, fmt.Errorf("create rental: %w", err)
			}
			return mutationOutcome{
				RentalID: rentalID,
				Audit:    auditActionRentalCreated,
				Tick:     true, // The new payment materializes its first planned.
			}, nil
		})
	if err != nil {
		return RentalView{}, err
	}
	scope, err := s.readScope(ctx, actor, propertyID)
	if err != nil {
		return RentalView{}, err
	}
	return s.loadView(ctx, scope, propertyID, rentalID)
}

// GetRental returns one rental as the assembled view. Any actor with the view
// capability may read; a missing or foreign rental is ErrNotFound. Reads never
// tick.
func (s *RentalService) GetRental(
	ctx context.Context, actor, propertyID, rentalID uuid.UUID,
) (RentalView, error) {
	scope, err := s.readScope(ctx, actor, propertyID)
	if err != nil {
		return RentalView{}, err
	}
	return s.loadView(ctx, scope, propertyID, rentalID)
}

// ListRentals returns the property's rentals as assembled views: unfinished
// first, then the completed by completion date, fresh on top (the store's
// ordering). Reads never tick.
func (s *RentalService) ListRentals(
	ctx context.Context, actor, propertyID uuid.UUID,
) ([]RentalView, error) {
	scope, err := s.readScope(ctx, actor, propertyID)
	if err != nil {
		return nil, err
	}
	rentals, err := s.rentals.ListByProperty(ctx, scope, propertyID)
	if err != nil {
		return nil, fmt.Errorf("list rentals: %w", err)
	}
	today, err := ownerToday(s.calendar, ctx, scope)
	if err != nil {
		return nil, err
	}
	views := make([]RentalView, 0, len(rentals))
	for _, r := range rentals {
		view, err := s.assembleView(ctx, scope, propertyID, r, today)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

// UpdateRental applies a diff-patch to the rental's terms (ADR 0053 §3,
// решение №14): omitted fields are left unchanged, the start date is not
// editable, the completed rental is final (ErrRentalCompleted). The editable
// payment terms — amount, payment day, auto-pay, planned end — sync into the
// payment, whose strictly future planned the in-transaction tick stands again
// with fresh snapshots; terms the payment does not carry (utilities, deposit,
// commission, tenant, comment) write the rentals row only and skip the tick.
func (s *RentalService) UpdateRental(
	ctx context.Context, actor, propertyID, rentalID uuid.UUID, cmd UpdateRentalCommand,
) (RentalView, error) {
	changedRentalID, err := runRentalMutation(s.conveyor(), ctx, actor, propertyID, rentalID, s.writeGate,
		func(
			ctx context.Context, stores *txStores, scope uuid.UUID, rental domain.Rental, today time.Time,
		) (mutationOutcome, error) {
			if rental.CompletedDate != nil {
				return mutationOutcome{}, ErrRentalCompleted
			}
			change, paymentChanged, err := paymentSyncFromUpdate(cmd, rental.StartDate, today)
			if err != nil {
				return mutationOutcome{}, err
			}
			applyRentalUpdate(&rental, cmd)
			if err := validateRentalRow(rental); err != nil {
				return mutationOutcome{}, err
			}
			if cmd.ContactID != nil && cmd.ContactID.Value != nil {
				ok, err := s.tenantExists(ctx, scope, *cmd.ContactID.Value)
				if err != nil {
					return mutationOutcome{}, err
				}
				if !ok {
					return mutationOutcome{}, ErrInvalidInput
				}
			}
			if paymentChanged {
				if err := stores.pay.Update(ctx, scope, propertyID, rental.PaymentID, change, today); err != nil {
					return mutationOutcome{}, fmt.Errorf("sync rent payment: %w", err)
				}
			}
			if err := stores.rentals.Update(ctx, rental); err != nil {
				return mutationOutcome{}, fmt.Errorf("update rental: %w", err)
			}
			return mutationOutcome{
				RentalID: rental.ID,
				Audit:    auditActionRentalUpdated,
				AuditCtx: map[string]any{"fields": updatedRentalFields(cmd)},
				Tick:     paymentChanged,
			}, nil
		})
	if err != nil {
		return RentalView{}, err
	}
	scope, err := s.readScope(ctx, actor, propertyID)
	if err != nil {
		return RentalView{}, err
	}
	return s.loadView(ctx, scope, propertyID, changedRentalID)
}

// CompleteRental records the completion (решение №8): the factual date within
// [start, today], the optional deposit return, and the managed payment
// stopped at the completion date — end_date moves there, the
// strictly-after-completion planned goes, the already due stays with the
// owner. The change step resolves the payment's plan itself; no tick runs. A
// second completion is ErrRentalCompleted (the 409).
func (s *RentalService) CompleteRental(
	ctx context.Context, actor, propertyID, rentalID uuid.UUID, cmd CompleteRentalCommand,
) (RentalView, error) {
	_, err := runRentalMutation(s.conveyor(), ctx, actor, propertyID, rentalID, s.writeGate,
		func(
			ctx context.Context, stores *txStores, scope uuid.UUID, rental domain.Rental, today time.Time,
		) (mutationOutcome, error) {
			if rental.CompletedDate != nil {
				return mutationOutcome{}, ErrRentalCompleted
			}
			if err := validateCompletedDate(cmd.CompletedDate, rental.StartDate, today); err != nil {
				return mutationOutcome{}, err
			}
			if err := validateDepositReturn(cmd.DepositReturn); err != nil {
				return mutationOutcome{}, err
			}
			if err := stores.pay.Stop(ctx, scope, propertyID, rental.PaymentID, cmd.CompletedDate); err != nil {
				return mutationOutcome{}, fmt.Errorf("stop rent payment: %w", err)
			}
			if err := stores.rentals.Complete(ctx, rental.ID, scope, cmd.CompletedDate, cmd.DepositReturn); err != nil {
				return mutationOutcome{}, fmt.Errorf("complete rental: %w", err)
			}
			out := mutationOutcome{
				RentalID: rental.ID,
				Audit:    auditActionRentalCompleted,
				Tick:     false,
			}
			if cmd.DepositReturn != nil {
				out.AuditCtx = map[string]any{"fields": []string{"deposit_return"}}
			}
			return out, nil
		})
	if err != nil {
		return RentalView{}, err
	}
	scope, err := s.readScope(ctx, actor, propertyID)
	if err != nil {
		return RentalView{}, err
	}
	return s.loadView(ctx, scope, propertyID, rentalID)
}

// DeleteRental hard-deletes the rental together with its managed payment
// (решение №10, ADR 0053 §3) — only for a completed rental or a not-yet-
// started one (start_date > today: передумал до старта, все planned будущие
// и сносятся чисто). A started unfinished rental can only go through the
// completion (ErrRentalStarted, the 409). The rentals row goes first (the
// RESTRICT FK releases the payment only afterwards); the overdue planned
// stays as debt (the keep_overdue=true semantics). Deletion is Owner-only.
func (s *RentalService) DeleteRental(
	ctx context.Context, actor, propertyID, rentalID uuid.UUID,
) error {
	_, err := runRentalMutation(s.conveyor(), ctx, actor, propertyID, rentalID, s.delGate,
		func(
			ctx context.Context, stores *txStores, scope uuid.UUID, rental domain.Rental, today time.Time,
		) (mutationOutcome, error) {
			if rental.CompletedDate == nil && !rental.StartDate.After(today) {
				return mutationOutcome{}, ErrRentalStarted
			}
			if err := stores.rentals.Delete(ctx, rental.ID, scope); err != nil {
				return mutationOutcome{}, fmt.Errorf("delete rental: %w", err)
			}
			if err := stores.pay.Delete(ctx, scope, rental.PaymentID, today); err != nil {
				return mutationOutcome{}, fmt.Errorf("delete rent payment: %w", err)
			}
			return mutationOutcome{
				RentalID: rental.ID,
				Audit:    auditActionRentalDeleted,
				Tick:     false,
			}, nil
		})
	return err
}

// RentalSummary totals the property's paid operations over the rental's
// period (решение №13): from the start date until the given date — the
// completion date for a completed rental, today otherwise — or the explicitly
// requested one (the completion master previews with it before completing).
// The period bounds by the operation date, not the payment date; profit may
// be negative.
func (s *RentalService) RentalSummary(
	ctx context.Context, actor, propertyID, rentalID uuid.UUID, until *time.Time,
) (RentalSummary, error) {
	scope, err := s.readScope(ctx, actor, propertyID)
	if err != nil {
		return RentalSummary{}, err
	}
	rental, err := s.rentals.Get(ctx, rentalID, scope, propertyID)
	if err != nil {
		return RentalSummary{}, err
	}
	today, err := ownerToday(s.calendar, ctx, scope)
	if err != nil {
		return RentalSummary{}, err
	}
	end := until
	if end == nil {
		if rental.CompletedDate != nil {
			end = rental.CompletedDate
		} else {
			end = &today
		}
	}
	if end.Before(rental.StartDate) {
		return RentalSummary{}, ErrInvalidInput
	}
	totals, err := s.gateway.SummarizePaidOperations(ctx, scope, propertyID, rental.StartDate, *end, today)
	if err != nil {
		return RentalSummary{}, fmt.Errorf("summarize paid operations: %w", err)
	}
	return RentalSummary{
		From:           rental.StartDate,
		Until:          *end,
		IncomeKopecks:  totals.IncomeKopecks,
		ExpenseKopecks: totals.ExpenseKopecks,
		ProfitKopecks:  totals.IncomeKopecks - totals.ExpenseKopecks,
	}, nil
}

// loadView re-reads the stored rental after commit and assembles the view.
func (s *RentalService) loadView(
	ctx context.Context, scope, propertyID, rentalID uuid.UUID,
) (RentalView, error) {
	rental, err := s.rentals.Get(ctx, rentalID, scope, propertyID)
	if err != nil {
		return RentalView{}, err
	}
	today, err := ownerToday(s.calendar, ctx, scope)
	if err != nil {
		return RentalView{}, err
	}
	return s.assembleView(ctx, scope, propertyID, rental, today)
}

// assembleView computes everything the TZ-blind client renders (ADR 0053
// §2): the status, the payment's render state, the single future planned
// operation, the «N из M» progress. The bounded read queries stay per-rental;
// the list response assembles the same way.
func (s *RentalService) assembleView(
	ctx context.Context, scope, propertyID uuid.UUID, rental domain.Rental, today time.Time,
) (RentalView, error) {
	state, err := s.gateway.RentPaymentState(ctx, scope, propertyID, rental.PaymentID)
	if err != nil {
		return RentalView{}, fmt.Errorf("rent payment state: %w", err)
	}
	// Идентификатор управляемого платежа знает аренда (связь 1:1) —
	// состояние рендера его только носит.
	state.PaymentID = rental.PaymentID
	next, err := s.gateway.NextPlannedOccurrence(ctx, scope, propertyID, rental.PaymentID, today)
	if err != nil {
		return RentalView{}, fmt.Errorf("next planned occurrence: %w", err)
	}
	paid, err := s.gateway.CountPaidOperations(ctx, scope, propertyID, rental.PaymentID)
	if err != nil {
		return RentalView{}, fmt.Errorf("count paid operations: %w", err)
	}
	// Серверная просрочка (#817, ADR 0053 §2 — всё считает сервер): счётчик
	// planned-вхождений раньше «сегодня» собственника; ноль — null.
	overdue, err := s.gateway.CountOverdueOccurrences(ctx, scope, propertyID, rental.PaymentID, today)
	if err != nil {
		return RentalView{}, fmt.Errorf("count overdue occurrences: %w", err)
	}
	progress := RentalProgress{PaidMonths: paid, OverdueMonths: nilIfZero(overdue)}
	if rental.PlannedEndDate != nil {
		total := domain.CountPaymentDays(rental.StartDate, *rental.PlannedEndDate, state.PaymentDay)
		progress.TotalMonths = &total
		remaining := domain.FullMonthsBetween(today, *rental.PlannedEndDate)
		progress.MonthsRemaining = &remaining
	}
	return RentalView{
		Rental:      rental,
		Status:      domain.StatusOf(rental, today),
		Payment:     state,
		NextPayment: next,
		Progress:    progress,
		Today:       today,
	}, nil
}

// nilIfZero folds a zero count into null — the wire's «просрочки нет»
// (#817: без просрочки красной строки нет).
func nilIfZero(n int) *int {
	if n == 0 {
		return nil
	}
	return &n
}

// tenantExists resolves the tenant contact through the book reader; the
// contacts owner check is the application's duty (ADR 0053 — the FK does not
// verify the owner). A missing reader is a wiring mistake only when a command
// actually names a contact.
func (s *RentalService) tenantExists(ctx context.Context, scope, contactID uuid.UUID) (bool, error) {
	if s.tenants == nil {
		return false, errors.New("rentals: tenant reader must be configured to name a contact")
	}
	ok, err := s.tenants.Exists(ctx, scope, contactID)
	if err != nil {
		return false, fmt.Errorf("check tenant contact: %w", err)
	}
	return ok, nil
}

// validateCreate enforces the create contract (ADR 0053 §4): the amount
// within 1..10⁹, the utilities mode, the start today or later (no backdated
// rentals), the planned end strictly after the start, the optional records in
// bounds and the comment within its limit.
func validateCreate(cmd CreateRentalCommand, today time.Time) error {
	if cmd.AmountKopecks < 1 || cmd.AmountKopecks > MaxAmountKopecks {
		return ErrInvalidInput
	}
	if !cmd.Utilities.Valid() {
		return ErrInvalidInput
	}
	if cmd.StartDate.Before(today) {
		return ErrInvalidInput
	}
	if cmd.PlannedEndDate != nil && !cmd.PlannedEndDate.After(cmd.StartDate) {
		return ErrInvalidInput
	}
	if err := validateOptionalAmount(cmd.DepositKopecks); err != nil {
		return err
	}
	if err := validateOptionalAmount(cmd.CommissionKopecks); err != nil {
		return err
	}
	if !paymentsapp.IsValidReminderOffset(cmd.ReminderOffsetDays) {
		return ErrInvalidInput
	}
	if utf8.RuneCountInString(strings.TrimSpace(cmd.Comment)) > MaxCommentLength {
		return ErrInvalidInput
	}
	return nil
}

// paymentSyncFromUpdate builds the managed payment's partial sync from the
// terms edit and reports whether the payment changed at all (the tick
// verdict, ADR 0053 §3). The amounts and the planned end re-check their
// create-time bounds here.
func paymentSyncFromUpdate(
	cmd UpdateRentalCommand, start, today time.Time,
) (RentPaymentChange, bool, error) {
	var change RentPaymentChange
	paymentChanged := false
	if cmd.AmountKopecks != nil {
		if *cmd.AmountKopecks < 1 || *cmd.AmountKopecks > MaxAmountKopecks {
			return change, false, ErrInvalidInput
		}
		change.AmountKopecks = cmd.AmountKopecks
		paymentChanged = true
	}
	if cmd.PaymentDay != nil {
		change.PaymentDay = cmd.PaymentDay
		paymentChanged = true
	}
	if cmd.AutoPay != nil {
		change.AutoPay = cmd.AutoPay
		paymentChanged = true
	}
	if cmd.PlannedEndDate != nil {
		if v := cmd.PlannedEndDate.Value; v != nil {
			if err := validatePlannedEnd(*v, start, today); err != nil {
				return change, false, err
			}
		}
		change.PlannedEndDate = cmd.PlannedEndDate
		paymentChanged = true
	}
	return change, paymentChanged, nil
}

// validatePlannedEnd enforces the edit rule for the planned end (ADR 0053 §3,
// решение №14): freely forward and back, but never into the past — not before
// today and never at or before the start.
func validatePlannedEnd(plannedEnd, start, today time.Time) error {
	if !plannedEnd.After(start) || plannedEnd.Before(today) {
		return ErrInvalidInput
	}
	return nil
}

// validateCompletedDate enforces the completion window (решение №8): the
// start ≤ date ≤ today — the future has not happened yet.
func validateCompletedDate(completedDate, start, today time.Time) error {
	if completedDate.Before(start) || completedDate.After(today) {
		return ErrInvalidInput
	}
	return nil
}

// validateDepositReturn enforces the completion record: the amount within
// 0..10⁹ (a zero sum is valid — «не вернул») and the comment only with an
// amount (the schema CHECK keeps the same invariant durably).
func validateDepositReturn(ret *DepositReturn) error {
	if ret == nil {
		return nil
	}
	if ret.AmountKopecks < 0 || ret.AmountKopecks > MaxAmountKopecks {
		return ErrInvalidInput
	}
	if ret.Comment != nil && utf8.RuneCountInString(*ret.Comment) > MaxCommentLength {
		return ErrInvalidInput
	}
	return nil
}

// validateOptionalAmount enforces the deposit/commission bounds: nil is
// «нет записи», a value within 0..10⁹ (решение №6).
func validateOptionalAmount(amount *int64) error {
	if amount == nil {
		return nil
	}
	if *amount < 0 || *amount > MaxAmountKopecks {
		return ErrInvalidInput
	}
	return nil
}

// applyRentalUpdate folds the command's diff into the loaded rental; only
// provided fields change, each validated as at creation by the caller's pass.
func applyRentalUpdate(rental *domain.Rental, cmd UpdateRentalCommand) {
	if cmd.PlannedEndDate != nil {
		rental.PlannedEndDate = cmd.PlannedEndDate.Value
	}
	if cmd.Utilities != nil {
		rental.Utilities = *cmd.Utilities
	}
	if cmd.DepositKopecks != nil {
		rental.DepositKopecks = cmd.DepositKopecks.Value
	}
	if cmd.CommissionKopecks != nil {
		rental.CommissionKopecks = cmd.CommissionKopecks.Value
	}
	if cmd.ContactID != nil {
		rental.ContactID = cmd.ContactID.Value
	}
	if cmd.Comment != nil {
		if cmd.Comment.Value == nil {
			rental.Comment = ""
		} else {
			rental.Comment = strings.TrimSpace(*cmd.Comment.Value)
		}
	}
}

// validateRentalRow re-validates the rental after the fold: the same
// invariants as at creation, minus the payment-derived ones.
func validateRentalRow(rental domain.Rental) error {
	if !rental.Utilities.Valid() {
		return ErrInvalidInput
	}
	if rental.PlannedEndDate != nil && !rental.PlannedEndDate.After(rental.StartDate) {
		return ErrInvalidInput
	}
	if err := validateOptionalAmount(rental.DepositKopecks); err != nil {
		return err
	}
	if err := validateOptionalAmount(rental.CommissionKopecks); err != nil {
		return err
	}
	if utf8.RuneCountInString(rental.Comment) > MaxCommentLength {
		return ErrInvalidInput
	}
	return nil
}

// updatedRentalFields lists the field names a command changes; the audit
// context carries names only, never values (the comment is user data).
func updatedRentalFields(cmd UpdateRentalCommand) []string {
	fields := make([]string, 0, 9)
	if cmd.AmountKopecks != nil {
		fields = append(fields, "amount_kopecks")
	}
	if cmd.PaymentDay != nil {
		fields = append(fields, "payment_day")
	}
	if cmd.AutoPay != nil {
		fields = append(fields, "auto_pay")
	}
	if cmd.PlannedEndDate != nil {
		fields = append(fields, "planned_end_date")
	}
	if cmd.Utilities != nil {
		fields = append(fields, "utilities")
	}
	if cmd.DepositKopecks != nil {
		fields = append(fields, "deposit_kopecks")
	}
	if cmd.CommissionKopecks != nil {
		fields = append(fields, "commission_kopecks")
	}
	if cmd.ContactID != nil {
		fields = append(fields, "contact_id")
	}
	if cmd.Comment != nil {
		fields = append(fields, "comment")
	}
	return fields
}
