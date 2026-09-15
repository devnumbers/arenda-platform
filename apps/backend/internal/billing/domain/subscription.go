// Package domain defines the billing domain model: tariffs, subscriptions and the transition log, subscription
// payments, payment methods and card binding sessions.
package domain

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SubscriptionSource distinguishes owner-paid subscriptions from service
// subscriptions assigned by an admin without payment.
type SubscriptionSource string

const (
	SubscriptionSourcePaid    SubscriptionSource = "paid"
	SubscriptionSourceService SubscriptionSource = "service"
)

// SubscriptionStatus is the ADR 0008 lifecycle state of a subscription.
type SubscriptionStatus string

const (
	SubscriptionStatusActive    SubscriptionStatus = "active"
	SubscriptionStatusGrace     SubscriptionStatus = "grace"
	SubscriptionStatusCancelled SubscriptionStatus = "cancelled"
)

// SubscriptionPeriod is the billing period a subscription is paid for.
type SubscriptionPeriod string

const (
	PeriodMonth SubscriptionPeriod = "month"
	PeriodYear  SubscriptionPeriod = "year"
)

// ParseSubscriptionPeriod validates and converts a string to SubscriptionPeriod.
func ParseSubscriptionPeriod(s string) (SubscriptionPeriod, error) {
	switch SubscriptionPeriod(s) {
	case PeriodMonth, PeriodYear:
		return SubscriptionPeriod(s), nil
	}
	return "", ErrInvalidPeriod
}

type Subscription struct {
	ID                    uuid.UUID
	UserID                uuid.UUID
	TariffID              uuid.UUID
	Source                SubscriptionSource
	Status                SubscriptionStatus
	ValidUntil            *time.Time
	AutoRenewEnabled      bool
	PendingTariffID       *uuid.UUID
	PendingChangeAt       *time.Time
	PendingPeriod         *SubscriptionPeriod
	ActivePaymentMethodID *uuid.UUID
	LastAppliedPaymentID  *uuid.UUID
	// CurrentPeriod is the billing period the subscription is currently paid up
	// for. It is set when a tariff change, renewal or scheduled downgrade is
	// applied and cleared on downgrade to basic, so renewal resolution does not
	// depend on the last succeeded payment.
	CurrentPeriod *SubscriptionPeriod
	// GraceRemindedAt records when the grace-expiry reminder worker last
	// reminded this grace window (issue #253). Nil means the current window
	// has not been reminded yet; entering a new grace window resets it.
	GraceRemindedAt *time.Time
	// GraceArchivedPropertyIDs is the snapshot of the properties billing
	// archived when this subscription entered its grace window (grace v2,
	// ADR 0055): grace keeps one active property, and the owner is owed the
	// restoration of these ids on the next successful payment. The order is
	// the restoration priority — the newest archived property first. The debt
	// is settled by the payment that restores them (the application seam
	// clears the field) and dropped when the subscription falls to basic or
	// is overwritten by a service assignment.
	GraceArchivedPropertyIDs []uuid.UUID
	// KeepPropertyID is the property the owner chose to keep when cancelling
	// (issue #617): when the cancelled subscription later expires and the
	// worker applies the basic limit, this property survives and the excess
	// ones are archived. Set by Cancel, consumed by the first lifecycle move
	// that makes it moot (the fall to basic, a resume, a reactivation, a
	// service overwrite).
	KeepPropertyID *uuid.UUID
}

// SetGraceArchive records the ids the grace entry archived, replacing any
// previous snapshot: a fresh grace window owes a fresh restoration.
func (s *Subscription) SetGraceArchive(ids []uuid.UUID) {
	s.GraceArchivedPropertyIDs = ids
}

// ClearGraceArchive drops the restoration debt: the snapshot ids were restored
// (or the subscription state they belonged to is gone), so a later payment
// must not restore them again.
func (s *Subscription) ClearGraceArchive() {
	s.GraceArchivedPropertyIDs = nil
}

