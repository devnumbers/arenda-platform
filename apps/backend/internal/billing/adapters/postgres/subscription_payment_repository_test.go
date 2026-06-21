package postgres

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

func TestMapSubscriptionPayment(t *testing.T) {
	id := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")
	subscriptionID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13")
	tariffID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14")
	paymentMethodID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a15")
	providerPaymentID := "provider_payment_123"
	errorCode := "card_declined"
	createdAt := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)

	row := genpostgres.SubscriptionPayment{
		ID:                pgtype.UUID{Bytes: id, Valid: true},
		UserID:            pgtype.UUID{Bytes: userID, Valid: true},
		SubscriptionID:    pgtype.UUID{Bytes: subscriptionID, Valid: true},
		TariffID:          pgtype.UUID{Bytes: tariffID, Valid: true},
		PaymentMethodID:   pgtype.UUID{Bytes: paymentMethodID, Valid: true},
		Period:            string(domain.PeriodMonth),
		AmountKopecks:     49000,
		Provider:          string(domain.ProviderFake),
		ProviderPaymentID: pgtype.Text{String: providerPaymentID, Valid: true},
		Status:            string(domain.PaymentStatusPending),
		ErrorCode:         pgtype.Text{String: errorCode, Valid: true},
		CreatedAt:         pgtype.Timestamptz{Time: createdAt, Valid: true},
		UpdatedAt:         pgtype.Timestamptz{Time: updatedAt, Valid: true},
	}

	got := mapSubscriptionPayment(row)
	want := domain.SubscriptionPayment{
		ID:                id,
		UserID:            userID,
		SubscriptionID:    subscriptionID,
		TariffID:          tariffID,
		PaymentMethodID:   &paymentMethodID,
		Period:            domain.PeriodMonth,
		AmountKopecks:     49000,
		Provider:          domain.ProviderFake,
		ProviderPaymentID: &providerPaymentID,
		Status:            domain.PaymentStatusPending,
		ErrorCode:         &errorCode,
		CreatedAt:         createdAt,
		UpdatedAt:         updatedAt,
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("mapSubscriptionPayment() = %+v, want %+v", got, want)
	}
}

func TestMapSubscriptionPaymentNullables(t *testing.T) {
	id := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")
	subscriptionID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13")
	tariffID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14")
	createdAt := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)

	row := genpostgres.SubscriptionPayment{
		ID:                pgtype.UUID{Bytes: id, Valid: true},
		UserID:            pgtype.UUID{Bytes: userID, Valid: true},
		SubscriptionID:    pgtype.UUID{Bytes: subscriptionID, Valid: true},
		TariffID:          pgtype.UUID{Bytes: tariffID, Valid: true},
		PaymentMethodID:   pgtype.UUID{Valid: false},
		Period:            string(domain.PeriodYear),
		AmountKopecks:     440000,
		Provider:          string(domain.ProviderFake),
		ProviderPaymentID: pgtype.Text{Valid: false},
		Status:            string(domain.PaymentStatusSucceeded),
		ErrorCode:         pgtype.Text{Valid: false},
		CreatedAt:         pgtype.Timestamptz{Time: createdAt, Valid: true},
		UpdatedAt:         pgtype.Timestamptz{Time: updatedAt, Valid: true},
	}

	got := mapSubscriptionPayment(row)
	want := domain.SubscriptionPayment{
		ID:             id,
		UserID:         userID,
		SubscriptionID: subscriptionID,
		TariffID:       tariffID,
		Period:         domain.PeriodYear,
		AmountKopecks:  440000,
		Provider:       domain.ProviderFake,
		Status:         domain.PaymentStatusSucceeded,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("mapSubscriptionPayment() = %+v, want %+v", got, want)
	}
}
