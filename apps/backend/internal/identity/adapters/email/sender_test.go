package email

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
)

// ttlFiveMinutes is the genitive-plural rendering of five minutes shared by
// the TTL tests (both the domain TTL and the plain 5m duration render it).
const ttlFiveMinutes = "5 минут"

func TestFormatLoginCodeTTL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{name: "actual domain TTL", d: domain.LoginCodeTTL, want: ttlFiveMinutes},
		{name: "1 minute", d: 1 * time.Minute, want: "1 минута"},
		{name: "2 minutes", d: 2 * time.Minute, want: "2 минуты"},
		{name: "5 minutes", d: 5 * time.Minute, want: ttlFiveMinutes},
		{name: "11 minutes", d: 11 * time.Minute, want: "11 минут"},
		{name: "21 minute", d: 21 * time.Minute, want: "21 минута"},
		{name: "22 minutes", d: 22 * time.Minute, want: "22 минуты"},
		{name: "90 minutes", d: 90 * time.Minute, want: "90 минут"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := formatLoginCodeTTL(tc.d); got != tc.want {
				t.Fatalf("formatLoginCodeTTL(%v) = %q, want %q", tc.d, got, tc.want)
			}
		})
	}
}

// fakeRenderer is a renderer stub that captures the template name and data
// passed to Render. It returns the configured plain/html output or an error.
type fakeRenderer struct {
	plain string
	html  string
	err   error

	gotName string
	gotData any
}

func (r *fakeRenderer) Render(name string, data any) (plain, html string, err error) {
	r.gotName = name
	r.gotData = data
	return r.plain, r.html, r.err
}

// fakeMailerSender captures the Message passed to Send.
type fakeMailerSender struct {
	gotMsg mailer.Message
	err    error
}

func (s *fakeMailerSender) Send(_ context.Context, msg mailer.Message) error {
	s.gotMsg = msg
	return s.err
}

func mustPhone(t *testing.T, raw string) domain.Phone {
	t.Helper()
	phone, err := domain.NewPhone(raw)
	if err != nil {
		t.Fatalf("parse phone: %v", err)
	}
	return phone
}

func mustEmail(t *testing.T, raw string) domain.Email {
	t.Helper()
	email, err := domain.NewEmail(raw)
	if err != nil {
		t.Fatalf("parse email: %v", err)
	}
	return email
}

func TestSender_Send_RendersAndSends(t *testing.T) {
	t.Parallel()
	phone := mustPhone(t, "+79160005000")
	emailAddr := mustEmail(t, "owner@example.com")

	renderer := &fakeRenderer{plain: "Your code: 123456", html: "<p>123456</p>"}
	sender := &fakeMailerSender{}
	s := NewSender(sender, renderer)

	if err := s.Send(t.Context(), phone, emailAddr, "123456"); err != nil {
		t.Fatalf("Send error = %v", err)
	}

	// The login_code template was rendered with Code and TTL in the data.
	if renderer.gotName != "login_code" {
		t.Fatalf("render template name = %q, want login_code", renderer.gotName)
	}
	data, ok := renderer.gotData.(map[string]any)
	if !ok {
		t.Fatalf("render data type = %T, want map[string]any", renderer.gotData)
	}
	if data["Code"] != "123456" {
		t.Fatalf("render data Code = %v, want 123456", data["Code"])
	}
	ttl, ok := data["TTL"].(string)
	if !ok {
		t.Fatalf("render data TTL type = %T, want string", data["TTL"])
	}
	if ttl != ttlFiveMinutes {
		t.Fatalf("render data TTL = %q, want %s", ttl, ttlFiveMinutes)
	}

	// The message was sent to the email with the rendered bodies and subject.
	if len(sender.gotMsg.To) != 1 || sender.gotMsg.To[0] != "owner@example.com" {
		t.Fatalf("message To = %v, want [owner@example.com]", sender.gotMsg.To)
	}
	if sender.gotMsg.Subject != loginCodeSubject {
		t.Fatalf("message Subject = %q, want %q", sender.gotMsg.Subject, loginCodeSubject)
	}
	if sender.gotMsg.TextBody != "Your code: 123456" {
		t.Fatalf("message TextBody = %q, want rendered plain", sender.gotMsg.TextBody)
	}
	if sender.gotMsg.HTMLBody != "<p>123456</p>" {
		t.Fatalf("message HTMLBody = %q, want rendered html", sender.gotMsg.HTMLBody)
	}
}

func TestSender_Send_RenderErrorIsWrapped(t *testing.T) {
	t.Parallel()
	phone := mustPhone(t, "+79160005001")
	emailAddr := mustEmail(t, "owner@example.com")

	renderErr := errors.New("template not found")
	renderer := &fakeRenderer{err: renderErr}
	s := NewSender(&fakeMailerSender{}, renderer)

	err := s.Send(t.Context(), phone, emailAddr, "123456")
	if !errors.Is(err, renderErr) {
		t.Fatalf("Send error = %v, want wrap of renderErr", err)
	}
}

func TestSender_Send_SenderErrorIsWrapped(t *testing.T) {
	t.Parallel()
	phone := mustPhone(t, "+79160005002")
	emailAddr := mustEmail(t, "owner@example.com")

	sendErr := errors.New("smtp refused")
	renderer := &fakeRenderer{plain: "plain", html: "html"}
	sender := &fakeMailerSender{err: sendErr}
	s := NewSender(sender, renderer)

	err := s.Send(t.Context(), phone, emailAddr, "123456")
	if !errors.Is(err, sendErr) {
		t.Fatalf("Send error = %v, want wrap of sendErr", err)
	}
}