// NewBasicSubscription creates the free basic subscription for a
// newly-registered owner (issue #245 onboarding). Source is set to "paid"
// because the owner is on the paid-subscription track, even though the initial
// basic tariff itself is free: active, no expiry, auto-renew off.
func NewBasicSubscription(userID, tariffID uuid.UUID) (Subscription, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Subscription{}, err
	}
	return Subscription{
		ID:               id,
		UserID:           userID,
		TariffID:         tariffID,
		Source:           SubscriptionSourcePaid,
		Status:           SubscriptionStatusActive,
		AutoRenewEnabled: false,
		ValidUntil:       nil,
	}, nil
}

// ReconstituteSubscription validates a Subscription assembled from persisted
// state and returns it. Persistence adapters build the aggregate from raw
// storage values (including unchecked enum casts) and pass it here so that
// unknown statuses/sources/periods or missing identity fields are
// rejected with a descriptive error instead of silently producing an invalid
// aggregate.
func ReconstituteSubscription(sub Subscription) (Subscription, error) {
	if sub.ID == uuid.Nil {
		return Subscription{}, errors.New("reconstitute subscription: missing id")
	}
	if sub.UserID == uuid.Nil {
		return Subscription{}, errors.New("reconstitute subscription: missing user id")
	}
	if sub.TariffID == uuid.Nil {
		return Subscription{}, errors.New("reconstitute subscription: missing tariff id")
	}
	switch sub.Status {
	case SubscriptionStatusActive, SubscriptionStatusGrace, SubscriptionStatusCancelled:
	default:
		return Subscription{}, fmt.Errorf("reconstitute subscription: unknown status %q", sub.Status)
	}
	switch sub.Source {
	case SubscriptionSourcePaid, SubscriptionSourceService:
	default:
		return Subscription{}, fmt.Errorf("reconstitute subscription: unknown source %q", sub.Source)
	}
	if sub.PendingPeriod != nil {
		switch *sub.PendingPeriod {
		case PeriodMonth, PeriodYear:
		default:
			return Subscription{}, fmt.Errorf("reconstitute subscription: unknown pending period %q", *sub.PendingPeriod)
		}
	}
	if sub.CurrentPeriod != nil {
		switch *sub.CurrentPeriod {
		case PeriodMonth, PeriodYear:
		default:
			return Subscription{}, fmt.Errorf("reconstitute subscription: unknown current period %q", *sub.CurrentPeriod)
		}
	}
	return sub, nil
}

// CanMutateData reports whether the subscription allows the user to mutate
// property and finance data at the given moment. Active subscriptions whose
// paid period has already expired are treated as non-mutable until the worker
// transitions them to grace or basic. Cancelled subscriptions remain mutable
// until the end of the already paid period, matching the behaviour of active
// subscriptions.
func (s *Subscription) CanMutateData(now time.Time) bool {
	if s.Status == SubscriptionStatusGrace {
		return s.IsInGrace(now)
	}
	if s.Status != SubscriptionStatusActive && s.Status != SubscriptionStatusCancelled {
		return false
	}
	if s.ValidUntil != nil && now.After(*s.ValidUntil) {
		return false
	}
	return true
}

// IsPaidSource reports whether the subscription is paid for by the owner.
func (s *Subscription) IsPaidSource() bool {
	return s.Source == SubscriptionSourcePaid
}

// CanInitiatePayment reports whether the subscription status allows a new
// payment to be started at the given moment. A cancelled subscription pays
// too: its restoration goes through paying for a tariff (ADR 0008, billing
// CONTEXT.md, issue #429). Only a grace window that has expired — the worker
// downgrade to basic is due — refuses to start one.
func (s *Subscription) CanInitiatePayment(now time.Time) bool {
	if s.Status == SubscriptionStatusActive || s.Status == SubscriptionStatusCancelled {
		return true
	}
	return s.IsInGrace(now)
}

// IsInGrace reports whether the subscription is currently within its grace period.
func (s *Subscription) IsInGrace(now time.Time) bool {
	if s.Status != SubscriptionStatusGrace || s.ValidUntil == nil {
		return false
	}
	return !now.After(*s.ValidUntil)
}

