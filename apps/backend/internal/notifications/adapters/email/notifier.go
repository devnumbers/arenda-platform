package email

import (
	"context"
	"errors"
	"fmt"

	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
)

// Notifier sends reminders via email.
type Notifier struct {
	sender   mailer.Sender
	renderer *mailer.Renderer
}

// NewNotifier creates an email notifier.
func NewNotifier(sender mailer.Sender, renderer *mailer.Renderer) *Notifier {
	return &Notifier{sender: sender, renderer: renderer}
}

func reminderSubject(title string) string {
	return "Напоминание от Рентли: " + title
}

// SendDirect renders the named template and sends a one-off email outside the
// reminder lifecycle (issue #253): the direct-notification service owns the
// subject and content, so unlike Notify it carries no reminder subject prefix.
func (n *Notifier) SendDirect(ctx context.Context, to, subject, template string, data map[string]any) error {
	plain, html, err := n.renderer.Render(template, data)
	if err != nil {
		return fmt.Errorf("render %s email: %w", template, err)
	}
	msg := mailer.Message{
		To:       []string{to},
		Subject:  subject,
		TextBody: plain,
		HTMLBody: html,
	}
	if err := n.sender.Send(ctx, msg); err != nil {
		return fmt.Errorf("send %s email: %w", template, err)
	}
	return nil
}

// Notify sends a reminder email.
func (n *Notifier) Notify(
	ctx context.Context,
	notification application.Notification,
) (providerResponse, renderedPlainBody string, err error) {
	if notification.Contact == nil || notification.Contact.Email == "" {
		return "", "", errors.New("notification contact missing email")
	}

	plain, html, err := n.renderer.Render("reminder", map[string]any{
		"Subject": notification.Title,
		"Title":   notification.Title,
		"Body":    notification.Body,
	})
	if err != nil {
		return "", "", fmt.Errorf("render reminder email: %w", err)
	}

	msg := mailer.Message{
		To:       []string{notification.Contact.Email},
		Subject:  reminderSubject(notification.Title),
		TextBody: plain,
		HTMLBody: html,
	}
	if err := n.sender.Send(ctx, msg); err != nil {
		return "", "", fmt.Errorf("send reminder email: %w", err)
	}
	return "", plain, nil
}

var _ application.Notifier = (*Notifier)(nil)

var _ application.DirectEmailSender = (*Notifier)(nil)
