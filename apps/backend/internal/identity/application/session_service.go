package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

// DeviceContext carries the transport-derived client facts the session is
// opened from: the raw User-Agent header and the client IP from the
// real-IP middleware. The application parses/resolves them once, at session
// creation (issue #728).
type DeviceContext struct {
	UserAgent string
	IP        string
}

// lastSeenWriteInterval throttles last-seen writes: a request only persists
// session activity when the stored stamp is at least this old, so a burst of
// requests costs at most one UPDATE per interval.
const lastSeenWriteInterval = time.Minute

// SessionService is the application-layer facade for session lookups, updates,
// issuance, and per-request activity maintenance. It wraps the
// SessionRepository so that transport code does not depend directly on
// persistence details.
type SessionService interface {
	Load(ctx context.Context, rawToken string, now time.Time) (domain.Session, domain.User, error)
	// Touch maintains the session on an authenticated request: it slides the
	// expiry (pure sliding, ADR 0056), persists the last-seen stamp throttled
	// by lastSeenWriteInterval, refreshes the client IP and its GeoIP city on
	// an IP change, and rotates the token when the 14-day renewal window has
	// elapsed — atomically, inside the UoW (ADR 0033). It returns the updated
	// session and, when a rotation won the race, the fresh raw token the
	// transport must deliver as the new cookie value.
	Touch(ctx context.Context, session domain.Session, clientIP string, now time.Time) (updated domain.Session, rotatedToken string, err error)
	// Issue finds or creates the user for the phone+email pair and opens a new
	// session for them. It runs inside the caller's transaction via stores so
	// verification, mark-used, issuance, and audit share one commit (ADR 0033,
	// migration step 4). The returned isNew flag is true only when a brand-new
	// user row was inserted; a create that races an concurrent insert and ends
	// up loading the existing row reports isNew=false. Audit and the
	// UserRegistered event stay the orchestrator's responsibility. The device
	// context is captured once, onto the fresh session: the User-Agent is
	// parsed and the city resolved from the IP here and never again.
	Issue(
		ctx context.Context,
		stores *txStores,
		phone domain.Phone,
		email domain.Email,
		device DeviceContext,
		now time.Time,
	) (domain.RawSession, domain.User, bool, error)
}

type sessionService struct {
	txStoreFactory
	hasher TokenHasher
	parser DeviceInfoParser
	geo    GeoResolver
}

// NewSessionService creates a SessionService backed by the provided repository.
// The hasher (used to look up sessions by the hashed value of the raw token and
// to hash freshly issued tokens before persisting them) is carried in cfg for
// uniformity with the other identity constructors. The shared identity
// txStoreFactory is embedded so Issue can run inside the caller's runInTx.
// A nil Parser or GeoResolver degrades gracefully: the device type stays
// unknown and no city is resolved.
func NewSessionService(
	factory txStoreFactory,
	cfg SessionServiceConfig,
) *sessionService {
	return &sessionService{
		txStoreFactory: factory,
		hasher:         cfg.Hasher,
		parser:         cfg.Parser,
		geo:            cfg.Geo,
	}
}

// SessionServiceConfig carries the non-transactional dependencies for
// sessionService. The transactional repositories, audit recorder, and UoW
// live in the shared txStoreFactory passed to NewSessionService.
type SessionServiceConfig struct {
	Hasher TokenHasher
	// Parser turns the raw User-Agent into the stored device description.
	// Nil means "no parsing": the session keeps the raw UA with an unknown
	// device type.
	Parser DeviceInfoParser
	// Geo resolves client IPs to cities. Nil means "no geolocation": the
	// session stores the IP without a city.
	Geo GeoResolver
}

func (s *sessionService) Load(ctx context.Context, rawToken string, now time.Time) (domain.Session, domain.User, error) {
	return s.sessions.GetByTokenHash(ctx, s.hasher.HashToken(rawToken), now)
}

func (s *sessionService) Touch(
	ctx context.Context, session domain.Session, clientIP string, now time.Time,
) (domain.Session, string, error) {
	if session.RenewalDue(now) {
		return s.rotate(ctx, session, clientIP, now)
	}

	// Pure sliding makes the expiry candidate move on every request, so the
	// last-seen throttle is the single write gate: at most one UPDATE per
	// interval per session. The write carries the expiry, the stamp, and the
	// IP/city together.
	stale := now.Sub(session.LastUsedAt) >= lastSeenWriteInterval
	ipChanged := clientIP != "" && clientIP != session.LastIP
	if !stale && !ipChanged {
		return session, "", nil
	}

	session.Refresh(now)
	if ipChanged {
		city := s.cityFor(ctx, session, clientIP)
		session.LastIP = clientIP
		session.City = city
	}
	if err := s.sessions.Touch(ctx, session); err != nil {
		return session, "", fmt.Errorf("touch session: %w", err)
	}
	return session, "", nil
}