// HasPendingChange reports whether a tariff change is scheduled for the subscription.
func (s *Subscription) HasPendingChange() bool {
	return s.PendingTariffID != nil && s.PendingChangeAt != nil
}

// ScheduleDowngrade schedules a downgrade to take effect when the current paid
// period ends. It requires a valid_until date and enables auto-renew so the new
// tariff keeps renewing on the normal cycle after it is applied. The new tariff
// must be a downgrade from the current tariff.
func (s *Subscription) ScheduleDowngrade(currentTariff, newTariff Tariff, period SubscriptionPeriod, changeAt time.Time) error {
	if s.Status != SubscriptionStatusActive {
		return ErrInvalidSubscriptionState
	}
	if period != PeriodMonth && period != PeriodYear {
		return ErrInvalidPeriod
	}
	if s.TariffID != currentTariff.ID {
		return ErrInvalidTariffChange
	}
	if s.TariffID == newTariff.ID {
		return ErrAlreadyOnTariff
	}
	if s.ValidUntil == nil {
		return ErrInvalidTariffChange
	}
	if changeAt.Before(*s.ValidUntil) {
		return ErrInvalidTariffChange
	}
	if ClassifyTariffChange(currentTariff, newTariff) != TariffChangeDowngrade {
		return ErrInvalidTariffChange
	}
	s.PendingTariffID = &newTariff.ID
	s.PendingChangeAt = &changeAt
	s.PendingPeriod = &period
	s.AutoRenewEnabled = true
	return nil
}

// ApplyTariffChange applies a successful tariff change immediately. It sets the
// new tariff, extends validity by the chosen period, enables auto-renew,
// records the applied payment and clears any pending change. A paid tariff
// change on top of a service subscription converts it to the paid track
// (issue #255): the owner paid, so the subscription is theirs from the new
// period.
func (s *Subscription) ApplyTariffChange(
	paymentID uuid.UUID, currentTariff, newTariff Tariff, period SubscriptionPeriod, now time.Time,
) error {
	if period != PeriodMonth && period != PeriodYear {
		return ErrInvalidPeriod
	}
	if s.TariffID != currentTariff.ID {
		return ErrInvalidTariffChange
	}
	if s.TariffID == newTariff.ID {
		return ErrAlreadyOnTariff
	}
	if ClassifyTariffChange(currentTariff, newTariff) == TariffChangeSame {
		return ErrInvalidTariffChange
	}
	validUntil := addSubscriptionPeriod(now, period)
	s.TariffID = newTariff.ID
	s.Source = SubscriptionSourcePaid
	s.ValidUntil = &validUntil
	s.AutoRenewEnabled = true
	s.LastAppliedPaymentID = &paymentID
	s.CurrentPeriod = &period
	s.PendingTariffID = nil
	s.PendingChangeAt = nil
	s.PendingPeriod = nil
	s.KeepPropertyID = nil
	s.Status = SubscriptionStatusActive
	return nil
}

// ApplyRenewal extends the subscription validity by one period and records the
// applied payment. For an active subscription the extension stacks on the
// current valid_until when it exists and is in the future, so an early renewal
// keeps the paid remainder. Renewals in grace or after expiry start from now:
// grace is not paid time and must not be gifted. A renewal landing on a
// cancelled subscription is its reactivation (issue #429): the paid period
// starts from the payment moment — the cancelled remainder does not stack —
// the status returns to active and auto-renew switches back on, putting the
// subscription on the normal renewal cycle again. A successful renewal also
// clears any scheduled downgrade. Like every applied payment, it puts the
// subscription on the paid track (issue #255).
func (s *Subscription) ApplyRenewal(paymentID uuid.UUID, period SubscriptionPeriod, now time.Time) error {
	if period != PeriodMonth && period != PeriodYear {
		return ErrInvalidPeriod
	}
	base := now
	if s.Status == SubscriptionStatusActive && s.ValidUntil != nil && s.ValidUntil.After(base) {
		base = *s.ValidUntil
	}
	validUntil := addSubscriptionPeriod(base, period)
	s.ValidUntil = &validUntil
	s.Source = SubscriptionSourcePaid
	if s.Status == SubscriptionStatusCancelled {
		// Reactivation (#429): the payment puts the subscription back on
		// the normal renewal cycle.
		s.AutoRenewEnabled = true
	}
	s.LastAppliedPaymentID = &paymentID
	s.CurrentPeriod = &period
	s.Status = SubscriptionStatusActive
	s.PendingTariffID = nil
	s.PendingChangeAt = nil
	s.PendingPeriod = nil
	// A reactivation payment (issue #429) supersedes the keep choice of the
	// cancelled state it pulls the subscription out of (issue #617).
	s.KeepPropertyID = nil
	return nil
}

