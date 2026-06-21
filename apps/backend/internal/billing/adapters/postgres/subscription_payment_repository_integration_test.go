package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

func setupSubscriptionPaymentTest(t *testing.T) (context.Context, pgx.Tx, func(), *SubscriptionPaymentRepository, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	pool := setupIntegrationDB(t)
	ctx, tx, cleanup := beginTx(t, pool)

	q := genpostgres.New(tx)
	userID := createTestUser(t, ctx, q)
	tariff := getTestTariff(t, ctx, q, "pro")
	tariffID := uuid.UUID(tariff.ID.Bytes)

	sub, err := domain.NewOwnerSubscription(userID, tariffID)
	if err != nil {
		t.Fatalf("new subscription: %v", err)
	}
	subscriptionRepo := NewSubscriptionRepository(tx)
	createdSub, err := subscriptionRepo.Create(ctx, sub)
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	pmRepo := NewPaymentMethodRepository(tx, noopEncryptor(t))
	pm, _ := domain.NewPaymentMethod(userID, domain.ProviderFake, "token", "*1", time.Now().UTC())
	pm, _ = pmRepo.Create(ctx, pm)

	repo := NewSubscriptionPaymentRepository(tx)
	return ctx, tx, cleanup, repo, userID, tariffID, createdSub.ID, pm.ID
}

func TestSubscriptionPaymentRepositoryIntegration_CreateAndMarkSucceeded(t *testing.T) {
	ctx, _, cleanup, repo, userID, tariffID, subID, pmID := setupSubscriptionPaymentTest(t)
	defer cleanup()

	payment, err := domain.NewSubscriptionPayment(userID, subID, tariffID, &pmID, domain.PeriodMonth, 49000, domain.ProviderFake, time.Now().UTC())
	if err != nil {
		t.Fatalf("new subscription payment: %v", err)
	}
	created, err := repo.Create(ctx, payment)
	if err != nil {
		t.Fatalf("Create error = %v", err)
	}

	if err := repo.MarkSucceeded(ctx, created.ID, time.Now().UTC()); err != nil {
		t.Fatalf("MarkSucceeded error = %v", err)
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID error = %v", err)
	}
	if got.Status != domain.PaymentStatusSucceeded {
		t.Errorf("Status = %q, want %q", got.Status, domain.PaymentStatusSucceeded)
	}
}

func TestSubscriptionPaymentRepositoryIntegration_CreateAndMarkFailed(t *testing.T) {
	ctx, _, cleanup, repo, userID, tariffID, subID, pmID := setupSubscriptionPaymentTest(t)
	defer cleanup()

	payment, err := domain.NewSubscriptionPayment(userID, subID, tariffID, &pmID, domain.PeriodMonth, 49000, domain.ProviderFake, time.Now().UTC())
	if err != nil {
		t.Fatalf("new subscription payment: %v", err)
	}
	created, err := repo.Create(ctx, payment)
	if err != nil {
		t.Fatalf("Create error = %v", err)
	}

	errorCode := "card_declined"
	if err := repo.MarkFailed(ctx, created.ID, &errorCode, time.Now().UTC()); err != nil {
		t.Fatalf("MarkFailed error = %v", err)
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID error = %v", err)
	}
	if got.Status != domain.PaymentStatusFailed {
		t.Errorf("Status = %q, want %q", got.Status, domain.PaymentStatusFailed)
	}
	if got.ErrorCode == nil || *got.ErrorCode != errorCode {
		t.Errorf("ErrorCode = %v, want %q", got.ErrorCode, errorCode)
	}
}

func TestSubscriptionPaymentRepositoryIntegration_MarkFailed_NilErrorCode(t *testing.T) {
	ctx, _, cleanup, repo, userID, tariffID, subID, pmID := setupSubscriptionPaymentTest(t)
	defer cleanup()

	payment, err := domain.NewSubscriptionPayment(userID, subID, tariffID, &pmID, domain.PeriodMonth, 49000, domain.ProviderFake, time.Now().UTC())
	if err != nil {
		t.Fatalf("new subscription payment: %v", err)
	}
	created, err := repo.Create(ctx, payment)
	if err != nil {
		t.Fatalf("Create error = %v", err)
	}

	if err := repo.MarkFailed(ctx, created.ID, nil, time.Now().UTC()); err != nil {
		t.Fatalf("MarkFailed error = %v", err)
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID error = %v", err)
	}
	if got.Status != domain.PaymentStatusFailed {
		t.Errorf("Status = %q, want %q", got.Status, domain.PaymentStatusFailed)
	}
	if got.ErrorCode != nil {
		t.Errorf("ErrorCode = %v, want nil", got.ErrorCode)
	}
}

