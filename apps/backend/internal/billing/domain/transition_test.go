package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func validSubscription(t *testing.T) Subscription {
	t.Helper()
	sub, err := NewBasicSubscription(
		uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"),
		uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12"),
	)
	if err != nil {
		t.Fatalf("NewBasicSubscription() error = %v", err)
	}
	return sub
}

func TestNewTransition_Registered(t *testing.T) {
	sub := validSubscription(t)

	transition, err := NewTransition(sub, nil, nil, TransitionReasonRegistered, InitiatorSystem, nil)
	if err != nil {
		t.Fatalf("NewTransition() error = %v", err)
	}
	if transition.SubscriptionID != sub.ID {
		t.Errorf("SubscriptionID = %v, want %v", transition.SubscriptionID, sub.ID)
	}
	if transition.FromStatus != nil {
		t.Errorf("FromStatus = %v, want nil (creation has no prior status)", *transition.FromStatus)
	}
	if transition.FromTariffID != nil {
		t.Errorf("FromTariffID = %v, want nil (creation has no prior tariff)", *transition.FromTariffID)
	}
	if transition.ToStatus != SubscriptionStatusActive {
		t.Errorf("ToStatus = %q, want %q", transition.ToStatus, SubscriptionStatusActive)
	}
	if transition.ToTariffID != sub.TariffID {
		t.Errorf("ToTariffID = %v, want %v", transition.ToTariffID, sub.TariffID)
	}
	if transition.Initiator != InitiatorSystem {
		t.Errorf("Initiator = %q, want %q", transition.Initiator, InitiatorSystem)
	}
	if transition.InitiatorID != nil {
		t.Errorf("InitiatorID = %v, want nil for the system initiator", *transition.InitiatorID)
	}
	if transition.ID == uuid.Nil {
		t.Error("ID = nil, want generated UUIDv7")
	}
	if transition.CreatedAt != (time.Time{}) {
		t.Errorf("CreatedAt = %v, want zero (set by persistence)", transition.CreatedAt)
	}
}

func TestNewTransition_RecordsAppliedState(t *testing.T) {
	sub := validSubscription(t)
	from := SubscriptionStatusActive
	fromTariff := uuid.New()
	if err := sub.Cancel(); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	actorID := uuid.New()

	transition, err := NewTransition(sub, &from, &fromTariff, "user_cancel", InitiatorUser, &actorID)
	if err != nil {
		t.Fatalf("NewTransition() error = %v", err)
	}
	if transition.ToStatus != SubscriptionStatusCancelled {
		t.Errorf("ToStatus = %q, want %q", transition.ToStatus, SubscriptionStatusCancelled)
	}
	if transition.FromStatus == nil || *transition.FromStatus != SubscriptionStatusActive {
		t.Errorf("FromStatus = %v, want %q", transition.FromStatus, SubscriptionStatusActive)
	}
	if transition.InitiatorID == nil || *transition.InitiatorID != actorID {
		t.Errorf("InitiatorID = %v, want %v", transition.InitiatorID, actorID)
	}
}

func TestNewTransition_RejectsInvalidInput(t *testing.T) {
	sub := validSubscription(t)

	cases := []struct {
		name        string
		sub         Subscription
		reason      TransitionReason
		initiator   TransitionInitiator
		initiatorID *uuid.UUID
	}{
		{name: "empty reason", sub: sub, reason: "", initiator: InitiatorSystem},
		{name: "unknown initiator", sub: sub, reason: TransitionReasonRegistered, initiator: "robot"},
		{name: "missing subscription id", sub: Subscription{}, reason: TransitionReasonRegistered, initiator: InitiatorSystem},
		{name: "system initiator with actor id", sub: sub, reason: TransitionReasonRegistered, initiator: InitiatorSystem, initiatorID: new(uuid.New())},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewTransition(tc.sub, nil, nil, tc.reason, tc.initiator, tc.initiatorID); err == nil {
				t.Fatal("NewTransition() error = nil, want error")
			}
		})
	}
}

