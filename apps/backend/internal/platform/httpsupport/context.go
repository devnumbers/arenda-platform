package httpsupport

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	identitydomain "github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

type (
	userIDKey struct{}
	userKey   struct{}
)

// UserIDFromContext returns the authenticated user ID from the request context.
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey{}).(uuid.UUID)
	return id, ok
}

// UserFromContext returns the authenticated user loaded by the session middleware.
func UserFromContext(ctx context.Context) (identitydomain.User, bool) {
	user, ok := ctx.Value(userKey{}).(identitydomain.User)
	return user, ok
}

// OwnerIDFromContext extracts the authenticated user ID from the request context.
// It returns false (without writing a response) when the user is not authenticated.
// Callers should write their own unauthorized response.
func OwnerIDFromContext(r *http.Request) (uuid.UUID, bool) {
	userID, ok := UserIDFromContext(r.Context())
	return userID, ok
}

// LoggerFromContext returns the logger stored in the context, or the default
// logger if none is present.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerCtxKey{}).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}
