package http

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/actor"
)

// sessionLoader adapts the identity [identityapp.SessionService] to the
// platform-neutral [httpsupport.SessionLoader] seam. It is the boundary that
// keeps platform/httpsupport free of any identity/domain import (ADR 0034):
// the platform only ever sees the actor identity and the cookie decision, so
// session state never needs a platform-side mirror.
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

func (l sessionLoader) Load(ctx context.Context, token string, now time.Time) (uuid.UUID, actor.Role, error) {
	_, user, err := l.svc.Load(ctx, token, now)
	if err != nil {
		return uuid.Nil, "", mapSessionLoadErr(err)
	}
	return user.ID, user.Role, nil
}

// mapSessionLoadErr maps the identity "not found" sentinel onto the
// platform-neutral session-not-found sentinel so the middleware treats an
// unknown or expired session as a public, cookie-clearing miss rather than an
// internal error.
func mapSessionLoadErr(err error) error {
	if errors.Is(err, identityapp.ErrNotFound) {
		return httpsupport.SessionNotFound(err)
	}
	return err
}

// Touch re-resolves the session by the presented token and maps the service
// outcome onto the transport's cookie decision: a rotation delivers the fresh
// raw token, an expiry move re-stamps the presented one. The re-resolve is the
// deliberate price of carrying no session state across the seam (#755): the
// maintenance works on a fresh snapshot, and the platform never mirrors
// identity fields. A token that died between the middleware Load and this
// reload surfaces as [httpsupport.SessionNotFound] — the middleware keeps the
// request authenticated (fail-open) without re-issuing a cookie.
func (l sessionLoader) Touch(
	ctx context.Context, rawToken, clientIP string, now time.Time,
) (string, *time.Time, error) {
	sess, _, err := l.svc.Load(ctx, rawToken, now)
	if err != nil {
		return "", nil, mapSessionLoadErr(err)
	}
	expiresBefore := sess.ExpiresAt

	updated, rotatedToken, err := l.svc.Touch(ctx, sess, clientIP, now)
	if err != nil {
		return "", nil, err
	}
	if rotatedToken != "" {
		return rotatedToken, &updated.ExpiresAt, nil
	}
	if !updated.ExpiresAt.Equal(expiresBefore) {
		return "", &updated.ExpiresAt, nil
	}
	return "", nil, nil
}
