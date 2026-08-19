package application

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// The admin tariff management operations of issue #256 on the in-memory
// fakes: creation with validation and the duplicate-name backstop, editing
// prices/limit/activity with audit, and the hidden-tariff selection guard of
// the user ChangeTariff flow.

// tariffHarness bundles the tariff service over in-memory fakes with the
// canonical tariffs and a capture audit recorder.
type tariffHarness struct {
	svc     *TariffService
	stores  *fakeStores
	audit   *captureRecorder
	tariffs []domain.Tariff
}

func newTariffHarness(t *testing.T) *tariffHarness {
	t.Helper()
	tariffs := testTariffs()
	stores := newFakeStores(tariffs...)
	audit := &captureRecorder{}
	return &tariffHarness{svc: NewTariffService(stores.factory(audit), TariffServiceConfig{}), stores: stores, audit: audit, tariffs: tariffs}
}

func (h *tariffHarness) tariffID(t *testing.T, name domain.TariffName) uuid.UUID {
	t.Helper()
	for _, tariff := range h.tariffs {
		if tariff.Name == name {
			return tariff.ID
		}
	}
	t.Fatalf("harness tariffs contain no %q plan", name)
	return uuid.Nil
}

// TestCreateTariff_PersistsValidatedPlanWithAudit proves the admin creation
// (issue #256): the plan lands with a minted id and the requested fields, and
// the audit trail names the acting admin.
func TestCreateTariff_PersistsValidatedPlanWithAudit(t *testing.T) {
	h := newTariffHarness(t)
	// Drop pro so the name is free to create; the closed vocabulary keeps the
	// same name space the user contract pins.
	h.stores.tariffs.tariffs = h.stores.tariffs.tariffs[:1]
	adminID := uuid.Must(uuid.NewV7())

	created, err := h.svc.CreateTariff(t.Context(), adminID, CreateTariffRequest{
		Name:                domain.TariffPro,
		ActivePropertyLimit: 5,
		MonthlyPriceKopecks: 59000,
		YearlyPriceKopecks:  540000,
	})
	if err != nil {
		t.Fatalf("CreateTariff() error = %v", err)
	}
	if created.ID == uuid.Nil || !created.IsActive {
		t.Errorf("created tariff = %+v, want a minted id and an active plan", created)
	}

	stored, err := h.stores.tariffs.GetByName(t.Context(), domain.TariffPro)
	if err != nil {
		t.Fatalf("GetByName() error = %v", err)
	}
	if stored.MonthlyPriceKopecks != 59000 || stored.YearlyPriceKopecks != 540000 || stored.ActivePropertyLimit != 5 {
		t.Errorf("stored tariff = %+v, want the requested pricing", stored)
	}

	entries := h.auditEntries(auditdomain.ActionTariffCreated)
	if len(entries) != 1 {
		t.Fatalf("audit entries = %d, want 1", len(entries))
	}
	if entries[0].ActorID == nil || *entries[0].ActorID != adminID {
		t.Errorf("audit actor = %v, want %v", entries[0].ActorID, adminID)
	}
	if entries[0].EntityType != auditdomain.EntityTariff || entries[0].EntityID == nil || *entries[0].EntityID != created.ID {
		t.Errorf("audit entity = %s/%v, want tariff/%v", entries[0].EntityType, entries[0].EntityID, created.ID)
	}
}

