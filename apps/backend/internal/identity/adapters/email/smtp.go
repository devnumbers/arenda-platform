package email

import (
	"context"
	"fmt"
	"net/smtp"

	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

// SMTPConfig holds connection details for an SMTP server.
type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

// SMTPSender sends login codes over SMTP.
type SMTPSender struct {
	cfg SMTPConfig
}

// NewSMTPSender creates a new SMTP email sender.
func NewSMTPSender(cfg SMTPConfig) *SMTPSender {
	return &SMTPSender{cfg: cfg}
}

func (s *SMTPSender) Send(ctx context.Context, email domain.Email, code string) error {
	addr := fmt.Sprintf("%s:%s", s.cfg.Host, s.cfg.Port)
	subject := "Код для входа в Arenda"
	body := fmt.Sprintf("Ваш код для входа: %s\n\nКод действителен 5 минут.", code)
	msg := []byte("To: " + email.String() + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" +
		body + "\r\n")

	var auth smtp.Auth
	if s.cfg.Username != "" && s.cfg.Password != "" {
		auth = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	}

	if err := smtp.SendMail(addr, auth, s.cfg.From, []string{email.String()}, msg); err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	return nil
}
