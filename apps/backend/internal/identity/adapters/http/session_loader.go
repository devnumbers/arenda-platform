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

func (l sessionLoader) Touch(
	ctx context.Context, sess httpsupport.Session, clientIP string, now time.Time,
) (httpsupport.Session, string, error) {
	updated, rotatedToken, err := l.svc.Touch(ctx, fromPlatformSession(sess), clientIP, now)
	if err != nil {
		return httpsupport.Session{}, "", err
	}
	return toPlatformSession(updated), rotatedToken, nil
}

// toPlatformSession maps the identity session aggregate onto the platform-neutral
// [httpsupport.Session] view. Only the fields the session middleware and its
// Touch seam read or write are carried; UserID — the one field with no meaning
// outside the aggregate — is intentionally dropped.
func toPlatformSession(sess domain.Session) httpsupport.Session {
	return httpsupport.Session{
		ID:         sess.ID,
		TokenHash:  sess.TokenHash,
		ExpiresAt:  sess.ExpiresAt,
		CreatedAt:  sess.CreatedAt,
		LastUsedAt: sess.LastUsedAt,
		RotatedAt:  sess.RotatedAt,
		LastIP:     sess.LastIP,
		City:       sess.City,
	}
}

// fromPlatformSession rebuilds the identity session aggregate from the
// platform-neutral view for maintenance. UserID, the device description and
// the raw User-Agent are not part of the platform view; the maintenance paths
// key rows by TokenHash/ID and never rewrite them, so the zero values are safe.
func fromPlatformSession(sess httpsupport.Session) domain.Session {
	return domain.Session{
		ID:         sess.ID,
		TokenHash:  sess.TokenHash,
		ExpiresAt:  sess.ExpiresAt,
		CreatedAt:  sess.CreatedAt,
		LastUsedAt: sess.LastUsedAt,
		RotatedAt:  sess.RotatedAt,
		LastIP:     sess.LastIP,
		City:       sess.City,
	}
}
