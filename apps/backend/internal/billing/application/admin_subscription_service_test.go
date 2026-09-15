package application

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// The admin subscription operations of issue #255 on the in-memory fakes:
// service assignment overwriting a paid subscription, force tariff change,
// grace extension, cancellation on the user's behalf, the transition-history
// view and the upgrade-over-service payment path of ChangeTariff.

// seedServiceSubscription creates a service subscription on the given tariff
// with a month of term and returns it.
func (h *subscriptionHarness) seedServiceSubscription(t *testing.T, name domain.TariffName) domain.Subscription {
	t.Helper()
	sub := h.seedPaidSubscription(t, name)
	sub.Source = domain.SubscriptionSourceService
	sub.AutoRenewEnabled = false
	sub.CurrentPeriod = nil
	if err := h.stores.subscriptions.Update(t.Context(), sub); err != nil {
		t.Fatalf("seed Update() error = %v", err)
	}
	return sub
}

// transitionsOfSubscription loads the transition log of the user's
// subscription, newest first.
func (h *subscriptionHarness) transitionsOfSubscription(t *testing.T, userID uuid.UUID) []domain.Transition {
	t.Helper()
	sub, err := h.stores.subscriptions.GetByUserID(t.Context(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	transitions, err := h.stores.transitions.ListBySubscriptionID(t.Context(), sub.ID)
	if err != nil {
		t.Fatalf("ListBySubscriptionID() error = %v", err)
	}
	return transitions
}

// auditEntry finds the recorded audit entries with the given action.
func (h *subscriptionHarness) auditEntries(action auditdomain.Action) []auditdomain.Entry {
	var found []auditdomain.Entry
	for _, entry := range h.audit.recorded() {
		if entry.Action == action {
			found = append(found, entry)
		}
	}
	return found
}

// requireOverwrittenByService asserts the assignment overwrote the paid
// subscription: the source flips to service, auto-renew switches off, the paid
// remainder does not stack, and the plan is the assigned one for its term.
func (h *subscriptionHarness) requireOverwrittenByService(t *testing.T, sub domain.Subscription) {
	t.Helper()
	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.Source != domain.SubscriptionSourceService {
		t.Errorf("Source = %q, want service", stored.Source)
	}
	if stored.Status != domain.SubscriptionStatusActive {
		t.Errorf("Status = %q, want active", stored.Status)
	}
	if stored.AutoRenewEnabled {
		t.Error("AutoRenewEnabled = true, want false")
	}
	if stored.CurrentPeriod != nil {
		t.Errorf("CurrentPeriod = %v, want nil", stored.CurrentPeriod)
	}
	wantUntil := h.now.AddDate(0, 1, 0)
	if stored.ValidUntil == nil || !stored.ValidUntil.Equal(wantUntil) {
		t.Errorf("ValidUntil = %v, want %v (a month from the fixed now)", stored.ValidUntil, wantUntil)
	}
	if stored.TariffID != h.tariffID(t, domain.TariffBusiness) {
		t.Errorf("TariffID = %v, want business", stored.TariffID)
	}
}

// requireServiceAssignedTransition asserts the transition log's newest entry
// is the admin-initiated service assignment from the overwritten paid state.
func (h *subscriptionHarness) requireServiceAssignedTransition(t *testing.T, sub domain.Subscription, adminID uuid.UUID) {
	t.Helper()
	transitions := h.transitionsOfSubscription(t, sub.UserID)
	if len(transitions) == 0 {
		t.Fatal("no transitions recorded")
	}
	assignment := transitions[0]
	if assignment.Reason != domain.TransitionReasonServiceAssigned {
		t.Errorf("latest transition reason = %q, want service_assigned", assignment.Reason)
	}
	if assignment.Initiator != domain.InitiatorAdmin || assignment.InitiatorID == nil || *assignment.InitiatorID != adminID {
		t.Errorf("transition initiator = %q/%v, want the acting admin", assignment.Initiator, assignment.InitiatorID)
	}
	if assignment.FromStatus == nil || *assignment.FromStatus != domain.SubscriptionStatusActive {
		t.Errorf("from status = %v, want the pre-assignment active", assignment.FromStatus)
	}
	if assignment.FromTariffID == nil || *assignment.FromTariffID != sub.TariffID {
		t.Errorf("from tariff = %v, want the overwritten pro", assignment.FromTariffID)
	}
}

// requireServiceAssignedAudit asserts the audit trail holds exactly the
// service assignment attributed to the acting admin.
func (h *subscriptionHarness) requireServiceAssignedAudit(t *testing.T, adminID uuid.UUID) {
	t.Helper()
	entries := h.auditEntries(auditdomain.ActionSubscriptionServiceAssigned)
	if len(entries) != 1 {
		t.Fatalf("service_assigned audit entries = %d, want 1", len(entries))
	}
	if entries[0].ActorRole != auditdomain.ActorRoleAdmin || entries[0].ActorID == nil || *entries[0].ActorID != adminID {
		t.Errorf("audit actor = %v/%v, want the acting admin", entries[0].ActorRole, entries[0].ActorID)
	}
}

// TestAdminAssignServiceSubscription_OverwritesPaidWithTermTransitionAudit
// proves the assignment (issue #255): a paid subscription is overwritten by
// the service one — source flips, auto-renew switches off, the paid remainder
// does not stack — and both the transition log and the audit trail attribute
// the change to the acting admin.
func TestAdminAssignServiceSubscription_OverwritesPaidWithTermTransitionAudit(t *testing.T) {
	t.Parallel()
	h := newSubscriptionHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffPro)
	adminID := uuid.Must(uuid.NewV7())

	req := AssignServiceSubscriptionRequest{
		TariffName: domain.TariffBusiness,
		TermType:   ServiceTermMonth,
	}
	if err := h.svc.AssignServiceSubscription(t.Context(), adminID, sub.UserID, req); err != nil {
		t.Fatalf("AssignServiceSubscription() error = %v", err)
	}

	h.requireOverwrittenByService(t, sub)
	h.requireServiceAssignedTransition(t, sub, adminID)
	h.requireServiceAssignedAudit(t, adminID)
}

// TestAdminAssignServiceSubscription_TermResolution proves the three term
// shapes: month and year count from now, a date runs until the end of that UTC
// date inclusive, and a past or missing date answers ErrInvalidTerm before
// anything is written.
func TestAdminAssignServiceSubscription_TermResolution(t *testing.T) {
	t.Parallel()
	h := newSubscriptionHarness(t)
	adminID := uuid.Must(uuid.NewV7())

	cases := []struct {
		name      string
		req       AssignServiceSubscriptionRequest
		wantUntil time.Time
		wantErr   error
	}{
		{
			name:      "year term",
			req:       AssignServiceSubscriptionRequest{TariffName: domain.TariffPro, TermType: ServiceTermYear},
			wantUntil: h.now.AddDate(1, 0, 0),
		},
		{
			name: "date term runs to the end of the day",
			req: AssignServiceSubscriptionRequest{
				TariffName: domain.TariffPro,
				TermType:   ServiceTermDate,
				UntilDate:  new(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)),
			},
			wantUntil: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "today is allowed",
			req: AssignServiceSubscriptionRequest{
				TariffName: domain.TariffPro,
				TermType:   ServiceTermDate,
				UntilDate:  new(time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)),
			},
			wantUntil: time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "yesterday is rejected",
			req: AssignServiceSubscriptionRequest{
				TariffName: domain.TariffPro,
				TermType:   ServiceTermDate,
				UntilDate:  new(time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC)),
			},
			wantErr: domain.ErrInvalidTerm,
		},
		{
			name:    "date term without a date is rejected",
			req:     AssignServiceSubscriptionRequest{TariffName: domain.TariffPro, TermType: ServiceTermDate},
			wantErr: domain.ErrInvalidTerm,
		},
		{
			name:    "unknown term type is rejected",
			req:     AssignServiceSubscriptionRequest{TariffName: domain.TariffPro, TermType: ServiceTermType("quarter")},
			wantErr: domain.ErrInvalidTerm,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sub := h.seedPaidSubscription(t, domain.TariffBasic)
			err := h.svc.AssignServiceSubscription(t.Context(), adminID, sub.UserID, tt.req)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("AssignServiceSubscription() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("AssignServiceSubscription() error = %v", err)
			}
			stored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
			if err != nil {
				t.Fatalf("GetByUserID() error = %v", err)
			}
			if stored.ValidUntil == nil || !stored.ValidUntil.Equal(tt.wantUntil) {
				t.Errorf("ValidUntil = %v, want %v", stored.ValidUntil, tt.wantUntil)
			}
		})
	}
}