// ApplyScheduledDowngrade applies a deferred downgrade at the end of the paid
// period. It requires the subscription to be active with a matching pending
// change that is already due: the pending tariff and period must equal the
// requested ones and pending_change_at must be set at or before now.
// Per ADR 0008 §3 this is the free path: a scheduled downgrade to a free
// target period applies with no charge — it switches the tariff, extends
// valid_until by the chosen period from now, enables auto-renew, sets status
// to active, and clears the pending change. A scheduled downgrade to a paid
// target period is charged at apply time via the renewal path and never
// reaches this method. LastAppliedPaymentID is intentionally left untouched
// because no payment is involved here.
func (s *Subscription) ApplyScheduledDowngrade(newTariff Tariff, period SubscriptionPeriod, now time.Time) error {
	if period != PeriodMonth && period != PeriodYear {
		return ErrInvalidPeriod
	}
	if s.TariffID == newTariff.ID {
		return ErrAlreadyOnTariff
	}
	if s.Status != SubscriptionStatusActive {
		return ErrInvalidSubscriptionState
	}
	if s.PendingTariffID == nil || *s.PendingTariffID != newTariff.ID {
		return ErrInvalidSubscriptionState
	}
	if s.PendingPeriod == nil || *s.PendingPeriod != period {
		return ErrInvalidSubscriptionState
	}
	if s.PendingChangeAt == nil || s.PendingChangeAt.After(now) {
		return ErrInvalidSubscriptionState
	}
	validUntil := addSubscriptionPeriod(now, period)
	s.TariffID = newTariff.ID
	s.ValidUntil = &validUntil
	s.AutoRenewEnabled = true
	s.Status = SubscriptionStatusActive
	s.CurrentPeriod = &period
	s.PendingTariffID = nil
	s.PendingChangeAt = nil
	s.PendingPeriod = nil
	return nil
}

// ClearPendingChange clears any scheduled tariff change pending on the
// subscription without modifying the current tariff, validity or status.
func (s *Subscription) ClearPendingChange() {
	s.PendingTariffID = nil
	s.PendingChangeAt = nil
	s.PendingPeriod = nil
}

// SetActivePaymentMethod records the payment method that future renewals
// should charge.
func (s *Subscription) SetActivePaymentMethod(id uuid.UUID) {
	s.ActivePaymentMethodID = &id
}

// MarkPaymentApplied records the payment whose effects are already reflected
// in the subscription state, so the same payment is never applied twice.
func (s *Subscription) MarkPaymentApplied(paymentID uuid.UUID) {
	s.LastAppliedPaymentID = &paymentID
}

// SetAutoRenew toggles automatic subscription renewal. Enabling auto-renew is
// only allowed when the subscription has a validity period and is not
// cancelled: a cancelled subscription never renews — restoration goes through
// paying for a tariff (ADR 0008, billing CONTEXT.md).
func (s *Subscription) SetAutoRenew(enabled bool) error {
	if enabled {
		if s.ValidUntil == nil {
			return ErrCannotEnableAutoRenew
		}
		if s.Status == SubscriptionStatusCancelled {
			return ErrInvalidSubscriptionState
		}
	}
	s.AutoRenewEnabled = enabled
	return nil
}

