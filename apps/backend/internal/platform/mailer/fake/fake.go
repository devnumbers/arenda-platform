// Package fake is a logging no-send mailer for development and tests.
package fake

import (
	"context"
	"log/slog"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
)

// FakeSender logs emails instead of sending them.
// Intended for development and testing only; do not use in production.
type FakeSender struct {
	logger *slog.Logger
}

// NewFakeSender creates a fake mailer.
func NewFakeSender(logger *slog.Logger) *FakeSender {
	return &FakeSender{logger: logger}
}

// Send logs the message and succeeds.
func (s *FakeSender) Send(ctx context.Context, msg mailer.Message) error {
	s.logger.InfoContext(ctx, "fake email sent",
		slog.Any("to", msg.To),
		slog.String("subject", msg.Subject),
		slog.String("text", msg.TextBody),
	)
	return nil
}
