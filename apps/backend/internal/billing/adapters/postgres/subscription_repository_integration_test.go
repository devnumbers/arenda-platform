package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

func setupSubscriptionRepositoryIntegrationTest(t *testing.T) (context.Context, pgx.Tx, func(), *SubscriptionRepository, uuid.UUID, uuid.UUID) {
	t.Helper()
	pool := setupIntegrationDB(t)
	ctx, tx, cleanup := beginTx(t, pool)

	q := genpostgres.New(tx)
	userID := createTestUser(t, ctx, q)
	tariff := getTestTariff(t, ctx, q, "pro")
	tariffID := uuid.UUID(tariff.ID.Bytes)

	repo := NewSubscriptionRepository(tx)
	return ctx, tx, cleanup, repo, userID, tariffID
}

func TestSubscriptionRepositoryIntegration_GetByID(t *testing.T) {
	ctx, _, cleanup, repo, userID, tariffID := setupSubscriptionRepositoryIntegrationTest(t)
	defer cleanup()

	sub, err := domain.NewOwnerSubscription(userID, tariffID)
	if err != nil {
		t.Fatalf("new subscription: %v", err)
	}
	created, err := repo.Create(ctx, sub)
	if err != nil {
		t.Fatalf("Create error = %v", err)
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID error = %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("GetByID ID = %v, want %v", got.ID, created.ID)
	}
	if got.UserID != userID {
		t.Errorf("GetByID UserID = %v, want %v", got.UserID, userID)
	}

	randomID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	_, err = repo.GetByID(ctx, randomID)
	if !errors.Is(err, application.ErrNotFound) {
		t.Errorf("GetByID not found error = %v, want ErrNotFound", err)
	}
}

func TestSubscriptionRepositoryIntegration_UpdateSavesLastAppliedPaymentID(t *testing.T) {
	ctx, _, cleanup, repo, userID, tariffID := setupSubscriptionRepositoryIntegrationTest(t)
	defer cleanup()

	sub, err := domain.NewOwnerSubscription(userID, tariffID)
	if err != nil {
		t.Fatalf("new subscription: %v", err)
	}
	created, err := repo.Create(ctx, sub)
	if err != nil {
		t.Fatalf("Create error = %v", err)
	}

	paymentID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a21")
	created.LastAppliedPaymentID = &paymentID
	if err := repo.Update(ctx, created); err != nil {
		t.Fatalf("Update error = %v", err)
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID error = %v", err)
	}
	if got.LastAppliedPaymentID == nil || *got.LastAppliedPaymentID != paymentID {
		t.Errorf("LastAppliedPaymentID = %v, want %v", got.LastAppliedPaymentID, paymentID)
	}
}