// Cancel terminates the paid subscription at the end of the already paid period.
// The validity date is retained so the worker can downgrade the subscription to
// basic once the period expires. Auto-renew is disabled immediately and any
// scheduled tariff change is dropped: a cancelled subscription no longer
// switches tariffs, it runs out its paid period and falls to basic. The
// optional keepPropertyID (issue #617) records which property the owner wants
// to survive that fall; the expiry worker honours it when applying the basic
// limit.
func (s *Subscription) Cancel(keepPropertyID *uuid.UUID) error {
	if s.Status != SubscriptionStatusActive && s.Status != SubscriptionStatusGrace {
		return ErrInvalidSubscriptionState
	}
	s.Status = SubscriptionStatusCancelled
	s.AutoRenewEnabled = false
	s.ClearPendingChange()
	s.KeepPropertyID = keepPropertyID
	return nil
}

// Resume undoes a cancellation without a charge (issue #617): a cancelled
// subscription inside its already paid period returns to active with
// auto-renew switched back on, and the paid remainder is kept as is. The free
// resume is the counterpart of the paid reactivation (ADR 0008: restoration
// goes through paying for a tariff) — that one stays for periods that have
// already expired. The keep choice of the undone cancellation is dropped.
func (s *Subscription) Resume(now time.Time) error {
	if s.Status != SubscriptionStatusCancelled {
		return ErrResumeNotAvailable
	}
	if s.ValidUntil == nil || now.After(*s.ValidUntil) {
		return ErrResumeNotAvailable
	}
	s.Status = SubscriptionStatusActive
	s.AutoRenewEnabled = true
	s.KeepPropertyID = nil
	return nil
}

// AssignService overwrites the subscription with an admin-assigned service
// subscription (issue #255, billing CONTEXT.md): the given tariff runs until
// validUntil without payment, auto-renew is off, and every planning field of
// the overwritten subscription is cleared — the paid remainder does not stack.
// The active payment method survives: it belongs to the owner and serves the
// paid track the upgrade on top of this assignment returns them to. At the end
// of the term the expiry worker downgrades the subscription to basic through
// the common expiry path.
func (s *Subscription) AssignService(tariffID uuid.UUID, validUntil time.Time) {
	s.TariffID = tariffID
	s.Source = SubscriptionSourceService
	s.Status = SubscriptionStatusActive
	s.ValidUntil = &validUntil
	s.AutoRenewEnabled = false
	s.PendingTariffID = nil
	s.PendingChangeAt = nil
	s.PendingPeriod = nil
	s.CurrentPeriod = nil
	s.GraceRemindedAt = nil
	// The overwrite discards the previous subscription's state, the grace
	// restoration debt with it (issue #255).
	s.ClearGraceArchive()
	s.KeepPropertyID = nil
}

// ForceApplyTariffChange switches the subscription to the given tariff
// immediately without payment (issue #255): the new tariff runs for one period
// from now, the status moves to active and any pending change is dropped. The
// source and the auto-renew setting keep their value — the operation repairs
// the tariff, it does not convert a paid subscription into a service one or
// vice versa. A cancelled subscription is out of scope: restoration goes
// through paying for a tariff (ADR 0008).
func (s *Subscription) ForceApplyTariffChange(newTariffID uuid.UUID, period SubscriptionPeriod, now time.Time) error {
	if period != PeriodMonth && period != PeriodYear {
		return ErrInvalidPeriod
	}
	if s.Status != SubscriptionStatusActive && s.Status != SubscriptionStatusGrace {
		return ErrInvalidSubscriptionState
	}
	if s.TariffID == newTariffID {
		return ErrAlreadyOnTariff
	}
	validUntil := addSubscriptionPeriod(now, period)
	s.TariffID = newTariffID
	s.ValidUntil = &validUntil
	s.Status = SubscriptionStatusActive
	s.CurrentPeriod = &period
	s.PendingTariffID = nil
	s.PendingChangeAt = nil
	s.PendingPeriod = nil
	s.KeepPropertyID = nil
	s.GraceRemindedAt = nil
	return nil
}

