package sms

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	identitydomain "github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
)

// SentSMSReminderRepository tracks successfully sent SMS reminders for audit and deduplication.
type SentSMSReminderRepository interface {
	Save(ctx context.Context, reminderID *uuid.UUID, ownerID uuid.UUID, phone, message, providerResponse string, sentAt time.Time) error
}

// Notifier dispatches reminders as SMS messages using identity.Sender.
type Notifier struct {
	resolver application.ContactResolver
	sender   identityapp.Sender
	sentRepo SentSMSReminderRepository
	logger   *slog.Logger
}

// NewNotifier creates a new SMS notifier.
func NewNotifier(resolver application.ContactResolver, sender identityapp.Sender, sentRepo SentSMSReminderRepository, logger *slog.Logger) *Notifier {
	if logger == nil {
		logger = slog.Default()
	}
	return &Notifier{resolver: resolver, sender: sender, sentRepo: sentRepo, logger: logger}
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

	phone := identitydomain.Phone(contact.Address)
	if err := n.sender.Send(ctx, phone, notification.Body); err != nil {
		return fmt.Errorf("send sms: %w", err)
	}

	sentAt := time.Now().UTC()
	var reminderID *uuid.UUID
	if notification.ReminderID != uuid.Nil {
		reminderID = &notification.ReminderID
	}
	if err := n.sentRepo.Save(ctx, reminderID, notification.RecipientID, phone.String(), notification.Body, "", sentAt); err != nil {
		return fmt.Errorf("track sent sms: %w", err)
	}

	n.logger.InfoContext(ctx, "sms reminder sent", "recipient_id", notification.RecipientID, "event_type", notification.EventType)
	return nil
}
