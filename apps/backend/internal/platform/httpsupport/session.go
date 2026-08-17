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
	//nolint:gosec // Secure is configured dynamically based on APP_ENV; SameSite is fixed to Lax (ADR 0018).
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
	//nolint:gosec // Secure is configured dynamically based on APP_ENV; SameSite is fixed to Lax (ADR 0018).
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

// Session is the platform-neutral view of an authenticated session. It carries
// only the fields the session middleware touches (sliding-window refresh and the
// cookie expiry); it deliberately avoids any bounded-context type so that
// platform/httpsupport does not depend on identity/domain (ADR 0034).
//
// The Refresh sliding-window logic mirrors identity/domain.Session.Refresh; the
// TTL constants are identical. The duplication is the price of layer isolation:
// the platform cannot import the identity aggregate, and the identity aggregate
// is not lifted into the shared kernel.
type Session struct {
	TokenHash  string
	ExpiresAt  time.Time
	CreatedAt  time.Time
	LastUsedAt time.Time
}

// SessionBaseTTL is the per-request sliding window for an active session.
const SessionBaseTTL = 7 * 24 * time.Hour

// SessionMaxTTL is the hard upper bound for a session since creation.
const SessionMaxTTL = 30 * 24 * time.Hour

// IsExpired reports whether the session has passed its expiry at now.
func (s *Session) IsExpired(now time.Time) bool {
	return now.After(s.ExpiresAt)
}

// Refresh extends the session expiration by SessionBaseTTL, capped at
// CreatedAt + SessionMaxTTL. It returns true when the expiration was actually
// moved forward. Expired sessions are never refreshed. Mirrors
// identity/domain.Session.Refresh (ADR 0034).
func (s *Session) Refresh(now time.Time) bool {
	if s.IsExpired(now) {
		return false
	}

	maxExpires := s.CreatedAt.Add(SessionMaxTTL)
	candidate := now.Add(SessionBaseTTL)
	if candidate.After(maxExpires) {
		candidate = maxExpires
	}
	if !candidate.After(s.ExpiresAt) {
		return false
	}

	s.ExpiresAt = candidate
	s.LastUsedAt = now
	return true
}

// SessionLoader loads and persists the platform-neutral session view. It is the
// seam through which the session middleware reads a session without depending
// on identity/domain: an adapter in the identity context maps the identity
// session/user aggregate onto (Session, userID, actor.Role) (ADR 0034).
type SessionLoader interface {
	// Load resolves a raw session token to the platform-neutral session together
	// with the actor identity (user ID and role). now seeds the sliding window.
	Load(ctx context.Context, token string, now time.Time) (Session, uuid.UUID, actor.Role, error)
	// Update persists a refreshed session's sliding-window fields.
	Update(ctx context.Context, session Session) error
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

// SessionMiddleware loads the authenticated actor from the session cookie into
// the request context. It depends only on the platform-neutral SessionLoader
// seam, not on any bounded context's domain (ADR 0034).
func SessionMiddleware(logger *slog.Logger, loader SessionLoader, secure bool, clk clock.Clock) func(http.Handler) http.Handler {
	if clk == nil {
		clk = fallbackClock{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isPublicSessionSkippedPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			token := SessionTokenFromRequest(r, secure)
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}

			now := clk.Now()
			session, userID, role, err := loader.Load(r.Context(), token, now)
			if err != nil {
				if errors.Is(err, errSessionNotFound) {
					ClearSessionCookie(w, secure)
					next.ServeHTTP(w, r)
					return
				}
				if logger != nil {
					logger.ErrorContext(r.Context(), "session lookup failed", slog.String("error", SanitizeError(err)))
				}
				WriteProblem(w, http.StatusInternalServerError, InternalError(r.Context(), err))
				return
			}
			if session.IsExpired(now) {
				ClearSessionCookie(w, secure)
				next.ServeHTTP(w, r)
				return
			}

			if session.Refresh(now) {
				if err := loader.Update(r.Context(), session); err != nil {
					if logger != nil {
						logger.ErrorContext(r.Context(), "failed to refresh session", slog.String("error", SanitizeError(err)))
					}
				} else {
					SetSessionCookie(w, token, session.ExpiresAt, secure)
				}
			}

			ctx := WithUserID(r.Context(), userID)
			ctx = WithActor(ctx, userID, role)
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
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
