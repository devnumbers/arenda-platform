// Package email holds the email adapters for the access bounded context: the
// property invite email of the invitation lifecycle (issue #161, T5) and the
// sharing lifecycle emails (issue #162, T6) — to the (former) member on
// revoke, property deletion, suspension, downgrade and recovery, and to the
// owner on invitation activation and member self-exit.
package email

import (
	"context"
	"fmt"

	"github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
)

const (
	inviteSubject              = "Приглашение к совместному доступу в Рентли"
	accessRevokedSubject       = "Ваш доступ к объекту в Рентли отозван"
	propertyDeletedSubject     = "Объект в Рентли удалён владельцем"
	accessSuspendedSubject     = "Доступ к объекту в Рентли ждёт свободного слота"
	downgradeSummarySubject    = "Часть доступов в Рентли приостановлена по вашему тарифу"
	accessRestoredSubject      = "Доступ к объекту в Рентли восстановлен"
	invitationActivatedSubject = "Приглашение к совместному доступу в Рентли принято"
	memberLeftSubject          = "Участник вышел из объекта в Рентли"
)

// Sender renders and sends the access lifecycle emails through the shared
// mailer.
type Sender struct {
	sender     mailer.Sender
	renderer   *mailer.Renderer
	appBaseURL string
}

// NewSender creates an access email sender backed by the shared mailer.
// appBaseURL is the public web app URL the emails point the recipient at.
func NewSender(sender mailer.Sender, renderer *mailer.Renderer, appBaseURL string) *Sender {
	return &Sender{sender: sender, renderer: renderer, appBaseURL: appBaseURL}
}

var _ application.AccessMailer = (*Sender)(nil)

// SendInvite renders and sends the invite email. An empty propertyTitle
// degrades to a generic text (the template handles it).
func (s *Sender) SendInvite(ctx context.Context, to, propertyTitle string, role domain.Role) error {
	return s.send(ctx, to, inviteSubject, "property_invite", map[string]any{
		"PropertyTitle": propertyTitle,
		"Role":          roleLabel(role),
		"AppURL":        s.appBaseURL,
	})
}

// SendAccessRevoked emails the former member that their access was revoked.
func (s *Sender) SendAccessRevoked(ctx context.Context, to, propertyTitle string) error {
	return s.send(ctx, to, accessRevokedSubject, "access_revoked", map[string]any{
		"PropertyTitle": propertyTitle,
	})
}

// SendPropertyDeleted emails a former member that the owner deleted the object.
func (s *Sender) SendPropertyDeleted(ctx context.Context, to, propertyTitle string) error {
	return s.send(ctx, to, propertyDeletedSubject, "property_deleted", map[string]any{
		"PropertyTitle": propertyTitle,
	})
}

// SendAccessSuspended emails the member that their access waits for a free
// tariff slot.
func (s *Sender) SendAccessSuspended(ctx context.Context, to, propertyTitle string) error {
	return s.send(ctx, to, accessSuspendedSubject, "access_suspended", map[string]any{
		"PropertyTitle": propertyTitle,
	})
}

// SendDowngradeSummary emails the recipient the single summary of the
// memberships suspended by one enforcement call.
func (s *Sender) SendDowngradeSummary(ctx context.Context, to string, propertyTitles []string) error {
	return s.send(ctx, to, downgradeSummarySubject, "downgrade_summary", map[string]any{
		"PropertyTitles": propertyTitles,
	})
}

// SendAccessRestored emails the member that a suspended membership became
// active again.
func (s *Sender) SendAccessRestored(ctx context.Context, to, propertyTitle string) error {
	return s.send(ctx, to, accessRestoredSubject, "access_restored", map[string]any{
		"PropertyTitle": propertyTitle,
		"AppURL":        s.appBaseURL,
	})
}

// SendInvitationActivated emails the property owner that an invited member
// activated their access at registration.
func (s *Sender) SendInvitationActivated(ctx context.Context, to, propertyTitle, memberEmail string) error {
	return s.send(ctx, to, invitationActivatedSubject, "invitation_activated", map[string]any{
		"PropertyTitle": propertyTitle,
		"MemberEmail":   memberEmail,
	})
}

// SendMemberLeft emails the property owner that a member left the object.
func (s *Sender) SendMemberLeft(ctx context.Context, to, propertyTitle, memberName string) error {
	return s.send(ctx, to, memberLeftSubject, "member_left", map[string]any{
		"PropertyTitle": propertyTitle,
		"MemberName":    memberName,
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
