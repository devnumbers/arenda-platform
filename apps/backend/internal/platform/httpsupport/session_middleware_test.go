package httpsupport

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/actor"
)

// fakeTouchLoader drives the session middleware tests: Load answers with a
// preset actor, Touch applies the preset cookie decision.
type fakeTouchLoader struct {
	loadUserID uuid.UUID
	loadRole   actor.Role
	loadErr    error
	// CookieToken/cookieExpires preset the Touch result: a non-nil
	// cookieExpires means the transport must re-issue the cookie, cookieToken
	// carries the fresh token after a rotation (empty = keep the presented one).
	cookieToken      string
	cookieExpires    *time.Time
	touchErr         error
	touchCalls       int
	lastToken        string
	setCookieHeaders []string
}

func (l *fakeTouchLoader) Load(context.Context, string, time.Time) (uuid.UUID, actor.Role, error) {
	if l.loadErr != nil {
		return uuid.Nil, "", l.loadErr
	}
	return l.loadUserID, l.loadRole, nil
}

func (l *fakeTouchLoader) Touch(_ context.Context, token, _ string, _ time.Time) (string, *time.Time, error) {
	l.touchCalls++
	l.lastToken = token
	if l.touchErr != nil {
		return "", nil, l.touchErr
	}
	return l.cookieToken, l.cookieExpires, nil
}

// touchMiddlewareRequest runs the request through the session middleware. It
// fails the test when the next handler is not reached and reports whether the
// handler saw an authenticated user in the request context.
func touchMiddlewareRequest(t *testing.T, loader *fakeTouchLoader, r *http.Request) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	rr := httptest.NewRecorder()
	var authenticated bool
	SessionMiddleware(nil, loader, false, nil)(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		_, authenticated = UserIDFromContext(req.Context())
	})).ServeHTTP(rr, r)
	if cookies := rr.Header().Values("Set-Cookie"); len(cookies) > 0 {
		loader.setCookieHeaders = cookies
	}
	return rr, authenticated
}

func requestWithSessionCookie(rawToken string) *http.Request {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/me", nil)
	r.AddCookie(&http.Cookie{Name: SessionCookieName(false), Value: rawToken})
	return r
}

func TestSessionMiddleware_RotationDeliversFreshCookie(t *testing.T) {
	t.Parallel()
	expires := time.Now().Add(time.Hour)
	loader := &fakeTouchLoader{
		loadUserID:    uuid.Must(uuid.NewV7()),
		cookieToken:   "fresh-token",
		cookieExpires: &expires,
	}

	rr, authenticated := touchMiddlewareRequest(t, loader, requestWithSessionCookie("old-raw"))

	if loader.touchCalls != 1 {
		t.Fatalf("Touch calls = %d, want 1", loader.touchCalls)
	}
	if loader.lastToken != "old-raw" {
		t.Fatalf("Touch token = %q, want the presented cookie value", loader.lastToken)
	}
	if !authenticated {
		t.Fatal("next handler ran without the authenticated user in context")
	}
	if len(loader.setCookieHeaders) != 1 {
		t.Fatalf("Set-Cookie headers = %d, want 1", len(loader.setCookieHeaders))
	}
	if !containsCookieValue(loader.setCookieHeaders[0], "fresh-token") || strings.Contains(loader.setCookieHeaders[0], "Max-Age=0") {
		t.Fatalf("Set-Cookie = %q, want the rotated token with a live expiry", loader.setCookieHeaders[0])
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}

func TestSessionMiddleware_SlidingMoveReissuesSameToken(t *testing.T) {
	t.Parallel()
	expires := time.Now().Add(7 * 24 * time.Hour)
	loader := &fakeTouchLoader{
		loadUserID:    uuid.Must(uuid.NewV7()),
		cookieExpires: &expires,
	}

	_, authenticated := touchMiddlewareRequest(t, loader, requestWithSessionCookie("raw"))

	if !authenticated {
		t.Fatal("next handler ran without the authenticated user in context")
	}
	if len(loader.setCookieHeaders) != 1 {
		t.Fatalf("Set-Cookie headers = %d, want 1 (expiry moved)", len(loader.setCookieHeaders))
	}
	if !containsCookieValue(loader.setCookieHeaders[0], "raw") || strings.Contains(loader.setCookieHeaders[0], "Max-Age=0") {
		t.Fatalf("Set-Cookie = %q, want the same token with a live expiry", loader.setCookieHeaders[0])
	}
}

func TestSessionMiddleware_NoCookieDecisionSendsNoCookie(t *testing.T) {
	t.Parallel()
	loader := &fakeTouchLoader{loadUserID: uuid.Must(uuid.NewV7())}

	_, authenticated := touchMiddlewareRequest(t, loader, requestWithSessionCookie("raw"))

	if !authenticated {
		t.Fatal("next handler ran without the authenticated user in context")
	}
	if len(loader.setCookieHeaders) != 0 {
		t.Fatalf("Set-Cookie headers = %d, want 0 when Touch reports no change", len(loader.setCookieHeaders))
	}
}

func TestSessionMiddleware_TouchErrorStillAuthenticates(t *testing.T) {
	t.Parallel()
	loader := &fakeTouchLoader{
		loadUserID: uuid.Must(uuid.NewV7()),
		touchErr:   errors.New("maintenance down"),
	}

	rr, authenticated := touchMiddlewareRequest(t, loader, requestWithSessionCookie("raw"))

	if !authenticated {
		t.Fatal("next handler ran without the authenticated user in context")
	}
	if len(loader.setCookieHeaders) != 0 {
		t.Fatalf("Set-Cookie headers = %d, want 0 on maintenance failure", len(loader.setCookieHeaders))
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 from the next handler (maintenance is fail-open)", rr.Code)
	}
}

// TestSessionMiddleware_UnknownOrExpiredTokenClearsCookieAndStaysAnonymous
// pins the Load miss contract: an unknown token and an expired one surface
// identically as the SessionNotFound sentinel — the cookie is cleared and the
// request proceeds anonymously.
func TestSessionMiddleware_UnknownOrExpiredTokenClearsCookieAndStaysAnonymous(t *testing.T) {
	t.Parallel()
	loader := &fakeTouchLoader{loadErr: SessionNotFound(nil)}

	rr, authenticated := touchMiddlewareRequest(t, loader, requestWithSessionCookie("dead-raw"))

	if authenticated {
		t.Fatal("a session-loader miss must not authenticate the request")
	}
	if loader.touchCalls != 0 {
		t.Fatalf("Touch calls = %d, want 0 on a loader miss", loader.touchCalls)
	}
	if len(loader.setCookieHeaders) != 1 || !strings.Contains(loader.setCookieHeaders[0], "Max-Age=0") {
		t.Fatalf("Set-Cookie headers = %v, want one clearing cookie", loader.setCookieHeaders)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (anonymous)", rr.Code)
	}
}

func TestSessionMiddleware_LoadFailureIs500(t *testing.T) {
	t.Parallel()
	loader := &fakeTouchLoader{loadErr: errors.New("db down")}

	rr, _ := touchMiddlewareRequest(t, loader, requestWithSessionCookie("raw"))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 on a loader failure", rr.Code)
	}
}

func containsCookieValue(cookie, value string) bool { return strings.Contains(cookie, value) }
