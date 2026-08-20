package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// Unit tests of the applyTransition module (issue #282): the single seam every
// subscription state change goes through. The module owns the from-side
// capture, the aggregate update, the transition-log entry and the audit
// record, so the invariant "no subscription state change without a
// transition-log entry" holds by construction. The scenarios mirror the
// dimensions of the ticket: a status change, a tariff change, both at once,
// and the three initiators (user, admin, system).

// transitionHarness runs applyTransition over the in-memory fakes inside a
// real factory transaction.
type transitionHarness struct {
	stores  *fakeStores
	audit   *captureRecorder
	now     time.Time
	tariffs []domain.Tariff
}

func newTransitionHarness(t *testing.T) *transitionHarness {
	t.Helper()
	tariffs := testTariffs()
	return &transitionHarness{
		stores:  newFakeStores(tariffs...),
		audit:   &captureRecorder{},
		now:     time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC),
		tariffs: tariffs,
	}
}

func (h *transitionHarness) tariffID(t *testing.T, name domain.TariffName) uuid.UUID {
	t.Helper()
	return h.tariffByName(t, name).ID
}

func (h *transitionHarness) tariffByName(t *testing.T, name domain.TariffName) domain.Tariff {
	t.Helper()
	for _, tariff := range h.tariffs {
		if tariff.Name == name {
			return tariff
		}
	}
	t.Fatalf("harness tariffs contain no %q plan", name)
	return domain.Tariff{}
}

// seedSubscription stores an active paid month on the given tariff, shaped by
// mutate, and returns the stored aggregate.
func (h *transitionHarness) seedSubscription(t *testing.T, name domain.TariffName, mutate func(*domain.Subscription)) domain.Subscription {
	t.Helper()
	sub, err := domain.NewBasicSubscription(uuid.Must(uuid.NewV7()), h.tariffID(t, name))
	if err != nil {
		t.Fatalf("NewBasicSubscription() error = %v", err)
	}
	validUntil := h.now.AddDate(0, 1, 0)
	period := domain.PeriodMonth
	sub.ValidUntil = &validUntil
	sub.AutoRenewEnabled = true
	sub.CurrentPeriod = &period
	if mutate != nil {
		mutate(&sub)
	}
	if _, err := h.stores.subscriptions.Create(t.Context(), sub); err != nil {
		t.Fatalf("seed Create() error = %v", err)
	}
	return sub
}

// apply runs one applyTransition call inside a factory transaction and
// returns the applied transition and the call's error.
func (h *transitionHarness) apply(
	t *testing.T, sub *domain.Subscription, mutate func(*domain.Subscription) error, spec transitionSpec,
) (domain.Transition, error) {
	t.Helper()
	var applied domain.Transition
	factory := h.stores.factory(h.audit)
	err := factory.runInTx(t.Context(), func(stores *txStores) error {
		var callErr error
		applied, callErr = stores.applyTransition(t.Context(), sub, mutate, spec)
		return callErr
	})
	if err != nil {
		return domain.Transition{}, err
	}
	return applied, nil
}

