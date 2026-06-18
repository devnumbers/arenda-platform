package postgres

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/generated/postgres"
)

func TestMapSubscription(t *testing.T) {
	id := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")
	tariffID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13")
	pendingTariffID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14")
	paymentMethodID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a15")
	validUntil := time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)
	pendingChangeAt := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	pendingPeriod := domain.PeriodYear

	row := genpostgres.UserSubscription{
		ID:                    pgtype.UUID{Bytes: id, Valid: true},
		UserID:                pgtype.UUID{Bytes: userID, Valid: true},
		TariffID:              pgtype.UUID{Bytes: tariffID, Valid: true},
		Source:                string(domain.SubscriptionSourcePaid),
		Status:                string(domain.SubscriptionStatusGrace),
		ValidUntil:            pgtype.Timestamptz{Time: validUntil, Valid: true},
		AutoRenewEnabled:      true,
		PendingTariffID:       pgtype.UUID{Bytes: pendingTariffID, Valid: true},
		PendingChangeAt:       pgtype.Timestamptz{Time: pendingChangeAt, Valid: true},
		PendingPeriod:         pgtype.Text{String: string(pendingPeriod), Valid: true},
		ActivePaymentMethodID: pgtype.UUID{Bytes: paymentMethodID, Valid: true},
	}

	got := mapSubscription(row)
	want := domain.Subscription{
		ID:                    id,
		UserID:                userID,
		TariffID:              tariffID,
		Source:                domain.SubscriptionSourcePaid,
		Status:                domain.SubscriptionStatusGrace,
		ValidUntil:            &validUntil,
		AutoRenewEnabled:      true,
		PendingTariffID:       &pendingTariffID,
		PendingChangeAt:       &pendingChangeAt,
		PendingPeriod:         &pendingPeriod,
		ActivePaymentMethodID: &paymentMethodID,
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("mapSubscription() = %+v, want %+v", got, want)
	}
}

func TestMapSubscriptionNullables(t *testing.T) {
	id := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")
	tariffID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13")

	row := genpostgres.UserSubscription{
		ID:                    pgtype.UUID{Bytes: id, Valid: true},
		UserID:                pgtype.UUID{Bytes: userID, Valid: true},
		TariffID:              pgtype.UUID{Bytes: tariffID, Valid: true},
		Source:                string(domain.SubscriptionSourcePaid),
		Status:                string(domain.SubscriptionStatusActive),
		ValidUntil:            pgtype.Timestamptz{Valid: false},
		AutoRenewEnabled:      false,
		PendingTariffID:       pgtype.UUID{Valid: false},
		PendingChangeAt:       pgtype.Timestamptz{Valid: false},
		ActivePaymentMethodID: pgtype.UUID{Valid: false},
	}

	got := mapSubscription(row)
	want := domain.Subscription{
		ID:               id,
		UserID:           userID,
		TariffID:         tariffID,
		Source:           domain.SubscriptionSourcePaid,
		Status:           domain.SubscriptionStatusActive,
		AutoRenewEnabled: false,
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("mapSubscription() = %+v, want %+v", got, want)
	}
}
