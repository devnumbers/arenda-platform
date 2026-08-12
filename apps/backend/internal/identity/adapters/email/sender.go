package email

import (
	"context"
	"fmt"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
)

const (
	loginCodeSubject = "Код для входа в Рентли"
)

// formatLoginCodeTTL renders the login-code TTL duration as a human-readable
// Russian phrase derived from domain.LoginCodeTTL, so the email text always
// tracks the domain constant. Presentation lives here, not in the domain.
func formatLoginCodeTTL(d time.Duration) string {
	minutes := int(d / time.Minute)
	switch {
	case minutes%100 >= 11 && minutes%100 <= 14:
		return fmt.Sprintf("%d минут", minutes)
	case minutes%10 == 1:
		return fmt.Sprintf("%d минута", minutes)
	case minutes%10 >= 2 && minutes%10 <= 4:
		return fmt.Sprintf("%d минуты", minutes)
	default:
		return fmt.Sprintf("%d минут", minutes)
	}
}

// Sender renders and sends login code emails through the shared mailer.
type Sender struct {
	sender   mailer.Sender
	renderer *mailer.Renderer
}

// NewSender creates an identity email sender backed by the shared mailer.
func NewSender(sender mailer.Sender, renderer *mailer.Renderer) *Sender {
	return &Sender{sender: sender, renderer: renderer}
}

var _ application.LoginCodeSender = (*Sender)(nil)

// Send renders and sends a login code email.
// Phone is received for logging/context parity with the generic port but is not used in the email body.
func (s *Sender) Send(ctx context.Context, phone domain.Phone, email domain.Email, code string) error {
	plain, html, err := s.renderer.Render("login_code", map[string]any{
		"Code": code,
		"TTL":  formatLoginCodeTTL(domain.LoginCodeTTL),
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
