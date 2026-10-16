package httpsupport

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/actor"
)

type (
	userIDKey struct{}
	actorKey  struct{}
)

// UserIDFromContext returns the authenticated user ID from the request context.
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey{}).(uuid.UUID)
	return id, ok
}

// WithUserID returns a context carrying the authenticated user ID, as stored
// by the session middleware. It is the write counterpart of UserIDFromContext
// and is also used by handler tests.
func WithUserID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey{}, id)
}

// WithActor returns a context carrying the authenticated actor identity
// (user ID and role). It is the write counterpart of ActorFromContext, used by
// the session middleware. The full identity aggregate is intentionally not
// placed in the context (ADR 0034); handlers needing the full profile fetch it
// from the identity service.
func WithActor(ctx context.Context, userID uuid.UUID, role actor.Role) context.Context {
	return context.WithValue(ctx, actorKey{}, actorIdentity{userID: userID, role: role})
}

// ActorFromContext returns the authenticated actor identity (user ID and role)
// from the request context, as loaded by the session middleware. The third
// result is false when no actor is present.
func ActorFromContext(ctx context.Context) (uuid.UUID, actor.Role, bool) {
	a, ok := ctx.Value(actorKey{}).(actorIdentity)
	return a.userID, a.role, ok
}

type actorIdentity struct {
	userID uuid.UUID
	role   actor.Role
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
