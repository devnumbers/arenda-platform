package email

import (
	"context"
	"fmt"

	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
)

const (
	loginCodeSubject = "Код для входа в Рентли"
	loginCodeTTL     = "5 минут"
)

// LoginCodeSender renders and sends login code emails through the shared mailer.
type LoginCodeSender struct {
	sender   mailer.Sender
	renderer *mailer.Renderer
}

// NewLoginCodeSender creates an identity email sender backed by the shared mailer.
func NewLoginCodeSender(sender mailer.Sender, renderer *mailer.Renderer) *LoginCodeSender {
	return &LoginCodeSender{sender: sender, renderer: renderer}
}

// Send renders and sends a login code email.
func (s *LoginCodeSender) Send(ctx context.Context, email domain.Email, code string) error {
	plain, html, err := s.renderer.Render("login_code", map[string]any{
		"Code": code,
		"TTL":  loginCodeTTL,
	})
	if err != nil {
		return fmt.Errorf("render login code email: %w", err)
	}

	msg := mailer.Message{
		To:       []string{email.String()},
		Subject:  loginCodeSubject,
		TextBody: plain,
		HTMLBody: html,
	}
	if err := s.sender.Send(ctx, msg); err != nil {
		return fmt.Errorf("send login code email: %w", err)
	}
	return nil
}
