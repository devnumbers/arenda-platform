package smtp

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
)

// TestTLSConfig_PinsTLS12Floor verifies the TLS configuration used for both
// implicit TLS (port 465) and STARTTLS (port 587): the server name must be
// carried for certificate validation, and the minimum protocol version must be
// pinned to TLS 1.2 explicitly (semgrep p/ci missing-ssl-minversion, #318) —
// Go's client default floor is TLS 1.2 today, and the pin keeps it from
// silently dropping if that default ever changes.
func TestTLSConfig_PinsTLS12Floor(t *testing.T) {
	t.Parallel()

	cfg := tlsConfig("smtp.example.com")

	if cfg.ServerName != "smtp.example.com" {
		t.Fatalf("ServerName = %q, want %q", cfg.ServerName, "smtp.example.com")
	}
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion = %d, want %d (tls.VersionTLS12)", cfg.MinVersion, tls.VersionTLS12)
	}
}

// fakeSMTPServer is a minimal SMTP server recording the EHLO/HELO greeting
// argument. It answers just enough of the transaction for the stdlib client
// and runs over plain TCP on an ephemeral port — with no Username configured
// Send skips both STARTTLS and AUTH, so no TLS machinery is needed here.
type fakeSMTPServer struct {
	listener net.Listener
	port     string

	mu   sync.Mutex
	helo string
}

func newFakeSMTPServer(t *testing.T) *fakeSMTPServer {
	t.Helper()

	var lc net.ListenConfig
	listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := &fakeSMTPServer{listener: listener}
	t.Cleanup(func() {
		if cerr := listener.Close(); cerr != nil {
			t.Logf("fake smtp: close listener: %v", cerr)
		}
	})

	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split listener address %q: %v", listener.Addr().String(), err)
	}
	server.port = port

	go server.serve()
	return server
}

func (s *fakeSMTPServer) heloName() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.helo
}

func (s *fakeSMTPServer) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *fakeSMTPServer) handle(conn net.Conn) {
	writeSMTP(conn, "220 test ESMTP\r\n")

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		verb, arg, _ := strings.Cut(scanner.Text(), " ")
		switch strings.ToUpper(verb) {
		case "EHLO", "HELO":
			s.mu.Lock()
			s.helo = arg
			s.mu.Unlock()
			writeSMTP(conn, "250-test\r\n250-8BITMIME\r\n250 SIZE 10485760\r\n")
		case "MAIL", "RCPT":
			writeSMTP(conn, "250 2.1.0 OK\r\n")
		case "DATA":
			writeSMTP(conn, "354 end data with <CRLF>.<CRLF>\r\n")
			for scanner.Scan() {
				if scanner.Text() == "." {
					break
				}
			}
			writeSMTP(conn, "250 2.0.0 OK\r\n")
		case "QUIT":
			writeSMTP(conn, "221 bye\r\n")
		}
	}

	// The client side always closes first (Send's deferred conn.Close or the
	// test teardown), so a close failure here has no better destination —
	// the dialog is over either way.
	if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		return
	}
}

// writeSMTP sends one SMTP response line. A failed write means the client has
// gone away; the read loop unwinds on the next scan error.
func writeSMTP(conn net.Conn, resp string) {
	if _, err := fmt.Fprint(conn, resp); err != nil {
		return
	}
}

func sendTestMessage(t *testing.T, sender *Sender) error {
	t.Helper()

	return sender.Send(context.Background(), mailer.Message{
		To:       []string{"user@example.com"},
		Subject:  "Код для входа в Рентли",
		TextBody: "Код: 123456",
	})
}

// TestSend_GreetsWithConfiguredHeloName pins the greeting seam: the EHLO line
// must carry the configured domain, not the stdlib default. Relays reject
// "localhost"-greeted sessions at AUTH with 455 4.4.5 (Yandex, 2026-10-12),
// which surfaces as "smtp auth: EOF" on every send.
func TestSend_GreetsWithConfiguredHeloName(t *testing.T) {
	t.Parallel()

	server := newFakeSMTPServer(t)
	sender := NewSender(Config{
		Host:     "127.0.0.1",
		Port:     server.port,
		HeloName: "dev.rentlee.ru",
		From:     "no-reply@rentlee.ru",
		Timeout:  2 * time.Second,
	})

	if err := sendTestMessage(t, sender); err != nil {
		t.Fatalf("send: %v", err)
	}
	if got := server.heloName(); got != "dev.rentlee.ru" {
		t.Fatalf("EHLO = %q, want %q", got, "dev.rentlee.ru")
	}
}

// TestSend_EmptyHeloNameKeepsStdlibDefault documents the fallback: an unset
// HeloName leaves the stdlib "localhost" greeting (local development against
// test relays that do not care).
func TestSend_EmptyHeloNameKeepsStdlibDefault(t *testing.T) {
	t.Parallel()

	server := newFakeSMTPServer(t)
	sender := NewSender(Config{
		Host:    "127.0.0.1",
		Port:    server.port,
		From:    "no-reply@rentlee.ru",
		Timeout: 2 * time.Second,
	})

	if err := sendTestMessage(t, sender); err != nil {
		t.Fatalf("send: %v", err)
	}
	if got := server.heloName(); got != "localhost" {
		t.Fatalf("EHLO = %q, want stdlib default %q", got, "localhost")
	}
}