// ExtendGrace lengthens the current grace window by extra (issue #255): the
// new deadline counts from the later of now and the current deadline, so a
// window the expiry worker has not closed yet still yields the full extra
// time. The reminder flag resets: the extended window is a fresh one for the
// grace-expiry reminder (#253). Only a subscription in grace can be extended.
func (s *Subscription) ExtendGrace(now time.Time, extra time.Duration) error {
	if s.Status != SubscriptionStatusGrace || s.ValidUntil == nil {
		return ErrInvalidSubscriptionState
	}
	base := now
	if s.ValidUntil.After(base) {
		base = *s.ValidUntil
	}
	validUntil := base.Add(extra)
	s.ValidUntil = &validUntil
	s.GraceRemindedAt = nil
	return nil
}

// ShiftTime moves the subscription's own temporal boundaries by the signed
// delta (issue #665): valid_until — the paid-period end or the grace deadline,
// whatever the current status means by it — and any pending-change deadline
// travel by the same delta, so the pending-change CHECK
// (pending_change_at >= valid_until) survives. The reminder flag resets: the
// shifted window counts as a fresh one for the grace-expiry reminder (#253),
// the same rule ExtendGrace applies. Status, tariff, source and every
// non-temporal field stay untouched — the lifecycle phases pick the moved
// boundaries up on their next tick, unchanged. The caller owns the size limit
// and the enabling railguard; this method only refuses a zero delta and a
// subscription without a validity boundary (the free basic state has no time
// to travel). The dunning retry anchor and the payments' created_at are
// store-side boundaries the application shift moves alongside (issue #665) —
// they live outside the aggregate.
func (s *Subscription) ShiftTime(delta time.Duration) error {
	if delta == 0 {
		return ErrInvalidTimeShift
	}
	if s.ValidUntil == nil {
		return ErrInvalidTimeShift
	}
	validUntil := s.ValidUntil.Add(delta)
	s.ValidUntil = &validUntil
	if s.PendingChangeAt != nil {
		pendingAt := s.PendingChangeAt.Add(delta)
		s.PendingChangeAt = &pendingAt
	}
	s.GraceRemindedAt = nil
	return nil
}

// DowngradeToBasic resets the subscription to the free basic tariff. It clears
// any validity period, current period, pending change and auto-renewal state,
// and returns the subscription to the paid track: the free basic plan is the
// owner's default state (NewBasicSubscription), so an expired or withdrawn
// service subscription converges on it too (issue #255).
func (s *Subscription) DowngradeToBasic(basicTariffID uuid.UUID) {
	s.TariffID = basicTariffID
	s.Source = SubscriptionSourcePaid
	s.Status = SubscriptionStatusActive
	s.ValidUntil = nil
	s.AutoRenewEnabled = false
	s.PendingTariffID = nil
	s.PendingChangeAt = nil
	s.PendingPeriod = nil
	s.CurrentPeriod = nil
	// The fall to basic ends the grace window without payment: the
	// restoration debt is dropped, the grace archive stays archived
	// (grace v2, ADR 0055).
	s.ClearGraceArchive()
	// The fall to basic is where the cancel keep choice is consumed: the
	// expiry enforcement that follows keeps this very property alive
	// (issue #617).
	s.KeepPropertyID = nil
}

