// Package email holds the email adapter of the access bounded context: the
// property invite email of the invitation lifecycle (issue #161, T5). The
// sharing lifecycle emails of issue #162 (T6) are cut (issue #695) until a
// full notification system replaces them.
package email

import (
	"context"
	"fmt"

	"github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
)

const (
	inviteSubject = "Приглашение к совместному доступу в Рентли"
)

// Sender renders and sends the access invite email through the shared mailer.
type Sender struct {
	sender     mailer.Sender
	renderer   *mailer.Renderer
	appBaseURL string
}

// NewSender creates an access email sender backed by the shared mailer.
// AppBaseURL is the public web app URL the emails point the recipient at.
func NewSender(sender mailer.Sender, renderer *mailer.Renderer, appBaseURL string) *Sender {
	return &Sender{sender: sender, renderer: renderer, appBaseURL: appBaseURL}
}

var _ application.AccessMailer = (*Sender)(nil)

// SendInvite renders and sends the invite email. Since the multi-object
// invitation (issue #694) PropertyTitles carries the whole batch — a single
// title renders exactly like the pre-#694 text; empty titles degrade to a
// generic text (the template handles it).
func (s *Sender) SendInvite(ctx context.Context, to string, propertyTitles []string, role domain.Role) error {
	return s.send(ctx, to, inviteSubject, "property_invite", map[string]any{
		"PropertyTitles": propertyTitles,
		"Role":           roleLabel(role),
		"AppURL":         s.appBaseURL,
	})
}

// send renders the named template and sends the message to a single recipient.
func (s *Sender) send(ctx context.Context, to, subject, template string, data map[string]any) error {
	plain, html, err := s.renderer.Render(template, data)
	if err != nil {
		return fmt.Errorf("render %s email: %w", template, err)
	}

	msg := mailer.Message{
		To:       []string{to},
		Subject:  subject,
		TextBody: plain,
		HTMLBody: html,
	}
	if err := s.sender.Send(ctx, msg); err != nil {
		return fmt.Errorf("send %s email: %w", template, err)
	}
	return nil
}

// roleLabel renders the sharing role in the glossary wording (CONTEXT.md).
func roleLabel(role domain.Role) string {
	switch role {
	case domain.RoleFullAccess:
		return "Полный доступ"
	case domain.RoleViewer:
		return "Просмотр"
	default:
		return role.String()
	}
}