// storedSubscription loads the user's subscription from the fake repository.
func (h *transitionHarness) storedSubscription(t *testing.T, userID uuid.UUID) domain.Subscription {
	t.Helper()
	sub, err := h.stores.subscriptions.GetByUserID(t.Context(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	return sub
}

// appendedTransitions loads the subscription's transition log from the fake
// repository.
func (h *transitionHarness) appendedTransitions(t *testing.T, subscriptionID uuid.UUID) []domain.Transition {
	t.Helper()
	transitions, err := h.stores.transitions.ListBySubscriptionID(t.Context(), subscriptionID)
	if err != nil {
		t.Fatalf("ListBySubscriptionID() error = %v", err)
	}
	return transitions
}

// requireCancelledSubscription asserts the stored aggregate is cancelled with
// auto-renew off.
func (h *transitionHarness) requireCancelledSubscription(t *testing.T, userID uuid.UUID) {
	t.Helper()
	stored := h.storedSubscription(t, userID)
	if stored.Status != domain.SubscriptionStatusCancelled || stored.AutoRenewEnabled {
		t.Errorf("stored subscription = %q/auto-renew %t, want cancelled with auto-renew off", stored.Status, stored.AutoRenewEnabled)
	}
}

// requireCancelledTransitionEntry asserts the log holds exactly the returned
// transition and it captures the cancellation shape: the pre-change active
// from-side, an unchanged tariff, and the cancelled reason by the user
// initiator.
func (h *transitionHarness) requireCancelledTransitionEntry(
	t *testing.T, applied domain.Transition, sub domain.Subscription,
) {
	t.Helper()
	entries := h.appendedTransitions(t, sub.ID)
	if len(entries) != 1 {
		t.Fatalf("appended transitions = %d, want 1", len(entries))
	}
	if entries[0].ID != applied.ID {
		t.Errorf("returned transition id = %v, want the appended %v", applied.ID, entries[0].ID)
	}
	if applied.FromStatus == nil || *applied.FromStatus != domain.SubscriptionStatusActive {
		t.Errorf("from status = %v, want the pre-change active", applied.FromStatus)
	}
	if applied.ToStatus != domain.SubscriptionStatusCancelled {
		t.Errorf("to status = %q, want cancelled", applied.ToStatus)
	}
	if applied.FromTariffID == nil || *applied.FromTariffID != sub.TariffID || applied.ToTariffID != sub.TariffID {
		t.Errorf("tariff side = %v→%v, want unchanged %v", applied.FromTariffID, applied.ToTariffID, sub.TariffID)
	}
	if applied.Reason != domain.TransitionReasonCancelled || applied.Initiator != domain.InitiatorUser {
		t.Errorf("reason/initiator = %q/%q, want cancelled/user", applied.Reason, applied.Initiator)
	}
}

// requireUserCancellationAudit asserts the audit entry attributes the
// cancellation to the owning user on the subscription entity.
func (h *transitionHarness) requireUserCancellationAudit(t *testing.T, userID, subscriptionID uuid.UUID) {
	t.Helper()
	records := h.audit.recorded()
	if len(records) != 1 {
		t.Fatalf("audit entries = %d, want 1", len(records))
	}
	entry := records[0]
	if entry.ActorRole != auditdomain.ActorRoleOwner || entry.ActorID == nil || *entry.ActorID != userID {
		t.Errorf("audit actor = %q/%v, want the owning user", entry.ActorRole, entry.ActorID)
	}
	if entry.Action != auditdomain.ActionSubscriptionCancelled || entry.EntityType != auditdomain.EntitySubscription {
		t.Errorf("audit action/entity = %q/%q, want subscription cancelled", entry.Action, entry.EntityType)
	}
	if entry.EntityID == nil || *entry.EntityID != subscriptionID {
		t.Errorf("audit entity id = %v, want the subscription", entry.EntityID)
	}
}

// TestApplyTransition_StatusChange proves the status-only shape through the
// user's cancellation: the aggregate is persisted, the log entry captures the
// from-side, and the audit entry attributes the change to the user.
func TestApplyTransition_StatusChange(t *testing.T) {
	h := newTransitionHarness(t)
	sub := h.seedSubscription(t, domain.TariffPro, nil)
	userID := sub.UserID

	applied, err := h.apply(t, &sub,
		func(s *domain.Subscription) error { return s.Cancel() },
		transitionSpec{
			reason:      domain.TransitionReasonCancelled,
			initiator:   domain.InitiatorUser,
			initiatorID: &userID,
			auditAction: auditdomain.ActionSubscriptionCancelled,
		},
	)
	if err != nil {
		t.Fatalf("applyTransition() error = %v", err)
	}

	h.requireCancelledSubscription(t, userID)
	h.requireCancelledTransitionEntry(t, applied, sub)
	h.requireUserCancellationAudit(t, userID, sub.ID)
}

// TestApplyTransition_TariffChange proves the tariff-only shape through the
// worker's expiry downgrade: the status stays, the tariff moves to basic, and
// no audit entry appears — worker phases audit their payments, not the
// subscription change.
func TestApplyTransition_TariffChange(t *testing.T) {
	h := newTransitionHarness(t)
	sub := h.seedSubscription(t, domain.TariffPro, nil)
	basicID := h.tariffID(t, domain.TariffBasic)

	applied, err := h.apply(t, &sub,
		func(s *domain.Subscription) error { s.DowngradeToBasic(basicID); return nil },
		transitionSpec{
			reason:    domain.TransitionReasonExpired,
			initiator: domain.InitiatorSystem,
		},
	)
	if err != nil {
		t.Fatalf("applyTransition() error = %v", err)
	}

	stored := h.storedSubscription(t, sub.UserID)
	if stored.Status != domain.SubscriptionStatusActive || stored.TariffID != basicID || stored.ValidUntil != nil {
		t.Errorf("stored subscription = %q/%v, want active on basic with no validity", stored.Status, stored.TariffID)
	}

	if applied.FromStatus == nil || *applied.FromStatus != domain.SubscriptionStatusActive ||
		applied.ToStatus != domain.SubscriptionStatusActive {
		t.Errorf("status side = %v→%q, want active unchanged", applied.FromStatus, applied.ToStatus)
	}
	if applied.FromTariffID == nil || *applied.FromTariffID != h.tariffID(t, domain.TariffPro) || applied.ToTariffID != basicID {
		t.Errorf("tariff side = %v→%v, want pro→basic", applied.FromTariffID, applied.ToTariffID)
	}
	if applied.Initiator != domain.InitiatorSystem || applied.InitiatorID != nil {
		t.Errorf("initiator = %q/%v, want system without an actor id", applied.Initiator, applied.InitiatorID)
	}
	if entries := h.audit.recorded(); len(entries) != 0 {
		t.Errorf("audit entries = %d, want none for a system transition without an audit action", len(entries))
	}
}

// TestApplyTransition_StatusAndTariffChange proves the both-at-once shape
// through the admin's service assignment: a grace subscription on pro is
// overwritten by an active service subscription on business, with the audit
// context built from the post-change subscription.
func TestApplyTransition_StatusAndTariffChange(t *testing.T) {
	h := newTransitionHarness(t)
	sub := h.seedSubscription(t, domain.TariffPro, func(s *domain.Subscription) {
		s.Status = domain.SubscriptionStatusGrace
	})
	businessID := h.tariffID(t, domain.TariffBusiness)
	adminID := uuid.Must(uuid.NewV7())
	until := h.now.AddDate(0, 1, 0)

	applied, err := h.apply(t, &sub,
		func(s *domain.Subscription) error { s.AssignService(businessID, until); return nil },
		transitionSpec{
			reason:      domain.TransitionReasonServiceAssigned,
			initiator:   domain.InitiatorAdmin,
			initiatorID: &adminID,
			auditAction: auditdomain.ActionSubscriptionServiceAssigned,
			auditContext: func(s domain.Subscription, _ domain.Transition) map[string]any {
				return map[string]any{auditKeyTariffName: string(domain.TariffBusiness), auditKeyValidUntil: *s.ValidUntil}
			},
		},
	)
	if err != nil {
		t.Fatalf("applyTransition() error = %v", err)
	}

	if applied.FromStatus == nil || *applied.FromStatus != domain.SubscriptionStatusGrace ||
		applied.ToStatus != domain.SubscriptionStatusActive {
		t.Errorf("status side = %v→%q, want grace→active", applied.FromStatus, applied.ToStatus)
	}
	if applied.FromTariffID == nil || *applied.FromTariffID != h.tariffID(t, domain.TariffPro) || applied.ToTariffID != businessID {
		t.Errorf("tariff side = %v→%v, want pro→business", applied.FromTariffID, applied.ToTariffID)
	}

	records := h.audit.recorded()
	if len(records) != 1 {
		t.Fatalf("audit entries = %d, want 1", len(records))
	}
	if records[0].ActorRole != auditdomain.ActorRoleAdmin || records[0].ActorID == nil || *records[0].ActorID != adminID {
		t.Errorf("audit actor = %q/%v, want the acting admin", records[0].ActorRole, records[0].ActorID)
	}
	if records[0].Context[auditKeyTariffName] != string(domain.TariffBusiness) {
		t.Errorf("audit context tariff = %v, want business", records[0].Context[auditKeyTariffName])
	}
	if got, ok := records[0].Context[auditKeyValidUntil].(time.Time); !ok || !got.Equal(until) {
		t.Errorf("audit context valid_until = %v, want the post-change %v", records[0].Context[auditKeyValidUntil], until)
	}
}

// requireInitiatorAudit asserts the audit entry derives its actor from the
// transition initiator: the expected role, the initiator's id for user and
// admin initiators, and no actor id for the system.
func (h *transitionHarness) requireInitiatorAudit(
	t *testing.T, initiator domain.TransitionInitiator, wantRole auditdomain.ActorRole, initiatorID uuid.UUID,
) {
	t.Helper()
	records := h.audit.recorded()
	if len(records) != 1 {
		t.Fatalf("audit entries = %d, want 1", len(records))
	}
	if records[0].ActorRole != wantRole {
		t.Errorf("audit actor role = %q, want %q", records[0].ActorRole, wantRole)
	}
	if initiator == domain.InitiatorSystem {
		if records[0].ActorID != nil {
			t.Errorf("audit actor id = %v, want nil for the system initiator", records[0].ActorID)
		}
		return
	}
	if records[0].ActorID == nil || *records[0].ActorID != initiatorID {
		t.Errorf("audit actor id = %v, want the initiator id", records[0].ActorID)
	}
}

// TestApplyTransition_Initiators proves the three initiators of the transition
// vocabulary and the audit actor each of them derives: the user audits as the
// owner, the admin as the admin, the system as the system without an actor
// id.
func TestApplyTransition_Initiators(t *testing.T) {
	for _, tc := range []struct {
		name          string
		initiator     domain.TransitionInitiator
		wantAuditRole auditdomain.ActorRole
	}{
		{name: "user", initiator: domain.InitiatorUser, wantAuditRole: auditdomain.ActorRoleOwner},
		{name: "admin", initiator: domain.InitiatorAdmin, wantAuditRole: auditdomain.ActorRoleAdmin},
		{name: "system", initiator: domain.InitiatorSystem, wantAuditRole: auditdomain.ActorRoleSystem},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTransitionHarness(t)
			sub := h.seedSubscription(t, domain.TariffPro, nil)

			var initiatorID *uuid.UUID
			if tc.initiator != domain.InitiatorSystem {
				initiatorID = new(sub.UserID)
			}
			applied, err := h.apply(t, &sub,
				func(s *domain.Subscription) error { return s.Cancel() },
				transitionSpec{
					reason:      domain.TransitionReasonCancelled,
					initiator:   tc.initiator,
					initiatorID: initiatorID,
					auditAction: auditdomain.ActionSubscriptionCancelled,
				},
			)
			if err != nil {
				t.Fatalf("applyTransition() error = %v", err)
			}
			if applied.Initiator != tc.initiator || applied.InitiatorID == nil != (initiatorID == nil) {
				t.Fatalf("transition initiator = %q/%v, want %q/%v", applied.Initiator, applied.InitiatorID, tc.initiator, initiatorID)
			}

			h.requireInitiatorAudit(t, tc.initiator, tc.wantAuditRole, sub.UserID)
		})
	}
}

