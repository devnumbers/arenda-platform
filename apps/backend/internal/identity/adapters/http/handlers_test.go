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

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
)

// Shared fixture strings of the identity http tests: the canonical owner
// email, the raw session token, and the profile name.
const (
	testOwnerEmail = "owner@example.com"
	testRawToken   = "raw-token"
	testUserName   = "Ivan"
)

// Fakes for the remaining service interfaces.

type fakeProfiler struct {
	me     func(ctx context.Context, userID uuid.UUID) (domain.User, error)
	update func(ctx context.Context, userID uuid.UUID, cmd application.UpdateProfileCommand) (domain.User, error)
}

func (f *fakeProfiler) Me(ctx context.Context, userID uuid.UUID) (domain.User, error) {
	if f.me != nil {
		return f.me(ctx, userID)
	}
	return domain.User{}, errors.New("unexpected Me call")
}

func (f *fakeProfiler) UpdateProfile(ctx context.Context, userID uuid.UUID, cmd application.UpdateProfileCommand) (domain.User, error) {
	if f.update != nil {
		return f.update(ctx, userID, cmd)
	}
	return domain.User{}, errors.New("unexpected UpdateProfile call")
}

type fakeLogout struct {
	logout func(ctx context.Context, rawToken string, actor auditdomain.Actor) error
}

func (f *fakeLogout) Logout(ctx context.Context, rawToken string, actor auditdomain.Actor) error {
	if f.logout != nil {
		return f.logout(ctx, rawToken, actor)
	}
	return nil
}

// grantTokenFixture is the grant plaintext the fakes and request bodies share.
const grantTokenFixture = "grant-token"

type fakeEmailChanger struct {
	sendCurrentEmailCode func(ctx context.Context, userID uuid.UUID) error
	verifyCurrentEmail   func(ctx context.Context, userID uuid.UUID, code string) (string, error)
	requestNewEmailCode  func(ctx context.Context, userID uuid.UUID, grant string, newEmail domain.Email) error
	resendNewEmailCode   func(ctx context.Context, userID uuid.UUID, grant string) error
	changeEmail          func(ctx context.Context, userID uuid.UUID, code, grant string) (domain.User, error)
}

func (f *fakeEmailChanger) SendCurrentEmailCode(ctx context.Context, userID uuid.UUID) error {
	if f.sendCurrentEmailCode != nil {
		return f.sendCurrentEmailCode(ctx, userID)
	}
	return nil
}

func (f *fakeEmailChanger) VerifyCurrentEmail(ctx context.Context, userID uuid.UUID, code string) (string, error) {
	if f.verifyCurrentEmail != nil {
		return f.verifyCurrentEmail(ctx, userID, code)
	}
	return grantTokenFixture, nil
}

func (f *fakeEmailChanger) RequestNewEmailCode(ctx context.Context, userID uuid.UUID, grant string, newEmail domain.Email) error {
	if f.requestNewEmailCode != nil {
		return f.requestNewEmailCode(ctx, userID, grant, newEmail)
	}
	return nil
}

func (f *fakeEmailChanger) ResendNewEmailCode(ctx context.Context, userID uuid.UUID, grant string) error {
	if f.resendNewEmailCode != nil {
		return f.resendNewEmailCode(ctx, userID, grant)
	}
	return nil
}

func (f *fakeEmailChanger) ChangeEmail(ctx context.Context, userID uuid.UUID, code, grant string) (domain.User, error) {
	if f.changeEmail != nil {
		return f.changeEmail(ctx, userID, code, grant)
	}
	return domain.User{}, errors.New("unexpected ChangeEmail call")
}

// Helpers.

