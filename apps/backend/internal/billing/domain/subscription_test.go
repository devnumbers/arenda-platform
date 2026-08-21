package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// graceDuration mirrors the production default (ADR 0008) for the tests below.
const graceDuration = 7 * 24 * time.Hour

func TestNewBasicSubscription(t *testing.T) {
	t.Parallel()
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	tariffID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")

	sub, err := NewBasicSubscription(userID, tariffID)
	if err != nil {
		t.Fatalf("NewBasicSubscription() error = %v", err)
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
	t.Parallel()
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
			t.Parallel()
			sub := Subscription{Status: tt.status, ValidUntil: tt.validUntil}
			if got := sub.CanMutateData(now); got != tt.want {
				t.Errorf("CanMutateData() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubscriptionIsPaidSource(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			sub := Subscription{Source: tt.source}
			if got := sub.IsPaidSource(); got != tt.want {
				t.Errorf("IsPaidSource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubscriptionIsInGrace(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			sub := Subscription{Status: tt.status, ValidUntil: tt.validUntil}
			if got := sub.IsInGrace(now); got != tt.want {
				t.Errorf("IsInGrace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubscriptionHasPendingChange(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			sub := Subscription{PendingTariffID: tt.pendingTariffID, PendingChangeAt: tt.pendingChangeAt}
			if got := sub.HasPendingChange(); got != tt.want {
				t.Errorf("HasPendingChange() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubscriptionScheduleDowngrade(t *testing.T) {
	t.Parallel()
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
	if !sub.AutoRenewEnabled {
		t.Error("ScheduleDowngrade should enable auto-renew")
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

	subGrace := Subscription{TariffID: currentTariffID, Status: SubscriptionStatusGrace, ValidUntil: &validUntil}
	if err := subGrace.ScheduleDowngrade(currentTariff, newTariff, PeriodMonth, validUntil); !errors.Is(err, ErrInvalidSubscriptionState) {
		t.Errorf("ScheduleDowngrade grace status error = %v, want ErrInvalidSubscriptionState", err)
	}
}

// assertAppliedTariffChange checks the subscription shape after a successful
// tariff change: the new tariff, the paid validity, auto-renew on, the
// applied payment, the current period, and the pending change cleared.
func assertAppliedTariffChange(
	t *testing.T, sub *Subscription, newTariffID, paymentID uuid.UUID, wantValidUntil time.Time,
) {
	t.Helper()
	if sub.TariffID != newTariffID {
		t.Errorf("TariffID = %v, want %v", sub.TariffID, newTariffID)
	}
	if sub.ValidUntil == nil || !sub.ValidUntil.Equal(wantValidUntil) {
		t.Errorf("ValidUntil = %v, want %v", sub.ValidUntil, wantValidUntil)
	}
	if !sub.AutoRenewEnabled {
		t.Error("expected auto-renew enabled")
	}
	if sub.LastAppliedPaymentID == nil || *sub.LastAppliedPaymentID != paymentID {
		t.Errorf("LastAppliedPaymentID = %v, want %v", sub.LastAppliedPaymentID, paymentID)
	}
	if sub.CurrentPeriod == nil || *sub.CurrentPeriod != PeriodMonth {
		t.Errorf("CurrentPeriod = %v, want %s", sub.CurrentPeriod, PeriodMonth)
	}
	if sub.PendingTariffID != nil || sub.PendingChangeAt != nil || sub.PendingPeriod != nil {
		t.Error("expected pending change cleared")
	}
	if sub.Status != SubscriptionStatusActive {
		t.Errorf("Status = %v, want active", sub.Status)
	}
}

func TestSubscriptionApplyTariffChange(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	basicID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")
	proID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13")
	pendingID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14")
	paymentID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a19")
	pendingAt := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	pendingPeriod := PeriodMonth

	sub := Subscription{
		ID:              uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a15"),
		UserID:          userID,
		TariffID:        basicID,
		Status:          SubscriptionStatusActive,
		PendingTariffID: &pendingID,
		PendingChangeAt: &pendingAt,
		PendingPeriod:   &pendingPeriod,
	}

	basic := Tariff{ID: basicID, Name: TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000}
	pro := Tariff{ID: proID, Name: TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000}
	if err := sub.ApplyTariffChange(paymentID, basic, pro, PeriodMonth, now); err != nil {
		t.Fatalf("ApplyTariffChange() error = %v", err)
	}
	assertAppliedTariffChange(t, &sub, proID, paymentID, now.AddDate(0, 1, 0))

	if err := sub.ApplyTariffChange(paymentID, pro, Tariff{ID: proID}, PeriodMonth, now); !errors.Is(err, ErrAlreadyOnTariff) {
		t.Errorf("ApplyTariffChange same tariff error = %v, want ErrAlreadyOnTariff", err)
	}
	if err := sub.ApplyTariffChange(paymentID, pro, pro, "invalid", now); !errors.Is(err, ErrInvalidPeriod) {
		t.Errorf("ApplyTariffChange invalid period error = %v, want ErrInvalidPeriod", err)
	}
}

// renewalStacksOntoExistingValidity proves a renewal on a still-valid
// subscription extends the remaining validity by the paid period and clears
// a pending downgrade.
func renewalStacksOntoExistingValidity(
	t *testing.T, tariffID, paymentID uuid.UUID, now, existingValidUntil time.Time,
) {
	t.Helper()
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
	if sub.CurrentPeriod == nil || *sub.CurrentPeriod != PeriodMonth {
		t.Errorf("CurrentPeriod = %v, want %s", sub.CurrentPeriod, PeriodMonth)
	}
	if sub.PendingTariffID != nil || sub.PendingChangeAt != nil || sub.PendingPeriod != nil {
		t.Error("expected pending downgrade cleared after renewal")
	}
}

// renewalFromPastValidityStartsAtNow proves that when valid_until is in the
// past, renewal starts from now.
func renewalFromPastValidityStartsAtNow(t *testing.T, tariffID, paymentID uuid.UUID, now time.Time) {
	t.Helper()
	past := now.AddDate(0, -1, 0)
	sub := Subscription{TariffID: tariffID, ValidUntil: &past}
	if err := sub.ApplyRenewal(paymentID, PeriodYear, now); err != nil {
		t.Fatalf("ApplyRenewal() error = %v", err)
	}
	wantValidUntil := now.AddDate(1, 0, 0)
	if sub.ValidUntil == nil || !sub.ValidUntil.Equal(wantValidUntil) {
		t.Errorf("ValidUntil = %v, want %v", sub.ValidUntil, wantValidUntil)
	}
	if sub.LastAppliedPaymentID == nil || *sub.LastAppliedPaymentID != paymentID {
		t.Errorf("LastAppliedPaymentID = %v, want %v", sub.LastAppliedPaymentID, paymentID)
	}
	if sub.CurrentPeriod == nil || *sub.CurrentPeriod != PeriodYear {
		t.Errorf("CurrentPeriod = %v, want %s", sub.CurrentPeriod, PeriodYear)
	}
}

// renewalFromGraceStartsAtNow proves a renewal paid during grace starts from
// the payment moment, not from the grace end: grace is not paid time and must
// not stack onto the new period.
func renewalFromGraceStartsAtNow(
	t *testing.T, tariffID, paymentID uuid.UUID, now, existingValidUntil time.Time,
) {
	t.Helper()
	sub := Subscription{TariffID: tariffID, Status: SubscriptionStatusGrace, ValidUntil: &existingValidUntil}
	if err := sub.ApplyRenewal(paymentID, PeriodMonth, now); err != nil {
		t.Fatalf("ApplyRenewal() error = %v", err)
	}
	wantValidUntil := now.AddDate(0, 1, 0)
	if sub.ValidUntil == nil || !sub.ValidUntil.Equal(wantValidUntil) {
		t.Errorf("ValidUntil = %v, want %v", sub.ValidUntil, wantValidUntil)
	}
	if sub.CurrentPeriod == nil || *sub.CurrentPeriod != PeriodMonth {
		t.Errorf("CurrentPeriod = %v, want %s", sub.CurrentPeriod, PeriodMonth)
	}
}

func TestSubscriptionApplyRenewal(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	existingValidUntil := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	tariffID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	paymentID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a18")

	renewalStacksOntoExistingValidity(t, tariffID, paymentID, now, existingValidUntil)
	renewalFromPastValidityStartsAtNow(t, tariffID, paymentID, now)
	renewalFromGraceStartsAtNow(t, tariffID, paymentID, now, existingValidUntil)

	sub := Subscription{TariffID: tariffID, Status: SubscriptionStatusActive, ValidUntil: &existingValidUntil}
	if err := sub.ApplyRenewal(paymentID, "invalid", now); !errors.Is(err, ErrInvalidPeriod) {
		t.Errorf("ApplyRenewal invalid period error = %v, want ErrInvalidPeriod", err)
	}
}

func TestSubscriptionSetAutoRenew(t *testing.T) {
	t.Parallel()
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

	// A cancelled subscription never renews: restoration goes through paying
	// for a tariff (ADR 0008).
	subCancelled := Subscription{Status: SubscriptionStatusCancelled, ValidUntil: &validUntil}
	if err := subCancelled.SetAutoRenew(true); !errors.Is(err, ErrInvalidSubscriptionState) {
		t.Errorf("SetAutoRenew(true) on cancelled error = %v, want ErrInvalidSubscriptionState", err)
	}
}

func TestSubscriptionCancel(t *testing.T) {
	t.Parallel()
	validUntil := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	pendingAt := validUntil
	period := PeriodMonth
	pendingID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14")

	sub := Subscription{
		Status:           SubscriptionStatusActive,
		ValidUntil:       &validUntil,
		AutoRenewEnabled: true,
		PendingTariffID:  &pendingID,
		PendingChangeAt:  &pendingAt,
		PendingPeriod:    &period,
	}

	if err := sub.Cancel(); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if sub.Status != SubscriptionStatusCancelled {
		t.Errorf("Status = %q, want cancelled", sub.Status)
	}
	if sub.AutoRenewEnabled {
		t.Error("expected auto-renew disabled")
	}
	if sub.ValidUntil == nil || !sub.ValidUntil.Equal(validUntil) {
		t.Errorf("ValidUntil = %v, want retained %v", sub.ValidUntil, validUntil)
	}
	// A cancelled subscription no longer switches tariffs: the scheduled
	// change is dropped and the expiry path moves the user to basic.
	if sub.PendingTariffID != nil || sub.PendingChangeAt != nil || sub.PendingPeriod != nil {
		t.Error("expected pending change fields cleared on cancel")
	}

	cancelled := Subscription{Status: SubscriptionStatusCancelled}
	if err := cancelled.Cancel(); !errors.Is(err, ErrInvalidSubscriptionState) {
		t.Errorf("second Cancel() error = %v, want ErrInvalidSubscriptionState", err)
	}
}

func TestSubscriptionDowngradeToBasic(t *testing.T) {
	t.Parallel()
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
		CurrentPeriod:    &period,
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
	if sub.CurrentPeriod != nil {
		t.Errorf("CurrentPeriod = %v, want nil", sub.CurrentPeriod)
	}
}

func TestSubscriptionEnterGrace(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	proID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")

	t.Run("extends ValidUntil from nil", func(t *testing.T) {
		t.Parallel()
		sub := Subscription{
			ID:       uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13"),
			UserID:   userID,
			TariffID: proID,
			Status:   SubscriptionStatusActive,
		}

		sub.EnterGrace(now, graceDuration)

		if sub.Status != SubscriptionStatusGrace {
			t.Errorf("Status = %v, want %v", sub.Status, SubscriptionStatusGrace)
		}
		wantValidUntil := now.Add(graceDuration)
		if sub.ValidUntil == nil || !sub.ValidUntil.Equal(wantValidUntil) {
			t.Errorf("ValidUntil = %v, want %v", sub.ValidUntil, wantValidUntil)
		}
	})

	t.Run("does not shorten already-paid ValidUntil", func(t *testing.T) {
		t.Parallel()
		farFuture := now.Add(30 * 24 * time.Hour)
		sub := Subscription{
			ID:         uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14"),
			UserID:     userID,
			TariffID:   proID,
			Status:     SubscriptionStatusActive,
			ValidUntil: &farFuture,
		}

		sub.EnterGrace(now, graceDuration)

		if sub.Status != SubscriptionStatusGrace {
			t.Errorf("Status = %v, want %v", sub.Status, SubscriptionStatusGrace)
		}
		if sub.ValidUntil == nil || !sub.ValidUntil.Equal(farFuture) {
			t.Errorf("ValidUntil = %v, want %v", sub.ValidUntil, farFuture)
		}
	})

	t.Run("clears pending scheduled change", func(t *testing.T) {
		t.Parallel()
		pendingID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a15")
		validUntil := now.Add(-time.Hour)
		pendingAt := validUntil
		period := PeriodYear
		sub := Subscription{
			ID:              uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a16"),
			UserID:          userID,
			TariffID:        proID,
			Status:          SubscriptionStatusActive,
			ValidUntil:      &validUntil,
			PendingTariffID: &pendingID,
			PendingChangeAt: &pendingAt,
			PendingPeriod:   &period,
		}

		sub.EnterGrace(now, graceDuration)

		if sub.Status != SubscriptionStatusGrace {
			t.Errorf("Status = %v, want %v", sub.Status, SubscriptionStatusGrace)
		}
		if sub.PendingTariffID != nil || sub.PendingChangeAt != nil || sub.PendingPeriod != nil {
			t.Errorf("expected pending change fields cleared, got %+v/%+v/%+v",
				sub.PendingTariffID, sub.PendingChangeAt, sub.PendingPeriod)
		}
		wantValidUntil := now.Add(graceDuration)
		if sub.ValidUntil == nil || !sub.ValidUntil.Equal(wantValidUntil) {
			t.Errorf("ValidUntil = %v, want %v", sub.ValidUntil, wantValidUntil)
		}
	})
}

// assertAppliedScheduledDowngrade checks the subscription shape after a
// successful scheduled downgrade: the basic tariff, the free period's
// validity, auto-renew kept on, the pending change cleared, and the last
// applied payment untouched.
func assertAppliedScheduledDowngrade(
	t *testing.T, sub Subscription, baseLastPayment *uuid.UUID,
	basicID uuid.UUID, period SubscriptionPeriod, wantValidUntil time.Time,
) {
	t.Helper()
	if sub.TariffID != basicID {
		t.Errorf("TariffID = %v, want %v", sub.TariffID, basicID)
	}
	if sub.ValidUntil == nil || !sub.ValidUntil.Equal(wantValidUntil) {
		t.Errorf("ValidUntil = %v, want %v", sub.ValidUntil, wantValidUntil)
	}
	if !sub.AutoRenewEnabled {
		t.Error("expected auto-renew enabled")
	}
	if sub.Status != SubscriptionStatusActive {
		t.Errorf("Status = %v, want %v", sub.Status, SubscriptionStatusActive)
	}
	if sub.PendingTariffID != nil || sub.PendingChangeAt != nil || sub.PendingPeriod != nil {
		t.Error("expected pending change fields cleared")
	}
	if sub.CurrentPeriod == nil || *sub.CurrentPeriod != period {
		t.Errorf("CurrentPeriod = %v, want %s", sub.CurrentPeriod, period)
	}
	if sub.LastAppliedPaymentID != baseLastPayment {
		t.Errorf("LastAppliedPaymentID mutated = %v, want %v", sub.LastAppliedPaymentID, baseLastPayment)
	}
}

func TestSubscriptionApplyScheduledDowngrade(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	basicID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")
	proID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13")
	paymentID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a19")
	validUntil := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	basic := Tariff{ID: basicID, Name: TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000}

	newSub := func(period SubscriptionPeriod) Subscription {
		pendingTariffID := basicID
		pendingChangeAt := validUntil
		pendingPeriodCopy := period
		lastPayment := paymentID
		return Subscription{
			ID:                   uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a15"),
			UserID:               userID,
			TariffID:             proID,
			Source:               SubscriptionSourcePaid,
			Status:               SubscriptionStatusActive,
			ValidUntil:           &validUntil,
			AutoRenewEnabled:     true,
			LastAppliedPaymentID: &lastPayment,
			PendingTariffID:      &pendingTariffID,
			PendingChangeAt:      &pendingChangeAt,
			PendingPeriod:        &pendingPeriodCopy,
		}
	}

	tests := []struct {
		name           string
		period         SubscriptionPeriod
		wantValidUntil time.Time
	}{
		{"month", PeriodMonth, now.AddDate(0, 1, 0)},
		{"year", PeriodYear, now.AddDate(1, 0, 0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sub := newSub(tt.period)
			baseLastPayment := sub.LastAppliedPaymentID

			if err := sub.ApplyScheduledDowngrade(basic, tt.period, now); err != nil {
				t.Fatalf("ApplyScheduledDowngrade() error = %v", err)
			}

			assertAppliedScheduledDowngrade(t, sub, baseLastPayment, basicID, tt.period, tt.wantValidUntil)
		})
	}

	// An invalid period must be rejected and must not mutate any field.
	subInvalid := newSub(PeriodMonth)
	baseline := subInvalid
	if err := subInvalid.ApplyScheduledDowngrade(basic, SubscriptionPeriod("weekly"), now); !errors.Is(err, ErrInvalidPeriod) {
		t.Errorf("ApplyScheduledDowngrade invalid period error = %v, want ErrInvalidPeriod", err)
	}
	if subInvalid != baseline {
		t.Errorf("ApplyScheduledDowngrade mutated fields on invalid period: got %+v, want %+v", subInvalid, baseline)
	}

	// Applying the tariff the subscription is already on must be rejected and
	// must not mutate any field.
	subSame := newSub(PeriodMonth)
	baseline = subSame
	pro := Tariff{ID: proID, Name: TariffPro, ActivePropertyLimit: 50, MonthlyPriceKopecks: 5000}
	if err := subSame.ApplyScheduledDowngrade(pro, PeriodMonth, now); !errors.Is(err, ErrAlreadyOnTariff) {
		t.Errorf("ApplyScheduledDowngrade same tariff error = %v, want ErrAlreadyOnTariff", err)
	}
	if subSame != baseline {
		t.Errorf("ApplyScheduledDowngrade mutated fields on same tariff: got %+v, want %+v", subSame, baseline)
	}
}

func TestSubscriptionClearPendingChange(t *testing.T) {
	t.Parallel()
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	proID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")
	pendingID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14")
	validUntil := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	pendingAt := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	period := PeriodMonth

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

func TestSubscriptionApplyScheduledDowngradePreconditions(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	basicID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")
	proID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13")
	otherID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14")
	validUntil := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	pendingAt := now.Add(-time.Hour)
	otherPeriod := PeriodYear

	basic := Tariff{ID: basicID, Name: TariffBasic, ActivePropertyLimit: 5, MonthlyPriceKopecks: 1000}

	validSub := func() Subscription {
		pendingTariffID := basicID
		pendingChangeAt := pendingAt
		pendingPeriod := PeriodMonth
		return Subscription{
			ID:              uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a15"),
			UserID:          userID,
			TariffID:        proID,
			Source:          SubscriptionSourcePaid,
			Status:          SubscriptionStatusActive,
			ValidUntil:      &validUntil,
			PendingTariffID: &pendingTariffID,
			PendingChangeAt: &pendingChangeAt,
			PendingPeriod:   &pendingPeriod,
		}
	}

	tests := []struct {
		name   string
		mutate func(*Subscription)
	}{
		{"status grace", func(s *Subscription) { s.Status = SubscriptionStatusGrace }},
		{"status cancelled", func(s *Subscription) { s.Status = SubscriptionStatusCancelled }},
		{"pending tariff mismatch", func(s *Subscription) { s.PendingTariffID = &otherID }},
		{"no pending tariff", func(s *Subscription) { s.PendingTariffID = nil }},
		{"pending period mismatch", func(s *Subscription) { s.PendingPeriod = &otherPeriod }},
		{"no pending period", func(s *Subscription) { s.PendingPeriod = nil }},
		{"pending change in the future", func(s *Subscription) { future := now.Add(time.Hour); s.PendingChangeAt = &future }},
		{"no pending change at", func(s *Subscription) { s.PendingChangeAt = nil }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sub := validSub()
			tt.mutate(&sub)
			before := sub

			if err := sub.ApplyScheduledDowngrade(basic, PeriodMonth, now); !errors.Is(err, ErrInvalidSubscriptionState) {
				t.Errorf("ApplyScheduledDowngrade() error = %v, want ErrInvalidSubscriptionState", err)
			}
			if sub != before {
				t.Errorf("ApplyScheduledDowngrade mutated fields on precondition failure: got %+v, want %+v", sub, before)
			}
		})
	}
}

func TestReconstituteSubscription(t *testing.T) {
	t.Parallel()
	id := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")
	tariffID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13")
	paymentMethodID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a15")
	validUntil := time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)

	validSub := func() Subscription {
		return Subscription{
			ID:                    id,
			UserID:                userID,
			TariffID:              tariffID,
			Source:                SubscriptionSourcePaid,
			Status:                SubscriptionStatusActive,
			ValidUntil:            &validUntil,
			AutoRenewEnabled:      true,
			ActivePaymentMethodID: &paymentMethodID,
		}
	}

	t.Run("accepts valid aggregate", func(t *testing.T) {
		t.Parallel()
		want := validSub()
		got, err := ReconstituteSubscription(want)
		if err != nil {
			t.Fatalf("ReconstituteSubscription() error = %v", err)
		}
		if got != want {
			t.Errorf("ReconstituteSubscription() = %+v, want %+v", got, want)
		}
	})

	t.Run("accepts valid pending period", func(t *testing.T) {
		t.Parallel()
		sub := validSub()
		period := PeriodMonth
		sub.PendingPeriod = &period
		got, err := ReconstituteSubscription(sub)
		if err != nil {
			t.Fatalf("ReconstituteSubscription() error = %v", err)
		}
		if got.PendingPeriod == nil || *got.PendingPeriod != PeriodMonth {
			t.Errorf("ReconstituteSubscription() PendingPeriod = %v, want %v", got.PendingPeriod, PeriodMonth)
		}
	})

	t.Run("accepts valid current period", func(t *testing.T) {
		t.Parallel()
		sub := validSub()
		period := PeriodYear
		sub.CurrentPeriod = &period
		got, err := ReconstituteSubscription(sub)
		if err != nil {
			t.Fatalf("ReconstituteSubscription() error = %v", err)
		}
		if got.CurrentPeriod == nil || *got.CurrentPeriod != PeriodYear {
			t.Errorf("ReconstituteSubscription() CurrentPeriod = %v, want %v", got.CurrentPeriod, PeriodYear)
		}
	})

	tests := []struct {
		name   string
		mutate func(*Subscription)
	}{
		{"unknown status", func(s *Subscription) { s.Status = SubscriptionStatus("paused") }},
		{"empty status", func(s *Subscription) { s.Status = "" }},
		{"unknown source", func(s *Subscription) { s.Source = SubscriptionSource("trial") }},
		{"unknown pending period", func(s *Subscription) {
			period := SubscriptionPeriod("quarter")
			s.PendingPeriod = &period
		}},
		{"unknown current period", func(s *Subscription) {
			period := SubscriptionPeriod("quarter")
			s.CurrentPeriod = &period
		}},
		{"missing id", func(s *Subscription) { s.ID = uuid.Nil }},
		{"missing user id", func(s *Subscription) { s.UserID = uuid.Nil }},
		{"missing tariff id", func(s *Subscription) { s.TariffID = uuid.Nil }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sub := validSub()
			tt.mutate(&sub)
			if _, err := ReconstituteSubscription(sub); err == nil {
				t.Error("ReconstituteSubscription() error = nil, want error")
			}
		})
	}
}

// TestSubscriptionAssignService proves the admin service-subscription
// assignment (issue #255): the tariff runs a fixed term without payment,
// auto-renew is off, and every planning field of the overwritten subscription
// is cleared while the active payment method survives.
func TestSubscriptionAssignService(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	oldValidUntil := now.AddDate(0, 0, 10)
	termUntil := now.AddDate(0, 1, 0)
	businessID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13")
	pendingTariffID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14")
	methodID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a15")
	pendingPeriod := PeriodMonth
	remindedAt := now.Add(-24 * time.Hour)

	sub := Subscription{
		ID:               uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"),
		UserID:           uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12"),
		TariffID:         uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a16"),
		Source:           SubscriptionSourcePaid,
		Status:           SubscriptionStatusActive,
		ValidUntil:       &oldValidUntil,
		AutoRenewEnabled: true,
		PendingTariffID:  &pendingTariffID,
		PendingChangeAt:  &oldValidUntil,
		PendingPeriod:    &pendingPeriod,
		CurrentPeriod:    &pendingPeriod,
		GraceRemindedAt:  &remindedAt,
	}
	sub.SetActivePaymentMethod(methodID)

	sub.AssignService(businessID, termUntil)

	if sub.TariffID != businessID {
		t.Errorf("TariffID = %v, want %v", sub.TariffID, businessID)
	}
	if sub.Source != SubscriptionSourceService {
		t.Errorf("Source = %v, want service", sub.Source)
	}
	if sub.Status != SubscriptionStatusActive {
		t.Errorf("Status = %v, want active", sub.Status)
	}
	if sub.ValidUntil == nil || !sub.ValidUntil.Equal(termUntil) {
		t.Errorf("ValidUntil = %v, want %v", sub.ValidUntil, termUntil)
	}
	if sub.AutoRenewEnabled {
		t.Error("AutoRenewEnabled = true, want false")
	}
	if sub.PendingTariffID != nil || sub.PendingChangeAt != nil || sub.PendingPeriod != nil {
		t.Error("expected pending change cleared")
	}
	if sub.CurrentPeriod != nil {
		t.Errorf("CurrentPeriod = %v, want nil", sub.CurrentPeriod)
	}
	if sub.GraceRemindedAt != nil {
		t.Errorf("GraceRemindedAt = %v, want nil", sub.GraceRemindedAt)
	}
	if sub.ActivePaymentMethodID == nil || *sub.ActivePaymentMethodID != methodID {
		t.Errorf("ActivePaymentMethodID = %v, want the surviving method", sub.ActivePaymentMethodID)
	}
}

// TestSubscriptionForceApplyTariffChange proves the admin force change
// (issue #255): the new tariff applies immediately for the chosen period, the
// source and auto-renew setting keep their value, and a cancelled subscription
// is out of scope.
func TestSubscriptionForceApplyTariffChange(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	fromID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	toID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")

	sub := Subscription{
		ID:               uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13"),
		UserID:           uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14"),
		TariffID:         fromID,
		Source:           SubscriptionSourceService,
		Status:           SubscriptionStatusActive,
		AutoRenewEnabled: false,
	}
	if err := sub.ForceApplyTariffChange(toID, PeriodYear, now); err != nil {
		t.Fatalf("ForceApplyTariffChange() error = %v", err)
	}
	if sub.TariffID != toID {
		t.Errorf("TariffID = %v, want %v", sub.TariffID, toID)
	}
	wantValidUntil := now.AddDate(1, 0, 0)
	if sub.ValidUntil == nil || !sub.ValidUntil.Equal(wantValidUntil) {
		t.Errorf("ValidUntil = %v, want %v", sub.ValidUntil, wantValidUntil)
	}
	if sub.Status != SubscriptionStatusActive {
		t.Errorf("Status = %v, want active", sub.Status)
	}
	if sub.Source != SubscriptionSourceService {
		t.Errorf("Source = %v, want the unchanged service source", sub.Source)
	}
	if sub.AutoRenewEnabled {
		t.Error("AutoRenewEnabled = true, want the unchanged value false")
	}
	if sub.CurrentPeriod == nil || *sub.CurrentPeriod != PeriodYear {
		t.Errorf("CurrentPeriod = %v, want year", sub.CurrentPeriod)
	}

	if err := sub.ForceApplyTariffChange(toID, PeriodMonth, now); !errors.Is(err, ErrAlreadyOnTariff) {
		t.Errorf("ForceApplyTariffChange same tariff error = %v, want ErrAlreadyOnTariff", err)
	}
	if err := sub.ForceApplyTariffChange(fromID, "quarter", now); !errors.Is(err, ErrInvalidPeriod) {
		t.Errorf("ForceApplyTariffChange invalid period error = %v, want ErrInvalidPeriod", err)
	}
	sub.Status = SubscriptionStatusCancelled
	if err := sub.ForceApplyTariffChange(fromID, PeriodMonth, now); !errors.Is(err, ErrInvalidSubscriptionState) {
		t.Errorf("ForceApplyTariffChange cancelled error = %v, want ErrInvalidSubscriptionState", err)
	}
}

// TestSubscriptionForceApplyTariffChangeFromGrace proves a grace subscription
// leaves grace when the admin force-changes its tariff (issue #255): the
// repaired subscription is active for the new period.
func TestSubscriptionForceApplyTariffChangeFromGrace(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	fromID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	toID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")
	graceUntil := now.Add(2 * 24 * time.Hour)
	remindedAt := now.Add(-12 * time.Hour)

	sub := Subscription{
		ID:              uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13"),
		UserID:          uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14"),
		TariffID:        fromID,
		Source:          SubscriptionSourcePaid,
		Status:          SubscriptionStatusGrace,
		ValidUntil:      &graceUntil,
		GraceRemindedAt: &remindedAt,
	}
	if err := sub.ForceApplyTariffChange(toID, PeriodMonth, now); err != nil {
		t.Fatalf("ForceApplyTariffChange() error = %v", err)
	}
	if sub.Status != SubscriptionStatusActive {
		t.Errorf("Status = %v, want active", sub.Status)
	}
	if sub.GraceRemindedAt != nil {
		t.Errorf("GraceRemindedAt = %v, want nil", sub.GraceRemindedAt)
	}
	if !sub.IsPaidSource() {
		t.Error("expected the unchanged paid source")
	}
}

// TestSubscriptionExtendGrace proves the manual grace extension (issue #255):
// the window lengthens from the later of now and the current deadline, and
// only a subscription in grace qualifies.
func TestSubscriptionExtendGrace(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	graceUntil := now.Add(48 * time.Hour)
	remindedAt := now.Add(-6 * time.Hour)

	sub := Subscription{
		ID:              uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"),
		UserID:          uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12"),
		TariffID:        uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13"),
		Status:          SubscriptionStatusGrace,
		ValidUntil:      &graceUntil,
		GraceRemindedAt: &remindedAt,
	}

	// The deadline is still ahead: the extension stacks on it.
	if err := sub.ExtendGrace(now, 3*24*time.Hour); err != nil {
		t.Fatalf("ExtendGrace() error = %v", err)
	}
	want := graceUntil.Add(3 * 24 * time.Hour)
	if sub.ValidUntil == nil || !sub.ValidUntil.Equal(want) {
		t.Errorf("ValidUntil = %v, want %v", sub.ValidUntil, want)
	}
	if sub.GraceRemindedAt != nil {
		t.Errorf("GraceRemindedAt = %v, want nil for the fresh reminder window", sub.GraceRemindedAt)
	}

	// The window has expired but the worker has not closed it: the extension
	// counts from now, so the user still gets the full extra time.
	later := want.Add(6 * time.Hour)
	if err := sub.ExtendGrace(later, 24*time.Hour); err != nil {
		t.Fatalf("ExtendGrace(expired window) error = %v", err)
	}
	wantFromNow := later.Add(24 * time.Hour)
	if sub.ValidUntil == nil || !sub.ValidUntil.Equal(wantFromNow) {
		t.Errorf("ValidUntil = %v, want %v", sub.ValidUntil, wantFromNow)
	}

	// Only a grace subscription can be extended.
	sub.Status = SubscriptionStatusActive
	if err := sub.ExtendGrace(later, 24*time.Hour); !errors.Is(err, ErrInvalidSubscriptionState) {
		t.Errorf("ExtendGrace(active) error = %v, want ErrInvalidSubscriptionState", err)
	}
}

// TestSubscriptionPaymentFlipsServiceSourceToPaid proves the upgrade-over-
// service rule (issue #255): both a tariff change and a renewal applied by a
// succeeded payment put the subscription on the paid track.
func TestSubscriptionPaymentFlipsServiceSourceToPaid(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	basicID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	proID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")
	paymentID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13")
	basic := Tariff{ID: basicID, Name: TariffBasic, ActivePropertyLimit: 1}
	pro := Tariff{ID: proID, Name: TariffPro, ActivePropertyLimit: 5, MonthlyPriceKopecks: 49000}

	sub := Subscription{
		ID:       uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14"),
		UserID:   uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a15"),
		TariffID: basicID,
		Source:   SubscriptionSourceService,
		Status:   SubscriptionStatusActive,
	}
	if err := sub.ApplyTariffChange(paymentID, basic, pro, PeriodMonth, now); err != nil {
		t.Fatalf("ApplyTariffChange() error = %v", err)
	}
	if sub.Source != SubscriptionSourcePaid {
		t.Errorf("Source = %v, want paid after the applied upgrade", sub.Source)
	}
	if !sub.AutoRenewEnabled {
		t.Error("expected auto-renew enabled after the applied upgrade")
	}

	sub.Source = SubscriptionSourceService
	sub.TariffID = proID
	if err := sub.ApplyRenewal(paymentID, PeriodMonth, now); err != nil {
		t.Fatalf("ApplyRenewal() error = %v", err)
	}
	if sub.Source != SubscriptionSourcePaid {
		t.Errorf("Source = %v, want paid after the applied renewal", sub.Source)
	}
}
