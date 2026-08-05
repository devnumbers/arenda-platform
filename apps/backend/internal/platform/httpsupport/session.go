package httpsupport

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
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

// SessionMiddleware loads the authenticated user from the session cookie into the request context.
func SessionMiddleware(logger *slog.Logger, sessions application.SessionService, secure bool, clock clock.Clock) func(http.Handler) http.Handler {
	if clock == nil {
		clock = fallbackClock{}
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

			now := clock.Now()
			session, user, err := sessions.Load(r.Context(), token, now)
			if err != nil {
				if errors.Is(err, application.ErrNotFound) {
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

			refreshedSession := session
			if refreshedSession.Refresh(now) {
				if err := sessions.Update(r.Context(), refreshedSession); err != nil {
					if logger != nil {
						logger.ErrorContext(r.Context(), "failed to refresh session", slog.String("error", SanitizeError(err)))
					}
				} else {
					SetSessionCookie(w, token, refreshedSession.ExpiresAt, secure)
					session = refreshedSession
				}
			}

			ctx := context.WithValue(r.Context(), userIDKey{}, user.ID)
			ctx = context.WithValue(ctx, userKey{}, user)
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}
