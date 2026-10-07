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
)

type fakeAuthenticator struct {
	sendCode        func(ctx context.Context, phone domain.Phone, email domain.Email, purpose domain.LoginCodePurpose) error
	sendCodeByPhone func(ctx context.Context, phone domain.Phone) (bool, error)
	verifyCode      func(
		ctx context.Context, phone domain.Phone, email *domain.Email, code string, timezone *string, device application.DeviceContext,
	) (domain.RawSession, domain.User, error)
}

func (f *fakeAuthenticator) SendCode(ctx context.Context, phone domain.Phone, email domain.Email, purpose domain.LoginCodePurpose) error {
	if f.sendCode != nil {
		return f.sendCode(ctx, phone, email, purpose)
	}
	return nil
}

func (f *fakeAuthenticator) SendCodeByPhone(ctx context.Context, phone domain.Phone) (bool, error) {
	if f.sendCodeByPhone != nil {
		return f.sendCodeByPhone(ctx, phone)
	}
	return false, nil
}

func (f *fakeAuthenticator) VerifyCode(
	ctx context.Context,
	phone domain.Phone,
	email *domain.Email,
	code string,
	timezone *string,
	device application.DeviceContext,
) (domain.RawSession, domain.User, error) {
	if f.verifyCode != nil {
		return f.verifyCode(ctx, phone, email, code, timezone, device)
	}
	return domain.RawSession{}, domain.User{}, errors.New("unexpected VerifyCode call")
}

func newTestAuthHandlers(auth Authenticator) *AuthHandlers {
	return NewAuthHandlers(auth, nil, nil, nil, nil, nil, nil, false, slog.New(slog.DiscardHandler), AuthRateLimits{}, nil)
}

func doJSON(t *testing.T, handler http.HandlerFunc, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	return rr
}

func TestSendCode_PhoneOnly_RegisteredUser(t *testing.T) {
	t.Parallel()
	auth := &fakeAuthenticator{
		sendCodeByPhone: func(_ context.Context, phone domain.Phone) (bool, error) {
			if phone.String() != "+79150000001" {
				t.Errorf("SendCodeByPhone phone = %s, want +79150000001", phone)
			}
			return true, nil
		},
	}
	h := newTestAuthHandlers(auth)

	rr := doJSON(t, h.SendCode, "/auth/send", `{"phone":"+79150000001"}`)

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
}

func TestSendCode_PhoneOnly_CodeSentTooRecentlyReturns429(t *testing.T) {
	t.Parallel()
	auth := &fakeAuthenticator{
		sendCodeByPhone: func(context.Context, domain.Phone) (bool, error) {
			return false, application.ErrCodeSentTooRecently
		},
	}
	h := newTestAuthHandlers(auth)

	rr := doJSON(t, h.SendCode, "/auth/send", `{"phone":"+79150000001"}`)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429: %s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Retry-After"); got != "60" {
		t.Fatalf("Retry-After = %q, want 60", got)
	}
}

func TestSendCode_PhoneOnly_UnknownUser(t *testing.T) {
	t.Parallel()
	auth := &fakeAuthenticator{
		sendCodeByPhone: func(context.Context, domain.Phone) (bool, error) {
			return false, nil
		},
	}
	h := newTestAuthHandlers(auth)

	rr := doJSON(t, h.SendCode, "/auth/send", `{"phone":"+79150000002"}`)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var resp authSendCodeResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Sent {
		t.Fatal("sent = true, want false")
	}
	if resp.RetryAfter != nil {
		t.Fatalf("retryAfter = %v, want absent", *resp.RetryAfter)
	}
}

