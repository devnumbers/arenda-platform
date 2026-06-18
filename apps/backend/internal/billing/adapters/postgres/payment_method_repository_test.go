package postgres

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/encryption"
)

func TestMapPaymentMethod(t *testing.T) {
	id := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")
	createdAt := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)

	row := genpostgres.PaymentMethod{
		ID:            pgtype.UUID{Bytes: id, Valid: true},
		UserID:        pgtype.UUID{Bytes: userID, Valid: true},
		Provider:      string(domain.ProviderFake),
		ProviderToken: "fake_token_123",
		DisplayMask:   pgtype.Text{String: "*1234", Valid: true},
		IsActive:      true,
		CreatedAt:     pgtype.Timestamptz{Time: createdAt, Valid: true},
		UpdatedAt:     pgtype.Timestamptz{Time: updatedAt, Valid: true},
	}

	got, err := mapPaymentMethod(context.Background(), row, mustNoopEncryptor(t))
	if err != nil {
		t.Fatalf("mapPaymentMethod() error = %v", err)
	}
	want := domain.PaymentMethod{
		ID:            id,
		UserID:        userID,
		Provider:      domain.ProviderFake,
		ProviderToken: "fake_token_123",
		DisplayMask:   "*1234",
		IsActive:      true,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("mapPaymentMethod() = %+v, want %+v", got, want)
	}
}

func TestMapPaymentMethodNullDisplayMask(t *testing.T) {
	id := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")
	createdAt := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)

	row := genpostgres.PaymentMethod{
		ID:            pgtype.UUID{Bytes: id, Valid: true},
		UserID:        pgtype.UUID{Bytes: userID, Valid: true},
		Provider:      string(domain.ProviderFake),
		ProviderToken: "fake_token_123",
		DisplayMask:   pgtype.Text{Valid: false},
		IsActive:      false,
		CreatedAt:     pgtype.Timestamptz{Time: createdAt, Valid: true},
		UpdatedAt:     pgtype.Timestamptz{Time: updatedAt, Valid: true},
	}

	got, err := mapPaymentMethod(context.Background(), row, mustNoopEncryptor(t))
	if err != nil {
		t.Fatalf("mapPaymentMethod() error = %v", err)
	}
	want := domain.PaymentMethod{
		ID:            id,
		UserID:        userID,
		Provider:      domain.ProviderFake,
		ProviderToken: "fake_token_123",
		DisplayMask:   "",
		IsActive:      false,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("mapPaymentMethod() = %+v, want %+v", got, want)
	}
}

func mustNoopEncryptor(t *testing.T) encryption.Encryptor {
	t.Helper()
	enc, err := encryption.NewEncryptor("")
	if err != nil {
		t.Fatalf("create noop encryptor: %v", err)
	}
	return enc
}
