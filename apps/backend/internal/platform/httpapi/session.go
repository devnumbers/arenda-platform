package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
)

const sessionCookieName = "session_id"

type contextKey int

const userIDKey contextKey = 0

// UserIDFromContext returns the authenticated user ID from the request context.
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey).(uuid.UUID)
	return id, ok
}

func setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time, secure bool) {
	//nolint:gosec // Secure flag is configured via COOKIE_SECURE for local dev.
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter, secure bool) {
	//nolint:gosec // Secure flag is configured via COOKIE_SECURE for local dev.
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func sessionTokenFromRequest(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookieName)
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

type defaultClock struct{}

func (defaultClock) Now() time.Time { return time.Now().UTC() }

// SessionMiddleware loads the authenticated user from the session cookie into the request context.
func SessionMiddleware(logger *slog.Logger, sessions application.SessionRepository, secure bool, clock application.Clock) func(http.Handler) http.Handler {
	if clock == nil {
		clock = defaultClock{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := sessionTokenFromRequest(r)
			if token != "" {
				now := clock.Now()
				session, user, err := sessions.GetByTokenHash(r.Context(), hashSessionToken(token), now)
				if err != nil {
					if logger != nil {
						logger.ErrorContext(r.Context(), "session lookup failed", slog.String("error", err.Error()))
					}
				} else if !session.IsExpired(now) {
					r = r.WithContext(context.WithValue(r.Context(), userIDKey, user.ID))
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