func TestNewScheduledTariffTransition_RecordsTargetTariff(t *testing.T) {
	sub := validSubscription(t)
	target := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13")
	actorID := uuid.New()

	transition, err := NewScheduledTariffTransition(sub, target, TransitionReasonDowngradeScheduled, InitiatorUser, &actorID)
	if err != nil {
		t.Fatalf("NewScheduledTariffTransition() error = %v", err)
	}
	if transition.ToTariffID != target {
		t.Errorf("ToTariffID = %v, want the scheduled target %v", transition.ToTariffID, target)
	}
	if transition.FromTariffID == nil || *transition.FromTariffID != sub.TariffID {
		t.Errorf("FromTariffID = %v, want the current tariff %v", transition.FromTariffID, sub.TariffID)
	}
	if transition.ToStatus != SubscriptionStatusActive || transition.FromStatus == nil || *transition.FromStatus != SubscriptionStatusActive {
		t.Errorf("status = %v -> %v, want active -> active (scheduling changes no status)", transition.FromStatus, transition.ToStatus)
	}
	if transition.Initiator != InitiatorUser || transition.InitiatorID == nil || *transition.InitiatorID != actorID {
		t.Errorf("initiator = %q/%v, want user/%v", transition.Initiator, transition.InitiatorID, actorID)
	}
	if transition.CreatedAt != (time.Time{}) {
		t.Errorf("CreatedAt = %v, want zero (set by persistence)", transition.CreatedAt)
	}
}

func TestNewScheduledTariffTransition_RejectsInvalidInput(t *testing.T) {
	sub := validSubscription(t)

	cases := []struct {
		name        string
		target      uuid.UUID
		reason      TransitionReason
		initiator   TransitionInitiator
		initiatorID *uuid.UUID
	}{
		{name: "missing target tariff", reason: TransitionReasonDowngradeScheduled, initiator: InitiatorUser},
		{name: "empty reason", target: uuid.New(), reason: "", initiator: InitiatorUser},
		{name: "unknown initiator", target: uuid.New(), reason: TransitionReasonDowngradeScheduled, initiator: "robot"},
		{name: "system initiator with actor id", target: uuid.New(), reason: TransitionReasonDowngradeScheduled, initiator: InitiatorSystem, initiatorID: new(uuid.New())},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewScheduledTariffTransition(sub, tc.target, tc.reason, tc.initiator, tc.initiatorID); err == nil {
				t.Fatal("NewScheduledTariffTransition() error = nil, want error")
			}
		})
	}
}

func TestParseTransitionInitiator(t *testing.T) {
	for _, raw := range []string{"user", "admin", "system"} {
		if _, err := ParseTransitionInitiator(raw); err != nil {
			t.Errorf("ParseTransitionInitiator(%q) error = %v", raw, err)
		}
	}
	if _, err := ParseTransitionInitiator("robot"); err == nil {
		t.Error(`ParseTransitionInitiator("robot") error = nil, want error`)
	}
}

func TestReconstituteTransition(t *testing.T) {
	sub := validSubscription(t)
	valid, err := NewTransition(sub, nil, nil, TransitionReasonRegistered, InitiatorSystem, nil)
	if err != nil {
		t.Fatalf("NewTransition() error = %v", err)
	}
	if _, err := ReconstituteTransition(valid); err != nil {
		t.Errorf("ReconstituteTransition(valid) error = %v", err)
	}

	badStatus := valid
	badStatus.ToStatus = "blocked"
	if _, err := ReconstituteTransition(badStatus); err == nil {
		t.Error("ReconstituteTransition(unknown to-status) error = nil, want error")
	}

	badFrom := valid
	blocked := SubscriptionStatus("blocked")
	badFrom.FromStatus = &blocked
	if _, err := ReconstituteTransition(badFrom); err == nil {
		t.Error("ReconstituteTransition(unknown from-status) error = nil, want error")
	}

	badInitiator := valid
	badInitiator.Initiator = "robot"
	if _, err := ReconstituteTransition(badInitiator); err == nil {
		t.Error("ReconstituteTransition(unknown initiator) error = nil, want error")
	}

	noReason := valid
	noReason.Reason = ""
	if _, err := ReconstituteTransition(noReason); err == nil {
		t.Error("ReconstituteTransition(empty reason) error = nil, want error")
	}
}