// TestAdminAssignServiceSubscription_TariffNotFound proves an unknown plan
// answers the tariff sentinel without touching the subscription.
func TestAdminAssignServiceSubscription_TariffNotFound(t *testing.T) {
	t.Parallel()
	h := newSubscriptionHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffBasic)

	err := h.svc.AssignServiceSubscription(t.Context(), uuid.Must(uuid.NewV7()), sub.UserID, AssignServiceSubscriptionRequest{
		TariffName: domain.TariffName("platinum"),
		TermType:   ServiceTermMonth,
	})
	if !errors.Is(err, ErrTariffNotFound) {
		t.Fatalf("AssignServiceSubscription() error = %v, want ErrTariffNotFound", err)
	}
}

// requireForceChangedSubscription asserts the force change applied the new
// tariff for the chosen period while the source, the auto-renew setting and
// the period kept their value.
func (h *subscriptionHarness) requireForceChangedSubscription(t *testing.T, sub domain.Subscription) {
	t.Helper()
	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.TariffID != h.tariffID(t, domain.TariffPro) {
		t.Errorf("TariffID = %v, want pro", stored.TariffID)
	}
	wantUntil := h.now.AddDate(1, 0, 0)
	if stored.ValidUntil == nil || !stored.ValidUntil.Equal(wantUntil) {
		t.Errorf("ValidUntil = %v, want %v", stored.ValidUntil, wantUntil)
	}
	if stored.Source != domain.SubscriptionSourceService {
		t.Errorf("Source = %q, want the unchanged service source", stored.Source)
	}
	if stored.AutoRenewEnabled {
		t.Error("AutoRenewEnabled = true, want the unchanged value false")
	}
	if stored.CurrentPeriod == nil || *stored.CurrentPeriod != domain.PeriodYear {
		t.Errorf("CurrentPeriod = %v, want year", stored.CurrentPeriod)
	}
}

