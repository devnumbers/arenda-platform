package email

import (
	"context"
	"log/slog"

	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

// FakeSender logs email login codes instead of sending them.
// Intended for development and testing only; do not use in production.
type FakeSender struct {
	logger *slog.Logger
}

// NewFakeSender creates a new fake email sender.
func NewFakeSender(logger *slog.Logger) *FakeSender {
	return &FakeSender{logger: logger}
}

func (s *FakeSender) Send(ctx context.Context, email domain.Email, code string) error {
	s.logger.InfoContext(ctx, "fake email sent", "email", email.String(), "code", code)
	return nil
}
