package http

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/actor"
)

// sessionLoader adapts the identity [identityapp.SessionService] to the
// platform-neutral [httpsupport.SessionLoader] seam. It is the boundary that
// keeps platform/httpsupport free of any identity/domain import: the identity
// session/user aggregate is mapped here onto (httpsupport.Session, userID,
// actor.Role) (ADR 0034).
type sessionLoader struct {
	svc identityapp.SessionService
}

// NewSessionLoader returns a [httpsupport.SessionLoader] backed by the identity
// session service. It is the adapter the HTTP server passes to the platform
// session middleware.
func NewSessionLoader(svc identityapp.SessionService) httpsupport.SessionLoader {
	if svc == nil {
		return nil
	}
	return sessionLoader{svc: svc}
}

func (l sessionLoader) Load(ctx context.Context, token string, now time.Time) (httpsupport.Session, uuid.UUID, actor.Role, error) {
	sess, user, err := l.svc.Load(ctx, token, now)
	if err != nil {
		// Map the identity "not found" sentinel onto the platform-neutral
		// session-not-found sentinel so the middleware treats a dead session as
		// a public, cookie-clearing miss rather than an internal error.
		if errors.Is(err, identityapp.ErrNotFound) {
			return httpsupport.Session{}, uuid.Nil, "", httpsupport.SessionNotFound(err)
		}
		return httpsupport.Session{}, uuid.Nil, "", err
	}
	return toPlatformSession(sess), user.ID, user.Role, nil
}

func (l sessionLoader) Update(ctx context.Context, sess httpsupport.Session) error {
	return l.svc.Update(ctx, fromPlatformSession(sess))
}

// toPlatformSession maps the identity session aggregate onto the platform-neutral
// [httpsupport.Session] view. Only the fields the session middleware touches are
// carried; domain-only fields (ID, UserID) are intentionally dropped.
func toPlatformSession(sess domain.Session) httpsupport.Session {
	return httpsupport.Session{
		TokenHash:  sess.TokenHash,
		ExpiresAt:  sess.ExpiresAt,
		CreatedAt:  sess.CreatedAt,
		LastUsedAt: sess.LastUsedAt,
	}
}

// fromPlatformSession rebuilds the identity session aggregate from the
// platform-neutral view for persistence. The TokenHash identifies the row;
// ExpiresAt and LastUsedAt are the sliding-window fields persisted by Update.
// ID, UserID and CreatedAt are not known to the platform view and are left zero,
// which is correct for the update path (see SessionRepository.Update).
func fromPlatformSession(sess httpsupport.Session) domain.Session {
	return domain.Session{
		TokenHash:  sess.TokenHash,
		ExpiresAt:  sess.ExpiresAt,
		CreatedAt:  sess.CreatedAt,
		LastUsedAt: sess.LastUsedAt,
	}
}
