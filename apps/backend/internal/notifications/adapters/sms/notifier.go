package sms

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// Notifier dispatches reminders as SMS messages using an SMSSender port.
type Notifier struct {
	resolver     application.ContactResolver
	sender       application.SMSSender
	sentRepo     application.SentSMSReminderRepository
	reminderRepo application.ReminderRepository
	clock        clock.Clock
	logger       *slog.Logger
}

// NewNotifier creates a new SMS notifier.
func NewNotifier(resolver application.ContactResolver, sender application.SMSSender, sentRepo application.SentSMSReminderRepository, reminderRepo application.ReminderRepository, clock clock.Clock, logger *slog.Logger) *Notifier {
	if logger == nil {
		logger = slog.Default()
	}
	return &Notifier{resolver: resolver, sender: sender, sentRepo: sentRepo, reminderRepo: reminderRepo, clock: clock, logger: logger}
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

	if notification.ReminderID != uuid.Nil {
		exists, err := n.reminderRepo.ExistsSentSMSReminder(ctx, notification.ReminderID)
		if err != nil {
			return fmt.Errorf("check sent sms reminder: %w", err)
		}
		if exists {
			n.logger.InfoContext(ctx, "sms reminder already sent, skipping", "reminder_id", notification.ReminderID, "event_type", notification.EventType)
			return nil
		}
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

	n.logger.InfoContext(ctx, "sms reminder sent", "reminder_id", notification.ReminderID, "event_type", notification.EventType)
	return nil
}
