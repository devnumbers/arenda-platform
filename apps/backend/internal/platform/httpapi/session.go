package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

func sessionCookieName(secure bool) string {
	if secure {
		return "__Host-session_id"
	}
	return "session_id"
}

type userIDKey struct{}
type userKey struct{}

// UserIDFromContext returns the authenticated user ID from the request context.
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey{}).(uuid.UUID)
	return id, ok
}

// UserFromContext returns the authenticated user loaded by the session middleware.
func UserFromContext(ctx context.Context) (domain.User, bool) {
	user, ok := ctx.Value(userKey{}).(domain.User)
	return user, ok
}

func setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time, secure bool) {
	sameSite := http.SameSiteLaxMode
	if secure {
		sameSite = http.SameSiteStrictMode
	}
	maxAge := int(time.Until(expiresAt).Seconds())
	if maxAge < 1 {
		maxAge = 1
	}
	//nolint:gosec // Secure/HttpOnly/SameSite are configured dynamically based on APP_ENV.
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName(secure),
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})
}

func clearSessionCookie(w http.ResponseWriter, secure bool) {
	sameSite := http.SameSiteLaxMode
	if secure {
		sameSite = http.SameSiteStrictMode
	}
	//nolint:gosec // Secure/HttpOnly/SameSite are configured dynamically based on APP_ENV.
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName(secure),
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})
}

func sessionTokenFromRequest(r *http.Request, secure bool) string {
	cookie, err := r.Cookie(sessionCookieName(secure))
	if err != nil {
		return ""
	}
	return cookie.Value
}

// hashSessionToken hashes a raw session token for repository lookup.
func hashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

type fallbackClock struct{}

func (fallbackClock) Now() time.Time { return time.Now().UTC() }

// publicSessionSkippedPaths are paths that never require a session lookup.
// They are explicitly public endpoints; authenticated handlers on these paths
// must validate session themselves if they need it.
var publicSessionSkippedPaths = []string{
	"/auth/phone/send",
	"/auth/phone/verify",
	"/webhooks/",
	"/internal/perf/",
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
func SessionMiddleware(logger *slog.Logger, sessions application.SessionRepository, secure bool, clock clock.Clock) func(http.Handler) http.Handler {
	if clock == nil {
		clock = fallbackClock{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isPublicSessionSkippedPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			token := sessionTokenFromRequest(r, secure)
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}

			now := clock.Now()
			session, user, err := sessions.GetByTokenHash(r.Context(), hashSessionToken(token), now)
			if err != nil {
				if errors.Is(err, application.ErrNotFound) {
					clearSessionCookie(w, secure)
					next.ServeHTTP(w, r)
					return
				}
				if logger != nil {
					logger.ErrorContext(r.Context(), "session lookup failed", slog.String("error", sanitizeError(err)))
				}
				writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
				return
			}
			if session.IsExpired(now) {
				clearSessionCookie(w, secure)
				next.ServeHTTP(w, r)
				return
			}
			ctx := context.WithValue(r.Context(), userIDKey{}, user.ID)
			ctx = context.WithValue(ctx, userKey{}, user)
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}