// TestApplyTransition_ValidityOnlyChange proves the validity-only shape of the
// admin's grace extension: neither status nor tariff moves, the extended
// deadline is what changed, and the audit context quotes the post-change
// deadline.
func TestApplyTransition_ValidityOnlyChange(t *testing.T) {
	h := newTransitionHarness(t)
	sub := h.seedSubscription(t, domain.TariffPro, func(s *domain.Subscription) {
		s.Status = domain.SubscriptionStatusGrace
		deadline := h.now.Add(24 * time.Hour)
		s.ValidUntil = &deadline
	})
	adminID := uuid.Must(uuid.NewV7())
	extra := 48 * time.Hour
	wantUntil := h.now.Add(24 * time.Hour).Add(extra)

	var auditQuoted any
	applied, err := h.apply(t, &sub,
		func(s *domain.Subscription) error { return s.ExtendGrace(h.now, extra) },
		transitionSpec{
			reason:      domain.TransitionReasonGraceExtended,
			initiator:   domain.InitiatorAdmin,
			initiatorID: &adminID,
			auditAction: auditdomain.ActionSubscriptionGraceExtended,
			auditContext: func(s domain.Subscription, _ domain.Transition) map[string]any {
				auditQuoted = *s.ValidUntil
				return map[string]any{auditKeyValidUntil: *s.ValidUntil}
			},
		},
	)
	if err != nil {
		t.Fatalf("applyTransition() error = %v", err)
	}

	if applied.FromStatus == nil || *applied.FromStatus != domain.SubscriptionStatusGrace ||
		applied.ToStatus != domain.SubscriptionStatusGrace {
		t.Errorf("status side = %v→%q, want grace unchanged", applied.FromStatus, applied.ToStatus)
	}
	if applied.FromTariffID == nil || *applied.FromTariffID != applied.ToTariffID {
		t.Errorf("tariff side = %v→%v, want unchanged", applied.FromTariffID, applied.ToTariffID)
	}
	if quoted, ok := auditQuoted.(time.Time); !ok || !quoted.Equal(wantUntil) {
		t.Errorf("audit context quoted valid_until = %v, want the extended %v", auditQuoted, wantUntil)
	}
}

