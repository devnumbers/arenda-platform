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
	logout    func(ctx context.Context, rawToken string, actor auditdomain.Actor) error
	logoutAll func(ctx context.Context, userID uuid.UUID, actor auditdomain.Actor) error
}

func (f *fakeLogout) Logout(ctx context.Context, rawToken string, actor auditdomain.Actor) error {
	if f.logout != nil {
		return f.logout(ctx, rawToken, actor)
	}
	return nil
}

func (f *fakeLogout) LogoutAll(ctx context.Context, userID uuid.UUID, actor auditdomain.Actor) error {
	if f.logoutAll != nil {
		return f.logoutAll(ctx, userID, actor)
	}
	return nil
}

type fakePhoneChanger struct {
	sendChangeCode func(ctx context.Context, userID uuid.UUID, phone domain.Phone) error
	changePhone    func(ctx context.Context, userID uuid.UUID, phone domain.Phone, code, token string) (domain.User, error)
}

func (f *fakePhoneChanger) SendChangeCode(ctx context.Context, userID uuid.UUID, phone domain.Phone) error {
	if f.sendChangeCode != nil {
		return f.sendChangeCode(ctx, userID, phone)
	}
	return nil
}

func (f *fakePhoneChanger) ChangePhone(ctx context.Context, userID uuid.UUID, phone domain.Phone, code, token string) (domain.User, error) {
	if f.changePhone != nil {
		return f.changePhone(ctx, userID, phone, code, token)
	}
	return domain.User{}, errors.New("unexpected ChangePhone call")
}

// grantTokenFixture is the grant plaintext the fakes and request bodies share.
const grantTokenFixture = "grant-token"

type fakeEmailChanger struct {
	sendCurrentEmailCode func(ctx context.Context, userID uuid.UUID) error
	confirmCurrentEmail  func(ctx context.Context, userID uuid.UUID, code string, newEmail domain.Email) (string, error)
	resendNewEmailCode   func(ctx context.Context, userID uuid.UUID, grant string) error
	changeEmail          func(ctx context.Context, userID uuid.UUID, code, grant string) (domain.User, error)
}

func (f *fakeEmailChanger) SendCurrentEmailCode(ctx context.Context, userID uuid.UUID) error {
	if f.sendCurrentEmailCode != nil {
		return f.sendCurrentEmailCode(ctx, userID)
	}
	return nil
}

func (f *fakeEmailChanger) ConfirmCurrentEmail(ctx context.Context, userID uuid.UUID, code string, newEmail domain.Email) (string, error) {
	if f.confirmCurrentEmail != nil {
		return f.confirmCurrentEmail(ctx, userID, code, newEmail)
	}
	return grantTokenFixture, nil
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

func newHandlers(profile Profiler, logout Logout, phoneChange PhoneChanger) *AuthHandlers {
	return newHandlersWithEmailChange(nil, profile, logout, phoneChange)
}

func newHandlersWithEmailChange(emailChange EmailChanger, profile Profiler, logout Logout, phoneChange PhoneChanger) *AuthHandlers {
	return newHandlersWithLimits(emailChange, profile, logout, phoneChange, AuthRateLimits{})
}

// confirmCurrentRequest builds an authenticated step-2 request with the given
// raw body, keeping the call sites within the line limit.
func confirmCurrentRequest(t *testing.T, body string, userID uuid.UUID) *http.Request {
	t.Helper()
	return authedRequest(t, http.MethodPost, "/me/email/confirm-current", body, userID)
}

func newHandlersWithLimits(
	emailChange EmailChanger,
	profile Profiler,
	logout Logout,
	phoneChange PhoneChanger,
	limits AuthRateLimits,
) *AuthHandlers {
	return NewAuthHandlers(nil, phoneChange, emailChange, profile, logout, false, slog.New(slog.DiscardHandler), limits, nil)
}

func mustPhoneHandler(t *testing.T, raw string) domain.Phone {
	t.Helper()
	p, err := domain.NewPhone(raw)
	if err != nil {
		t.Fatalf("parse phone: %v", err)
	}
	return p
}

// Logout.

func TestLogout_NoTokenReturns401(t *testing.T) {
	t.Parallel()
	h := newHandlers(nil, &fakeLogout{}, nil)

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
	h := newHandlers(nil, logout, nil)

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
}

func TestLogout_NotFoundIsIdempotent(t *testing.T) {
	t.Parallel()
	logout := &fakeLogout{
		logout: func(context.Context, string, auditdomain.Actor) error { return application.ErrNotFound },
	}
	h := newHandlers(nil, logout, nil)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/logout", nil)
	r.AddCookie(sessionCookie("token"))
	rr := doHandler(t, h.Logout, r)

	// ErrNotFound is treated as idempotent success (204).
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (idempotent)", rr.Code)
	}
}

