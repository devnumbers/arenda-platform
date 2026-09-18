package httpsupport

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/requestctx"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/actor"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

func SessionCookieName(secure bool) string {
	if secure {
		return "__Host-session_id"
	}
	return "session_id"
}

func SetSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time, secure bool) {
	maxAge := max(1, int(time.Until(expiresAt).Seconds()))
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName(secure),
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func ClearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName(secure),
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func SessionTokenFromRequest(r *http.Request, secure bool) string {
	cookie, err := r.Cookie(SessionCookieName(secure))
	if err != nil {
		return ""
	}
	return cookie.Value
}

type fallbackClock struct{}

func (fallbackClock) Now() time.Time { return time.Now().UTC() }

// SessionLoader resolves the raw session cookie to the acting user and
// maintains the session's cookie lifecycle. It is the seam through which the
// session middleware authenticates requests without depending on any bounded
// context's domain (ADR 0034): the session state itself — expiry, sliding
// window, token rotation — never crosses this boundary; it lives behind the
// interface, in the identity application. The platform stays a dumb
// orchestrator that only sees the actor identity and the cookie it must
// re-issue.
type SessionLoader interface {
	// Load resolves a raw session token to the actor identity (user ID and
	// role). Unknown and expired tokens surface identically as the
	// [SessionNotFound] sentinel — expiry is the loader's decision, not the
	// transport's.
	Load(ctx context.Context, token string, now time.Time) (uuid.UUID, actor.Role, error)
	// Touch performs the per-request session maintenance keyed by the raw
	// token: the sliding expiry (pure sliding, ADR 0056), the throttled
	// last-seen stamp, the client IP with its city on change, and the 14-day
	// token rotation. A non-nil cookieExpires means the transport must
	// re-issue the session cookie: cookieToken carries the fresh token after
	// a rotation, or is empty when the presented token stays valid and only
	// the expiry moved. A nil cookieExpires means no cookie change.
	Touch(ctx context.Context, token, clientIP string, now time.Time) (cookieToken string, cookieExpires *time.Time, err error)
}

// publicSessionSkippedPaths are paths that never require a session lookup.
// They are explicitly public endpoints; authenticated handlers on these paths
// must validate session themselves if they need it.
var publicSessionSkippedPaths = []string{
	"/auth/send",
	"/auth/verify",
	"/webhooks/",
	"/internal/perf/",
	"/client-errors",
}

func isPublicSessionSkippedPath(path string) bool {
	for _, prefix := range publicSessionSkippedPaths {
		if path == prefix || strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// sessionMiddleware carries the wired session dependencies so each step of
// the request path (load, authenticate, touch) is a named method.
type sessionMiddleware struct {
	logger *slog.Logger
	loader SessionLoader
	secure bool
	clock  clock.Clock
}

// SessionMiddleware loads the authenticated actor from the session cookie into
// the request context. It depends only on the platform-neutral SessionLoader
// seam, not on any bounded context's domain (ADR 0034).
func SessionMiddleware(logger *slog.Logger, loader SessionLoader, secure bool, clk clock.Clock) func(http.Handler) http.Handler {
	if clk == nil {
		clk = fallbackClock{}
	}
	m := sessionMiddleware{logger: logger, loader: loader, secure: secure, clock: clk}
	return m.wrap
}

func (m sessionMiddleware) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicSessionSkippedPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		token := SessionTokenFromRequest(r, m.secure)
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}

		now := m.clock.Now()
		userID, role, err := m.loader.Load(r.Context(), token, now)
		if err != nil {
			m.serveLoadError(w, r, next, err)
			return
		}

		m.touchSession(w, r, token, now)

		ctx := WithUserID(r.Context(), userID)
		ctx = WithActor(ctx, userID, role)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

// serveLoadError maps a session lookup failure: the not-found sentinel —
// an unknown or an expired token, the loader's call — keeps the request
// anonymous with the cookie cleared, anything else is a 500 problem response.
func (m sessionMiddleware) serveLoadError(w http.ResponseWriter, r *http.Request, next http.Handler, err error) {
	if IsSessionNotFound(err) {
		m.serveWithClearedCookie(w, r, next)
		return
	}
	if m.logger != nil {
		m.logger.ErrorContext(r.Context(), "session lookup failed", slog.String("error", SanitizeError(err)))
	}
	WriteProblem(r.Context(), w, http.StatusInternalServerError, InternalError(r.Context(), err))
}

// serveWithClearedCookie clears the session cookie and continues the request
// anonymously.
func (m sessionMiddleware) serveWithClearedCookie(w http.ResponseWriter, r *http.Request, next http.Handler) {
	ClearSessionCookie(w, m.secure)
	next.ServeHTTP(w, r)
}

// touchSession runs the per-request session maintenance (sliding expiry,
// throttled last-seen, IP/city refresh, token rotation) and re-issues the
// cookie when the loader reports a change: a rotation delivers the fresh
// token, an expiry move re-stamps the presented one. A maintenance failure is
// logged once and leaves the previous cookie in place — the request still
// proceeds authenticated.
func (m sessionMiddleware) touchSession(w http.ResponseWriter, r *http.Request, token string, now time.Time) {
	cookieToken, cookieExpires, err := m.loader.Touch(r.Context(), token, requestctx.ClientIPFromContext(r.Context()), now)
	if err != nil {
		if m.logger != nil {
			m.logger.ErrorContext(r.Context(), "failed to touch session", slog.String("error", SanitizeError(err)))
		}
		return
	}
	if cookieExpires == nil {
		return
	}
	if cookieToken == "" {
		cookieToken = token
	}
	SetSessionCookie(w, cookieToken, *cookieExpires, m.secure)
}

// errSessionNotFound is the sentinel the SessionLoader seam uses to signal that
// the raw token did not resolve to a live session. The session middleware treats
// it as a public (cookie-clearing) miss rather than an internal error. Adapters
// map their context-specific "not found" error onto it.
var errSessionNotFound = errors.New("session not found")

// IsSessionNotFound reports whether err is the session-loader not-found sentinel.
func IsSessionNotFound(err error) bool { return errors.Is(err, errSessionNotFound) }

// SessionNotFound wraps the provided error (or returns the sentinel directly)
// so adapters can signal a not-found result through the platform-neutral seam
// without leaking their own sentinel types. The middleware matches it via
// [IsSessionNotFound].
func SessionNotFound(err error) error {
	if err == nil {
		return errSessionNotFound
	}
	return fmt.Errorf("%w: %w", errSessionNotFound, err)
}