// FailedRenewalIsCurrent reports whether a failed renewal charge with the
// given id is still relevant to the subscription's current paid period: the
// period has ended (nothing is paid for beyond now — renewal charges exist
// only for expired periods, the renewal worker charges at expiry) and no
// payment created after the failed one has been applied since. A late failure
// of a superseded payment — the subscription was renewed or upgraded by a
// newer payment, or received a newer term while the charge hung at the
// provider — must not move it into grace (issue #426); the failure is
// recorded on the payment alone and the subscription keeps the period it was
// already granted.
func (s *Subscription) FailedRenewalIsCurrent(paymentID uuid.UUID, now time.Time) bool {
	if s.ValidUntil == nil || s.ValidUntil.After(now) {
		// Paid for beyond now (or nothing to renew at all): whatever this
		// charge was for, a later payment or term has already covered it.
		return false
	}
	if s.LastAppliedPaymentID == nil {
		return true
	}
	// Payment ids are app-side UUIDv7, so byte order is creation order: a
	// last applied payment ordered after the failed one was created later and
	// supersedes its outcome.
	return paymentIDBefore(*s.LastAppliedPaymentID, paymentID)
}

// SucceededPaymentIsCurrent reports whether a succeeded payment is still the
// newest payment the subscription's state reflects: no payment created after
// it has been applied since. A late success of a superseded payment — e.g. a
// pro charge landing after a newer business payment was applied — must not be
// applied as a downgrade (issue #428); the subscription keeps the tariff and
// period the newer payment bought, and the late success is recorded on the
// payment alone. With no payment applied yet there is nothing to supersede,
// so any success is current.
func (s *Subscription) SucceededPaymentIsCurrent(paymentID uuid.UUID) bool {
	if s.LastAppliedPaymentID == nil {
		return true
	}
	return !paymentIDBefore(paymentID, *s.LastAppliedPaymentID)
}

// RefundedPaymentBoughtCurrentPeriod reports whether the refunded payment is
// the payment the subscription's current paid state reflects — the last
// applied one. A refund of a superseded payment — the subscription was
// renewed or upgraded by a newer payment after it — returns the money but must
// not reset the tariff and period the newer payment bought (issue #430); only
// the payment the current period was bought with carries the subscription
// effects of a refund. With no payment applied on record there is nothing to
// supersede, so the refund keeps its effects.
func (s *Subscription) RefundedPaymentBoughtCurrentPeriod(paymentID uuid.UUID) bool {
	if s.LastAppliedPaymentID == nil {
		return true
	}
	return *s.LastAppliedPaymentID == paymentID
}

// paymentIDBefore reports whether payment id a was created before payment id
// b. Repository ids are UUIDv7 — the leading bytes carry the generation
// timestamp — so byte comparison is creation-time comparison.
func paymentIDBefore(a, b uuid.UUID) bool {
	return bytes.Compare(a[:], b[:]) < 0
}

// EnterGrace moves the subscription into the grace period. The validity date is
// extended to now+grace unless the subscription is already paid up beyond that
// point, so an already-paid period is never shortened. Any pending scheduled
// tariff change is dropped: it is considered consumed by the failed charge that
// triggered grace, and re-scheduling after renewal is a fresh user action.
// Clearing pending_change_at also keeps the
// user_subscriptions_pending_change_at_check constraint satisfied
// (pending_change_at must be NULL or not earlier than valid_until).
// The grace duration is an operational parameter passed by the caller
// (application Config, ADR 0008 default: 7 days).
func (s *Subscription) EnterGrace(now time.Time, grace time.Duration) {
	s.Status = SubscriptionStatusGrace
	graceUntil := now.Add(grace)
	if s.ValidUntil == nil || graceUntil.After(*s.ValidUntil) {
		s.ValidUntil = &graceUntil
	}
	s.ClearPendingChange()
	// A fresh window starts unreminded: the expiry reminder is due again even
	// when a previous window was already reminded (issue #253).
	s.GraceRemindedAt = nil
}

// MarkGraceReminded records that the grace-expiry reminder of the current
// grace window was dispatched, so the reminder worker does not repeat it every
// tick (issue #253).
func (s *Subscription) MarkGraceReminded(now time.Time) {
	s.GraceRemindedAt = &now
}

// addSubscriptionPeriod returns the time one subscription period after start.
func addSubscriptionPeriod(start time.Time, period SubscriptionPeriod) time.Time {
	if period == PeriodYear {
		return start.AddDate(1, 0, 0)
	}
	return start.AddDate(0, 1, 0)
}