func TestSendCode_WithEmail_MismatchReturns409(t *testing.T) {
	t.Parallel()
	var gotEmail domain.Email
	auth := &fakeAuthenticator{
		sendCode: func(_ context.Context, _ domain.Phone, email domain.Email, _ domain.LoginCodePurpose) error {
			gotEmail = email
			return application.ErrEmailDoesNotMatch
		},
		sendCodeByPhone: func(context.Context, domain.Phone) (bool, error) {
			t.Fatal("SendCodeByPhone must not be called when email is provided")
			return false, nil
		},
	}
	h := newTestAuthHandlers(auth)

	rr := doJSON(t, h.SendCode, "/auth/send", `{"phone":"+79150000001","email":"owner@example.com"}`)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409: %s", rr.Code, rr.Body.String())
	}
	if gotEmail.String() != testOwnerEmail {
		t.Fatalf("SendCode email = %s, want owner@example.com", gotEmail)
	}
}

func TestSendCode_WithEmail_EmailAlreadyTakenReturns409(t *testing.T) {
	t.Parallel()
	auth := &fakeAuthenticator{
		sendCode: func(context.Context, domain.Phone, domain.Email, domain.LoginCodePurpose) error {
			return application.ErrEmailAlreadyTaken
		},
		sendCodeByPhone: func(context.Context, domain.Phone) (bool, error) {
			t.Fatal("SendCodeByPhone must not be called when email is provided")
			return false, nil
		},
	}
	h := newTestAuthHandlers(auth)

	rr := doJSON(t, h.SendCode, "/auth/send", `{"phone":"+79150000009","email":"taken@example.com"}`)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409: %s", rr.Code, rr.Body.String())
	}
	var resp openapi.Problem
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Detail == nil || *resp.Detail != detailEmailTaken {
		t.Fatalf("detail = %v, want %q", resp.Detail, detailEmailTaken)
	}
}

func TestVerifyCode_WithoutEmail_PassesNilEmailAndSetsCookie(t *testing.T) {
	t.Parallel()
	phone, err := domain.NewPhone("+79150000001")
	if err != nil {
		t.Fatalf("parse phone: %v", err)
	}
	user, err := domain.NewOwner(phone)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	raw := domain.RawSession{
		Token:   testRawToken,
		Session: domain.Session{UserID: user.ID, ExpiresAt: time.Now().Add(time.Hour)},
	}

	var gotEmail *domain.Email
	var gotTimezone *string
	auth := &fakeAuthenticator{
		verifyCode: func(
			_ context.Context, _ domain.Phone, email *domain.Email, code string, timezone *string, _ application.DeviceContext,
		) (domain.RawSession, domain.User, error) {
			if code != "123456" {
				t.Errorf("VerifyCode code = %s, want 123456", code)
			}
			gotEmail = email
			gotTimezone = timezone
			return raw, user, nil
		},
	}
	h := newTestAuthHandlers(auth)

	rr := doJSON(t, h.VerifyCode, "/auth/verify", `{"phone":"+79150000001","code":"123456"}`)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	if gotEmail != nil {
		t.Fatalf("VerifyCode email = %v, want nil", gotEmail)
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
	phone, err := domain.NewPhone("+79150000002")
	if err != nil {
		t.Fatalf("parse phone: %v", err)
	}
	user, err := domain.NewOwner(phone)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	raw := domain.RawSession{
		Token:   testRawToken,
		Session: domain.Session{UserID: user.ID, ExpiresAt: time.Now().Add(time.Hour)},
	}

	var gotTimezone *string
	auth := &fakeAuthenticator{
		verifyCode: func(_ context.Context, _ domain.Phone, _ *domain.Email, _ string, timezone *string,
			_ application.DeviceContext,
		) (domain.RawSession, domain.User, error) {
			gotTimezone = timezone
			return raw, user, nil
		},
	}
	h := newTestAuthHandlers(auth)

	rr := doJSON(t, h.VerifyCode, "/auth/verify", `{"phone":"+79150000002","code":"123456","timezone":"Asia/Yekaterinburg"}`)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	if gotTimezone == nil || *gotTimezone != "Asia/Yekaterinburg" {
		t.Fatalf("VerifyCode timezone = %v, want Asia/Yekaterinburg", gotTimezone)
	}
}