// TestApplyTransition_ScheduledTariffChange proves the deferred shape of the
// user's downgrade scheduling: the aggregate keeps its current tariff and
// status, the log entry records the future target as its to-side, and the
// audit context may quote the transition's captured from-side.
func TestApplyTransition_ScheduledTariffChange(t *testing.T) {
	h := newTransitionHarness(t)
	sub := h.seedSubscription(t, domain.TariffBusiness, nil)
	business, pro := h.tariffID(t, domain.TariffBusiness), h.tariffID(t, domain.TariffPro)
	userID := sub.UserID

	var auditedFrom any
	applied, err := h.apply(t, &sub,
		func(s *domain.Subscription) error {
			businessTariff, proTariff := h.tariffByName(t, domain.TariffBusiness), h.tariffByName(t, domain.TariffPro)
			return s.ScheduleDowngrade(businessTariff, proTariff, domain.PeriodMonth, *s.ValidUntil)
		},
		transitionSpec{
			reason:            domain.TransitionReasonDowngradeScheduled,
			initiator:         domain.InitiatorUser,
			initiatorID:       &userID,
			scheduledTariffID: &pro,
			auditAction:       auditdomain.ActionSubscriptionTariffChanged,
			auditContext: func(_ domain.Subscription, tr domain.Transition) map[string]any {
				auditedFrom = *tr.FromTariffID
				return map[string]any{auditKeyFromTariffID: *tr.FromTariffID, auditKeyToTariffID: tr.ToTariffID}
			},
		},
	)
	if err != nil {
		t.Fatalf("applyTransition() error = %v", err)
	}

	if applied.FromStatus == nil || *applied.FromStatus != domain.SubscriptionStatusActive ||
		applied.ToStatus != domain.SubscriptionStatusActive {
		t.Errorf("status side = %v→%q, want active unchanged", applied.FromStatus, applied.ToStatus)
	}
	if applied.FromTariffID == nil || *applied.FromTariffID != business {
		t.Errorf("from tariff = %v, want the current business", applied.FromTariffID)
	}
	if applied.ToTariffID != pro {
		t.Errorf("to tariff = %v, want the scheduled pro target", applied.ToTariffID)
	}
	if from, ok := auditedFrom.(uuid.UUID); !ok || from != business {
		t.Errorf("audit context from_tariff = %v, want the captured business", auditedFrom)
	}
}