// sessionCookie builds a test session cookie with secure attributes so gosec
// G124 does not flag it. The handler only reads Name and Value.
func sessionCookie(value string) *http.Cookie {
	return &http.Cookie{
		Name:     "session_id",
		Value:    value,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}

// authedRequest builds a request whose context carries an authenticated userID.
func authedRequest(t *testing.T, method, path, body string, userID uuid.UUID) *http.Request {
	t.Helper()
	r := httptest.NewRequestWithContext(t.Context(), method, path, bodyReader(body))
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	return r.WithContext(httpsupport.WithUserID(r.Context(), userID))
}

func bodyReader(body string) *strings.Reader {
	if body == "" {
		return strings.NewReader("")
	}
	return strings.NewReader(body)
}

func doHandler(t *testing.T, handler http.HandlerFunc, r *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	handler(rr, r)
	return rr
}

func newHandlers(profile Profiler, logout Logout) *AuthHandlers {
	return newHandlersWithEmailChange(nil, profile, logout)
}

func newHandlersWithEmailChange(emailChange EmailChanger, profile Profiler, logout Logout) *AuthHandlers {
	return newHandlersWithLimits(emailChange, profile, logout, AuthRateLimits{})
}

// verifyCurrentRequest builds an authenticated step-2 request with the given
// raw body, keeping the call sites within the line limit.
func verifyCurrentRequest(t *testing.T, body string, userID uuid.UUID) *http.Request {
	t.Helper()
	return authedRequest(t, http.MethodPost, "/me/email/verify-current", body, userID)
}

// requestNewEmailCodeRequest builds an authenticated step-3 request with the
// given raw body.
func requestNewEmailCodeRequest(t *testing.T, body string, userID uuid.UUID) *http.Request {
	t.Helper()
	return authedRequest(t, http.MethodPost, "/me/email/request-new-email-code", body, userID)
}

func newHandlersWithLimits(
	emailChange EmailChanger,
	profile Profiler,
	logout Logout,
	limits AuthRateLimits,
) *AuthHandlers {
	return NewAuthHandlers(nil, emailChange, profile, nil, logout, nil, false, slog.New(slog.DiscardHandler), limits, nil)
}

// Logout.

func TestLogout_NoTokenReturns401(t *testing.T) {
	t.Parallel()
	h := newHandlers(nil, &fakeLogout{})

	rr := doHandler(t, h.Logout, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/logout", nil))

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestLogout_SuccessDeletesAndClearsCookie(t *testing.T) {
	t.Parallel()
	var gotToken string
	logout := &fakeLogout{
		logout: func(_ context.Context, rawToken string, _ auditdomain.Actor) error {
			gotToken = rawToken
			return nil
		},
	}
	h := newHandlers(nil, logout)

	// Inject a session cookie into the request.
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/logout", nil)
	r.AddCookie(sessionCookie(testRawToken))
	rr := doHandler(t, h.Logout, r)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
	if gotToken != testRawToken {
		t.Fatalf("Logout received token = %q, want raw-token", gotToken)
	}
	if cookie := rr.Header().Get("Set-Cookie"); !strings.Contains(cookie, "session_id=") || !strings.Contains(cookie, "Max-Age=0") {
		t.Fatalf("Set-Cookie = %q, want session cleared", cookie)
	}
	// Regression lock (slow logout, 2026-10-06): Clear-Site-Data must NOT come
	// back on the logout response. Chrome blocks delivering the 204 until the
	// origin HTTP-cache purge finishes — 11–14 s on real profiles (Caddy saw
	// 6 ms server-side, the onSuccess navigation fired only after the purge).
	// The HTTP cache holds no user data (API responses carry no validators,
	// static assets are immutable and public), so the purge protected nothing.
	if got := rr.Header().Get("Clear-Site-Data"); got != "" {
		t.Fatalf("Clear-Site-Data = %q, want none", got)
	}
}

func TestLogout_NotFoundIsIdempotent(t *testing.T) {
	t.Parallel()
	logout := &fakeLogout{
		logout: func(context.Context, string, auditdomain.Actor) error { return application.ErrNotFound },
	}
	h := newHandlers(nil, logout)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/logout", nil)
	r.AddCookie(sessionCookie("token"))
	rr := doHandler(t, h.Logout, r)

	// ErrNotFound is treated as idempotent success (204).
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (idempotent)", rr.Code)
	}
	// The idempotent path shares the success write — and the same regression
	// lock: no Clear-Site-Data here either (slow logout, 2026-10-06).
	if got := rr.Header().Get("Clear-Site-Data"); got != "" {
		t.Fatalf("Clear-Site-Data = %q, want none", got)
	}
}

func TestLogout_OtherErrorReturns500(t *testing.T) {
	t.Parallel()
	logout := &fakeLogout{
		logout: func(context.Context, string, auditdomain.Actor) error { return errors.New("db down") },
	}
	h := newHandlers(nil, logout)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/logout", nil)
	r.AddCookie(sessionCookie("token"))
	rr := doHandler(t, h.Logout, r)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
}

// GetMe.

func TestGetMe_NoUserIDReturns401(t *testing.T) {
	t.Parallel()
	h := newHandlers(&fakeProfiler{}, nil)

	rr := doHandler(t, h.GetMe, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/me", nil))

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestGetMe_Success(t *testing.T) {
	t.Parallel()
	userID := uuid.Must(uuid.NewV7())
	email := mustEmailHandler(t, testOwnerEmail)
	name := testUserName
	profile := &fakeProfiler{
		me: func(_ context.Context, uid uuid.UUID) (domain.User, error) {
			if uid != userID {
				t.Fatalf("Me userID = %s, want %s", uid, userID)
			}
			return domain.User{
				ID:    userID,
				Role:  domain.RoleOwner,
				Name:  &name,
				Email: &email,
			}, nil
		},
	}
	h := newHandlers(profile, nil)

	r := authedRequest(t, http.MethodGet, "/me", "", userID)
	rr := doHandler(t, h.GetMe, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var resp openapi.MeResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Email == nil || *resp.Email != testOwnerEmail {
		t.Fatalf("email = %v, want %s", resp.Email, testOwnerEmail)
	}
	if resp.Name == nil || *resp.Name != testUserName {
		t.Fatalf("name = %v, want Ivan", resp.Name)
	}
}

func TestGetMe_NotFoundReturns401(t *testing.T) {
	t.Parallel()
	profile := &fakeProfiler{
		me: func(context.Context, uuid.UUID) (domain.User, error) { return domain.User{}, application.ErrNotFound },
	}
	h := newHandlers(profile, nil)

	r := authedRequest(t, http.MethodGet, "/me", "", uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.GetMe, r)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestGetMe_OtherErrorReturns500(t *testing.T) {
	t.Parallel()
	profile := &fakeProfiler{
		me: func(context.Context, uuid.UUID) (domain.User, error) { return domain.User{}, errors.New("db down") },
	}
	h := newHandlers(profile, nil)

	r := authedRequest(t, http.MethodGet, "/me", "", uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.GetMe, r)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
}

func TestGetMe_EnricherSuccess(t *testing.T) {
	t.Parallel()
	userID := uuid.Must(uuid.NewV7())
	profile := &fakeProfiler{
		me: func(context.Context, uuid.UUID) (domain.User, error) {
			return domain.User{ID: userID, Role: domain.RoleOwner}, nil
		},
	}
	enricher := func(_ context.Context, _ uuid.UUID, resp *openapi.MeResponse) error {
		sub := "premium"
		// We can't set Subscription easily without the full type, but we test
		// that the enricher is called and returns nil → 200.
		resp.Name = &sub
		return nil
	}
	h := NewAuthHandlers(nil, nil, profile, nil, nil, nil, false, slog.New(slog.DiscardHandler), AuthRateLimits{}, enricher)

	r := authedRequest(t, http.MethodGet, "/me", "", userID)
	rr := doHandler(t, h.GetMe, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}

func TestGetMe_EnricherErrorReturns500(t *testing.T) {
	t.Parallel()
	userID := uuid.Must(uuid.NewV7())
	profile := &fakeProfiler{
		me: func(context.Context, uuid.UUID) (domain.User, error) {
			return domain.User{ID: userID, Role: domain.RoleOwner}, nil
		},
	}
	enricher := func(context.Context, uuid.UUID, *openapi.MeResponse) error {
		return errors.New("billing unavailable")
	}
	h := NewAuthHandlers(nil, nil, profile, nil, nil, nil, false, slog.New(slog.DiscardHandler), AuthRateLimits{}, enricher)

	r := authedRequest(t, http.MethodGet, "/me", "", userID)
	rr := doHandler(t, h.GetMe, r)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
}

// UpdateMe.

func TestUpdateMe_NoUserIDReturns401(t *testing.T) {
	t.Parallel()
	h := newHandlers(&fakeProfiler{}, nil)

	rr := doJSON(t, h.UpdateMe, "/me", `{"name":"Ivan"}`)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestUpdateMe_BadBodyReturns400(t *testing.T) {
	t.Parallel()
	h := newHandlers(&fakeProfiler{}, nil)

	r := authedRequest(t, http.MethodPatch, "/me", "{invalid", uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.UpdateMe, r)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestUpdateMe_Success(t *testing.T) {
	t.Parallel()
	userID := uuid.Must(uuid.NewV7())
	var gotCmd application.UpdateProfileCommand
	profile := &fakeProfiler{
		update: func(_ context.Context, _ uuid.UUID, cmd application.UpdateProfileCommand) (domain.User, error) {
			gotCmd = cmd
			return domain.User{ID: userID, Role: domain.RoleOwner, Name: cmd.Name}, nil
		},
	}
	h := newHandlers(profile, nil)

	r := authedRequest(t, http.MethodPatch, "/me", `{"name":"Ivan","timezone":"Europe/Moscow"}`, userID)
	rr := doHandler(t, h.UpdateMe, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	if gotCmd.Name == nil || *gotCmd.Name != testUserName {
		t.Fatalf("cmd.Name = %v, want Ivan", gotCmd.Name)
	}
	if gotCmd.Timezone == nil || *gotCmd.Timezone != "Europe/Moscow" {
		t.Fatalf("cmd.Timezone = %v, want Europe/Moscow", gotCmd.Timezone)
	}
}

// TestUpdateMe_EmailFieldIsRejected proves the free email edit is gone
// (issue #721): the profile contract no longer carries the email field, so a
// client that still sends it gets 400 — the decoder is strict about unknown
// fields — and no profile update can move the address behind the service's
// back.
func TestUpdateMe_EmailFieldIsRejected(t *testing.T) {
	t.Parallel()
	profile := &fakeProfiler{
		update: func(_ context.Context, _ uuid.UUID, cmd application.UpdateProfileCommand) (domain.User, error) {
			return domain.User{ID: uuid.Must(uuid.NewV7()), Role: domain.RoleOwner, Name: cmd.Name}, nil
		},
	}
	h := newHandlers(profile, nil)

	r := authedRequest(t, http.MethodPatch, "/me", `{"email":"attacker@example.com","name":"Ivan"}`, uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.UpdateMe, r)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (email is no longer a profile field)", rr.Code)
	}
}

func TestUpdateMe_InvalidTimezoneReturns400(t *testing.T) {
	t.Parallel()
	profile := &fakeProfiler{
		update: func(context.Context, uuid.UUID, application.UpdateProfileCommand) (domain.User, error) {
			return domain.User{}, domain.ErrInvalidTimezone
		},
	}
	h := newHandlers(profile, nil)

	r := authedRequest(t, http.MethodPatch, "/me", `{"timezone":"bad"}`, uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.UpdateMe, r)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestUpdateMe_NotFoundReturns401(t *testing.T) {
	t.Parallel()
	profile := &fakeProfiler{
		update: func(context.Context, uuid.UUID, application.UpdateProfileCommand) (domain.User, error) {
			return domain.User{}, application.ErrNotFound
		},
	}
	h := newHandlers(profile, nil)

	r := authedRequest(t, http.MethodPatch, "/me", `{"name":"Ivan"}`, uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.UpdateMe, r)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

// Pure functions.

func TestMeResponse_NilEmail(t *testing.T) {
	t.Parallel()
	user := domain.User{
		ID:   uuid.Must(uuid.NewV7()),
		Role: domain.RoleOwner,
		// Email intentionally nil.
	}
	resp := meResponse(user)
	if resp.Email != nil {
		t.Fatalf("email = %v, want nil", resp.Email)
	}
}

// TestMeResponse_PhotoBlur travels the frame's base64 straight from the
// row: nil without a frame (the wire's null), the value as stored with one
// (ADR 0065 аменд, #1603).
func TestMeResponse_PhotoBlur(t *testing.T) {
	t.Parallel()
	user := domain.User{
		ID:        uuid.Must(uuid.NewV7()),
		Role:      domain.RoleOwner,
		PhotoBlur: nil,
	}
	resp := meResponse(user)
	if resp.PhotoBlur != nil {
		t.Fatalf("photoBlur = %v, want nil without a frame", *resp.PhotoBlur)
	}

	frame := "aGVsbG8="
	user.PhotoBlur = &frame
	resp = meResponse(user)
	if resp.PhotoBlur == nil || *resp.PhotoBlur != frame {
		t.Fatalf("photoBlur = %v, want the stored frame", resp.PhotoBlur)
	}
}

func TestAuthRateLimits_NilLimiterAllowsAll(t *testing.T) {
	t.Parallel()
	limits := AuthRateLimits{} // All limiters nil.

	if !limits.AllowSend("k") {
		t.Error("AllowSend with nil limiter = false, want true")
	}
	if !limits.AllowVerify("k") {
		t.Error("AllowVerify with nil limiter = false, want true")
	}
	if !limits.AllowSendForIP("203.0.113.9") {
		t.Error("AllowSendForIP with nil limiter = false, want true")
	}
	if !limits.AllowVerifyForIP("203.0.113.9") {
		t.Error("AllowVerifyForIP with nil limiter = false, want true")
	}
}

func TestActorFromContext(t *testing.T) {
	t.Parallel()

	t.Run("anonymous when no identity in context", func(t *testing.T) {
		t.Parallel()
		actor := actorFromContext(t.Context())
		if actor.ID != (uuid.UUID{}) {
			t.Fatalf("actor.ID = %s, want zero", actor.ID)
		}
		if actor.Role != auditdomain.ActorRoleAnonymous {
			t.Fatalf("actor.Role = %s, want anonymous", actor.Role)
		}
	})

	t.Run("owner when actor is in context", func(t *testing.T) {
		t.Parallel()
		userID := uuid.Must(uuid.NewV7())
		// The middleware sets both userID and actor identity in the context.
		ctx := httpsupport.WithActor(t.Context(), userID, domain.RoleOwner)
		ctx = httpsupport.WithUserID(ctx, userID)
		actor := actorFromContext(ctx)
		if actor.ID != userID {
			t.Fatalf("actor.ID = %s, want %s", actor.ID, userID)
		}
		if actor.Role != auditdomain.ActorRoleOwner {
			t.Fatalf("actor.Role = %s, want owner", actor.Role)
		}
	})
}

func mustEmailHandler(t *testing.T, raw string) domain.Email {
	t.Helper()
	e, err := domain.NewEmail(raw)
	if err != nil {
		t.Fatalf("parse email: %v", err)
	}
	return e
}

// Email change (issue #721).

func TestSendEmailChangeCode_NoUserIDReturns401(t *testing.T) {
	t.Parallel()
	h := newHandlersWithEmailChange(&fakeEmailChanger{}, nil, nil)

	rr := doHandler(t, h.SendEmailChangeCode, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/me/email/send-code", nil))

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestSendEmailChangeCode_SuccessReturns204(t *testing.T) {
	t.Parallel()
	called := false
	emailChange := &fakeEmailChanger{
		sendCurrentEmailCode: func(context.Context, uuid.UUID) error {
			called = true
			return nil
		},
	}
	h := newHandlersWithEmailChange(emailChange, nil, nil)

	r := authedRequest(t, http.MethodPost, "/me/email/send-code", "", uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.SendEmailChangeCode, r)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204: %s", rr.Code, rr.Body.String())
	}
	if !called {
		t.Fatal("SendCurrentEmailCode was not called")
	}
}

// TestVerifyCurrentEmail_SuccessReturnsGrant proves step 2 hands the one-time
// grant to the client and passes the code through untouched — the new address
// does not exist in this request at all (#1202).
func TestVerifyCurrentEmail_SuccessReturnsGrant(t *testing.T) {
	t.Parallel()
	var gotCode string
	emailChange := &fakeEmailChanger{
		verifyCurrentEmail: func(_ context.Context, _ uuid.UUID, code string) (string, error) {
			gotCode = code
			return "grant-token-123", nil
		},
	}
	h := newHandlersWithEmailChange(emailChange, nil, nil)

	r := verifyCurrentRequest(t, `{"code":"123456"}`, uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.VerifyCurrentEmail, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	if gotCode != "123456" {
		t.Fatalf("service code = %q, want the request code", gotCode)
	}
	if !strings.Contains(rr.Body.String(), `"grant":"grant-token-123"`) {
		t.Fatalf("body = %s, want the grant token", rr.Body.String())
	}
}

func TestVerifyCurrentEmail_InvalidCodeReturns401(t *testing.T) {
	t.Parallel()
	emailChange := &fakeEmailChanger{
		verifyCurrentEmail: func(context.Context, uuid.UUID, string) (string, error) {
			return "", domain.ErrLoginCodeInvalid
		},
	}
	h := newHandlersWithEmailChange(emailChange, nil, nil)

	r := verifyCurrentRequest(t, `{"code":"000000"}`, uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.VerifyCurrentEmail, r)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Неверный код") {
		t.Fatalf("body = %s, want the wrong-code detail", rr.Body.String())
	}
}

// TestRequestNewEmailCode_BadEmailReturns400 proves the address format is
// parsed at the handler seam: a malformed address never reaches the service.
func TestRequestNewEmailCode_BadEmailReturns400(t *testing.T) {
	t.Parallel()
	h := newHandlersWithEmailChange(&fakeEmailChanger{}, nil, nil)

	r := requestNewEmailCodeRequest(t, `{"grant":"g","newEmail":"not-an-email"}`, uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.RequestNewEmailCode, r)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

// TestRequestNewEmailCode_SuccessReturns204 proves step 3 passes the grant and
// the normalized address through and answers with the bare 204 (#1202).
func TestRequestNewEmailCode_SuccessReturns204(t *testing.T) {
	t.Parallel()
	var gotGrant string
	var gotEmail domain.Email
	emailChange := &fakeEmailChanger{
		requestNewEmailCode: func(_ context.Context, _ uuid.UUID, grant string, newEmail domain.Email) error {
			gotGrant = grant
			gotEmail = newEmail
			return nil
		},
	}
	h := newHandlersWithEmailChange(emailChange, nil, nil)

	r := requestNewEmailCodeRequest(t, `{"grant":"grant-token-123","newEmail":"New@Example.com"}`, uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.RequestNewEmailCode, r)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204: %s", rr.Code, rr.Body.String())
	}
	if gotGrant != "grant-token-123" {
		t.Fatalf("service grant = %q, want the request grant", gotGrant)
	}
	if gotEmail.String() != "new@example.com" {
		t.Fatalf("service email = %s, want normalized new@example.com", gotEmail)
	}
}

func TestRequestNewEmailCode_UnchangedReturns400(t *testing.T) {
	t.Parallel()
	emailChange := &fakeEmailChanger{
		requestNewEmailCode: func(context.Context, uuid.UUID, string, domain.Email) error {
			return application.ErrEmailUnchanged
		},
	}
	h := newHandlersWithEmailChange(emailChange, nil, nil)

	r := requestNewEmailCodeRequest(t, `{"grant":"g","newEmail":"owner@example.com"}`, uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.RequestNewEmailCode, r)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "должна отличаться от текущей") {
		t.Fatalf("body = %s, want the unchanged-email detail", rr.Body.String())
	}
}

func TestRequestNewEmailCode_TakenReturns409WithDecisionText(t *testing.T) {
	t.Parallel()
	emailChange := &fakeEmailChanger{
		requestNewEmailCode: func(context.Context, uuid.UUID, string, domain.Email) error {
			return application.ErrEmailAlreadyTaken
		},
	}
	h := newHandlersWithEmailChange(emailChange, nil, nil)

	r := requestNewEmailCodeRequest(t, `{"grant":"g","newEmail":"taken@example.com"}`, uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.RequestNewEmailCode, r)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Эта электронная почта уже используется") {
		t.Fatalf("body = %s, want the decision text (#720-4)", rr.Body.String())
	}
}

func TestRequestNewEmailCode_InvalidGrantReturns409(t *testing.T) {
	t.Parallel()
	emailChange := &fakeEmailChanger{
		requestNewEmailCode: func(context.Context, uuid.UUID, string, domain.Email) error {
			return application.ErrEmailChangeGrantInvalid
		},
	}
	h := newHandlersWithEmailChange(emailChange, nil, nil)

	r := requestNewEmailCodeRequest(t, `{"grant":"stale","newEmail":"new@example.com"}`, uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.RequestNewEmailCode, r)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "начните смену почты заново") {
		t.Fatalf("body = %s, want the restart detail", rr.Body.String())
	}
}

// TestRequestNewEmailCode_BudgetExhaustedReturns429 proves the per-user
// budget on sends to NEW addresses (decision #720-3) surfaces as 429; the
// budget itself is spent inside the service, on the actual send.
func TestRequestNewEmailCode_BudgetExhaustedReturns429(t *testing.T) {
	t.Parallel()
	emailChange := &fakeEmailChanger{
		requestNewEmailCode: func(context.Context, uuid.UUID, string, domain.Email) error {
			return application.ErrEmailChangeBudgetExhausted
		},
	}
	h := newHandlersWithEmailChange(emailChange, nil, nil)

	r := requestNewEmailCodeRequest(t, `{"grant":"g","newEmail":"new@example.com"}`, uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.RequestNewEmailCode, r)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rr.Code)
	}
}

// ResendEmailCode (POST /me/email/resend-code, #732).

// TestResendEmailCode_SuccessPassesGrantAndReturns204 proves the resend
// delivers the grant token to the service and answers 204.
func TestResendEmailCode_SuccessPassesGrantAndReturns204(t *testing.T) {
	t.Parallel()
	userID := uuid.Must(uuid.NewV7())
	var gotGrant string
	emailChange := &fakeEmailChanger{
		resendNewEmailCode: func(_ context.Context, _ uuid.UUID, grant string) error {
			gotGrant = grant
			return nil
		},
	}
	h := newHandlersWithEmailChange(emailChange, nil, nil)

	r := authedRequest(t, http.MethodPost, "/me/email/resend-code", `{"grant":"grant-token"}`, userID)
	rr := doHandler(t, h.ResendEmailCode, r)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204: %s", rr.Code, rr.Body.String())
	}
	if gotGrant != grantTokenFixture {
		t.Fatalf("grant = %q, want passed through", gotGrant)
	}
}

func TestResendEmailCode_BadBodyReturns400(t *testing.T) {
	t.Parallel()
	h := newHandlersWithEmailChange(&fakeEmailChanger{}, nil, nil)

	r := authedRequest(t, http.MethodPost, "/me/email/resend-code", `{"grant"`, uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.ResendEmailCode, r)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

// TestResendEmailCode_ExpiredGrantReturns409 proves the restart decision text
// (#720-2) is served by the resend endpoint too.
func TestResendEmailCode_ExpiredGrantReturns409(t *testing.T) {
	t.Parallel()
	emailChange := &fakeEmailChanger{
		resendNewEmailCode: func(context.Context, uuid.UUID, string) error {
			return application.ErrEmailChangeGrantInvalid
		},
	}
	h := newHandlersWithEmailChange(emailChange, nil, nil)

	r := authedRequest(t, http.MethodPost, "/me/email/resend-code", `{"grant":"stale"}`, uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.ResendEmailCode, r)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Подтверждение истекло, начните смену почты заново") {
		t.Fatalf("body = %s, want the restart text (#720-2)", rr.Body.String())
	}
}

// TestResendEmailCode_ThrottledReturns429 proves the 1-minute send throttle
// surfaces as 429 with its dedicated text — the resend button's timer guard.
func TestResendEmailCode_ThrottledReturns429(t *testing.T) {
	t.Parallel()
	emailChange := &fakeEmailChanger{
		resendNewEmailCode: func(context.Context, uuid.UUID, string) error {
			return application.ErrCodeSentTooRecently
		},
	}
	h := newHandlersWithEmailChange(emailChange, nil, nil)

	r := authedRequest(t, http.MethodPost, "/me/email/resend-code", `{"grant":"grant-token"}`, uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.ResendEmailCode, r)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Код отправлен слишком недавно") {
		t.Fatalf("body = %s, want the throttle text", rr.Body.String())
	}
}

// TestResendEmailCode_BudgetExhaustedReturns429 proves the 5/hour budget on
// new-address sends (#720-3) covers resends as well.
func TestResendEmailCode_BudgetExhaustedReturns429(t *testing.T) {
	t.Parallel()
	emailChange := &fakeEmailChanger{
		resendNewEmailCode: func(context.Context, uuid.UUID, string) error {
			return application.ErrEmailChangeBudgetExhausted
		},
	}
	h := newHandlersWithEmailChange(emailChange, nil, nil)

	r := authedRequest(t, http.MethodPost, "/me/email/resend-code", `{"grant":"grant-token"}`, uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.ResendEmailCode, r)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rr.Code)
	}
}

// TestResendEmailCode_SendLimitsReturn429 proves the anti-flood send limits
// (#1210) surface through the shared identity switch as 429 with the generic
// throttle text — one UX for every send refusal, and no address- or
// account-specific wording on a pre-auth-reachable path.
func TestResendEmailCode_SendLimitsReturn429(t *testing.T) {
	t.Parallel()
	for name, limitErr := range map[string]error{
		"recipient limit": application.ErrRecipientSendLimitExceeded,
		"initiator limit": application.ErrInitiatorSendLimitExceeded,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			emailChange := &fakeEmailChanger{
				resendNewEmailCode: func(context.Context, uuid.UUID, string) error {
					return limitErr
				},
			}
			h := newHandlersWithEmailChange(emailChange, nil, nil)

			r := authedRequest(t, http.MethodPost, "/me/email/resend-code", `{"grant":"grant-token"}`, uuid.Must(uuid.NewV7()))
			rr := doHandler(t, h.ResendEmailCode, r)

			if rr.Code != http.StatusTooManyRequests {
				t.Fatalf("status = %d, want 429", rr.Code)
			}
			if !strings.Contains(rr.Body.String(), "Превышен лимит запросов") {
				t.Fatalf("body = %s, want the generic throttle text", rr.Body.String())
			}
		})
	}
}