// requireForcedChangeTransitionAudit asserts the transition log's newest entry
// is the admin-initiated forced change and the audit trail holds exactly its
// admin-acted entry.
func (h *subscriptionHarness) requireForcedChangeTransitionAudit(t *testing.T, userID, adminID uuid.UUID) {
	t.Helper()
	transitions := h.transitionsOfSubscription(t, userID)
	if len(transitions) == 0 {
		t.Fatal("no transitions recorded")
	}
	change := transitions[0]
	if change.Reason != domain.TransitionReasonForcedChange {
		t.Errorf("latest transition reason = %q, want forced_change", change.Reason)
	}
	if change.Initiator != domain.InitiatorAdmin || change.InitiatorID == nil || *change.InitiatorID != adminID {
		t.Errorf("transition initiator = %q/%v, want the acting admin", change.Initiator, change.InitiatorID)
	}

	entries := h.auditEntries(auditdomain.ActionSubscriptionTariffForced)
	if len(entries) != 1 {
		t.Fatalf("tariff_forced audit entries = %d, want 1", len(entries))
	}
	if entries[0].ActorRole != auditdomain.ActorRoleAdmin {
		t.Errorf("audit actor role = %v, want admin", entries[0].ActorRole)
	}
}

// TestAdminForceChangeTariff_AppliesWithoutPayment proves the force change
// (issue #255): the new tariff applies immediately for the chosen period, the
// source and auto-renew setting keep their value, and the transition log and
// audit trail attribute the change to the acting admin.
func TestAdminForceChangeTariff_AppliesWithoutPayment(t *testing.T) {
	t.Parallel()
	h := newSubscriptionHarness(t)
	sub := h.seedServiceSubscription(t, domain.TariffBusiness)
	adminID := uuid.Must(uuid.NewV7())

	if err := h.svc.ForceChangeTariff(t.Context(), adminID, sub.UserID, ForceChangeTariffRequest{
		TariffName: domain.TariffPro,
		Period:     domain.PeriodYear,
	}); err != nil {
		t.Fatalf("ForceChangeTariff() error = %v", err)
	}

	h.requireForceChangedSubscription(t, sub)
	h.requireForcedChangeTransitionAudit(t, sub.UserID, adminID)
}

