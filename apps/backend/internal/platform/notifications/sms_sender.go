package notifications

import (
	"context"
	"fmt"

	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	identitydomain "github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
)

// SMSSender wraps the identity application Sender to implement the notifications SMSSender port.
type SMSSender struct {
	sender identityapp.Sender
}

// NewSMSSender creates a new SMSSender adapter.
func NewSMSSender(sender identityapp.Sender) *SMSSender {
	return &SMSSender{sender: sender}
}

// Send sends an SMS message to the given phone number.
func (s *SMSSender) Send(ctx context.Context, phone string, message string) (string, error) {
	if err := s.sender.Send(ctx, identitydomain.Phone(phone), message); err != nil {
		return "", fmt.Errorf("send sms: %w", err)
	}
	return "", nil
}

var _ notificationsapp.SMSSender = (*SMSSender)(nil)