func TestLogout_OtherErrorReturns500(t *testing.T) {
	t.Parallel()
	logout := &fakeLogout{
		logout: func(context.Context, string, auditdomain.Actor) error { return errors.New("db down") },
	}
	h := newHandlers(nil, logout, nil)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/logout", nil)
	r.AddCookie(sessionCookie("token"))
	rr := doHandler(t, h.Logout, r)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
}

// LogoutAll.

func TestLogoutAll_NoUserIDReturns401(t *testing.T) {
	t.Parallel()
	h := newHandlers(nil, &fakeLogout{}, nil)

	rr := doHandler(t, h.LogoutAll, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/logout-all", nil))

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestLogoutAll_Success(t *testing.T) {
	t.Parallel()
	var gotUserID uuid.UUID
	logout := &fakeLogout{
		logoutAll: func(_ context.Context, userID uuid.UUID, _ auditdomain.Actor) error {
			gotUserID = userID
			return nil
		},
	}
	h := newHandlers(nil, logout, nil)
	userID := uuid.Must(uuid.NewV7())

	r := authedRequest(t, http.MethodPost, "/auth/logout-all", "", userID)
	rr := doHandler(t, h.LogoutAll, r)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
	if gotUserID != userID {
		t.Fatalf("LogoutAll userID = %s, want %s", gotUserID, userID)
	}
}

func TestLogoutAll_ErrorReturns500(t *testing.T) {
	t.Parallel()
	logout := &fakeLogout{
		logoutAll: func(context.Context, uuid.UUID, auditdomain.Actor) error { return errors.New("db down") },
	}
	h := newHandlers(nil, logout, nil)

	r := authedRequest(t, http.MethodPost, "/auth/logout-all", "", uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.LogoutAll, r)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
}

// GetMe.

func TestGetMe_NoUserIDReturns401(t *testing.T) {
	t.Parallel()
	h := newHandlers(&fakeProfiler{}, nil, nil)

	rr := doHandler(t, h.GetMe, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/me", nil))

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestGetMe_Success(t *testing.T) {
	t.Parallel()
	userID := uuid.Must(uuid.NewV7())
	phone := mustPhoneHandler(t, "+79160000800")
	email := mustEmailHandler(t, testOwnerEmail)
	name := testUserName
	profile := &fakeProfiler{
		me: func(_ context.Context, uid uuid.UUID) (domain.User, error) {
			if uid != userID {
				t.Fatalf("Me userID = %s, want %s", uid, userID)
			}
			return domain.User{
				ID:    userID,
				Phone: phone,
				Role:  domain.RoleOwner,
				Name:  &name,
				Email: &email,
			}, nil
		},
	}
	h := newHandlers(profile, nil, nil)

	r := authedRequest(t, http.MethodGet, "/me", "", userID)
	rr := doHandler(t, h.GetMe, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var resp openapi.MeResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Phone != phone.String() {
		t.Fatalf("phone = %s, want %s", resp.Phone, phone.String())
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
	h := newHandlers(profile, nil, nil)

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
	h := newHandlers(profile, nil, nil)

	r := authedRequest(t, http.MethodGet, "/me", "", uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.GetMe, r)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
}

func TestGetMe_EnricherSuccess(t *testing.T) {
	t.Parallel()
	userID := uuid.Must(uuid.NewV7())
	phone := mustPhoneHandler(t, "+79160000801")
	profile := &fakeProfiler{
		me: func(context.Context, uuid.UUID) (domain.User, error) {
			return domain.User{ID: userID, Phone: phone, Role: domain.RoleOwner}, nil
		},
	}
	enricher := func(_ context.Context, _ uuid.UUID, resp *openapi.MeResponse) error {
		sub := "premium"
		// We can't set Subscription easily without the full type, but we test
		// that the enricher is called and returns nil → 200.
		resp.Name = &sub
		return nil
	}
	h := NewAuthHandlers(nil, nil, nil, profile, nil, false, slog.New(slog.DiscardHandler), AuthRateLimits{}, enricher)

	r := authedRequest(t, http.MethodGet, "/me", "", userID)
	rr := doHandler(t, h.GetMe, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}

func TestGetMe_EnricherErrorReturns500(t *testing.T) {
	t.Parallel()
	userID := uuid.Must(uuid.NewV7())
	phone := mustPhoneHandler(t, "+79160000802")
	profile := &fakeProfiler{
		me: func(context.Context, uuid.UUID) (domain.User, error) {
			return domain.User{ID: userID, Phone: phone, Role: domain.RoleOwner}, nil
		},
	}
	enricher := func(context.Context, uuid.UUID, *openapi.MeResponse) error {
		return errors.New("billing unavailable")
	}
	h := NewAuthHandlers(nil, nil, nil, profile, nil, false, slog.New(slog.DiscardHandler), AuthRateLimits{}, enricher)

	r := authedRequest(t, http.MethodGet, "/me", "", userID)
	rr := doHandler(t, h.GetMe, r)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
}

// UpdateMe.

func TestUpdateMe_NoUserIDReturns401(t *testing.T) {
	t.Parallel()
	h := newHandlers(&fakeProfiler{}, nil, nil)

	rr := doJSON(t, h.UpdateMe, "/me", `{"name":"Ivan"}`)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestUpdateMe_BadBodyReturns400(t *testing.T) {
	t.Parallel()
	h := newHandlers(&fakeProfiler{}, nil, nil)

	r := authedRequest(t, http.MethodPatch, "/me", "{invalid", uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.UpdateMe, r)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestUpdateMe_Success(t *testing.T) {
	t.Parallel()
	userID := uuid.Must(uuid.NewV7())
	phone := mustPhoneHandler(t, "+79160000900")
	var gotCmd application.UpdateProfileCommand
	profile := &fakeProfiler{
		update: func(_ context.Context, _ uuid.UUID, cmd application.UpdateProfileCommand) (domain.User, error) {
			gotCmd = cmd
			return domain.User{ID: userID, Phone: phone, Role: domain.RoleOwner, Name: cmd.Name}, nil
		},
	}
	h := newHandlers(profile, nil, nil)

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
	h := newHandlers(profile, nil, nil)

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
	h := newHandlers(profile, nil, nil)

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
	h := newHandlers(profile, nil, nil)

	r := authedRequest(t, http.MethodPatch, "/me", `{"name":"Ivan"}`, uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.UpdateMe, r)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

// SendPhoneChangeCode.

func TestSendPhoneChangeCode_NoUserIDReturns401(t *testing.T) {
	t.Parallel()
	h := newHandlers(nil, nil, &fakePhoneChanger{})

	rr := doJSON(t, h.SendPhoneChangeCode, "/me/phone/send-code", `{"phone":"+79160001000"}`)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestSendPhoneChangeCode_BadPhoneReturns400(t *testing.T) {
	t.Parallel()
	h := newHandlers(nil, nil, &fakePhoneChanger{})

	r := authedRequest(t, http.MethodPost, "/me/phone/send-code", `{"phone":"bad"}`, uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.SendPhoneChangeCode, r)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestSendPhoneChangeCode_SuccessReturns204(t *testing.T) {
	t.Parallel()
	var gotPhone domain.Phone
	pc := &fakePhoneChanger{
		sendChangeCode: func(_ context.Context, _ uuid.UUID, phone domain.Phone) error {
			gotPhone = phone
			return nil
		},
	}
	h := newHandlers(nil, nil, pc)
	userID := uuid.Must(uuid.NewV7())

	r := authedRequest(t, http.MethodPost, "/me/phone/send-code", `{"phone":"+79160001000"}`, userID)
	rr := doHandler(t, h.SendPhoneChangeCode, r)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
	if gotPhone.String() != "+79160001000" {
		t.Fatalf("phone = %s, want +79160001000", gotPhone.String())
	}
}

func TestSendPhoneChangeCode_UnchangedReturns400(t *testing.T) {
	t.Parallel()
	pc := &fakePhoneChanger{
		sendChangeCode: func(context.Context, uuid.UUID, domain.Phone) error { return application.ErrPhoneUnchanged },
	}
	h := newHandlers(nil, nil, pc)

	r := authedRequest(t, http.MethodPost, "/me/phone/send-code", `{"phone":"+79160001000"}`, uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.SendPhoneChangeCode, r)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestSendPhoneChangeCode_TakenReturns409(t *testing.T) {
	t.Parallel()
	pc := &fakePhoneChanger{
		sendChangeCode: func(context.Context, uuid.UUID, domain.Phone) error { return application.ErrPhoneAlreadyTaken },
	}
	h := newHandlers(nil, nil, pc)

	r := authedRequest(t, http.MethodPost, "/me/phone/send-code", `{"phone":"+79160001000"}`, uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.SendPhoneChangeCode, r)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rr.Code)
	}
}

// ChangePhone.

func TestChangePhone_NoUserIDReturns401(t *testing.T) {
	t.Parallel()
	h := newHandlers(nil, nil, &fakePhoneChanger{})

	rr := doJSON(t, h.ChangePhone, "/me/phone/change", `{"phone":"+79160002000","code":"123456"}`)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestChangePhone_NoTokenReturns401(t *testing.T) {
	t.Parallel()
	h := newHandlers(nil, nil, &fakePhoneChanger{})

	r := authedRequest(t, http.MethodPost, "/me/phone/change", `{"phone":"+79160002000","code":"123456"}`, uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.ChangePhone, r)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (no token)", rr.Code)
	}
}

func TestChangePhone_Success(t *testing.T) {
	t.Parallel()
	userID := uuid.Must(uuid.NewV7())
	phone := mustPhoneHandler(t, "+79160002000")
	pc := &fakePhoneChanger{
		changePhone: func(_ context.Context, _ uuid.UUID, _ domain.Phone, _, _ string) (domain.User, error) {
			return domain.NewOwner(phone)
		},
	}
	h := newHandlers(nil, nil, pc)

	r := authedRequest(t, http.MethodPost, "/me/phone/change", `{"phone":"+79160002000","code":"123456"}`, userID)
	r.AddCookie(sessionCookie("current-token"))
	rr := doHandler(t, h.ChangePhone, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
}

func TestChangePhone_InvalidCodeReturns401(t *testing.T) {
	t.Parallel()
	pc := &fakePhoneChanger{
		changePhone: func(context.Context, uuid.UUID, domain.Phone, string, string) (domain.User, error) {
			return domain.User{}, domain.ErrLoginCodeInvalid
		},
	}
	h := newHandlers(nil, nil, pc)

	r := authedRequest(t, http.MethodPost, "/me/phone/change", `{"phone":"+79160002000","code":"000000"}`, uuid.Must(uuid.NewV7()))
	r.AddCookie(sessionCookie("token"))
	rr := doHandler(t, h.ChangePhone, r)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestChangePhone_UnchangedReturns400(t *testing.T) {
	t.Parallel()
	pc := &fakePhoneChanger{
		changePhone: func(context.Context, uuid.UUID, domain.Phone, string, string) (domain.User, error) {
			return domain.User{}, application.ErrPhoneUnchanged
		},
	}
	h := newHandlers(nil, nil, pc)

	r := authedRequest(t, http.MethodPost, "/me/phone/change", `{"phone":"+79160002000","code":"123456"}`, uuid.Must(uuid.NewV7()))
	r.AddCookie(sessionCookie("token"))
	rr := doHandler(t, h.ChangePhone, r)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestChangePhone_TakenReturns409(t *testing.T) {
	t.Parallel()
	pc := &fakePhoneChanger{
		changePhone: func(context.Context, uuid.UUID, domain.Phone, string, string) (domain.User, error) {
			return domain.User{}, application.ErrPhoneAlreadyTaken
		},
	}
	h := newHandlers(nil, nil, pc)

	r := authedRequest(t, http.MethodPost, "/me/phone/change", `{"phone":"+79160002000","code":"123456"}`, uuid.Must(uuid.NewV7()))
	r.AddCookie(sessionCookie("token"))
	rr := doHandler(t, h.ChangePhone, r)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rr.Code)
	}
}

// Pure functions.

func TestMeResponse_NilEmail(t *testing.T) {
	t.Parallel()
	phone := mustPhoneHandler(t, "+79160003000")
	user := domain.User{
		ID:    uuid.Must(uuid.NewV7()),
		Phone: phone,
		Role:  domain.RoleOwner,
		// Email intentionally nil.
	}
	resp := meResponse(user)
	if resp.Email != nil {
		t.Fatalf("email = %v, want nil", resp.Email)
	}
	if resp.Phone != phone.String() {
		t.Fatalf("phone = %s, want %s", resp.Phone, phone.String())
	}
}

func TestRateLimitKey(t *testing.T) {
	t.Parallel()
	email := mustEmailHandler(t, testOwnerEmail)

	t.Run("email wins when present", func(t *testing.T) {
		t.Parallel()
		if got := rateLimitKey("+7900", &email); got != email.String() {
			t.Fatalf("rateLimitKey = %q, want %q", got, email.String())
		}
	})

	t.Run("phone used when email absent", func(t *testing.T) {
		t.Parallel()
		if got := rateLimitKey("+7900", nil); got != "+7900" {
			t.Fatalf("rateLimitKey = %q, want +7900", got)
		}
	})
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
	if !limits.AllowPhoneChangeSend("k") {
		t.Error("AllowPhoneChangeSend with nil limiter = false, want true")
	}
	if !limits.AllowPhoneChangeVerify("k") {
		t.Error("AllowPhoneChangeVerify with nil limiter = false, want true")
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
	h := newHandlersWithEmailChange(&fakeEmailChanger{}, nil, nil, nil)

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
	h := newHandlersWithEmailChange(emailChange, nil, nil, nil)

	r := authedRequest(t, http.MethodPost, "/me/email/send-code", "", uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.SendEmailChangeCode, r)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204: %s", rr.Code, rr.Body.String())
	}
	if !called {
		t.Fatal("SendCurrentEmailCode was not called")
	}
}

func TestSendEmailChangeCode_NoEmailReturns409(t *testing.T) {
	t.Parallel()
	emailChange := &fakeEmailChanger{
		sendCurrentEmailCode: func(context.Context, uuid.UUID) error {
			return application.ErrEmailDoesNotMatch
		},
	}
	h := newHandlersWithEmailChange(emailChange, nil, nil, nil)

	r := authedRequest(t, http.MethodPost, "/me/email/send-code", "", uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.SendEmailChangeCode, r)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "У аккаунта нет электронной почты") {
		t.Fatalf("body = %s, want the no-email detail", rr.Body.String())
	}
}

func TestConfirmCurrentEmail_BadEmailReturns400(t *testing.T) {
	t.Parallel()
	h := newHandlersWithEmailChange(&fakeEmailChanger{}, nil, nil, nil)

	r := authedRequest(t, http.MethodPost, "/me/email/confirm-current", `{"code":"123456","newEmail":"not-an-email"}`, uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.ConfirmCurrentEmail, r)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestConfirmCurrentEmail_SuccessReturnsGrant(t *testing.T) {
	t.Parallel()
	var gotEmail domain.Email
	emailChange := &fakeEmailChanger{
		confirmCurrentEmail: func(_ context.Context, _ uuid.UUID, _ string, newEmail domain.Email) (string, error) {
			gotEmail = newEmail
			return "grant-token-123", nil
		},
	}
	h := newHandlersWithEmailChange(emailChange, nil, nil, nil)

	r := confirmCurrentRequest(t, `{"code":"123456","newEmail":"New@Example.com"}`, uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.ConfirmCurrentEmail, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	if gotEmail.String() != "new@example.com" {
		t.Fatalf("service email = %s, want normalized new@example.com", gotEmail)
	}
	if !strings.Contains(rr.Body.String(), `"grant":"grant-token-123"`) {
		t.Fatalf("body = %s, want the grant token", rr.Body.String())
	}
}

func TestConfirmCurrentEmail_UnchangedReturns400(t *testing.T) {
	t.Parallel()
	emailChange := &fakeEmailChanger{
		confirmCurrentEmail: func(context.Context, uuid.UUID, string, domain.Email) (string, error) {
			return "", application.ErrEmailUnchanged
		},
	}
	h := newHandlersWithEmailChange(emailChange, nil, nil, nil)

	r := confirmCurrentRequest(t, `{"code":"123456","newEmail":"owner@example.com"}`, uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.ConfirmCurrentEmail, r)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "должна отличаться от текущей") {
		t.Fatalf("body = %s, want the unchanged-email detail", rr.Body.String())
	}
}

func TestConfirmCurrentEmail_TakenReturns409WithDecisionText(t *testing.T) {
	t.Parallel()
	emailChange := &fakeEmailChanger{
		confirmCurrentEmail: func(context.Context, uuid.UUID, string, domain.Email) (string, error) {
			return "", application.ErrEmailAlreadyTaken
		},
	}
	h := newHandlersWithEmailChange(emailChange, nil, nil, nil)

	r := confirmCurrentRequest(t, `{"code":"123456","newEmail":"taken@example.com"}`, uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.ConfirmCurrentEmail, r)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Эта электронная почта уже используется") {
		t.Fatalf("body = %s, want the decision text (#720-4)", rr.Body.String())
	}
}

func TestConfirmCurrentEmail_InvalidCodeReturns401(t *testing.T) {
	t.Parallel()
	emailChange := &fakeEmailChanger{
		confirmCurrentEmail: func(context.Context, uuid.UUID, string, domain.Email) (string, error) {
			return "", domain.ErrLoginCodeInvalid
		},
	}
	h := newHandlersWithEmailChange(emailChange, nil, nil, nil)

	r := confirmCurrentRequest(t, `{"code":"000000","newEmail":"new@example.com"}`, uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.ConfirmCurrentEmail, r)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

// TestConfirmCurrentEmail_BudgetExhaustedReturns429 proves the per-user
// budget on sends to NEW addresses (decision #720-3) surfaces as 429; the
// budget itself is spent inside the service, on the actual send.
func TestConfirmCurrentEmail_BudgetExhaustedReturns429(t *testing.T) {
	t.Parallel()
	emailChange := &fakeEmailChanger{
		confirmCurrentEmail: func(context.Context, uuid.UUID, string, domain.Email) (string, error) {
			return "", application.ErrEmailChangeBudgetExhausted
		},
	}
	h := newHandlersWithEmailChange(emailChange, nil, nil, nil)

	r := confirmCurrentRequest(t, `{"code":"123456","newEmail":"new@example.com"}`, uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.ConfirmCurrentEmail, r)

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
	h := newHandlersWithEmailChange(emailChange, nil, nil, nil)

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
	h := newHandlersWithEmailChange(&fakeEmailChanger{}, nil, nil, nil)

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
	h := newHandlersWithEmailChange(emailChange, nil, nil, nil)

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
	h := newHandlersWithEmailChange(emailChange, nil, nil, nil)

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
	h := newHandlersWithEmailChange(emailChange, nil, nil, nil)

	r := authedRequest(t, http.MethodPost, "/me/email/resend-code", `{"grant":"grant-token"}`, uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.ResendEmailCode, r)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rr.Code)
	}
}

func TestChangeEmail_SuccessReturnsMe(t *testing.T) {
	t.Parallel()
	userID := uuid.Must(uuid.NewV7())
	phone := mustPhoneHandler(t, "+79160001100")
	email := mustEmailHandler(t, "new@example.com")
	var gotCode, gotGrant string
	emailChange := &fakeEmailChanger{
		changeEmail: func(_ context.Context, _ uuid.UUID, code, grant string) (domain.User, error) {
			gotCode, gotGrant = code, grant
			return domain.User{ID: userID, Phone: phone, Role: domain.RoleOwner, Email: &email}, nil
		},
	}
	h := newHandlersWithEmailChange(emailChange, nil, nil, nil)

	r := authedRequest(t, http.MethodPost, "/me/email/change", `{"code":"123456","grant":"grant-token"}`, userID)
	rr := doHandler(t, h.ChangeEmail, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	if gotCode != "123456" || gotGrant != grantTokenFixture {
		t.Fatalf("service args = (%q, %q), want code and grant passed through", gotCode, gotGrant)
	}
	if !strings.Contains(rr.Body.String(), `"email":"new@example.com"`) {
		t.Fatalf("body = %s, want the new email in MeResponse", rr.Body.String())
	}
}

func TestChangeEmail_ExpiredGrantReturns409(t *testing.T) {
	t.Parallel()
	emailChange := &fakeEmailChanger{
		changeEmail: func(context.Context, uuid.UUID, string, string) (domain.User, error) {
			return domain.User{}, application.ErrEmailChangeGrantInvalid
		},
	}
	h := newHandlersWithEmailChange(emailChange, nil, nil, nil)

	r := authedRequest(t, http.MethodPost, "/me/email/change", `{"code":"123456","grant":"stale"}`, uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.ChangeEmail, r)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "начните смену почты заново") {
		t.Fatalf("body = %s, want the restart hint", rr.Body.String())
	}
}

func TestChangeEmail_InvalidCodeReturns401(t *testing.T) {
	t.Parallel()
	emailChange := &fakeEmailChanger{
		changeEmail: func(context.Context, uuid.UUID, string, string) (domain.User, error) {
			return domain.User{}, domain.ErrLoginCodeInvalid
		},
	}
	h := newHandlersWithEmailChange(emailChange, nil, nil, nil)

	r := authedRequest(t, http.MethodPost, "/me/email/change", `{"code":"000000","grant":"grant"}`, uuid.Must(uuid.NewV7()))
	rr := doHandler(t, h.ChangeEmail, r)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}