// TestCreateTariff_DuplicateNameRejected proves the unique-name backstop: a
// name that already exists answers ErrAlreadyExists and writes nothing.
func TestCreateTariff_DuplicateNameRejected(t *testing.T) {
	h := newTariffHarness(t)
	before := len(h.stores.tariffs.tariffs)

	_, err := h.svc.CreateTariff(t.Context(), uuid.Must(uuid.NewV7()), CreateTariffRequest{
		Name:                domain.TariffPro,
		ActivePropertyLimit: 5,
		MonthlyPriceKopecks: 59000,
		YearlyPriceKopecks:  540000,
	})
	if !errors.Is(err, ErrTariffAlreadyExists) {
		t.Fatalf("CreateTariff(duplicate) error = %v, want %v", err, ErrTariffAlreadyExists)
	}
	if after := len(h.stores.tariffs.tariffs); after != before {
		t.Errorf("tariffs = %d, want %d (no write)", after, before)
	}
	if entries := h.auditEntries(auditdomain.ActionTariffCreated); len(entries) != 0 {
		t.Errorf("audit entries = %d, want 0", len(entries))
	}
}

// TestCreateTariff_InvalidPricingRejectedBeforeWrite proves validation runs
// before persistence: a negative price answers the domain sentinel and
// nothing is written or audited.
func TestCreateTariff_InvalidPricingRejectedBeforeWrite(t *testing.T) {
	h := newTariffHarness(t)
	h.stores.tariffs.tariffs = h.stores.tariffs.tariffs[:1]
	before := len(h.stores.tariffs.tariffs)

	_, err := h.svc.CreateTariff(t.Context(), uuid.Must(uuid.NewV7()), CreateTariffRequest{
		Name:                domain.TariffPro,
		ActivePropertyLimit: 5,
		MonthlyPriceKopecks: -1,
		YearlyPriceKopecks:  540000,
	})
	if !errors.Is(err, domain.ErrInvalidTariffPricing) {
		t.Fatalf("CreateTariff(invalid) error = %v, want %v", err, domain.ErrInvalidTariffPricing)
	}
	if after := len(h.stores.tariffs.tariffs); after != before {
		t.Errorf("tariffs = %d, want %d (no write)", after, before)
	}
}

// TestUpdateTariff_RewritesPricingLimitAndActivityWithAudit proves the admin
// edit (issue #256): prices, the property limit and the activity flag change
// in one operation, the name stays, and the audit entry captures the
// resulting values.
func TestUpdateTariff_RewritesPricingLimitAndActivityWithAudit(t *testing.T) {
	h := newTariffHarness(t)
	proID := h.tariffID(t, domain.TariffPro)
	adminID := uuid.Must(uuid.NewV7())

	updated, err := h.svc.UpdateTariff(t.Context(), adminID, proID, UpdateTariffRequest{
		ActivePropertyLimit: 7,
		MonthlyPriceKopecks: 59000,
		YearlyPriceKopecks:  540000,
		IsActive:            false,
	})
	if err != nil {
		t.Fatalf("UpdateTariff() error = %v", err)
	}
	if updated.Name != domain.TariffPro || updated.ActivePropertyLimit != 7 ||
		updated.MonthlyPriceKopecks != 59000 || updated.YearlyPriceKopecks != 540000 || updated.IsActive {
		t.Errorf("updated tariff = %+v, want the requested fields and the name kept", updated)
	}

	stored, err := h.stores.tariffs.GetByID(t.Context(), proID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if stored.IsActive || stored.MonthlyPriceKopecks != 59000 {
		t.Errorf("stored tariff = %+v, want hidden with the new price", stored)
	}

	entries := h.auditEntries(auditdomain.ActionTariffUpdated)
	if len(entries) != 1 {
		t.Fatalf("audit entries = %d, want 1", len(entries))
	}
	if entries[0].ActorID == nil || *entries[0].ActorID != adminID {
		t.Errorf("audit actor = %v, want %v", entries[0].ActorID, adminID)
	}
	if entries[0].EntityID == nil || *entries[0].EntityID != proID {
		t.Errorf("audit entity = %v, want %v", entries[0].EntityID, proID)
	}
}

// TestUpdateTariff_MissingTariffRejected proves an unknown id answers
// ErrTariffNotFound without audit.
func TestUpdateTariff_MissingTariffRejected(t *testing.T) {
	h := newTariffHarness(t)

	_, err := h.svc.UpdateTariff(t.Context(), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), UpdateTariffRequest{
		ActivePropertyLimit: 5,
		MonthlyPriceKopecks: 49000,
		YearlyPriceKopecks:  440000,
		IsActive:            true,
	})
	if !errors.Is(err, ErrTariffNotFound) {
		t.Fatalf("UpdateTariff(missing) error = %v, want %v", err, ErrTariffNotFound)
	}
	if entries := h.auditEntries(auditdomain.ActionTariffUpdated); len(entries) != 0 {
		t.Errorf("audit entries = %d, want 0", len(entries))
	}
}