// TestApplyTransition_PaymentTransitions proves the two payment-caused shapes:
// an applied payment references the payment that caused it with the system
// initiator, and a refund references the refunded payment with the acting
// admin.
func TestApplyTransition_PaymentTransitions(t *testing.T) {
	t.Run("applied payment", func(t *testing.T) {
		h := newTransitionHarness(t)
		sub := h.seedSubscription(t, domain.TariffPro, nil)
		paymentID := uuid.Must(uuid.NewV7())

		applied, err := h.apply(t, &sub,
			func(s *domain.Subscription) error { return s.ApplyRenewal(paymentID, domain.PeriodMonth, h.now) },
			transitionSpec{
				reason:    domain.TransitionReasonPaymentApplied,
				initiator: domain.InitiatorSystem,
				paymentID: &paymentID,
			},
		)
		if err != nil {
			t.Fatalf("applyTransition() error = %v", err)
		}
		if applied.Reason != domain.TransitionReasonPaymentApplied {
			t.Errorf("reason = %q, want payment_applied", applied.Reason)
		}
		if applied.PaymentID == nil || *applied.PaymentID != paymentID {
			t.Errorf("payment = %v, want %v", applied.PaymentID, paymentID)
		}
		if applied.Initiator != domain.InitiatorSystem {
			t.Errorf("initiator = %q, want system", applied.Initiator)
		}
	})

	t.Run("refunded payment", func(t *testing.T) {
		h := newTransitionHarness(t)
		sub := h.seedSubscription(t, domain.TariffPro, nil)
		basicID := h.tariffID(t, domain.TariffBasic)
		adminID := uuid.Must(uuid.NewV7())
		paymentID := uuid.Must(uuid.NewV7())

		applied, err := h.apply(t, &sub,
			func(s *domain.Subscription) error { s.DowngradeToBasic(basicID); return nil },
			transitionSpec{
				reason:      domain.TransitionReasonRefunded,
				initiator:   domain.InitiatorAdmin,
				initiatorID: &adminID,
				paymentID:   &paymentID,
			},
		)
		if err != nil {
			t.Fatalf("applyTransition() error = %v", err)
		}
		if applied.Reason != domain.TransitionReasonRefunded {
			t.Errorf("reason = %q, want refunded", applied.Reason)
		}
		if applied.PaymentID == nil || *applied.PaymentID != paymentID {
			t.Errorf("payment = %v, want %v", applied.PaymentID, paymentID)
		}
		if applied.Initiator != domain.InitiatorAdmin || applied.InitiatorID == nil || *applied.InitiatorID != adminID {
			t.Errorf("initiator = %q/%v, want the acting admin", applied.Initiator, applied.InitiatorID)
		}
	})
}