// rotate replaces the session token at the renewal window: a fresh token is
// generated, the swap runs atomically in the UoW (the old hash moves to the
// grace slot), and the raw token travels back so the transport can re-issue
// the cookie. A lost race (another request rotated first) is a no-op — the
// losing request keeps its (now previous) token, which the grace window still
// accepts.
func (s *sessionService) rotate(
	ctx context.Context, session domain.Session, clientIP string, now time.Time,
) (domain.Session, string, error) {
	token, err := domain.NewToken()
	if err != nil {
		return session, "", fmt.Errorf("generate rotated token: %w", err)
	}
	newHash := s.hasher.HashToken(token)

	session.Refresh(now) // Refresh also stamps LastUsedAt.
	if clientIP != "" {
		city := s.cityFor(ctx, session, clientIP)
		session.LastIP = clientIP
		session.City = city
	}
	session.PreviousTokenHash = session.TokenHash
	session.RotatedAt = now

	err = s.runInTx(ctx, func(stores *txStores) error {
		won, rErr := stores.sessions.Rotate(ctx, session, newHash)
		if rErr != nil {
			return rErr
		}
		if !won {
			return errRotationLost
		}
		return nil
	})
	if errors.Is(err, errRotationLost) {
		// Another request rotated this session first; the in-flight old token
		// stays valid through the grace window.
		return session, "", nil
	}
	if err != nil {
		return session, "", fmt.Errorf("rotate session token: %w", err)
	}

	session.TokenHash = newHash
	return session, token, nil
}

// errRotationLost signals that a concurrent request won the token rotation;
// runInTx rolls the losing transaction back.
var errRotationLost = errors.New("session token rotated concurrently")

// cityFor resolves the city for a changed client IP. An unknown IP (private,
// missing from the base) keeps the stored city — "unknown" never erases a
// known one.
func (s *sessionService) cityFor(ctx context.Context, session domain.Session, ip string) string {
	if s.geo == nil || ip == "" || ip == session.LastIP {
		return session.City
	}
	city, found := s.geo.ResolveCity(ctx, ip)
	if !found {
		return session.City
	}
	return city
}

// Issue finds or creates the user for phone, marking email verified, then opens
// and persists a new session. It operates inside the caller's transaction via
// stores. Returns the raw session token, the user, and isNew (true only when a
// new user row was inserted).
func (s *sessionService) Issue(
	ctx context.Context,
	stores *txStores,
	phone domain.Phone,
	email domain.Email,
	device DeviceContext,
	now time.Time,
) (domain.RawSession, domain.User, bool, error) {
	user, err := stores.users.GetByPhone(ctx, phone)
	isNewUser := false
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return domain.RawSession{}, domain.User{}, false, fmt.Errorf("get user: %w", err)
		}
		isNewUser = true
		newUser, createErr := domain.NewOwner(phone)
		if createErr != nil {
			return domain.RawSession{}, domain.User{}, false, fmt.Errorf("create user: %w", createErr)
		}
		newUser.VerifyEmail(email, now)
		user, createErr = stores.users.Create(ctx, newUser)
		if createErr != nil {
			return domain.RawSession{}, domain.User{}, false, fmt.Errorf("save user: %w", createErr)
		}
		if user.ID != newUser.ID {
			isNewUser = false
		}
	} else if user.Email == nil || *user.Email != email || user.EmailVerifiedAt == nil {
		updated, updateErr := stores.users.UpdateEmailVerified(ctx, user.ID, &email, &now)
		if updateErr != nil {
			return domain.RawSession{}, domain.User{}, false, fmt.Errorf("verify user email: %w", updateErr)
		}
		user = updated
	}

	raw, err := domain.NewSession(user.ID, now)
	if err != nil {
		return domain.RawSession{}, domain.User{}, false, fmt.Errorf("create session: %w", err)
	}
	raw.Session.TokenHash = s.hasher.HashToken(raw.Token)
	s.captureDevice(ctx, &raw.Session, device)

	if err := stores.sessions.Create(ctx, raw.Session); err != nil {
		return domain.RawSession{}, domain.User{}, false, fmt.Errorf("save session: %w", err)
	}

	return raw, user, isNewUser, nil
}

// captureDevice stores the client description on a brand-new session: the raw
// User-Agent always, the parsed device fields when a parser is wired, and the
// city for the client IP when the resolver knows it.
func (s *sessionService) captureDevice(ctx context.Context, session *domain.Session, device DeviceContext) {
	session.UserAgent = device.UserAgent
	session.LastIP = device.IP
	if s.parser != nil {
		info := s.parser.Parse(device.UserAgent)
		session.DeviceType = info.DeviceType
		session.Browser = info.Browser
		session.BrowserMajor = info.BrowserMajor
		session.OS = info.OS
	} else {
		session.DeviceType = domain.DeviceUnknown
	}
	if s.geo != nil && device.IP != "" {
		if city, found := s.geo.ResolveCity(ctx, device.IP); found {
			session.City = city
		}
	}
}