// TestUpdateTariff_InvalidPricingRejectedBeforeWrite proves the edit validates
// like the creation: a limit below -1 answers the domain sentinel and leaves
// the stored plan untouched.
func TestUpdateTariff_InvalidPricingRejectedBeforeWrite(t *testing.T) {
	h := newTariffHarness(t)
	proID := h.tariffID(t, domain.TariffPro)

	_, err := h.svc.UpdateTariff(t.Context(), uuid.Must(uuid.NewV7()), proID, UpdateTariffRequest{
		ActivePropertyLimit: -2,
		MonthlyPriceKopecks: 49000,
		YearlyPriceKopecks:  440000,
		IsActive:            true,
	})
	if !errors.Is(err, domain.ErrInvalidTariffPricing) {
		t.Fatalf("UpdateTariff(invalid) error = %v, want %v", err, domain.ErrInvalidTariffPricing)
	}
	stored, gerr := h.stores.tariffs.GetByID(t.Context(), proID)
	if gerr != nil {
		t.Fatalf("GetByID() error = %v", gerr)
	}
	if stored.ActivePropertyLimit != 5 {
		t.Errorf("stored limit = %d, want 5 (unchanged)", stored.ActivePropertyLimit)
	}
}

// TestChangeTariff_HiddenTariffNotSelectable proves the hiding semantics of
// issue #256 from the user's side: a hidden plan is not offered (ListTariffs)
// and a direct change request against it answers ErrTariffInactive, while the
// plan itself keeps resolving for the subscriptions that already reference it.
func TestChangeTariff_HiddenTariffNotSelectable(t *testing.T) {
	h := newSubscriptionHarness(t)
	businessID := h.tariffID(t, domain.TariffBusiness)
	sub := h.seedPaidSubscription(t, domain.TariffPro)

	// Hide business the way the admin service does: same row, IsActive=false.
	business, err := h.stores.tariffs.GetByName(t.Context(), domain.TariffBusiness)
	if err != nil {
		t.Fatalf("GetByName() error = %v", err)
	}
	business.IsActive = false
	if _, err := h.stores.tariffs.Update(t.Context(), business); err != nil {
		t.Fatalf("hide Update() error = %v", err)
	}

	listed, err := h.stores.tariffs.List(t.Context())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	for _, tariff := range listed {
		if tariff.ID == businessID {
			t.Error("hidden tariff is still offered by List")
		}
	}

	_, err = h.svc.ChangeTariff(t.Context(), sub.UserID, ChangeTariffRequest{
		TariffName: domain.TariffBusiness,
		Period:     domain.PeriodMonth,
	})
	if !errors.Is(err, ErrTariffInactive) {
		t.Fatalf("ChangeTariff(hidden target) error = %v, want %v", err, ErrTariffInactive)
	}

	// The hidden plan keeps resolving for existing references.
	if _, err := h.stores.tariffs.GetByID(t.Context(), businessID); err != nil {
		t.Errorf("GetByID(hidden) error = %v, want the plan to stay referable", err)
	}
}

// auditEntries filters the recorded audit entries by action.
func (h *tariffHarness) auditEntries(action auditdomain.Action) []auditdomain.Entry {
	var found []auditdomain.Entry
	for _, entry := range h.audit.recorded() {
		if entry.Action == action {
			found = append(found, entry)
		}
	}
	return found
}