// TestTransitionChangedTariff proves the enforcement guard: a transition that
// moved the tariff reports true, one that kept it false, and a transition
// without a from-side — the zero value a no-op returns — false, so an
// idempotent no-op never triggers the limit enforcement.
func TestTransitionChangedTariff(t *testing.T) {
	from := uuid.Must(uuid.NewV7())
	other := uuid.Must(uuid.NewV7())
	cases := []struct {
		name       string
		transition domain.Transition
		tariffID   uuid.UUID
		want       bool
	}{
		{name: "tariff moved", transition: domain.Transition{FromTariffID: &from}, tariffID: other, want: true},
		{name: "tariff kept", transition: domain.Transition{FromTariffID: &from}, tariffID: from, want: false},
		{name: "no from-side", transition: domain.Transition{}, tariffID: other, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := transitionChangedTariff(tc.transition, tc.tariffID); got != tc.want {
				t.Errorf("transitionChangedTariff() = %t, want %t", got, tc.want)
			}
		})
	}
}

// TestApplyTransition_RejectsMisusedPaymentSpec proves the spec contract is
// enforced before anything is written: a payment reference belongs only to an
// applied payment or a refund, and an applied payment is always
// system-initiated. A caller mix-up fails loudly instead of silently
// flattening into a wrong log entry.
func TestApplyTransition_RejectsMisusedPaymentSpec(t *testing.T) {
	userID := uuid.Must(uuid.NewV7())
	for _, tc := range []struct {
		name string
		spec transitionSpec
	}{
		{
			name: "payment reference with a plain reason",
			spec: transitionSpec{
				reason:      domain.TransitionReasonCancelled,
				initiator:   domain.InitiatorUser,
				initiatorID: &userID,
				paymentID:   new(uuid.Must(uuid.NewV7())),
			},
		},
		{
			name: "applied payment with an admin initiator",
			spec: transitionSpec{
				reason:      domain.TransitionReasonPaymentApplied,
				initiator:   domain.InitiatorAdmin,
				initiatorID: &userID,
				paymentID:   new(uuid.Must(uuid.NewV7())),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTransitionHarness(t)
			sub := h.seedSubscription(t, domain.TariffPro, nil)

			_, err := h.apply(t, &sub,
				func(s *domain.Subscription) error { return s.Cancel() },
				tc.spec,
			)
			if err == nil {
				t.Fatal("applyTransition() error = nil, want the spec misuse rejected")
			}
			if stored := h.storedSubscription(t, sub.UserID); stored.Status != domain.SubscriptionStatusActive {
				t.Errorf("stored status = %q, want the untouched active — validation runs before the mutation", stored.Status)
			}
			if entries := h.appendedTransitions(t, sub.ID); len(entries) != 0 {
				t.Errorf("appended transitions = %d, want 0", len(entries))
			}
			if entries := h.audit.recorded(); len(entries) != 0 {
				t.Errorf("audit entries = %d, want 0", len(entries))
			}
		})
	}
}

