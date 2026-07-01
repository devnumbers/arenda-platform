package smtp

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
)

// Config holds connection details for an SMTP server.
type Config struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
	Timeout  time.Duration
}

// Sender delivers email messages over SMTP.
type Sender struct {
	cfg Config
}

// NewSender creates an SMTP sender with the provided configuration.
// If cfg.Timeout is zero, a default timeout of 10 seconds is used.
func NewSender(cfg Config) *Sender {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &Sender{cfg: cfg}
}

// Send transmits the message to all recipients via SMTP.
func (s *Sender) Send(ctx context.Context, msg mailer.Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if len(msg.To) == 0 {
		return fmt.Errorf("no recipients")
	}

	if msg.TextBody == "" && msg.HTMLBody == "" {
		return fmt.Errorf("empty message body")
	}

	body := s.buildMessage(msg)

	addr := fmt.Sprintf("%s:%s", s.cfg.Host, s.cfg.Port)
	dialer := &net.Dialer{Timeout: s.cfg.Timeout}

	var conn net.Conn
	var err error
	switch s.cfg.Port {
	case "465":
		tlsConfig := &tls.Config{ServerName: s.cfg.Host}
		tlsDialer := &tls.Dialer{NetDialer: dialer, Config: tlsConfig}
		conn, err = tlsDialer.DialContext(ctx, "tcp", addr)
	default:
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("dial smtp server: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if err := conn.SetDeadline(time.Now().Add(s.cfg.Timeout)); err != nil {
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
	for _, to := range msg.To {
		if err := client.Rcpt(to); err != nil {
			return fmt.Errorf("smtp rcpt %q: %w", to, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write(body); err != nil {
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

func (s *Sender) buildMessage(msg mailer.Message) []byte {
	boundary := uuid.NewString()
	messageID := fmt.Sprintf("<%s@%s>", uuid.NewString(), s.cfg.Host)
	date := time.Now().UTC().Format(time.RFC1123Z)

	var buf bytes.Buffer
	headers := []string{
		"From: " + s.cfg.From,
		"To: " + strings.Join(msg.To, ", "),
		"Subject: " + msg.Subject,
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

	if msg.TextBody != "" {
		fmt.Fprintf(&buf, "--%s\r\n", boundary)
		fmt.Fprintf(&buf, "Content-Type: text/plain; charset=UTF-8\r\n")
		fmt.Fprintf(&buf, "Content-Transfer-Encoding: 8bit\r\n\r\n")
		fmt.Fprintf(&buf, "%s\r\n\r\n", msg.TextBody)
	}

	if msg.HTMLBody != "" {
		fmt.Fprintf(&buf, "--%s\r\n", boundary)
		fmt.Fprintf(&buf, "Content-Type: text/html; charset=UTF-8\r\n")
		fmt.Fprintf(&buf, "Content-Transfer-Encoding: 8bit\r\n\r\n")
		fmt.Fprintf(&buf, "%s\r\n\r\n", msg.HTMLBody)
	}

	fmt.Fprintf(&buf, "--%s--\r\n", boundary)

	return buf.Bytes()
}
