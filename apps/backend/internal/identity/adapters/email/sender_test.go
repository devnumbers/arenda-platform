package email

import (
	"context"
	"errors"
	"strings"
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

// assertLetterRender checks the template was rendered by name with the code
// and the domain TTL in the data.
func assertLetterRender(t *testing.T, renderer *fakeRenderer, wantTemplate string) {
	t.Helper()
	if renderer.gotName != wantTemplate {
		t.Fatalf("render template name = %q, want %q", renderer.gotName, wantTemplate)
	}
	data, ok := renderer.gotData.(map[string]any)
	if !ok {
		t.Fatalf("render data type = %T, want map[string]any", renderer.gotData)
	}
	if data["Code"] != "123456" {
		t.Fatalf("render data Code = %v, want 123456", data["Code"])
	}
	if ttl, ok := data["TTL"].(string); !ok || ttl != ttlFiveMinutes {
		t.Fatalf("render data TTL = %v, want %q", data["TTL"], ttlFiveMinutes)
	}
}

// assertLetterMessage checks the rendered message went to the address with the
// operation-naming subject that never carries the code.
func assertLetterMessage(t *testing.T, msg mailer.Message, wantTo, wantSubject string) {
	t.Helper()
	if len(msg.To) != 1 || msg.To[0] != wantTo {
		t.Fatalf("message To = %v, want [%s]", msg.To, wantTo)
	}
	if msg.Subject != wantSubject {
		t.Fatalf("message Subject = %q, want %q", msg.Subject, wantSubject)
	}
	if strings.Contains(msg.Subject, "123456") {
		t.Fatalf("message Subject %q carries the code", msg.Subject)
	}
}

// purpose+step pair renders its own template and sends its own operation-naming
// subject (issue #1204) that never carries the code (research #1201).
func TestSender_Send_LetterForPurposeAndStep(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		purpose      domain.LoginCodePurpose
		step         domain.LoginCodeStep
		wantTemplate string
		wantSubject  string
	}{
		{
			name:         "login code to the current email",
			purpose:      domain.LoginCodePurposeLogin,
			step:         domain.LoginCodeStepCurrentEmail,
			wantTemplate: "login_code",
			wantSubject:  "Код для входа в Рентли",
		},
		{
			name:         "phone change code to the current email",
			purpose:      domain.LoginCodePurposePhoneChange,
			step:         domain.LoginCodeStepCurrentEmail,
			wantTemplate: "phone_change_code",
			wantSubject:  "Код для смены телефона в Рентли",
		},
		{
			name:         "email change code to the current address",
			purpose:      domain.LoginCodePurposeEmailChange,
			step:         domain.LoginCodeStepCurrentEmail,
			wantTemplate: "email_change_current_code",
			wantSubject:  "Код для подтверждения смены email в Рентли",
		},
		{
			name:         "email change code to the new address",
			purpose:      domain.LoginCodePurposeEmailChange,
			step:         domain.LoginCodeStepNewEmail,
			wantTemplate: "email_change_new_code",
			wantSubject:  "Код для подтверждения нового email в Рентли",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			phone := mustPhone(t, "+79160005000")
			emailAddr := mustEmail(t, "owner@example.com")
			renderer := &fakeRenderer{plain: "plain body 123456", html: "<p>html body 123456</p>"}
			sender := &fakeMailerSender{}
			s := NewSender(sender, renderer)

			if err := s.Send(t.Context(), phone, emailAddr, "123456", tc.purpose, tc.step); err != nil {
				t.Fatalf("Send error = %v", err)
			}

			assertLetterRender(t, renderer, tc.wantTemplate)
			assertLetterMessage(t, sender.gotMsg, "owner@example.com", tc.wantSubject)

			if sender.gotMsg.TextBody != "plain body 123456" {
				t.Fatalf("message TextBody = %q, want rendered plain", sender.gotMsg.TextBody)
			}
			if sender.gotMsg.HTMLBody != "<p>html body 123456</p>" {
				t.Fatalf("message HTMLBody = %q, want rendered html", sender.gotMsg.HTMLBody)
			}
		})
	}
}

// TestSender_Send_UnknownPurposeStepPairFails pins the total mapping: a pair
// without a letter is a programming error that fails the delivery before any
// render or send happens.
func TestSender_Send_UnknownPurposeStepPairFails(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		purpose domain.LoginCodePurpose
		step    domain.LoginCodeStep
	}{
		{name: "login to the new address", purpose: domain.LoginCodePurposeLogin, step: domain.LoginCodeStepNewEmail},
		{name: "phone change to the new address", purpose: domain.LoginCodePurposePhoneChange, step: domain.LoginCodeStepNewEmail},
		{name: "empty step", purpose: domain.LoginCodePurposeLogin, step: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			phone := mustPhone(t, "+79160005001")
			emailAddr := mustEmail(t, "owner@example.com")
			renderer := &fakeRenderer{plain: "plain", html: "html"}
			sender := &fakeMailerSender{}
			s := NewSender(sender, renderer)

			err := s.Send(t.Context(), phone, emailAddr, "123456", tc.purpose, tc.step)
			if err == nil {
				t.Fatal("Send error = nil, want unknown-pair failure")
			}
			if renderer.gotName != "" {
				t.Fatalf("template %q was rendered for an unknown pair", renderer.gotName)
			}
			if sender.gotMsg.Subject != "" {
				t.Fatalf("message %q was sent for an unknown pair", sender.gotMsg.Subject)
			}
		})
	}
}

