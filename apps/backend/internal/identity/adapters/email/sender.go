// Package email delivers identity login codes: the LoginCodeSender adapter rendering the per-purpose one-time-code letters.
package email

import (
	"context"
	"fmt"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
)

// Subjects of the four letters. The subject names the operation — the
// anti-phishing mirror of the server-side purpose binding (research #1201) —
// and never carries the code.
const (
	loginSubject              = "Код для входа в Рентли"
	phoneChangeSubject        = "Код для смены телефона в Рентли"
	emailChangeCurrentSubject = "Код для подтверждения смены email в Рентли"
	emailChangeNewSubject     = "Код для подтверждения нового email в Рентли"
)

// letter is the presentation shape of one identity code email: the template
// rendered through the shared layout and the operation-naming subject.
type letter struct {
	template string
	subject  string
}

// letterFor resolves the unique letter for the purpose+step pair. Every
// operation carries its own template and subject (issue #1204): login keeps
// its letter, the phone-change code lands on the current email, and the
// email-change protocol splits its two codes — "confirm it is you" on the
// current address, "confirm ownership of the address" on the new one. A pair
// without a letter is a programming error and fails the delivery loudly;
// LoginCodeService.Deliver then cleans up the unsent code row.
func letterFor(purpose domain.LoginCodePurpose, step domain.LoginCodeStep) (letter, error) {
	switch {
	case purpose == domain.LoginCodePurposeLogin && step == domain.LoginCodeStepCurrentEmail:
		return letter{template: "login_code", subject: loginSubject}, nil
	case purpose == domain.LoginCodePurposePhoneChange && step == domain.LoginCodeStepCurrentEmail:
		return letter{template: "phone_change_code", subject: phoneChangeSubject}, nil
	case purpose == domain.LoginCodePurposeEmailChange && step == domain.LoginCodeStepCurrentEmail:
		return letter{template: "email_change_current_code", subject: emailChangeCurrentSubject}, nil
	case purpose == domain.LoginCodePurposeEmailChange && step == domain.LoginCodeStepNewEmail:
		return letter{template: "email_change_new_code", subject: emailChangeNewSubject}, nil
	default:
		return letter{}, fmt.Errorf("no letter for purpose %q step %q", purpose, step)
	}
}

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

// renderer is the consumer-side port for email template rendering (ADR 0035).
// It lets the email adapter accept any renderer — the production
// *mailer.Renderer or a test stub — without the platform/mailer package knowing
// about identity. Only the Render method is needed here.
type renderer interface {
	Render(name string, data any) (plain, html string, err error)
}

// Sender renders and sends login code emails through the shared mailer.
type Sender struct {
	sender   mailer.Sender
	renderer renderer
}

// NewSender creates an identity email sender backed by the shared mailer. The
// renderer parameter accepts the production *mailer.Renderer (which satisfies
// the consumer-side renderer interface) or a test stub.
func NewSender(sender mailer.Sender, renderer renderer) *Sender {
	return &Sender{sender: sender, renderer: renderer}
}

var _ application.LoginCodeSender = (*Sender)(nil)

// Send renders and sends the letter selected by the purpose+step pair. See
// LoginCodeSender for why phone is part of the signature though this channel
// only uses email and code.
func (s *Sender) Send(
	ctx context.Context,
	phone domain.Phone,
	email domain.Email,
	code string,
	purpose domain.LoginCodePurpose,
	step domain.LoginCodeStep,
) error {
	letter, err := letterFor(purpose, step)
	if err != nil {
		return err
	}

	plain, html, err := s.renderer.Render(letter.template, map[string]any{
		"Code": code,
		"TTL":  formatLoginCodeTTL(domain.LoginCodeTTL),
	})
	if err != nil {
		return fmt.Errorf("render %s email: %w", letter.template, err)
	}

	msg := mailer.Message{
		To:       []string{email.String()},
		Subject:  letter.subject,
		TextBody: plain,
		HTMLBody: html,
	}
	if err := s.sender.Send(ctx, msg); err != nil {
		return fmt.Errorf("send %s email: %w", letter.template, err)
	}
	return nil
}
