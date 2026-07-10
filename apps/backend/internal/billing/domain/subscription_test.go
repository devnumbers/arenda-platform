package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewOwnerSubscription(t *testing.T) {
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	tariffID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")

	sub, err := NewOwnerSubscription(userID, tariffID)
	if err != nil {
		t.Fatalf("NewOwnerSubscription() error = %v", err)
	}

	if sub.UserID != userID {
		t.Errorf("UserID = %v, want %v", sub.UserID, userID)
	}
	if sub.TariffID != tariffID {
		t.Errorf("TariffID = %v, want %v", sub.TariffID, tariffID)
	}
	if sub.Source != SubscriptionSourcePaid {
		t.Errorf("Source = %v, want %v", sub.Source, SubscriptionSourcePaid)
	}
	if sub.Status != SubscriptionStatusActive {
		t.Errorf("Status = %v, want %v", sub.Status, SubscriptionStatusActive)
	}
	if sub.AutoRenewEnabled {
		t.Errorf("AutoRenewEnabled = true, want false")
	}
	if sub.ValidUntil != nil {
		t.Errorf("ValidUntil = %v, want nil", sub.ValidUntil)
	}
}

func TestSubscriptionCanMutateData(t *testing.T) {
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	future := now.Add(24 * time.Hour)
	past := now.Add(-24 * time.Hour)

	tests := []struct {
		name       string
		status     SubscriptionStatus
		validUntil *time.Time
		want       bool
	}{
		{"active with future valid_until", SubscriptionStatusActive, &future, true},
		{"active without valid_until", SubscriptionStatusActive, nil, true},
		{"active with expired valid_until", SubscriptionStatusActive, &past, false},
		{"grace within grace period", SubscriptionStatusGrace, &future, true},
		{"grace after grace period", SubscriptionStatusGrace, &past, false},
		{"cancelled without valid_until", SubscriptionStatusCancelled, nil, true},
		{"cancelled with future valid_until", SubscriptionStatusCancelled, &future, true},
		{"cancelled with expired valid_until", SubscriptionStatusCancelled, &past, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := Subscription{Status: tt.status, ValidUntil: tt.validUntil}
			if got := sub.CanMutateData(now); got != tt.want {
				t.Errorf("CanMutateData() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubscriptionIsPaidSource(t *testing.T) {
	tests := []struct {
		name   string
		source SubscriptionSource
		want   bool
	}{
		{"paid", SubscriptionSourcePaid, true},
		{"service", SubscriptionSourceService, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := Subscription{Source: tt.source}
			if got := sub.IsPaidSource(); got != tt.want {
				t.Errorf("IsPaidSource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubscriptionIsInGrace(t *testing.T) {
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	future := now.Add(24 * time.Hour)
	past := now.Add(-24 * time.Hour)

	tests := []struct {
		name       string
		status     SubscriptionStatus
		validUntil *time.Time
		want       bool
	}{
		{"grace with future valid_until", SubscriptionStatusGrace, &future, true},
		{"grace with nil valid_until", SubscriptionStatusGrace, nil, false},
		{"grace with past valid_until", SubscriptionStatusGrace, &past, false},
		{"active with future valid_until", SubscriptionStatusActive, &future, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := Subscription{Status: tt.status, ValidUntil: tt.validUntil}
			if got := sub.IsInGrace(now); got != tt.want {
				t.Errorf("IsInGrace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubscriptionHasPendingChange(t *testing.T) {
	tariffID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	changeAt := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name            string
		pendingTariffID *uuid.UUID
		pendingChangeAt *time.Time
		want            bool
	}{
		{"pending change", &tariffID, &changeAt, true},
		{"missing tariff", nil, &changeAt, false},
		{"missing date", &tariffID, nil, false},
		{"no pending", nil, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := Subscription{PendingTariffID: tt.pendingTariffID, PendingChangeAt: tt.pendingChangeAt}
			if got := sub.HasPendingChange(); got != tt.want {
				t.Errorf("HasPendingChange() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubscriptionScheduleDowngrade(t *testing.T) {
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	currentTariffID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")
	newTariffID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13")
	validUntil := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	sub := Subscription{
		ID:               uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14"),
		UserID:           userID,
		TariffID:         currentTariffID,
		Status:           SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: false,
	}

	currentTariff := Tariff{ID: currentTariffID, Name: TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000}
	newTariff := Tariff{ID: newTariffID, Name: TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000}
	if err := sub.ScheduleDowngrade(currentTariff, newTariff, PeriodMonth, validUntil); err != nil {
		t.Fatalf("ScheduleDowngrade() error = %v", err)
	}
	if sub.PendingTariffID == nil || *sub.PendingTariffID != newTariffID {
		t.Errorf("PendingTariffID = %v, want %s", sub.PendingTariffID, newTariffID)
	}
	if sub.PendingChangeAt == nil || !sub.PendingChangeAt.Equal(validUntil) {
		t.Errorf("PendingChangeAt = %v, want %v", sub.PendingChangeAt, validUntil)
	}
	if sub.PendingPeriod == nil || *sub.PendingPeriod != PeriodMonth {
		t.Errorf("PendingPeriod = %v, want %s", sub.PendingPeriod, PeriodMonth)
	}
	if sub.AutoRenewEnabled {
		t.Error("ScheduleDowngrade should not change auto-renew")
	}

	if err := sub.ScheduleDowngrade(currentTariff, Tariff{ID: currentTariffID}, PeriodMonth, validUntil); !errors.Is(err, ErrAlreadyOnTariff) {
		t.Errorf("ScheduleDowngrade same tariff error = %v, want ErrAlreadyOnTariff", err)
	}

	subNoValid := Subscription{TariffID: currentTariffID, Status: SubscriptionStatusActive}
	if err := subNoValid.ScheduleDowngrade(currentTariff, newTariff, PeriodMonth, validUntil); !errors.Is(err, ErrInvalidTariffChange) {
		t.Errorf("ScheduleDowngrade without valid_until error = %v, want ErrInvalidTariffChange", err)
	}

	if err := sub.ScheduleDowngrade(currentTariff, newTariff, "invalid", validUntil); !errors.Is(err, ErrInvalidPeriod) {
		t.Errorf("ScheduleDowngrade invalid period error = %v, want ErrInvalidPeriod", err)
	}

	subCancelled := Subscription{TariffID: currentTariffID, Status: SubscriptionStatusCancelled, ValidUntil: &validUntil}
	if err := subCancelled.ScheduleDowngrade(currentTariff, newTariff, PeriodMonth, validUntil); !errors.Is(err, ErrInvalidSubscriptionState) {
		t.Errorf("ScheduleDowngrade cancelled status error = %v, want ErrInvalidSubscriptionState", err)
	}
}

func TestSubscriptionApplyTariffChange(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	basicID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")
	proID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13")
	pendingID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14")
	paymentID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a19")
	pendingAt := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	sub := Subscription{
		ID:              uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a15"),
		UserID:          userID,
		TariffID:        basicID,
		Status:          SubscriptionStatusActive,
		PendingTariffID: &pendingID,
		PendingChangeAt: &pendingAt,
	}

	basic := Tariff{ID: basicID, Name: TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000}
	pro := Tariff{ID: proID, Name: TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000}
	if err := sub.ApplyTariffChange(paymentID, basic, pro, PeriodMonth, now); err != nil {
		t.Fatalf("ApplyTariffChange() error = %v", err)
	}
	if sub.TariffID != proID {
		t.Errorf("TariffID = %v, want %v", sub.TariffID, proID)
	}
	wantValidUntil := now.AddDate(0, 1, 0)
	if sub.ValidUntil == nil || !sub.ValidUntil.Equal(wantValidUntil) {
		t.Errorf("ValidUntil = %v, want %v", sub.ValidUntil, wantValidUntil)
	}
	if !sub.AutoRenewEnabled {
		t.Error("expected auto-renew enabled")
	}
	if sub.LastAppliedPaymentID == nil || *sub.LastAppliedPaymentID != paymentID {
		t.Errorf("LastAppliedPaymentID = %v, want %v", sub.LastAppliedPaymentID, paymentID)
	}
	if sub.PendingTariffID != nil || sub.PendingChangeAt != nil {
		t.Error("expected pending change cleared")
	}
	if sub.Status != SubscriptionStatusActive {
		t.Errorf("Status = %v, want active", sub.Status)
	}

	if err := sub.ApplyTariffChange(paymentID, pro, Tariff{ID: proID}, PeriodMonth, now); !errors.Is(err, ErrAlreadyOnTariff) {
		t.Errorf("ApplyTariffChange same tariff error = %v, want ErrAlreadyOnTariff", err)
	}
	if err := sub.ApplyTariffChange(paymentID, pro, pro, "invalid", now); !errors.Is(err, ErrInvalidPeriod) {
		t.Errorf("ApplyTariffChange invalid period error = %v, want ErrInvalidPeriod", err)
	}
}

func TestSubscriptionApplyRenewal(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	existingValidUntil := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	tariffID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	paymentID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a18")

	sub := Subscription{
		ID:         uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12"),
		TariffID:   tariffID,
		Status:     SubscriptionStatusActive,
		ValidUntil: &existingValidUntil,
	}

	if err := sub.ApplyRenewal(paymentID, PeriodMonth, now); err != nil {
		t.Fatalf("ApplyRenewal() error = %v", err)
	}
	wantValidUntil := existingValidUntil.AddDate(0, 1, 0)
	if sub.ValidUntil == nil || !sub.ValidUntil.Equal(wantValidUntil) {
		t.Errorf("ValidUntil = %v, want %v", sub.ValidUntil, wantValidUntil)
	}
	if sub.LastAppliedPaymentID == nil || *sub.LastAppliedPaymentID != paymentID {
		t.Errorf("LastAppliedPaymentID = %v, want %v", sub.LastAppliedPaymentID, paymentID)
	}
	if sub.PendingTariffID != nil || sub.PendingChangeAt != nil || sub.PendingPeriod != nil {
		t.Error("expected pending downgrade cleared after renewal")
	}

	// When valid_until is in the past, renewal should start from now.
	past := now.AddDate(0, -1, 0)
	sub2 := Subscription{TariffID: tariffID, ValidUntil: &past}
	if err := sub2.ApplyRenewal(paymentID, PeriodYear, now); err != nil {
		t.Fatalf("ApplyRenewal() error = %v", err)
	}
	wantValidUntil2 := now.AddDate(1, 0, 0)
	if sub2.ValidUntil == nil || !sub2.ValidUntil.Equal(wantValidUntil2) {
		t.Errorf("ValidUntil = %v, want %v", sub2.ValidUntil, wantValidUntil2)
	}
	if sub2.LastAppliedPaymentID == nil || *sub2.LastAppliedPaymentID != paymentID {
		t.Errorf("LastAppliedPaymentID = %v, want %v", sub2.LastAppliedPaymentID, paymentID)
	}

	if err := sub.ApplyRenewal(paymentID, "invalid", now); !errors.Is(err, ErrInvalidPeriod) {
		t.Errorf("ApplyRenewal invalid period error = %v, want ErrInvalidPeriod", err)
	}
}

func TestSubscriptionSetAutoRenew(t *testing.T) {
	validUntil := time.Now().AddDate(0, 1, 0)
	sub := Subscription{AutoRenewEnabled: false, ValidUntil: &validUntil}
	if err := sub.SetAutoRenew(true); err != nil {
		t.Fatalf("SetAutoRenew(true) error: %v", err)
	}
	if !sub.AutoRenewEnabled {
		t.Error("expected auto-renew enabled")
	}
	if err := sub.SetAutoRenew(false); err != nil {
		t.Fatalf("SetAutoRenew(false) error: %v", err)
	}
	if sub.AutoRenewEnabled {
		t.Error("expected auto-renew disabled")
	}

	subNoValidUntil := Subscription{AutoRenewEnabled: false, ValidUntil: nil}
	if err := subNoValidUntil.SetAutoRenew(true); !errors.Is(err, ErrCannotEnableAutoRenew) {
		t.Errorf("SetAutoRenew(true) without ValidUntil error = %v, want ErrCannotEnableAutoRenew", err)
	}
}

func TestSubscriptionDowngradeToBasic(t *testing.T) {
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	basicID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")
	proID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13")
	pendingID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14")
	validUntil := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	pendingAt := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	period := PeriodMonth

	sub := Subscription{
		ID:               uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a15"),
		UserID:           userID,
		TariffID:         proID,
		Status:           SubscriptionStatusGrace,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: true,
		PendingTariffID:  &pendingID,
		PendingChangeAt:  &pendingAt,
		PendingPeriod:    &period,
	}

	sub.DowngradeToBasic(basicID)

	if sub.TariffID != basicID {
		t.Errorf("TariffID = %v, want %v", sub.TariffID, basicID)
	}
	if sub.Status != SubscriptionStatusActive {
		t.Errorf("Status = %v, want %v", sub.Status, SubscriptionStatusActive)
	}
	if sub.ValidUntil != nil {
		t.Errorf("ValidUntil = %v, want nil", sub.ValidUntil)
	}
	if sub.AutoRenewEnabled {
		t.Error("expected auto-renew disabled")
	}
	if sub.PendingTariffID != nil || sub.PendingChangeAt != nil || sub.PendingPeriod != nil {
		t.Error("expected pending change fields cleared")
	}
}

func TestSubscriptionEnterGrace(t *testing.T) {
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	proID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")

	t.Run("extends ValidUntil from nil", func(t *testing.T) {
		sub := Subscription{
			ID:       uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13"),
			UserID:   userID,
			TariffID: proID,
			Status:   SubscriptionStatusActive,
		}

		sub.EnterGrace(now)

		if sub.Status != SubscriptionStatusGrace {
			t.Errorf("Status = %v, want %v", sub.Status, SubscriptionStatusGrace)
		}
		wantValidUntil := now.Add(gracePeriod)
		if sub.ValidUntil == nil || !sub.ValidUntil.Equal(wantValidUntil) {
			t.Errorf("ValidUntil = %v, want %v", sub.ValidUntil, wantValidUntil)
		}
	})

	t.Run("does not shorten already-paid ValidUntil", func(t *testing.T) {
		farFuture := now.Add(30 * 24 * time.Hour)
		sub := Subscription{
			ID:         uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14"),
			UserID:     userID,
			TariffID:   proID,
			Status:     SubscriptionStatusActive,
			ValidUntil: &farFuture,
		}

		sub.EnterGrace(now)

		if sub.Status != SubscriptionStatusGrace {
			t.Errorf("Status = %v, want %v", sub.Status, SubscriptionStatusGrace)
		}
		if sub.ValidUntil == nil || !sub.ValidUntil.Equal(farFuture) {
			t.Errorf("ValidUntil = %v, want %v", sub.ValidUntil, farFuture)
		}
	})
}

func TestSubscriptionApplyScheduledChange(t *testing.T) {
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	basicID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")
	proID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13")
	paymentID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a19")
	validUntil := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	pendingAt := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	pendingPeriod := PeriodMonth

	sub := Subscription{
		ID:                   uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a15"),
		UserID:               userID,
		TariffID:             basicID,
		Source:               SubscriptionSourcePaid,
		Status:               SubscriptionStatusActive,
		ValidUntil:           &validUntil,
		AutoRenewEnabled:     true,
		LastAppliedPaymentID: &paymentID,
		PendingTariffID:      &proID,
		PendingChangeAt:      &pendingAt,
		PendingPeriod:        &pendingPeriod,
	}

	baseValidUntil := sub.ValidUntil
	baseAutoRenew := sub.AutoRenewEnabled
	baseLastPayment := sub.LastAppliedPaymentID
	baseStatus := sub.Status
	baseSource := sub.Source

	pendingTariff := Tariff{ID: proID, Name: TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000}
	if err := sub.ApplyScheduledChange(pendingTariff, PeriodMonth); err != nil {
		t.Fatalf("ApplyScheduledChange() error = %v", err)
	}

	if sub.TariffID != proID {
		t.Errorf("TariffID = %v, want %v", sub.TariffID, proID)
	}
	if sub.CurrentPeriod == nil || *sub.CurrentPeriod != PeriodMonth {
		t.Errorf("CurrentPeriod = %v, want %s", sub.CurrentPeriod, PeriodMonth)
	}
	if sub.PendingTariffID != nil || sub.PendingChangeAt != nil || sub.PendingPeriod != nil {
		t.Error("expected pending change fields cleared")
	}
	if sub.ValidUntil != baseValidUntil {
		t.Errorf("ValidUntil mutated = %v, want %v", sub.ValidUntil, baseValidUntil)
	}
	if sub.AutoRenewEnabled != baseAutoRenew {
		t.Errorf("AutoRenewEnabled mutated = %v, want %v", sub.AutoRenewEnabled, baseAutoRenew)
	}
	if sub.LastAppliedPaymentID != baseLastPayment {
		t.Errorf("LastAppliedPaymentID mutated = %v, want %v", sub.LastAppliedPaymentID, baseLastPayment)
	}
	if sub.Status != baseStatus {
		t.Errorf("Status mutated = %v, want %v", sub.Status, baseStatus)
	}
	if sub.Source != baseSource {
		t.Errorf("Source mutated = %v, want %v", sub.Source, baseSource)
	}

	// An invalid period must be rejected and must not mutate any field.
	baseline := sub
	if err := sub.ApplyScheduledChange(pendingTariff, SubscriptionPeriod("weekly")); !errors.Is(err, ErrInvalidPeriod) {
		t.Errorf("ApplyScheduledChange invalid period error = %v, want ErrInvalidPeriod", err)
	}
	if sub != baseline {
		t.Errorf("ApplyScheduledChange mutated fields on invalid period: got %+v, want %+v", sub, baseline)
	}
}

func TestSubscriptionClearPendingChange(t *testing.T) {
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	proID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")
	pendingID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14")
	validUntil := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	pendingAt := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	period := PeriodMonth
	currentPeriod := PeriodYear

	sub := Subscription{
		ID:               uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a15"),
		UserID:           userID,
		TariffID:         proID,
		Source:           SubscriptionSourcePaid,
		Status:           SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: true,
		PendingTariffID:  &pendingID,
		PendingChangeAt:  &pendingAt,
		PendingPeriod:    &period,
		CurrentPeriod:    &currentPeriod,
	}

	baseline := sub
	sub.ClearPendingChange()

	if sub.PendingTariffID != nil || sub.PendingChangeAt != nil || sub.PendingPeriod != nil {
		t.Error("expected pending change fields cleared")
	}

	// Nothing but the pending_* fields may change.
	baseline.PendingTariffID = nil
	baseline.PendingChangeAt = nil
	baseline.PendingPeriod = nil
	if sub != baseline {
		t.Errorf("ClearPendingChange mutated non-pending fields: got %+v, want %+v", sub, baseline)
	}
}
