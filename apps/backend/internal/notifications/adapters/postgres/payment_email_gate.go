package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// The adapter satisfies the consumer-declared port (CODING_STANDARDS).
var _ application.PaymentEmailSetting = (*PaymentEmailGate)(nil)

// PaymentEmailGate answers the payment rule's per-payment email flag at
// delivery time (карта #1551, тикет #1554, решение владельца 22.10.2026):
// the «Уведомление на почту» flag (payments.notify_email) gates the rule's
// letters on top of the account matrix — the matrix is the master, the flag
// speaks only for its own rule. No cache: the delivery reads the live flag.
type PaymentEmailGate struct {
	db postgres.DBTX
}

// NewPaymentEmailGate creates the delivery-time per-payment email gate.
func NewPaymentEmailGate(db postgres.DBTX) *PaymentEmailGate {
	return &PaymentEmailGate{db: db}
}

// EmailAllowed reports whether the rule's flag lets the email leg through.
// A vanished rule reads as allowed — the gate silences living rules, a
// deleted one leaves the in-flight delivery as it was.
func (g *PaymentEmailGate) EmailAllowed(ctx context.Context, paymentID uuid.UUID) (bool, error) {
	flag, err := postgres.New(g.db).GetPaymentEmailFlag(ctx, pgconv.UUIDToPgtype(paymentID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return true, nil
		}
		return false, fmt.Errorf("payment email gate: %w", err)
	}
	return flag, nil
}