func TestSubscriptionRepositoryIntegration_ListFilters(t *testing.T) {
	ctx, tx, cleanup, repo, _, proID := setupSubscriptionRepositoryIntegrationTest(t)
	defer cleanup()

	q := genpostgres.New(tx)
	basicTariff, err := q.GetTariffByName(ctx, "basic")
	if err != nil {
		t.Fatalf("get basic tariff: %v", err)
	}
	basicID := uuid.UUID(basicTariff.ID.Bytes)

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	older := now.Add(-2 * time.Hour)
	future := now.Add(time.Hour)
	month := domain.PeriodMonth

	makeSub := func(status domain.SubscriptionStatus, autoRenew bool, validUntil *time.Time, pendingTariffID *uuid.UUID, pendingChangeAt *time.Time) (uuid.UUID, domain.Subscription) {
		t.Helper()
		userID := createTestUser(t, ctx, q)
		sub, err := domain.NewOwnerSubscription(userID, proID)
		if err != nil {
			t.Fatalf("new subscription: %v", err)
		}
		sub.Status = status
		sub.AutoRenewEnabled = autoRenew
		if validUntil != nil {
			sub.ValidUntil = validUntil
		}
		if pendingTariffID != nil {
			sub.PendingTariffID = pendingTariffID
			sub.PendingPeriod = &month
		}
		if pendingChangeAt != nil {
			sub.PendingChangeAt = pendingChangeAt
		}
		created, err := repo.Create(ctx, sub)
		if err != nil {
			t.Fatalf("create subscription: %v", err)
		}
		return userID, created
	}

	_, upForRenewal := makeSub(domain.SubscriptionStatusActive, true, &past, nil, nil)
	_, expiredGrace := makeSub(domain.SubscriptionStatusGrace, false, &past, nil, nil)
	_, expiredNonRenewing := makeSub(domain.SubscriptionStatusActive, false, &past, nil, nil)
	_, expiredCancelled := makeSub(domain.SubscriptionStatusCancelled, false, &past, nil, nil)
	_, pendingChange := makeSub(domain.SubscriptionStatusActive, true, &older, &basicID, &past)
	notExpiredUser, notExpired := makeSub(domain.SubscriptionStatusActive, true, &future, nil, nil)
	_, futurePendingChange := makeSub(domain.SubscriptionStatusActive, true, nil, &basicID, &future)

	assertContains := func(t *testing.T, subs []domain.Subscription, want domain.Subscription) {
		t.Helper()
		for _, s := range subs {
			if s.ID == want.ID {
				return
			}
		}
		t.Errorf("expected subscription %s in result", want.ID)
	}

	assertNotContains := func(t *testing.T, subs []domain.Subscription, want domain.Subscription) {
		t.Helper()
		for _, s := range subs {
			if s.ID == want.ID {
				t.Errorf("did not expect subscription %s in result", want.ID)
				return
			}
		}
	}

	t.Run("ListUpForRenewal", func(t *testing.T) {
		subs, err := repo.ListUpForRenewal(ctx, now, 100)
		if err != nil {
			t.Fatalf("ListUpForRenewal error: %v", err)
		}
		assertContains(t, subs, upForRenewal)
		assertContains(t, subs, pendingChange)
		assertNotContains(t, subs, expiredGrace)
		assertNotContains(t, subs, expiredNonRenewing)
		assertNotContains(t, subs, expiredCancelled)
		assertNotContains(t, subs, notExpired)
		assertNotContains(t, subs, futurePendingChange)
		if len(subs) != 2 {
			t.Errorf("expected 2 subscriptions, got %d", len(subs))
		}
	})

	t.Run("ListInExpiredGrace", func(t *testing.T) {
		subs, err := repo.ListInExpiredGrace(ctx, now, 100)
		if err != nil {
			t.Fatalf("ListInExpiredGrace error: %v", err)
		}
		assertContains(t, subs, expiredGrace)
		assertNotContains(t, subs, upForRenewal)
		assertNotContains(t, subs, expiredNonRenewing)
		assertNotContains(t, subs, expiredCancelled)
		assertNotContains(t, subs, pendingChange)
		assertNotContains(t, subs, notExpired)
		assertNotContains(t, subs, futurePendingChange)
	})

	t.Run("ListExpiredNonRenewing", func(t *testing.T) {
		subs, err := repo.ListExpiredNonRenewing(ctx, now, 100)
		if err != nil {
			t.Fatalf("ListExpiredNonRenewing error: %v", err)
		}
		assertContains(t, subs, expiredNonRenewing)
		assertNotContains(t, subs, upForRenewal)
		assertNotContains(t, subs, expiredGrace)
		assertNotContains(t, subs, expiredCancelled)
		assertNotContains(t, subs, pendingChange)
		assertNotContains(t, subs, notExpired)
		assertNotContains(t, subs, futurePendingChange)
	})

	t.Run("ListExpiredCancelled", func(t *testing.T) {
		subs, err := repo.ListExpiredCancelled(ctx, now, 100)
		if err != nil {
			t.Fatalf("ListExpiredCancelled error: %v", err)
		}
		assertContains(t, subs, expiredCancelled)
		assertNotContains(t, subs, upForRenewal)
		assertNotContains(t, subs, expiredGrace)
		assertNotContains(t, subs, expiredNonRenewing)
		assertNotContains(t, subs, pendingChange)
		assertNotContains(t, subs, notExpired)
		assertNotContains(t, subs, futurePendingChange)
	})

	t.Run("ListPendingChanges", func(t *testing.T) {
		subs, err := repo.ListPendingChanges(ctx, now, 100)
		if err != nil {
			t.Fatalf("ListPendingChanges error: %v", err)
		}
		assertContains(t, subs, pendingChange)
		assertNotContains(t, subs, upForRenewal)
		assertNotContains(t, subs, expiredGrace)
		assertNotContains(t, subs, expiredNonRenewing)
		assertNotContains(t, subs, expiredCancelled)
		assertNotContains(t, subs, notExpired)
		assertNotContains(t, subs, futurePendingChange)
	})

	t.Run("GetByUserID", func(t *testing.T) {
		got, err := repo.GetByUserID(ctx, notExpiredUser)
		if err != nil {
			t.Fatalf("GetByUserID error: %v", err)
		}
		if got.ID != notExpired.ID {
			t.Errorf("GetByUserID ID = %v, want %v", got.ID, notExpired.ID)
		}

		otherUser := createTestUser(t, ctx, q)
		_, err = repo.GetByUserID(ctx, otherUser)
		if !errors.Is(err, application.ErrNotFound) {
			t.Errorf("GetByUserID for other user error = %v, want ErrNotFound", err)
		}
	})

	t.Run("Update", func(t *testing.T) {
		notExpired.AutoRenewEnabled = true
		newValidUntil := now.AddDate(0, 1, 0)
		notExpired.ValidUntil = &newValidUntil
		if err := repo.Update(ctx, notExpired); err != nil {
			t.Fatalf("Update error: %v", err)
		}

		got, err := repo.GetByID(ctx, notExpired.ID)
		if err != nil {
			t.Fatalf("GetByID after Update error: %v", err)
		}
		if !got.AutoRenewEnabled {
			t.Errorf("AutoRenewEnabled not persisted")
		}
		if got.ValidUntil == nil || !got.ValidUntil.Equal(newValidUntil) {
			t.Errorf("ValidUntil = %v, want %v", got.ValidUntil, newValidUntil)
		}
	})
}