// TestAdminForceChangeTariff_Rejections proves the force-change guards: the
// same tariff answers ErrAlreadyOnTariff and a cancelled subscription is out
// of scope.
func TestAdminForceChangeTariff_Rejections(t *testing.T) {
	t.Parallel()
	h := newSubscriptionHarness(t)
	adminID := uuid.Must(uuid.NewV7())

	sub := h.seedPaidSubscription(t, domain.TariffPro)
	if err := h.svc.ForceChangeTariff(t.Context(), adminID, sub.UserID, ForceChangeTariffRequest{
		TariffName: domain.TariffPro,
		Period:     domain.PeriodMonth,
	}); !errors.Is(err, domain.ErrAlreadyOnTariff) {
		t.Fatalf("ForceChangeTariff(same tariff) error = %v, want ErrAlreadyOnTariff", err)
	}

	cancelled := h.seedPaidSubscription(t, domain.TariffPro)
	if err := cancelled.Cancel(nil); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if err := h.stores.subscriptions.Update(t.Context(), cancelled); err != nil {
		t.Fatalf("seed Update() error = %v", err)
	}
	if err := h.svc.ForceChangeTariff(t.Context(), adminID, cancelled.UserID, ForceChangeTariffRequest{
		TariffName: domain.TariffBusiness,
		Period:     domain.PeriodMonth,
	}); !errors.Is(err, domain.ErrInvalidSubscriptionState) {
		t.Fatalf("ForceChangeTariff(cancelled) error = %v, want ErrInvalidSubscriptionState", err)
	}
}

// TestAdminExtendGrace_LengthensWindowWithTransitionAudit proves the manual
// grace extension (issue #255): the window lengthens from the current
// deadline, the reminder flag resets for the fresh window, and the transition
// log and audit trail attribute the extension to the acting admin.
func TestAdminExtendGrace_LengthensWindowWithTransitionAudit(t *testing.T) {
	t.Parallel()
	h := newSubscriptionHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffPro)
	graceUntil := h.now.Add(48 * time.Hour)
	sub.Status = domain.SubscriptionStatusGrace
	sub.ValidUntil = &graceUntil
	if err := h.stores.subscriptions.Update(t.Context(), sub); err != nil {
		t.Fatalf("seed Update() error = %v", err)
	}
	adminID := uuid.Must(uuid.NewV7())

	if err := h.svc.ExtendGrace(t.Context(), adminID, sub.UserID, 3); err != nil {
		t.Fatalf("ExtendGrace() error = %v", err)
	}

	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	want := graceUntil.Add(3 * 24 * time.Hour)
	if stored.ValidUntil == nil || !stored.ValidUntil.Equal(want) {
		t.Errorf("ValidUntil = %v, want %v", stored.ValidUntil, want)
	}
	if stored.GraceRemindedAt != nil {
		t.Errorf("GraceRemindedAt = %v, want nil for the fresh reminder window", stored.GraceRemindedAt)
	}

	transitions := h.transitionsOfSubscription(t, sub.UserID)
	if len(transitions) == 0 {
		t.Fatal("no transitions recorded")
	}
	extension := transitions[0]
	if extension.Reason != domain.TransitionReasonGraceExtended {
		t.Errorf("latest transition reason = %q, want grace_extended", extension.Reason)
	}
	if extension.Initiator != domain.InitiatorAdmin || extension.InitiatorID == nil || *extension.InitiatorID != adminID {
		t.Errorf("transition initiator = %q/%v, want the acting admin", extension.Initiator, extension.InitiatorID)
	}

	if entries := h.auditEntries(auditdomain.ActionSubscriptionGraceExtended); len(entries) != 1 {
		t.Fatalf("grace_extended audit entries = %d, want 1", len(entries))
	}
}

