package email

import (
	"context"
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

// Notify sends a reminder email.
func (n *Notifier) Notify(ctx context.Context, notification application.Notification) (string, string, error) {
	if notification.Contact == nil || notification.Contact.Email == "" {
		return "", "", fmt.Errorf("notification contact missing email")
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
		Subject:  notification.Title,
		TextBody: plain,
		HTMLBody: html,
	}
	if err := n.sender.Send(ctx, msg); err != nil {
		return "", "", fmt.Errorf("send reminder email: %w", err)
	}
	return "", plain, nil
}

var _ application.Notifier = (*Notifier)(nil)
