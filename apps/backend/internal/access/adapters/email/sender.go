// Package email holds the email adapters for the access bounded context: the
// property invite email of the invitation lifecycle (issue #161, T5) and the
// "object deleted" notice (issue #162, T6) — the two direct emails that stay;
// the rest of the sharing lifecycle correspondence is the notifications feed's
// (карта #734, #751).
package email

import (
	"context"
	"fmt"

	"github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
)

const (
	inviteSubject          = "Приглашение к совместному доступу в Рентли"
	propertyDeletedSubject = "Объект в Рентли удалён владельцем"
	// PropertyTitleKey is the template data key carrying the display title of
	// the shared object.
	propertyTitleKey = "PropertyTitle"
)

// Sender renders and sends the direct access emails through the shared mailer.
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

// SendPropertyDeleted emails a former member that the owner deleted the object.
func (s *Sender) SendPropertyDeleted(ctx context.Context, to, propertyTitle string) error {
	return s.send(ctx, to, propertyDeletedSubject, "property_deleted", map[string]any{
		propertyTitleKey: propertyTitle,
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
