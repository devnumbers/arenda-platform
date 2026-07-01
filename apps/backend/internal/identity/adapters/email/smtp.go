package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

// SMTPConfig holds connection details for an SMTP server.
type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
	Timeout  time.Duration
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
	if err := ctx.Err(); err != nil {
		return err
	}

	timeout := s.cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	addr := fmt.Sprintf("%s:%s", s.cfg.Host, s.cfg.Port)
	subject := "Код для входа в Arenda"
	body := fmt.Sprintf("Ваш код для входа: %s\n\nКод действителен 5 минут.", code)
	msg := []byte("To: " + email.String() + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" +
		body + "\r\n")

	dialer := &net.Dialer{Timeout: timeout}

	var conn net.Conn
	var err error

	switch s.cfg.Port {
	case "465":
		// SMTPS: TLS handshake happens immediately after TCP connect.
		tlsConfig := &tls.Config{ServerName: s.cfg.Host}
		tlsDialer := &tls.Dialer{NetDialer: dialer, Config: tlsConfig}
		conn, err = tlsDialer.DialContext(ctx, "tcp", addr)
	default:
		// Plain SMTP; STARTTLS is attempted on port 587 below.
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("dial smtp server: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return fmt.Errorf("set smtp deadline: %w", err)
	}

	client, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		return fmt.Errorf("create smtp client: %w", err)
	}
	defer func() { _ = client.Close() }()

	if s.cfg.Port == "587" {
		tlsConfig := &tls.Config{ServerName: s.cfg.Host}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("start tls: %w", err)
		}
	}

	var auth smtp.Auth
	if s.cfg.Username != "" && s.cfg.Password != "" {
		auth = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	}

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := client.Mail(s.cfg.From); err != nil {
		return fmt.Errorf("smtp mail: %w", err)
	}
	if err := client.Rcpt(email.String()); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("write message: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close message writer: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("smtp quit: %w", err)
	}

	return nil
}
