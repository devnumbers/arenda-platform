package postgres

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

func TestPaymentMethodRepositoryIntegration_Create(t *testing.T) {
	pool := setupIntegrationDB(t)
	ctx, tx, cleanup := beginTx(t, pool)
	defer cleanup()

	q := genpostgres.New(tx)
	userID := createTestUser(t, ctx, q)
	repo := NewPaymentMethodRepository(tx, noopEncryptor(t))

	pm, err := domain.NewPaymentMethod(userID, domain.ProviderFake, "token_123", "****1234")
	if err != nil {
		t.Fatalf("new payment method: %v", err)
	}

	created, err := repo.Create(ctx, pm)
	if err != nil {
		t.Fatalf("Create error = %v", err)
	}
	if created.ID == uuid.Nil {
		t.Error("created payment method has nil ID")
	}
	if created.ProviderToken != pm.ProviderToken {
		t.Errorf("ProviderToken = %q, want %q", created.ProviderToken, pm.ProviderToken)
	}
	if created.DisplayMask != pm.DisplayMask {
		t.Errorf("DisplayMask = %q, want %q", created.DisplayMask, pm.DisplayMask)
	}
}

func TestPaymentMethodRepositoryIntegration_Create_DuplicateToken(t *testing.T) {
	pool := setupIntegrationDB(t)
	ctx, tx, cleanup := beginTx(t, pool)
	defer cleanup()

	q := genpostgres.New(tx)
	userID := createTestUser(t, ctx, q)
	repo := NewPaymentMethodRepository(tx, noopEncryptor(t))

	pm, _ := domain.NewPaymentMethod(userID, domain.ProviderFake, "same-token", "*1234")
	if _, err := repo.Create(ctx, pm); err != nil {
		t.Fatalf("first Create error = %v", err)
	}

	pm2, _ := domain.NewPaymentMethod(userID, domain.ProviderFake, "same-token", "*5678")
	_, err := repo.Create(ctx, pm2)
	if !errors.Is(err, application.ErrPaymentMethodAlreadyExists) {
		t.Errorf("second Create error = %v, want ErrPaymentMethodAlreadyExists", err)
	}
}

func TestPaymentMethodRepositoryIntegration_SetActive(t *testing.T) {
	pool := setupIntegrationDB(t)
	ctx, tx, cleanup := beginTx(t, pool)
	defer cleanup()

	q := genpostgres.New(tx)
	userID := createTestUser(t, ctx, q)
	repo := NewPaymentMethodRepository(tx, noopEncryptor(t))

	pm1, _ := domain.NewPaymentMethod(userID, domain.ProviderFake, "token_1", "*1")
	pm2, _ := domain.NewPaymentMethod(userID, domain.ProviderFake, "token_2", "*2")
	pm1, _ = repo.Create(ctx, pm1)
	pm2, _ = repo.Create(ctx, pm2)

	if err := repo.SetActive(ctx, userID, pm2.ID); err != nil {
		t.Fatalf("SetActive error = %v", err)
	}

	active, err := repo.GetByID(ctx, pm2.ID)
	if err != nil {
		t.Fatalf("GetByID active error = %v", err)
	}
	if !active.IsActive {
		t.Errorf("active method IsActive = false, want true")
	}

	inactive, err := repo.GetByID(ctx, pm1.ID)
	if err != nil {
		t.Fatalf("GetByID inactive error = %v", err)
	}
	if inactive.IsActive {
		t.Errorf("inactive method IsActive = true, want false")
	}
}

func TestPaymentMethodRepositoryIntegration_SetActive_WrongUser(t *testing.T) {
	pool := setupIntegrationDB(t)
	ctx, tx, cleanup := beginTx(t, pool)
	defer cleanup()

	q := genpostgres.New(tx)
	userA := createTestUser(t, ctx, q)
	userB := createTestUser(t, ctx, q)
	repo := NewPaymentMethodRepository(tx, noopEncryptor(t))

	pm, _ := domain.NewPaymentMethod(userA, domain.ProviderFake, "token", "*1")
	pm, _ = repo.Create(ctx, pm)

	err := repo.SetActive(ctx, userB, pm.ID)
	if !errors.Is(err, application.ErrNotFound) {
		t.Errorf("SetActive error = %v, want ErrNotFound", err)
	}
}

func TestPaymentMethodRepositoryIntegration_Delete(t *testing.T) {
	pool := setupIntegrationDB(t)
	ctx, tx, cleanup := beginTx(t, pool)
	defer cleanup()

	q := genpostgres.New(tx)
	userID := createTestUser(t, ctx, q)
	repo := NewPaymentMethodRepository(tx, noopEncryptor(t))

	pm, _ := domain.NewPaymentMethod(userID, domain.ProviderFake, "token", "*1")
	pm, _ = repo.Create(ctx, pm)

	if err := repo.Delete(ctx, userID, pm.ID); err != nil {
		t.Fatalf("Delete error = %v", err)
	}

	_, err := repo.GetByID(ctx, pm.ID)
	if !errors.Is(err, application.ErrNotFound) {
		t.Errorf("GetByID after delete error = %v, want ErrNotFound", err)
	}
}

func TestPaymentMethodRepositoryIntegration_Delete_WrongUser(t *testing.T) {
	pool := setupIntegrationDB(t)
	ctx, tx, cleanup := beginTx(t, pool)
	defer cleanup()

	q := genpostgres.New(tx)
	userA := createTestUser(t, ctx, q)
	userB := createTestUser(t, ctx, q)
	repo := NewPaymentMethodRepository(tx, noopEncryptor(t))

	pm, _ := domain.NewPaymentMethod(userA, domain.ProviderFake, "token", "*1")
	pm, _ = repo.Create(ctx, pm)

	err := repo.Delete(ctx, userB, pm.ID)
	if !errors.Is(err, application.ErrNotFound) {
		t.Errorf("Delete error = %v, want ErrNotFound", err)
	}
}

func TestPaymentMethodRepositoryIntegration_ListByUserID(t *testing.T) {
	pool := setupIntegrationDB(t)
	ctx, tx, cleanup := beginTx(t, pool)
	defer cleanup()

	q := genpostgres.New(tx)
	userA := createTestUser(t, ctx, q)
	userB := createTestUser(t, ctx, q)
	repo := NewPaymentMethodRepository(tx, noopEncryptor(t))

	pmA, _ := domain.NewPaymentMethod(userA, domain.ProviderFake, "token_a", "*A")
	pmB, _ := domain.NewPaymentMethod(userB, domain.ProviderFake, "token_b", "*B")
	pmA, _ = repo.Create(ctx, pmA)
	if _, err := repo.Create(ctx, pmB); err != nil {
		t.Fatalf("create payment method for userB: %v", err)
	}

	list, err := repo.ListByUserID(ctx, userA)
	if err != nil {
		t.Fatalf("ListByUserID error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListByUserID len = %d, want 1", len(list))
	}
	if list[0].ID != pmA.ID {
		t.Errorf("ListByUserID[0].ID = %v, want %v", list[0].ID, pmA.ID)
	}
	if list[0].ProviderToken != pmA.ProviderToken {
		t.Errorf("ListByUserID[0].ProviderToken = %q, want %q", list[0].ProviderToken, pmA.ProviderToken)
	}
}
