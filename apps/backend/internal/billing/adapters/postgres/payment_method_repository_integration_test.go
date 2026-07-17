package postgres

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

func TestPaymentMethodRepositoryIntegration_Create(t *testing.T) {
	pool := setupIntegrationDB(t)
	ctx, tx, cleanup := beginTx(t, pool)
	defer cleanup()

	q := genpostgres.New(tx)
	userID := createTestUser(t, ctx, q)
	repo := NewPaymentMethodRepository(tx, noopEncryptor(t))

	pm, err := domain.NewPaymentMethod(userID, domain.ProviderFake, "token_123", "****1234", time.Now().UTC())
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

	pm, _ := domain.NewPaymentMethod(userID, domain.ProviderFake, "same-token", "*1234", time.Now().UTC())
	if _, err := repo.Create(ctx, pm); err != nil {
		t.Fatalf("first Create error = %v", err)
	}

	pm2, _ := domain.NewPaymentMethod(userID, domain.ProviderFake, "same-token", "*5678", time.Now().UTC())
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

	pm1, _ := domain.NewPaymentMethod(userID, domain.ProviderFake, "token_1", "*1", time.Now().UTC())
	pm2, _ := domain.NewPaymentMethod(userID, domain.ProviderFake, "token_2", "*2", time.Now().UTC())
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

	pm, _ := domain.NewPaymentMethod(userA, domain.ProviderFake, "token", "*1", time.Now().UTC())
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

	pm, _ := domain.NewPaymentMethod(userID, domain.ProviderFake, "token", "*1", time.Now().UTC())
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

	pm, _ := domain.NewPaymentMethod(userA, domain.ProviderFake, "token", "*1", time.Now().UTC())
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

	pmA, _ := domain.NewPaymentMethod(userA, domain.ProviderFake, "token_a", "*A", time.Now().UTC())
	pmB, _ := domain.NewPaymentMethod(userB, domain.ProviderFake, "token_b", "*B", time.Now().UTC())
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

func TestPaymentMethodRepositoryIntegration_UpsertByTokenHash_EmptyFieldsPreserveCardData(t *testing.T) {
	pool := setupIntegrationDB(t)
	ctx, tx, cleanup := beginTx(t, pool)
	defer cleanup()

	q := genpostgres.New(tx)
	userID := createTestUser(t, ctx, q)
	repo := NewPaymentMethodRepository(tx, noopEncryptor(t))

	// Initial upsert with full card data, as the AddCard webhook or a
	// GetCardList sync stores it.
	pm, _ := domain.NewPaymentMethod(userID, domain.ProviderTkassa, "rebill_preserve_1", "430000******0777", time.Now().UTC())
	pm.ProviderCardID = "card_preserve_1"
	pm.ExpDate = "12/30"
	created, err := repo.UpsertByTokenHash(ctx, pm)
	if err != nil {
		t.Fatalf("initial UpsertByTokenHash error = %v", err)
	}

	// A recovery/status-poll upsert carries the same token but no card data:
	// it must not wipe the previously stored card details.
	recovery, _ := domain.NewPaymentMethod(userID, domain.ProviderTkassa, "rebill_preserve_1", "", time.Now().UTC())
	updated, err := repo.UpsertByTokenHash(ctx, recovery)
	if err != nil {
		t.Fatalf("recovery UpsertByTokenHash error = %v", err)
	}
	if updated.ID != created.ID {
		t.Errorf("expected the same row updated, got id %v, want %v", updated.ID, created.ID)
	}
	if updated.ProviderCardID != "card_preserve_1" {
		t.Errorf("ProviderCardID = %q, want preserved %q", updated.ProviderCardID, "card_preserve_1")
	}
	if updated.DisplayMask != "430000******0777" {
		t.Errorf("DisplayMask = %q, want preserved %q", updated.DisplayMask, "430000******0777")
	}
	if updated.ExpDate != "12/30" {
		t.Errorf("ExpDate = %q, want preserved %q", updated.ExpDate, "12/30")
	}

	// A later upsert with real card data still overwrites the stored values.
	refill, _ := domain.NewPaymentMethod(userID, domain.ProviderTkassa, "rebill_preserve_1", "430000******0999", time.Now().UTC())
	refill.ProviderCardID = "card_preserve_2"
	refill.ExpDate = "01/31"
	refilled, err := repo.UpsertByTokenHash(ctx, refill)
	if err != nil {
		t.Fatalf("refill UpsertByTokenHash error = %v", err)
	}
	if refilled.ProviderCardID != "card_preserve_2" || refilled.DisplayMask != "430000******0999" || refilled.ExpDate != "01/31" {
		t.Errorf("expected new card data to overwrite, got card=%q mask=%q exp=%q",
			refilled.ProviderCardID, refilled.DisplayMask, refilled.ExpDate)
	}
}