// failingAppendRepo fails Append while keeping the in-memory fake's reads.
type failingAppendRepo struct {
	*fakeTransitionRepo
	err error
}

func (r *failingAppendRepo) Append(context.Context, domain.Transition) error { return r.err }

func (r *failingAppendRepo) WithTx(transaction.Tx) (SubscriptionTransitionRepository, error) {
	return r, nil
}

// failingRecorder fails Record while keeping the noop recorder's binding.
type failingRecorder struct {
	auditapp.Noop
	err error
}

func (r *failingRecorder) Record(context.Context, auditdomain.Entry) error { return r.err }

func (r *failingRecorder) WithTx(transaction.Tx) auditapp.Recorder { return r }

// TestApplyTransition_MutateErrorWritesNothing proves a mutation error aborts
// the whole change: the aggregate, the transition log and the audit trail
// stay untouched, and the mutation's error is returned as-is.
func TestApplyTransition_MutateErrorWritesNothing(t *testing.T) {
	h := newTransitionHarness(t)
	sub := h.seedSubscription(t, domain.TariffPro, nil)
	mutateErr := errors.New("domain rule violated")

	_, err := h.apply(t, &sub,
		func(*domain.Subscription) error { return mutateErr },
		transitionSpec{
			reason:      domain.TransitionReasonCancelled,
			initiator:   domain.InitiatorUser,
			initiatorID: new(sub.UserID),
			auditAction: auditdomain.ActionSubscriptionCancelled,
		},
	)
	if !errors.Is(err, mutateErr) {
		t.Fatalf("applyTransition() error = %v, want the mutation error", err)
	}
	if stored := h.storedSubscription(t, sub.UserID); stored.Status != domain.SubscriptionStatusActive {
		t.Errorf("stored status = %q, want the untouched active", stored.Status)
	}
	if entries := h.appendedTransitions(t, sub.ID); len(entries) != 0 {
		t.Errorf("appended transitions = %d, want 0", len(entries))
	}
	if entries := h.audit.recorded(); len(entries) != 0 {
		t.Errorf("audit entries = %d, want 0", len(entries))
	}
}

