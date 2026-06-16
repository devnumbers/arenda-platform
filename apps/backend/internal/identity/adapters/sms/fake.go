package sms

import (
	"context"
	"log/slog"

	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

// FakeSender logs SMS messages instead of sending them.
type FakeSender struct {
	logger *slog.Logger
}

// NewFakeSender creates a new fake SMS sender.
func NewFakeSender(logger *slog.Logger) *FakeSender {
	return &FakeSender{logger: logger}
}

func (s *FakeSender) Send(ctx context.Context, phone domain.Phone, message string) error {
	s.logger.InfoContext(ctx, "fake sms sent", "phone", phone.String(), "message", message)
	return nil
}