// TestAdminExtendGrace_Rejections proves the guards: only a grace subscription
// can be extended, and the day count stays within the operational cap.
func TestAdminExtendGrace_Rejections(t *testing.T) {
	t.Parallel()
	h := newSubscriptionHarness(t)
	adminID := uuid.Must(uuid.NewV7())

	sub := h.seedPaidSubscription(t, domain.TariffPro)
	if err := h.svc.ExtendGrace(t.Context(), adminID, sub.UserID, 3); !errors.Is(err, domain.ErrInvalidSubscriptionState) {
		t.Fatalf("ExtendGrace(active) error = %v, want ErrInvalidSubscriptionState", err)
	}

	grace := h.seedPaidSubscription(t, domain.TariffPro)
	graceUntil := h.now.Add(48 * time.Hour)
	grace.Status = domain.SubscriptionStatusGrace
	grace.ValidUntil = &graceUntil
	if err := h.stores.subscriptions.Update(t.Context(), grace); err != nil {
		t.Fatalf("seed Update() error = %v", err)
	}
	if err := h.svc.ExtendGrace(t.Context(), adminID, grace.UserID, 0); !errors.Is(err, domain.ErrInvalidGraceExtension) {
		t.Fatalf("ExtendGrace(0 days) error = %v, want ErrInvalidGraceExtension", err)
	}
	if err := h.svc.ExtendGrace(t.Context(), adminID, grace.UserID,
		DefaultConfig().MaxGraceExtensionDays+1); !errors.Is(err, domain.ErrInvalidGraceExtension) {
		t.Fatalf("ExtendGrace(over cap) error = %v, want ErrInvalidGraceExtension", err)
	}
}

// TestAdminCancelSubscription_CancelsWithAdminAttribution proves the admin
// cancellation (issue #255): the user's own cancellation semantics with the
// admin initiator in the transition log and the admin actor in the audit
// trail. A service subscription cannot be cancelled this way.
func TestAdminCancelSubscription_CancelsWithAdminAttribution(t *testing.T) {
	t.Parallel()
	h := newSubscriptionHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffPro)
	adminID := uuid.Must(uuid.NewV7())

	if err := h.svc.CancelSubscriptionAsAdmin(t.Context(), adminID, sub.UserID); err != nil {
		t.Fatalf("CancelSubscriptionAsAdmin() error = %v", err)
	}

	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.Status != domain.SubscriptionStatusCancelled {
		t.Errorf("Status = %q, want cancelled", stored.Status)
	}
	if stored.AutoRenewEnabled {
		t.Error("AutoRenewEnabled = true, want false")
	}

	transitions := h.transitionsOfSubscription(t, sub.UserID)
	if len(transitions) == 0 {
		t.Fatal("no transitions recorded")
	}
	cancellation := transitions[0]
	if cancellation.Reason != domain.TransitionReasonCancelled {
		t.Errorf("latest transition reason = %q, want cancelled", cancellation.Reason)
	}
	if cancellation.Initiator != domain.InitiatorAdmin || cancellation.InitiatorID == nil || *cancellation.InitiatorID != adminID {
		t.Errorf("transition initiator = %q/%v, want the acting admin", cancellation.Initiator, cancellation.InitiatorID)
	}

	entries := h.auditEntries(auditdomain.ActionSubscriptionCancelled)
	if len(entries) != 1 {
		t.Fatalf("cancelled audit entries = %d, want 1", len(entries))
	}
	if entries[0].ActorRole != auditdomain.ActorRoleAdmin {
		t.Errorf("audit actor role = %v, want admin", entries[0].ActorRole)
	}

	service := h.seedServiceSubscription(t, domain.TariffPro)
	if err := h.svc.CancelSubscriptionAsAdmin(t.Context(), adminID, service.UserID); !errors.Is(err, domain.ErrInvalidSubscriptionState) {
		t.Fatalf("CancelSubscriptionAsAdmin(service) error = %v, want ErrInvalidSubscriptionState", err)
	}
}