// TestSender_Send_RealTemplates_CarryRequiredBlocks pins the copy contract of
// the four letters against the real template files (issue #1204, requirements
// from research #1201): the operation line unique to the letter, the code, the
// TTL, the do-not-share warning, the ignore line — and the shared text anchor
// «Код подтверждения: NNNNNN» the e2e fixture parses codes from.
func TestSender_Send_RealTemplates_CarryRequiredBlocks(t *testing.T) {
	t.Parallel()
	renderer, err := mailer.NewRenderer("../../../../templates/email")
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	tests := []struct {
		name          string
		purpose       domain.LoginCodePurpose
		step          domain.LoginCodeStep
		wantOperation string
	}{
		{
			name:          "login code",
			purpose:       domain.LoginCodePurposeLogin,
			step:          domain.LoginCodeStepCurrentEmail,
			wantOperation: "Вы запросили код для входа в Рентли",
		},
		{
			name:          "phone change code",
			purpose:       domain.LoginCodePurposePhoneChange,
			step:          domain.LoginCodeStepCurrentEmail,
			wantOperation: "Вы запросили смену телефона в Рентли",
		},
		{
			name:          "email change code to the current address",
			purpose:       domain.LoginCodePurposeEmailChange,
			step:          domain.LoginCodeStepCurrentEmail,
			wantOperation: "Вы запросили смену email в Рентли",
		},
		{
			name:          "email change code to the new address",
			purpose:       domain.LoginCodePurposeEmailChange,
			step:          domain.LoginCodeStepNewEmail,
			wantOperation: "Вы запросили смену email в Рентли",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			phone := mustPhone(t, "+79160005002")
			emailAddr := mustEmail(t, "letters@example.com")
			sender := &fakeMailerSender{}
			s := NewSender(sender, renderer)

			if err := s.Send(t.Context(), phone, emailAddr, "123456", tc.purpose, tc.step); err != nil {
				t.Fatalf("Send error = %v", err)
			}

			plain, html := sender.gotMsg.TextBody, sender.gotMsg.HTMLBody
			if plain == "" || html == "" {
				t.Fatalf("empty body: plain=%d bytes, html=%d bytes", len(plain), len(html))
			}
			// A slice, not a map: the missing-fragment report is deterministic.
			fragments := []struct {
				text  string
				where string
			}{
				{tc.wantOperation, "operation line"},
				{"Код подтверждения: 123456", "e2e code anchor"},
				{"Код действует " + ttlFiveMinutes, "TTL line"},
				{"Никому не сообщайте этот код", "do-not-share warning"},
				{"Если вы не запрашивали код", "ignore line"},
			}
			for _, fragment := range fragments {
				if !strings.Contains(plain, fragment.text) {
					t.Fatalf("text body misses the %s (%q):\n%s", fragment.where, fragment.text, plain)
				}
			}
			if !strings.Contains(html, "123456") {
				t.Fatalf("html body misses the code:\n%s", html)
			}
		})
	}
}

func TestSender_Send_RenderErrorIsWrapped(t *testing.T) {
	t.Parallel()
	phone := mustPhone(t, "+79160005003")
	emailAddr := mustEmail(t, "owner@example.com")

	renderErr := errors.New("template not found")
	renderer := &fakeRenderer{err: renderErr}
	s := NewSender(&fakeMailerSender{}, renderer)

	err := s.Send(t.Context(), phone, emailAddr, "123456", domain.LoginCodePurposeLogin, domain.LoginCodeStepCurrentEmail)
	if !errors.Is(err, renderErr) {
		t.Fatalf("Send error = %v, want wrap of renderErr", err)
	}
}

func TestSender_Send_SenderErrorIsWrapped(t *testing.T) {
	t.Parallel()
	phone := mustPhone(t, "+79160005004")
	emailAddr := mustEmail(t, "owner@example.com")

	sendErr := errors.New("smtp refused")
	renderer := &fakeRenderer{plain: "plain", html: "html"}
	sender := &fakeMailerSender{err: sendErr}
	s := NewSender(sender, renderer)

	err := s.Send(t.Context(), phone, emailAddr, "123456", domain.LoginCodePurposeLogin, domain.LoginCodeStepCurrentEmail)
	if !errors.Is(err, sendErr) {
		t.Fatalf("Send error = %v, want wrap of sendErr", err)
	}
}
