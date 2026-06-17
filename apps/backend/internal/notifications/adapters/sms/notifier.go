package sms

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// SentSMSReminderRepository tracks successfully sent SMS reminders for audit and deduplication.
type SentSMSReminderRepository interface {
	Save(ctx context.Context, reminderID *uuid.UUID, ownerID uuid.UUID, phone, message, providerResponse string, sentAt time.Time) error
}

// Notifier dispatches reminders as SMS messages using an SMSSender port.
type Notifier struct {
	resolver application.ContactResolver
	sender   application.SMSSender
	sentRepo SentSMSReminderRepository
	clock    clock.Clock
	logger   *slog.Logger
}

// NewNotifier creates a new SMS notifier.
func NewNotifier(resolver application.ContactResolver, sender application.SMSSender, sentRepo SentSMSReminderRepository, clock clock.Clock, logger *slog.Logger) *Notifier {
	if logger == nil {
		logger = slog.Default()
	}
	return &Notifier{resolver: resolver, sender: sender, sentRepo: sentRepo, clock: clock, logger: logger}
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

	sentAt := n.clock.Now().UTC()
	var reminderID *uuid.UUID
	if notification.ReminderID != uuid.Nil {
		reminderID = &notification.ReminderID
	}
	if err := n.sentRepo.Save(ctx, reminderID, notification.RecipientID, contact.Address, notification.Body, "", sentAt); err != nil {
		return fmt.Errorf("track sent sms: %w", err)
	}

	n.logger.InfoContext(ctx, "sms reminder sent", "recipient_id", notification.RecipientID, "event_type", notification.EventType)
	return nil
}