// TestAdminListTransitions_ResolvesTariffNames proves the transition-history
// view (issue #255): the newest-first log with the tariff names resolved and
// the payment reference kept.
func TestAdminListTransitions_ResolvesTariffNames(t *testing.T) {
	t.Parallel()
	h := newSubscriptionHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffPro)
	if err := h.svc.CancelSubscriptionAsAdmin(t.Context(), uuid.Must(uuid.NewV7()), sub.UserID); err != nil {
		t.Fatalf("CancelSubscriptionAsAdmin() error = %v", err)
	}

	views, err := h.svc.ListTransitions(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("ListTransitions() error = %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("transitions = %d, want 1", len(views))
	}
	view := views[0]
	if view.ToTariffName != string(domain.TariffPro) {
		t.Errorf("ToTariffName = %q, want pro", view.ToTariffName)
	}
	if view.FromTariffName == nil || *view.FromTariffName != string(domain.TariffPro) {
		t.Errorf("FromTariffName = %v, want pro", view.FromTariffName)
	}
	if view.Transition.Reason != domain.TransitionReasonCancelled {
		t.Errorf("reason = %q, want cancelled", view.Transition.Reason)
	}

	other, err := h.svc.ListTransitions(t.Context(), uuid.Must(uuid.NewV7()))
	if !errors.Is(err, ErrSubscriptionNotFound) {
		t.Fatalf("ListTransitions(unknown user) error = %v, want ErrSubscriptionNotFound", err)
	}
	_ = other
}

// TestChangeTariff_ServiceSubscriptionUpgradeTakesPaymentPath proves the
// upgrade-over-service rule (issue #255): a service subscription may take the
// paid upgrade path, while a downgrade request on it stays rejected.
func TestChangeTariff_ServiceSubscriptionUpgradeTakesPaymentPath(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, func(s *domain.Subscription) {
		s.Source = domain.SubscriptionSourceService
		s.AutoRenewEnabled = false
		s.CurrentPeriod = nil
	})

	result, err := h.subs.ChangeTariff(t.Context(), sub.UserID, ChangeTariffRequest{
		TariffName: domain.TariffBusiness,
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff(upgrade over service) error = %v", err)
	}
	if result.PaymentID == uuid.Nil {
		t.Fatal("ChangeTariff() returned no payment for the upgrade over service")
	}

	stored, err := h.stores.payments.GetByID(t.Context(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if stored.TariffID != h.tariffID(t, domain.TariffBusiness) {
		t.Errorf("payment tariff = %v, want business", stored.TariffID)
	}
	if stored.AmountKopecks != 99000 {
		t.Errorf("payment amount = %d, want 99000", stored.AmountKopecks)
	}
}

// TestChangeTariff_ServiceSubscriptionDowngradeRejected proves the flip side:
// the deferred downgrade path stays reserved for paid subscriptions — a
// service subscription is reassigned by an admin, not rescheduled by the user.
func TestChangeTariff_ServiceSubscriptionDowngradeRejected(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, func(s *domain.Subscription) {
		s.TariffID = h.tariffID(t, domain.TariffBusiness)
		s.Source = domain.SubscriptionSourceService
		s.AutoRenewEnabled = false
		s.CurrentPeriod = nil
	})

	_, err := h.subs.ChangeTariff(t.Context(), sub.UserID, ChangeTariffRequest{
		TariffName: domain.TariffPro,
		Period:     domain.PeriodMonth,
	})
	if !errors.Is(err, domain.ErrInvalidSubscriptionState) {
		t.Fatalf("ChangeTariff(downgrade on service) error = %v, want ErrInvalidSubscriptionState", err)
	}
}
