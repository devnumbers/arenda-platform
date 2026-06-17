package sms

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
)

// Notifier dispatches reminders as SMS messages using an SMSSender port.
type Notifier struct {
	resolver application.ContactResolver
	sender   application.SMSSender
	logger   *slog.Logger
}

// NewNotifier creates a new SMS notifier.
func NewNotifier(resolver application.ContactResolver, sender application.SMSSender, logger *slog.Logger) *Notifier {
	if logger == nil {
		logger = slog.Default()
	}
	return &Notifier{resolver: resolver, sender: sender, logger: logger}
}

// Notify resolves the recipient contact and sends the notification via SMS.
func (n *Notifier) Notify(ctx context.Context, notification application.Notification) error {
	contact, err := n.resolver.Resolve(ctx, notification.RecipientID)
	if err != nil {
		return fmt.Errorf("resolve contact: %w", err)
	}
	if contact.Channel != application.ChannelSMS {
		return fmt.Errorf("unsupported channel: %s", contact.Channel)
	}

	if err := n.sender.Send(ctx, contact.Address, notification.Body); err != nil {
		return fmt.Errorf("send sms: %w", err)
	}

	n.logger.InfoContext(ctx, "sms reminder sent", "reminder_id", notification.ReminderID, "event_type", notification.EventType)
	return nil
}
