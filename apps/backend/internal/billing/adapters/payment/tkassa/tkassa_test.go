package tkassa

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func TestProviderNotImplemented(t *testing.T) {
	p := NewProvider("test-terminal", "test-password", discardLogger())
	ctx := context.Background()
	paymentID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	userID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	_, err := p.Init(ctx, application.InitRequest{
		PaymentID:     paymentID,
		AmountKopecks: 10000,
		Period:        domain.PeriodMonth,
		UserID:        userID,
		Description:   "T-Kassa test",
	})
	if !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("Init expected ErrNotImplemented, got %v", err)
	}

	_, err = p.Charge(ctx, application.ChargeRequest{
		PaymentID:     paymentID,
		AmountKopecks: 10000,
		Token:         "tkassa-token",
	})
	if !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("Charge expected ErrNotImplemented, got %v", err)
	}

	_, err = p.ParseWebhook(ctx, []byte(`{}`))
	if !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("ParseWebhook expected ErrNotImplemented, got %v", err)
	}
}