func TestSubscriptionPaymentRepositoryIntegration_MarkSucceeded_GuardAlreadySucceeded(t *testing.T) {
	ctx, _, cleanup, repo, userID, tariffID, subID, pmID := setupSubscriptionPaymentTest(t)
	defer cleanup()

	payment, err := domain.NewSubscriptionPayment(userID, subID, tariffID, &pmID, domain.PeriodMonth, 49000, domain.ProviderFake, time.Now().UTC())
	if err != nil {
		t.Fatalf("new subscription payment: %v", err)
	}
	created, err := repo.Create(ctx, payment)
	if err != nil {
		t.Fatalf("Create error = %v", err)
	}

	if err := repo.MarkSucceeded(ctx, created.ID, time.Now().UTC()); err != nil {
		t.Fatalf("first MarkSucceeded error = %v", err)
	}

	err = repo.MarkSucceeded(ctx, created.ID, time.Now().UTC())
	if !errors.Is(err, domain.ErrInvalidPaymentStatus) {
		t.Errorf("second MarkSucceeded error = %v, want ErrInvalidPaymentStatus", err)
	}
}

func TestSubscriptionPaymentRepositoryIntegration_ListByUserID(t *testing.T) {
	ctx, _, cleanup, repo, userID, tariffID, subID, pmID := setupSubscriptionPaymentTest(t)
	defer cleanup()

	payment, err := domain.NewSubscriptionPayment(userID, subID, tariffID, &pmID, domain.PeriodMonth, 49000, domain.ProviderFake, time.Now().UTC())
	if err != nil {
		t.Fatalf("new subscription payment: %v", err)
	}
	created, err := repo.Create(ctx, payment)
	if err != nil {
		t.Fatalf("Create error = %v", err)
	}

	list, err := repo.ListByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUserID error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListByUserID len = %d, want 1", len(list))
	}
	if list[0].ID != created.ID {
		t.Errorf("ListByUserID[0].ID = %v, want %v", list[0].ID, created.ID)
	}
}

func TestSubscriptionPaymentRepositoryIntegration_UpdateProviderPaymentID(t *testing.T) {
	ctx, _, cleanup, repo, userID, tariffID, subID, pmID := setupSubscriptionPaymentTest(t)
	defer cleanup()

	payment, err := domain.NewSubscriptionPayment(userID, subID, tariffID, &pmID, domain.PeriodMonth, 49000, domain.ProviderFake, time.Now().UTC())
	if err != nil {
		t.Fatalf("new subscription payment: %v", err)
	}
	created, err := repo.Create(ctx, payment)
	if err != nil {
		t.Fatalf("Create error = %v", err)
	}

	updated, err := repo.UpdateProviderPaymentID(ctx, created.ID, "provider_123")
	if err != nil {
		t.Fatalf("UpdateProviderPaymentID error = %v", err)
	}
	if updated.ProviderPaymentID == nil || *updated.ProviderPaymentID != "provider_123" {
		t.Errorf("ProviderPaymentID = %v, want %q", updated.ProviderPaymentID, "provider_123")
	}
}

func TestSubscriptionPaymentRepositoryIntegration_GetByID_NotFound(t *testing.T) {
	ctx, _, cleanup, repo, _, _, _, _ := setupSubscriptionPaymentTest(t)
	defer cleanup()

	id := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	_, err := repo.GetByID(ctx, id)
	if !errors.Is(err, application.ErrNotFound) {
		t.Errorf("GetByID error = %v, want ErrNotFound", err)
	}
}

func TestSubscriptionPaymentRepositoryIntegration_ListPendingSubscriptionPaymentsByUserID(t *testing.T) {
	ctx, _, cleanup, repo, userID, tariffID, subID, pmID := setupSubscriptionPaymentTest(t)
	defer cleanup()

	pending, err := domain.NewSubscriptionPayment(userID, subID, tariffID, &pmID, domain.PeriodMonth, 49000, domain.ProviderFake, time.Now().UTC())
	if err != nil {
		t.Fatalf("new subscription payment: %v", err)
	}
	createdPending, err := repo.Create(ctx, pending)
	if err != nil {
		t.Fatalf("Create pending error = %v", err)
	}

	succeeded, err := domain.NewSubscriptionPayment(userID, subID, tariffID, &pmID, domain.PeriodYear, 440000, domain.ProviderFake, time.Now().UTC())
	if err != nil {
		t.Fatalf("new subscription payment: %v", err)
	}
	createdSucceeded, err := repo.Create(ctx, succeeded)
	if err != nil {
		t.Fatalf("Create succeeded error = %v", err)
	}
	if err := repo.MarkSucceeded(ctx, createdSucceeded.ID, time.Now().UTC()); err != nil {
		t.Fatalf("MarkSucceeded error = %v", err)
	}

	list, err := repo.ListPendingSubscriptionPaymentsByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("ListPendingSubscriptionPaymentsByUserID error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListPendingSubscriptionPaymentsByUserID len = %d, want 1", len(list))
	}
	if list[0].ID != createdPending.ID {
		t.Errorf("ListPendingSubscriptionPaymentsByUserID[0].ID = %v, want %v", list[0].ID, createdPending.ID)
	}
}
