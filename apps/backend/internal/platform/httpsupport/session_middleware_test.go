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

// testTokenHash is the token hash the preset session carries.
const testTokenHash = "hash"

// fakeTouchLoader drives the session middleware tests: Load answers with a
// preset session, Touch applies the preset maintenance outcome.
type fakeTouchLoader struct {
	loadSession Session
	loadUserID  uuid.UUID
	loadRole    actor.Role
	loadErr     error
	// Updated presets the Touch result; nil echoes the Touch argument.
	updated         *Session
	rotatedToken    string
	touchErr        error
	touchCalls      int
	lastClientIP    string
	setCookieValues []string
}

func (l *fakeTouchLoader) Load(context.Context, string, time.Time) (Session, uuid.UUID, actor.Role, error) {
	if l.loadErr != nil {
		return Session{}, uuid.Nil, "", l.loadErr
	}
	return l.loadSession, l.loadUserID, l.loadRole, nil
}

func (l *fakeTouchLoader) Touch(_ context.Context, session Session, clientIP string, _ time.Time) (Session, string, error) {
	l.touchCalls++
	l.lastClientIP = clientIP
	if l.touchErr != nil {
		return Session{}, "", l.touchErr
	}
	if l.updated != nil {
		return *l.updated, l.rotatedToken, nil
	}
	return session, l.rotatedToken, nil
}

func touchMiddlewareRequest(t *testing.T, loader *fakeTouchLoader, r *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	var reached bool
	SessionMiddleware(nil, loader, false, nil)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		reached = true
	})).ServeHTTP(rr, r)
	if !reached {
		t.Fatal("next handler was not reached")
	}
	if cookies := rr.Header().Values("Set-Cookie"); len(cookies) > 0 {
		loader.setCookieValues = cookies
	}
	return rr
}

func TestSessionMiddleware_RotationDeliversFreshCookie(t *testing.T) {
	t.Parallel()
	loader := &fakeTouchLoader{
		loadSession:  Session{TokenHash: "old-hash", ExpiresAt: time.Now().Add(time.Hour)},
		loadUserID:   uuid.Must(uuid.NewV7()),
		rotatedToken: "fresh-token",
	}
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/me", nil)
	r.AddCookie(&http.Cookie{Name: SessionCookieName(false), Value: "old-raw"})

	touchMiddlewareRequest(t, loader, r)

	if loader.touchCalls != 1 {
		t.Fatalf("Touch calls = %d, want 1", loader.touchCalls)
	}
	if len(loader.setCookieValues) != 1 {
		t.Fatalf("Set-Cookie headers = %d, want 1", len(loader.setCookieValues))
	}
	if !containsCookieValue(loader.setCookieValues[0], "fresh-token") {
		t.Fatalf("Set-Cookie = %q, want the rotated token", loader.setCookieValues[0])
	}
}

func TestSessionMiddleware_SlidingMoveReissuesSameToken(t *testing.T) {
	t.Parallel()
	expiry := time.Now().Add(time.Hour)
	loader := &fakeTouchLoader{
		loadSession: Session{TokenHash: "hash", ExpiresAt: expiry},
		loadUserID:  uuid.Must(uuid.NewV7()),
	}
	loader.updated = &Session{TokenHash: testTokenHash, ExpiresAt: expiry.Add(7 * 24 * time.Hour)}
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/me", nil)
	r.AddCookie(&http.Cookie{Name: SessionCookieName(false), Value: "raw"})

	touchMiddlewareRequest(t, loader, r)

	if len(loader.setCookieValues) != 1 {
		t.Fatalf("Set-Cookie headers = %d, want 1 (expiry moved)", len(loader.setCookieValues))
	}
	if !containsCookieValue(loader.setCookieValues[0], "raw") {
		t.Fatalf("Set-Cookie = %q, want the same token", loader.setCookieValues[0])
	}
}

func TestSessionMiddleware_TouchErrorStillAuthenticates(t *testing.T) {
	t.Parallel()
	loader := &fakeTouchLoader{
		loadSession: Session{TokenHash: testTokenHash, ExpiresAt: time.Now().Add(time.Hour)},
		loadUserID:  uuid.Must(uuid.NewV7()),
		touchErr:    errors.New("maintenance down"),
	}
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/me", nil)
	r.AddCookie(&http.Cookie{Name: SessionCookieName(false), Value: "raw"})

	rr := touchMiddlewareRequest(t, loader, r)

	if len(loader.setCookieValues) != 0 {
		t.Fatalf("Set-Cookie headers = %d, want 0 on maintenance failure", len(loader.setCookieValues))
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 from the next handler (maintenance is fail-open)", rr.Code)
	}
}

func containsCookieValue(cookie, value string) bool { return strings.Contains(cookie, value) }
