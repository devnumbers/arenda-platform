package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	"golang.org/x/time/rate"
)

type fakeAuthenticator struct {
	sendCode   func(ctx context.Context, email domain.Email, purpose domain.LoginCodePurpose) error
	verifyCode func(
		ctx context.Context, email domain.Email, code string, timezone *string, device application.DeviceContext,
	) (domain.RawSession, domain.User, error)
}

func (f *fakeAuthenticator) SendCode(ctx context.Context, email domain.Email, purpose domain.LoginCodePurpose) error {
	if f.sendCode != nil {
		return f.sendCode(ctx, email, purpose)
	}
	return nil
}

func (f *fakeAuthenticator) VerifyCode(
	ctx context.Context,
	email domain.Email,
	code string,
	timezone *string,
	device application.DeviceContext,
) (domain.RawSession, domain.User, error) {
	if f.verifyCode != nil {
		return f.verifyCode(ctx, email, code, timezone, device)
	}
	return domain.RawSession{}, domain.User{}, errors.New("unexpected VerifyCode call")
}

func newTestAuthHandlers(auth Authenticator) *AuthHandlers {
	return NewAuthHandlers(auth, nil, nil, nil, nil, nil, false, slog.New(slog.DiscardHandler), AuthRateLimits{}, nil)
}

func doJSON(t *testing.T, handler http.HandlerFunc, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	return rr
}

func TestSendCode_PassesCanonicalEmailToService(t *testing.T) {
	t.Parallel()
	var gotEmail domain.Email
	auth := &fakeAuthenticator{
		sendCode: func(_ context.Context, email domain.Email, purpose domain.LoginCodePurpose) error {
			gotEmail = email
			if purpose != domain.LoginCodePurposeLogin {
				t.Errorf("SendCode purpose = %s, want login", purpose)
			}
			return nil
		},
	}
	h := newTestAuthHandlers(auth)

	// Mixed case and surrounding whitespace: the handler hands the service the
	// canonical (trimmed, lowercased) form.
	rr := doJSON(t, h.SendCode, "/auth/send", `{"email":"  Owner@Example.com  "}`)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var resp authSendCodeResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Sent {
		t.Fatal("sent = false, want true")
	}
	if resp.RetryAfter == nil || *resp.RetryAfter != httpsupport.RetryAfterSeconds {
		t.Fatalf("retryAfter = %v, want %d", resp.RetryAfter, httpsupport.RetryAfterSeconds)
	}
	if gotEmail.String() != "owner@example.com" {
		t.Fatalf("SendCode email = %s, want the canonical owner@example.com", gotEmail)
	}
}

func TestSendCode_InvalidEmailReturns400(t *testing.T) {
	t.Parallel()
	h := newTestAuthHandlers(&fakeAuthenticator{})

	for name, body := range map[string]string{
		"malformed address": `{"email":"not-an-email"}`,
		"missing field":     `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rr := doJSON(t, h.SendCode, "/auth/send", body)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestSendCode_CodeSentTooRecentlyReturns429(t *testing.T) {
	t.Parallel()
	auth := &fakeAuthenticator{
		sendCode: func(context.Context, domain.Email, domain.LoginCodePurpose) error {
			return application.ErrCodeSentTooRecently
		},
	}
	h := newTestAuthHandlers(auth)

	rr := doJSON(t, h.SendCode, "/auth/send", `{"email":"owner@example.com"}`)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429: %s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Retry-After"); got != "60" {
		t.Fatalf("Retry-After = %q, want 60", got)
	}
}

// TestSendCode_PerIPLimiterBlocks pins the hard IP layer of the throttling
// pair: when the per-IP issuance budget is spent, the request is refused 429
// before the service is reached.
func TestSendCode_PerIPLimiterBlocks(t *testing.T) {
	t.Parallel()
	calls := 0
	auth := &fakeAuthenticator{
		sendCode: func(context.Context, domain.Email, domain.LoginCodePurpose) error {
			calls++
			return nil
		},
	}
	limits := AuthRateLimits{SendPerIP: httpsupport.NewRateLimiter(rate.Limit(1), 1, time.Hour)}
	h := NewAuthHandlers(auth, nil, nil, nil, nil, nil, false, slog.New(slog.DiscardHandler), limits, nil)

	rr := doJSON(t, h.SendCode, "/auth/send", `{"email":"first@example.com"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want 200", rr.Code)
	}
	rr = doJSON(t, h.SendCode, "/auth/send", `{"email":"second@example.com"}`)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d, want 429: %s", rr.Code, rr.Body.String())
	}
	if calls != 1 {
		t.Fatalf("service calls = %d, want 1 (the blocked request never reaches the service)", calls)
	}
}

