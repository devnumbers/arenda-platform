package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"time"

	"github.com/google/uuid"
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

	msg, err := s.buildMessage(email.String(), code)
	if err != nil {
		return fmt.Errorf("build message: %w", err)
	}

	addr := fmt.Sprintf("%s:%s", s.cfg.Host, s.cfg.Port)
	dialer := &net.Dialer{Timeout: timeout}

	var conn net.Conn
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

func (s *SMTPSender) buildMessage(to, code string) ([]byte, error) {
	subject := "Код для входа в Arenda"
	plainBody := fmt.Sprintf("Здравствуйте!\n\nКод для входа в личный кабинет Arenda: %s\n\nКод действителен 5 минут.\n\nЕсли вы не запрашивали код, просто проигнорируйте это письмо.", code)
	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html lang="ru">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Код для входа в Arenda</title>
</head>
<body style="font-family: Arial, Helvetica, sans-serif; line-height: 1.5; color: #1a1a1a;">
<p>Здравствуйте!</p>
<p>Код для входа в личный кабинет Arenda:</p>
<p style="font-size: 28px; font-weight: bold; letter-spacing: 4px; margin: 16px 0;">%s</p>
<p>Код действителен 5 минут.</p>
<p style="color: #666; font-size: 14px;">Если вы не запрашивали код, просто проигнорируйте это письмо.</p>
</body>
</html>`, code)

	boundary := uuid.NewString()
	messageID := fmt.Sprintf("<%s@%s>", uuid.NewString(), s.cfg.Host)
	date := time.Now().UTC().Format(time.RFC1123Z)

	var buf bytes.Buffer
	headers := []string{
		"From: " + s.cfg.From,
		"To: " + to,
		"Subject: " + subject,
		"Date: " + date,
		"Message-ID: " + messageID,
		"MIME-Version: 1.0",
		"Content-Type: multipart/alternative; boundary=\"" + boundary + "\"",
		"Reply-To: " + s.cfg.From,
	}
	for _, h := range headers {
		fmt.Fprintf(&buf, "%s\r\n", h)
	}
	fmt.Fprint(&buf, "\r\n")

	fmt.Fprintf(&buf, "--%s\r\n", boundary)
	fmt.Fprintf(&buf, "Content-Type: text/plain; charset=UTF-8\r\n")
	fmt.Fprintf(&buf, "Content-Transfer-Encoding: 8bit\r\n\r\n")
	fmt.Fprintf(&buf, "%s\r\n\r\n", plainBody)

	fmt.Fprintf(&buf, "--%s\r\n", boundary)
	fmt.Fprintf(&buf, "Content-Type: text/html; charset=UTF-8\r\n")
	fmt.Fprintf(&buf, "Content-Transfer-Encoding: 8bit\r\n\r\n")
	fmt.Fprintf(&buf, "%s\r\n\r\n", htmlBody)

	fmt.Fprintf(&buf, "--%s--\r\n", boundary)

	return buf.Bytes(), nil
}
