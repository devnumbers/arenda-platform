// Package email holds the email adapters for the access bounded context
// (issue #161, T5): the property invite email sender — the only email of the
// invitation lifecycle.
package email

import (
	"context"
	"fmt"

	"github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
)

const inviteSubject = "Приглашение к совместному доступу в Рентли"

// Sender renders and sends property invite emails through the shared mailer.
type Sender struct {
	sender     mailer.Sender
	renderer   *mailer.Renderer
	appBaseURL string
}

// NewSender creates an access invite email sender backed by the shared mailer.
// appBaseURL is the public web app URL the invite points the invitee at.
func NewSender(sender mailer.Sender, renderer *mailer.Renderer, appBaseURL string) *Sender {
	return &Sender{sender: sender, renderer: renderer, appBaseURL: appBaseURL}
}

var _ application.InvitationMailer = (*Sender)(nil)

// SendInvite renders and sends the invite email. An empty propertyTitle
// degrades to a generic text (the template handles it).
func (s *Sender) SendInvite(ctx context.Context, to, propertyTitle string, role domain.Role) error {
	plain, html, err := s.renderer.Render("property_invite", map[string]any{
		"PropertyTitle": propertyTitle,
		"Role":          roleLabel(role),
		"AppURL":        s.appBaseURL,
	})
	if err != nil {
		return fmt.Errorf("render property invite email: %w", err)
	}

	msg := mailer.Message{
		To:       []string{to},
		Subject:  inviteSubject,
		TextBody: plain,
		HTMLBody: html,
	}
	if err := s.sender.Send(ctx, msg); err != nil {
		return fmt.Errorf("send property invite email: %w", err)
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