// TestApplyTransition_UpdateFailureSkipsTransitionAndAudit proves the
// invariant's update side: when the aggregate cannot be persisted, no
// transition-log entry and no audit entry land.
func TestApplyTransition_UpdateFailureSkipsTransitionAndAudit(t *testing.T) {
	h := newTransitionHarness(t)
	sub := h.seedSubscription(t, domain.TariffPro, nil)
	// Drop the row so the fake's Update answers ErrNotFound.
	delete(h.stores.subscriptions.subs, sub.UserID)

	_, err := h.apply(t, &sub,
		func(s *domain.Subscription) error { return s.Cancel() },
		transitionSpec{
			reason:      domain.TransitionReasonCancelled,
			initiator:   domain.InitiatorUser,
			initiatorID: new(sub.UserID),
			auditAction: auditdomain.ActionSubscriptionCancelled,
		},
	)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("applyTransition() error = %v, want the repository's ErrNotFound", err)
	}
	if entries := h.appendedTransitions(t, sub.ID); len(entries) != 0 {
		t.Errorf("appended transitions = %d, want 0", len(entries))
	}
	if entries := h.audit.recorded(); len(entries) != 0 {
		t.Errorf("audit entries = %d, want 0", len(entries))
	}
}

// TestApplyTransition_AppendFailureFailsTheChange proves a transition-log
// failure fails the whole change — the log entry is not optional.
func TestApplyTransition_AppendFailureFailsTheChange(t *testing.T) {
	h := newTransitionHarness(t)
	sub := h.seedSubscription(t, domain.TariffPro, nil)
	appendErr := errors.New("transition log unavailable")
	factory := NewTxStoreFactory(
		h.stores.tariffs, h.stores.subscriptions,
		&failingAppendRepo{fakeTransitionRepo: h.stores.transitions, err: appendErr},
		h.stores.payments, h.stores.methods, h.stores.bindings,
		h.audit, &fakeUoW{beginner: h.stores.beginner},
	)

	err := factory.runInTx(t.Context(), func(stores *txStores) error {
		_, err := stores.applyTransition(t.Context(), &sub,
			func(s *domain.Subscription) error { return s.Cancel() },
			transitionSpec{reason: domain.TransitionReasonCancelled, initiator: domain.InitiatorUser},
		)
		return err
	})
	if !errors.Is(err, appendErr) {
		t.Fatalf("applyTransition() error = %v, want the append error", err)
	}
	if entries := h.audit.recorded(); len(entries) != 0 {
		t.Errorf("audit entries = %d, want 0 after the failed append", len(entries))
	}
}

// TestApplyTransition_AuditFailureFailsTheChange proves an audit failure fails
// the whole change — the audit record rides in the same transaction (ADR
// 0020), never after it.
func TestApplyTransition_AuditFailureFailsTheChange(t *testing.T) {
	h := newTransitionHarness(t)
	sub := h.seedSubscription(t, domain.TariffPro, nil)
	auditErr := errors.New("audit log unavailable")
	factory := NewTxStoreFactory(
		h.stores.tariffs, h.stores.subscriptions, h.stores.transitions,
		h.stores.payments, h.stores.methods, h.stores.bindings,
		&failingRecorder{err: auditErr}, &fakeUoW{beginner: h.stores.beginner},
	)

	err := factory.runInTx(t.Context(), func(stores *txStores) error {
		_, err := stores.applyTransition(t.Context(), &sub,
			func(s *domain.Subscription) error { return s.Cancel() },
			transitionSpec{
				reason:      domain.TransitionReasonCancelled,
				initiator:   domain.InitiatorUser,
				initiatorID: new(sub.UserID),
				auditAction: auditdomain.ActionSubscriptionCancelled,
			},
		)
		return err
	})
	if !errors.Is(err, auditErr) {
		t.Fatalf("applyTransition() error = %v, want the audit error", err)
	}
}
