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
	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	realtimeapp "github.com/nambers/arenda-planform/apps/backend/internal/realtime/application"
	realtimedom "github.com/nambers/arenda-planform/apps/backend/internal/realtime/domain"
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
// tenant, comment, reminder offset — are tri-state (явный null = очистить).
type UpdateRentalCommand struct {
	AmountKopecks      *int64
	PaymentDay         *domain.PaymentDay
	AutoPay            *bool
	PlannedEndDate     *DateUpdate
	ReminderOffsetDays *ReminderOffsetUpdate
	Utilities          *domain.Utilities
	DepositKopecks     *AmountUpdate
	CommissionKopecks  *AmountUpdate
	ContactID          *ContactUpdate
	Comment            *StringUpdate
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
// change step, its audit entry and its action journal row (ADR 0061) in the
// same transaction, and the payments tick when the managed payment changed.
// Reads never tick and never write.
type RentalService struct {
	txStoreFactory
	policy    sharedpolicy.Policy
	calendar  paymentsapp.OwnerCalendar
	viewGate  gateFunc // Viewer reads.
	writeGate gateFunc // Full Access+: create/edit/complete.
	delGate   gateFunc // Owner alone: deletion.
	// Realtime is the late-bound carrier the mutations' frames dispatch
	// through after the commit (карта #714, #716; ADR 0062); nil keeps the
	// pre-#716 silence.
	realtime realtimeapp.Publisher
}

// SetRealtimePublisher late-binds the realtime carrier (карта #714, #716;
// ADR 0062): the frames of the committed mutations dispatch through it —
// the grace-events canon, best-effort, a broken carrier never fails the
// mutation.
func (s *RentalService) SetRealtimePublisher(p realtimeapp.Publisher) {
	s.realtime = p
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
	return mutationGates{factory: s.txStoreFactory, calendar: s.calendar, realtime: s.realtime}
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
			tenantName, err := s.validatedTenantName(ctx, scope, cmd.ContactID)
			if err != nil {
				return mutationOutcome{}, err
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
				NotifyAutoPaid:     true,
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
				PaymentID:         &paymentID,
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
				History:  new(historydomain.RentalCreated(rentalID, tenantName, cmd.StartDate, derefDate(cmd.PlannedEndDate))),
				Changed: []realtimedom.Change{
					realtimedom.On(realtimedom.EntityRentals, propertyID),
					realtimedom.On(realtimedom.EntityPayments, propertyID),   // The managed payment is born with the rental.
					realtimedom.On(realtimedom.EntityOperations, propertyID), // The tick materializes the first planned in the same transaction.
				},
				Tick: true, // The new payment materializes its first planned.
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
// ordering). Reads never tick. The progress counters of the whole list ride
// one batched read (ticket #845) — both counters of every rental's payment
// in one SQL where the per-rental pair took two per row.
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
	paymentIDs := make([]uuid.UUID, 0, len(rentals))
	for _, r := range rentals {
		// A completed rental's payment is deleted (ревизия #1161) — the nil
		// link contributes no id; its view assembles from the terms archive.
		if r.PaymentID != nil {
			paymentIDs = append(paymentIDs, *r.PaymentID)
		}
	}
	// A payment with no operations is absent from the batch — the lookup
	// below defaults its zeros; the empty list never reaches the query.
	var counts map[uuid.UUID]ProgressCounts
	if len(paymentIDs) > 0 {
		counts, err = s.gateway.CountProgressByPayments(ctx, scope, propertyID, paymentIDs, today)
		if err != nil {
			return nil, fmt.Errorf("count progress by payments: %w", err)
		}
	}
	views := make([]RentalView, 0, len(rentals))
	for _, r := range rentals {
		var paid ProgressCounts
		if r.PaymentID != nil {
			paid = counts[*r.PaymentID]
		}
		view, err := s.assembleView(ctx, scope, propertyID, r, today, paid)
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
			return s.updatedRentalOutcome(ctx, stores, scope, propertyID, rental, today, cmd, actor)
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

// updatedRentalOutcome is the change step of the terms edit: it validates the
// patch against the loaded rental, syncs the editable payment terms and
// returns the outcome the conveyor journals, journals history from and ticks
// on.
func (s *RentalService) updatedRentalOutcome(
	ctx context.Context, stores *txStores, scope, propertyID uuid.UUID, rental domain.Rental, today time.Time,
	cmd UpdateRentalCommand, actor uuid.UUID,
) (mutationOutcome, error) {
	if rental.CompletedDate != nil {
		return mutationOutcome{}, ErrRentalCompleted
	}
	change, paymentChanged, err := paymentSyncFromUpdate(cmd, rental.StartDate, today)
	if err != nil {
		return mutationOutcome{}, err
	}
	if err := s.validateTermsWindow(ctx, stores, scope, propertyID, cmd, rental); err != nil {
		return mutationOutcome{}, err
	}
	applyRentalUpdate(&rental, cmd)
	if err := validateRentalRow(rental); err != nil {
		return mutationOutcome{}, err
	}
	// The journal label is resolved once: a set contact carries the name
	// its validation already read, an unchanged one reads the stored card.
	var tenantName string
	if cmd.ContactID != nil && cmd.ContactID.Value != nil {
		if tenantName, err = s.validatedTenantName(ctx, scope, cmd.ContactID.Value); err != nil {
			return mutationOutcome{}, err
		}
	}
	if paymentChanged {
		if err := stores.pay.Update(ctx, scope, propertyID, *rental.PaymentID, actor, change, today); err != nil {
			return mutationOutcome{}, fmt.Errorf("sync rent payment: %w", err)
		}
	}
	if err := stores.rentals.Update(ctx, rental); err != nil {
		return mutationOutcome{}, fmt.Errorf("update rental: %w", err)
	}
	if cmd.ContactID == nil || cmd.ContactID.Value == nil {
		if tenantName, err = s.tenantName(ctx, scope, rental.ContactID); err != nil {
			return mutationOutcome{}, err
		}
	}
	changed := []realtimedom.Change{
		realtimedom.On(realtimedom.EntityRentals, propertyID),
		realtimedom.On(realtimedom.EntityPayments, propertyID), // The managed payment follows the rental.
	}
	if paymentChanged {
		// The tick re-stands the planned operations in the same transaction.
		changed = append(changed, realtimedom.On(realtimedom.EntityOperations, propertyID))
	}
	return mutationOutcome{
		RentalID: rental.ID,
		Audit:    auditActionRentalUpdated,
		AuditCtx: map[string]any{"fields": updatedRentalFields(cmd)},
		History:  new(historydomain.RentalUpdated(rental.ID, tenantName, rental.StartDate, derefDate(rental.PlannedEndDate))),
		Changed:  changed,
		Tick:     paymentChanged,
	}, nil
}

// CompleteRental records the completion (решение №8, ревизия ADR 0053
// #1161): the factual date within [start, today], the optional deposit
// return, and the managed payment deleted in the same transaction with the
// keep_overdue semantics — the planned from the owner's today goes, the
// overdue survives the payment as the property's debt, the paid operations
// stay in the property's history. The payment's terms are archived on the
// rental (the terms archive) and the payment link is released before the
// payment row goes (the RESTRICT FK). The change step resolves the payment's
// plan itself; no tick runs. A second completion is ErrRentalCompleted (the
// 409). The compound mutation journals both rows: rental.completed and
// payment.deleted.
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
			if rental.PaymentID == nil {
				// The schema CHECK keeps a payment link on every unfinished
				// rental; reaching this line means the durable invariant and
				// the app disagree — refuse rather than dereference.
				return mutationOutcome{}, errors.New("complete rental: unfinished rental without managed payment")
			}
			if err := validateCompletedDate(cmd.CompletedDate, rental.StartDate, today); err != nil {
				return mutationOutcome{}, err
			}
			if err := validateDepositReturn(cmd.DepositReturn); err != nil {
				return mutationOutcome{}, err
			}
			paymentID := *rental.PaymentID
			terms, err := stores.pay.State(ctx, scope, propertyID, paymentID)
			if err != nil {
				return mutationOutcome{}, fmt.Errorf("read rent payment terms: %w", err)
			}
			if err := stores.rentals.Complete(ctx, rental.ID, scope, cmd.CompletedDate,
				cmd.DepositReturn, domain.TermsArchive{
					AmountKopecks: terms.AmountKopecks,
					PaymentDay:    terms.PaymentDay,
					AutoPay:       terms.AutoPay,
				}); err != nil {
				return mutationOutcome{}, fmt.Errorf("complete rental: %w", err)
			}
			if err := stores.pay.Delete(ctx, scope, paymentID, today); err != nil {
				return mutationOutcome{}, fmt.Errorf("delete rent payment: %w", err)
			}
			tenantName, err := s.tenantName(ctx, scope, rental.ContactID)
			if err != nil {
				return mutationOutcome{}, err
			}
			out := mutationOutcome{
				RentalID: rental.ID,
				Audit:    auditActionRentalCompleted,
				History:  new(historydomain.RentalCompleted(rental.ID, tenantName)),
				// The compound mutation's payment half (ревизия #1161): the
				// same keep_overdue verdict the payments deletion audits.
				ExtraAudit: []auditdomain.Entry{{
					Action:     auditdomain.ActionPaymentDeleted,
					EntityType: auditdomain.EntityPayment,
					EntityID:   &paymentID,
					Context:    map[string]any{"keep_overdue": true},
				}},
				ExtraHistory: []*historydomain.Entry{
					new(historydomain.PaymentDeleted(paymentID, terms.Title)),
				},
				Changed: []realtimedom.Change{
					realtimedom.On(realtimedom.EntityRentals, propertyID),
					realtimedom.On(realtimedom.EntityPayments, propertyID), // The managed payment died with the completion.
					realtimedom.On(realtimedom.EntityOperations, propertyID),
				},
				Tick: false, // The change step resolved the plan itself.
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

// DeleteRental hard-deletes the rental (решение №10, ADR 0053 §3; уточнено
// ревизией #1161) — only for a completed rental or a not-yet-started one
// (start_date > today: передумал до старта, все planned будущие и сносятся
// чисто). A started unfinished rental can only go through the completion
// (ErrRentalStarted, the 409). The not-yet-started rental's managed payment
// goes with it: the rentals row goes first (the RESTRICT FK releases the
// payment only afterwards), the overdue planned stays as debt (the
// keep_overdue=true semantics). A completed rental's payment died with the
// completion — the link is nil and only the row goes. Deletion is Owner-only.
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
			// The completed rental's payment died with the completion
			// (ревизия #1161) — the link is nil and there is nothing left to
			// delete; the not-yet-started rental's payment goes with it (the
			// rentals row has already released the RESTRICT FK).
			if rental.PaymentID != nil {
				if err := stores.pay.Delete(ctx, scope, *rental.PaymentID, today); err != nil {
					return mutationOutcome{}, fmt.Errorf("delete rent payment: %w", err)
				}
			}
			tenantName, err := s.tenantName(ctx, scope, rental.ContactID)
			if err != nil {
				return mutationOutcome{}, err
			}
			return mutationOutcome{
				RentalID: rental.ID,
				Audit:    auditActionRentalDeleted,
				History:  new(historydomain.RentalDeleted(rental.ID, tenantName)),
				Changed: []realtimedom.Change{
					realtimedom.On(realtimedom.EntityRentals, propertyID),
					realtimedom.On(realtimedom.EntityPayments, propertyID), // The managed payment dies with the rental.
				},
				Tick: false,
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
// The single view reads its progress counters per-rental — the batched
// read (#845) carries only the list's. A completed rental without a payment
// (ревизия #1161) skips the payment-scoped counters: its paid months died
// with the payment, its overdue is nobody's red line — the zeros pass.
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
	counts := ProgressCounts{}
	if rental.PaymentID != nil {
		paid, err := s.gateway.CountPaidOperations(ctx, scope, propertyID, *rental.PaymentID)
		if err != nil {
			return RentalView{}, fmt.Errorf("count paid operations: %w", err)
		}
		overdue, err := s.gateway.CountOverdueOccurrences(ctx, scope, propertyID, *rental.PaymentID, today)
		if err != nil {
			return RentalView{}, fmt.Errorf("count overdue occurrences: %w", err)
		}
		counts = ProgressCounts{PaidCount: paid, OverdueCount: overdue}
	}
	return s.assembleView(ctx, scope, propertyID, rental, today, counts)
}

// assembleView computes everything the TZ-blind client renders (ADR 0053
// §2): the status, the payment's render state, the single future planned
// operation, the «N из M» progress. The state and next-planned reads stay
// per-rental; the progress counters arrive pre-read — the list carries them
// in one batched GROUP BY over its payments (ticket #845), the single view
// in its own per-rental pair. A completed rental without a payment
// (ревизия #1161) assembles from the terms archive: the render state and
// the term math read the archived terms, there is no next payment and
// PaymentID stays zero — the wire's null.
func (s *RentalService) assembleView(
	ctx context.Context, scope, propertyID uuid.UUID, rental domain.Rental, today time.Time,
	counts ProgressCounts,
) (RentalView, error) {
	var state RentPaymentState
	if rental.PaymentID != nil {
		var err error
		state, err = s.gateway.RentPaymentState(ctx, scope, propertyID, *rental.PaymentID)
		if err != nil {
			return RentalView{}, fmt.Errorf("rent payment state: %w", err)
		}
		// Идентификатор управляемого платежа знает аренда (связь 1:1) —
		// состояние рендера его только носит.
		state.PaymentID = *rental.PaymentID
	} else {
		// Завершённая без платежа: условия — только Архив (схема держит его
		// рядом с фактом завершения); платёжного идентификатора нет.
		if rental.TermsArchive == nil {
			return RentalView{}, errors.New("rent payment state: completed rental without terms archive")
		}
		state = RentPaymentState{
			AmountKopecks: rental.TermsArchive.AmountKopecks,
			PaymentDay:    rental.TermsArchive.PaymentDay,
			AutoPay:       rental.TermsArchive.AutoPay,
		}
	}
	var next *PlannedOccurrence
	if rental.PaymentID != nil {
		var err error
		next, err = s.gateway.NextPlannedOccurrence(ctx, scope, propertyID, *rental.PaymentID, today)
		if err != nil {
			return RentalView{}, fmt.Errorf("next planned occurrence: %w", err)
		}
	}
	// Серверная просрочка (#817, ADR 0053 §2 — всё считает сервер): счётчик
	// planned-вхождений раньше «сегодня» собственника; ноль — null.
	progress := RentalProgress{PaidMonths: counts.PaidCount, OverdueMonths: nilIfZero(counts.OverdueCount)}
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
	// The schedule window (ticket #1154): the planned end never stands
	// before the rent schedule's first payment-day occurrence — such a term
	// holds zero payments.
	if cmd.PlannedEndDate != nil {
		if err := validatePlannedEndWindow(*cmd.PlannedEndDate, cmd.StartDate, cmd.PaymentDay); err != nil {
			return err
		}
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
	if cmd.ReminderOffsetDays != nil {
		// The reminder contract (1/3/7, nil = «Не напоминать») is the
		// payments' vocabulary; the clear leg travels as the nil Value
		// (#1208, как PaymentUpdateRequest.reminderOffsetDays).
		if !paymentsapp.IsValidReminderOffset(cmd.ReminderOffsetDays.Value) {
			return change, false, ErrInvalidInput
		}
		change.ReminderOffsetDays = cmd.ReminderOffsetDays
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

// validatePlannedEndWindow enforces the schedule window (ticket #1154): the
// planned end never stands before the rent schedule's first payment-day
// occurrence. The create guard validates the command's own pair through it;
// the edit guard validates the merged (effective day, merged end) pair.
func validatePlannedEndWindow(plannedEnd, start time.Time, day domain.PaymentDay) error {
	if plannedEnd.Before(day.FirstPaymentDate(start)) {
		return ErrInvalidInput
	}
	return nil
}

// validateTermsWindow guards the terms edit's schedule window (ticket
// #1154): the merged pair of the effective payment day and the merged
// planned end is validated as a whole, so a day change re-checks the
// standing end the same way an end change re-checks against the effective
// day. The trigger is narrow — only a command touching the day or setting an
// end date runs it: a stored pair with a hole (a legacy row created before
// the fix) must not block unrelated edits. The stored day rides one
// in-transaction State read, only when the command changes the end without
// carrying the day; the schema CHECK keeps a payment link on every
// unfinished rental, so the read's payment id is safe to dereference.
func (s *RentalService) validateTermsWindow(
	ctx context.Context, stores *txStores, scope, propertyID uuid.UUID,
	cmd UpdateRentalCommand, rental domain.Rental,
) error {
	setsEnd := cmd.PlannedEndDate != nil && cmd.PlannedEndDate.Value != nil
	if cmd.PaymentDay == nil && !setsEnd {
		return nil
	}
	end := rental.PlannedEndDate
	if cmd.PlannedEndDate != nil {
		end = cmd.PlannedEndDate.Value
	}
	if end == nil {
		return nil // Open-ended: nothing to cut short.
	}
	var day domain.PaymentDay
	if cmd.PaymentDay != nil {
		day = *cmd.PaymentDay
	} else {
		terms, err := stores.pay.State(ctx, scope, propertyID, *rental.PaymentID)
		if err != nil {
			return fmt.Errorf("read rent payment terms: %w", err)
		}
		day = terms.PaymentDay
	}
	return validatePlannedEndWindow(*end, rental.StartDate, day)
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
	fields := make([]string, 0, 10)
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
	if cmd.ReminderOffsetDays != nil {
		fields = append(fields, "reminder_offset_days")
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

// tenantName resolves the tenant card's display name for the action journal
// row (ADR 0061 §3); a rental without a tenant card yields an empty label.
// The read is loud: a failing card read is a database trouble, not a missing
// label. The card belongs to the stored rental — its ownership has been
// validated at write time, so the boolean half of the port is not consulted.
func (s *RentalService) tenantName(ctx context.Context, scope uuid.UUID, contactID *uuid.UUID) (string, error) {
	if contactID == nil || s.tenants == nil {
		return "", nil
	}
	name, _, err := s.tenants.ValidatedTenantLabel(ctx, scope, *contactID)
	if err != nil {
		return "", fmt.Errorf("resolve tenant name: %w", err)
	}
	return name, nil
}

// validatedTenantName checks a create/update contact reference and resolves
// its display name for the action journal row: an unknown or foreign card is
// ErrInvalidInput, a nil contact is a tenant-less rental. The card is read
// once — the owner check and the label come from the same port call. A
// missing reader is a wiring mistake only when a command actually names a
// contact.
func (s *RentalService) validatedTenantName(
	ctx context.Context, scope uuid.UUID, contactID *uuid.UUID,
) (string, error) {
	if contactID == nil {
		return "", nil
	}
	if s.tenants == nil {
		return "", errors.New("rentals: tenant reader must be configured to name a contact")
	}
	label, ok, err := s.tenants.ValidatedTenantLabel(ctx, scope, *contactID)
	if err != nil {
		return "", fmt.Errorf("check tenant contact: %w", err)
	}
	if !ok {
		return "", ErrInvalidInput
	}
	return label, nil
}

// derefDate materializes an optional date for the row-text builders: a nil
// becomes the zero time the builders omit.
func derefDate(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}