func TestVerifyCode_PassesEmailAndSetsCookie(t *testing.T) {
	t.Parallel()
	email, err := domain.NewEmail("owner@example.com")
	if err != nil {
		t.Fatalf("parse email: %v", err)
	}
	user, err := domain.NewOwner(email)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	raw := domain.RawSession{
		Token:   testRawToken,
		Session: domain.Session{UserID: user.ID, ExpiresAt: time.Now().Add(time.Hour)},
	}

	var gotEmail domain.Email
	var gotTimezone *string
	auth := &fakeAuthenticator{
		verifyCode: func(
			_ context.Context, got domain.Email, code string, timezone *string, _ application.DeviceContext,
		) (domain.RawSession, domain.User, error) {
			if code != "123456" {
				t.Errorf("VerifyCode code = %s, want 123456", code)
			}
			gotEmail = got
			gotTimezone = timezone
			return raw, user, nil
		},
	}
	h := newTestAuthHandlers(auth)

	rr := doJSON(t, h.VerifyCode, "/auth/verify", `{"email":"owner@example.com","code":"123456"}`)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	if gotEmail.String() != "owner@example.com" {
		t.Fatalf("VerifyCode email = %s, want owner@example.com", gotEmail)
	}
	if gotTimezone != nil {
		t.Fatalf("VerifyCode timezone = %v, want nil for a body without it", *gotTimezone)
	}
	if cookie := rr.Header().Get("Set-Cookie"); !strings.Contains(cookie, "session_id=raw-token") {
		t.Fatalf("Set-Cookie = %q, want session_id=raw-token", cookie)
	}
}

func TestVerifyCode_PassesTimezoneToService(t *testing.T) {
	t.Parallel()
	email, err := domain.NewEmail("tz@example.com")
	if err != nil {
		t.Fatalf("parse email: %v", err)
	}
	user, err := domain.NewOwner(email)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	raw := domain.RawSession{
		Token:   testRawToken,
		Session: domain.Session{UserID: user.ID, ExpiresAt: time.Now().Add(time.Hour)},
	}

	var gotTimezone *string
	auth := &fakeAuthenticator{
		verifyCode: func(_ context.Context, _ domain.Email, _ string, timezone *string,
			_ application.DeviceContext,
		) (domain.RawSession, domain.User, error) {
			gotTimezone = timezone
			return raw, user, nil
		},
	}
	h := newTestAuthHandlers(auth)

	rr := doJSON(t, h.VerifyCode, "/auth/verify", `{"email":"tz@example.com","code":"123456","timezone":"Asia/Yekaterinburg"}`)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	if gotTimezone == nil || *gotTimezone != "Asia/Yekaterinburg" {
		t.Fatalf("VerifyCode timezone = %v, want Asia/Yekaterinburg", gotTimezone)
	}
}

func TestVerifyCode_InvalidCodeReturns401(t *testing.T) {
	t.Parallel()
	auth := &fakeAuthenticator{
		verifyCode: func(context.Context, domain.Email, string, *string, application.DeviceContext) (domain.RawSession, domain.User, error) {
			return domain.RawSession{}, domain.User{}, domain.ErrLoginCodeInvalid
		},
	}
	h := newTestAuthHandlers(auth)

	rr := doJSON(t, h.VerifyCode, "/auth/verify", `{"email":"owner@example.com","code":"000000"}`)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401: %s", rr.Code, rr.Body.String())
	}
	var resp openapi.Problem
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Detail == nil || *resp.Detail != "Неверная почта или код" {
		t.Fatalf("detail = %v, want the unified invalid-code text", resp.Detail)
	}
}
