// Package email renders notification emails from named templates and sends
// them through the platform mailer.
package email

import (
	"context"
	"fmt"

	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
)

// Notifier sends template-rendered notification emails.
type Notifier struct {
	sender   mailer.Sender
	renderer *mailer.Renderer
}

// NewNotifier creates an email notifier.
func NewNotifier(sender mailer.Sender, renderer *mailer.Renderer) *Notifier {
	return &Notifier{sender: sender, renderer: renderer}
}

// SendTemplate renders the named template and sends one email: the caller
// (the email delivery worker) owns the subject and content.
func (n *Notifier) SendTemplate(ctx context.Context, to, subject, template string, data map[string]any) error {
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

var _ application.TemplateEmailSender = (*Notifier)(nil)
