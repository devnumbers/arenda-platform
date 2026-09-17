package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

const (
	testStoredIP   = "203.0.113.10"
	testStoredCity = "Москва"
	testNewIP      = "198.51.100.7"
	testNewCity    = "Казань"
	testPrivateIP  = "10.1.2.3"
	testUserAgent  = "Mozilla/5.0 TestUA"
)

// touchRepo records Touch/Rotate traffic for the Touch tests; every other
// session method is an inert stub.
type touchRepo struct {
	sessions map[string]domain.Session

	touched     []domain.Session
	touchErr    error
	rotated     []domain.Session
	lastNewHash string
	rotateWon   bool
	rotateErr   error
}

func newTouchRepo(seed domain.Session) *touchRepo {
	return &touchRepo{
		sessions:  map[string]domain.Session{seed.TokenHash: seed},
		rotateWon: true,
	}
}

func (r *touchRepo) Create(_ context.Context, s domain.Session) error {
	r.sessions[s.TokenHash] = s
	return nil
}

func (r *touchRepo) GetByTokenHash(context.Context, string, time.Time) (domain.Session, domain.User, error) {
	return domain.Session{}, domain.User{}, ErrNotFound
}

func (r *touchRepo) Update(context.Context, domain.Session) error { return nil }

func (r *touchRepo) Touch(_ context.Context, s domain.Session) error {
	if r.touchErr != nil {
		return r.touchErr
	}
	r.touched = append(r.touched, s)
	r.sessions[s.TokenHash] = s
	return nil
}

func (r *touchRepo) Rotate(_ context.Context, s domain.Session, newTokenHash string) (bool, error) {
	if r.rotateErr != nil {
		return false, r.rotateErr
	}
	r.rotated = append(r.rotated, s)
	r.lastNewHash = newTokenHash
	if !r.rotateWon {
		return false, nil
	}
	delete(r.sessions, s.PreviousTokenHash)
	r.sessions[newTokenHash] = s
	return true, nil
}

func (r *touchRepo) ListByUserID(context.Context, uuid.UUID) ([]domain.Session, error) {
	return nil, nil
}

func (r *touchRepo) GetByID(_ context.Context, id uuid.UUID) (domain.Session, error) {
	for _, s := range r.sessions {
		if s.ID == id {
			return s, nil
		}
	}
	return domain.Session{}, ErrNotFound
}

func (r *touchRepo) DeleteByTokenHash(context.Context, string) error { return nil }

func (r *touchRepo) DeleteByUserID(context.Context, uuid.UUID) error { return nil }

func (r *touchRepo) DeleteByUserIDExcept(context.Context, uuid.UUID, string) (int64, error) {
	return 0, nil
}

func (r *touchRepo) DeleteByIDForUser(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func (r *touchRepo) WithTx(transaction.Tx) (SessionRepository, error) { return r, nil }

// countingGeo records the IPs the resolver was asked about.
type countingGeo struct {
	calls    []string
	cityByIP map[string]string
}

func (g *countingGeo) ResolveCity(_ context.Context, ip string) (string, bool) {
	g.calls = append(g.calls, ip)
	city, ok := g.cityByIP[ip]
	return city, ok
}

// fixedParser returns a device description keyed off the UA string, so tests
// can assert the parser ran exactly once per issuance.
type fixedParser struct {
	calls []string
}

func (p *fixedParser) Parse(userAgent string) domain.DeviceInfo {
	p.calls = append(p.calls, userAgent)
	return domain.DeviceInfo{DeviceType: domain.DevicePhone, Browser: "TestBrowser", BrowserMajor: 121, OS: "TestOS"}
}

// testSessionHash is the canonical token hash every seeded session carries.
const testSessionHash = "hash-1"

// seededSession builds a live session created 10 minutes ago whose last
// persisted write happened 30 seconds ago (the throttle window has not
// elapsed): the expiry equals last-write + base TTL, as Touch leaves it.
func seededSession(now time.Time) domain.Session {
	lastWrite := now.Add(-30 * time.Second)
	return domain.Session{
		ID:         uuid.Must(uuid.NewV7()),
		UserID:     uuid.Must(uuid.NewV7()),
		TokenHash:  testSessionHash,
		ExpiresAt:  lastWrite.Add(domain.SessionBaseTTL),
		CreatedAt:  now.Add(-10 * time.Minute),
		LastUsedAt: lastWrite,
		RotatedAt:  now.Add(-10 * time.Minute),
		LastIP:     testStoredIP,
		City:       testStoredCity,
	}
}

func newTouchService(repo *touchRepo, geo *countingGeo) SessionService {
	factory := NewTxStoreFactory(
		newFakeUserRepo(), newFakeCodeRepo(), newFakeAttemptRepo(),
		repo, newFakeGrantRepo(), nil, &fakeUoW{beginner: &fakeBeginner{}},
	)
	return NewSessionService(factory, SessionServiceConfig{Hasher: fakeHasher{}, Geo: geo})
}

func TestSessionServiceTouch_NoWriteWhenNothingChanged(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	repo := newTouchRepo(seededSession(now))
	geo := &countingGeo{cityByIP: map[string]string{testStoredIP: testStoredCity}}

	svc := newTouchService(repo, geo)
	got, rotatedToken, err := svc.Touch(context.Background(), repo.sessions[testSessionHash], testStoredIP, now.Add(15*time.Second))
	if err != nil {
		t.Fatalf("Touch error = %v", err)
	}
	if rotatedToken != "" {
		t.Fatalf("rotatedToken = %q, want empty", rotatedToken)
	}
	if len(repo.touched) != 0 {
		t.Fatalf("Touch persisted %d times, want 0 (throttled)", len(repo.touched))
	}
	if len(repo.rotated) != 0 {
		t.Fatalf("rotation ran, want none")
	}
	if len(geo.calls) != 0 {
		t.Fatalf("geo resolved %v, want no calls", geo.calls)
	}
	if !got.ExpiresAt.Equal(repo.sessions[testSessionHash].ExpiresAt) || !got.LastUsedAt.Equal(repo.sessions[testSessionHash].LastUsedAt) {
		t.Fatalf("in-memory session moved without a write: %+v", got)
	}
}

func TestSessionServiceTouch_PersistsThrottledActivity(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	repo := newTouchRepo(seededSession(now))
	geo := &countingGeo{cityByIP: map[string]string{testStoredIP: testStoredCity}}

	svc := newTouchService(repo, geo)
	requestAt := now.Add(2 * time.Minute) // The throttle window has elapsed.
	got, _, err := svc.Touch(context.Background(), repo.sessions[testSessionHash], testStoredIP, requestAt)
	if err != nil {
		t.Fatalf("Touch error = %v", err)
	}
	if len(repo.touched) != 1 {
		t.Fatalf("Touch persisted %d times, want 1", len(repo.touched))
	}
	if !got.LastUsedAt.Equal(requestAt) {
		t.Fatalf("LastUsedAt = %v, want %v", got.LastUsedAt, requestAt)
	}
	// The single write carries the sliding expiry with it: pure sliding keeps
	// the session alive 7 days from the latest persisted activity.
	wantExpiry := requestAt.Add(domain.SessionBaseTTL)
	if !got.ExpiresAt.Equal(wantExpiry) {
		t.Fatalf("ExpiresAt = %v, want %v", got.ExpiresAt, wantExpiry)
	}
	persisted := repo.sessions[testSessionHash]
	if !persisted.ExpiresAt.Equal(wantExpiry) || !persisted.LastUsedAt.Equal(requestAt) {
		t.Fatalf("persisted session = %+v, want expiry %v last-used %v", persisted, wantExpiry, requestAt)
	}
}

func TestSessionServiceTouch_IPChangeResolvesCity(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	repo := newTouchRepo(seededSession(now))
	geo := &countingGeo{cityByIP: map[string]string{testNewIP: testNewCity}}

	svc := newTouchService(repo, geo)
	requestAt := now.Add(30 * time.Second)
	got, _, err := svc.Touch(context.Background(), repo.sessions[testSessionHash], testNewIP, requestAt)
	if err != nil {
		t.Fatalf("Touch error = %v", err)
	}
	if len(repo.touched) != 1 {
		t.Fatalf("Touch persisted %d times, want 1", len(repo.touched))
	}
	if len(geo.calls) != 1 || geo.calls[0] != testNewIP {
		t.Fatalf("geo calls = %v, want exactly [198.51.100.7]", geo.calls)
	}
	persisted := repo.sessions[testSessionHash]
	if persisted.LastIP != testNewIP {
		t.Fatalf("persisted LastIP = %q, want 198.51.100.7", persisted.LastIP)
	}
	if persisted.City != testNewCity {
		t.Fatalf("persisted City = %q, want Казань", persisted.City)
	}
	if got.City != testNewCity {
		t.Fatalf("returned City = %q, want Казань", got.City)
	}
}

func TestSessionServiceTouch_UnknownCityKeepsStoredOne(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	repo := newTouchRepo(seededSession(now))
	// Resolver knows nothing about the new (private) IP.
	geo := &countingGeo{cityByIP: map[string]string{}}

	svc := newTouchService(repo, geo)
	_, _, err := svc.Touch(context.Background(), repo.sessions[testSessionHash], testPrivateIP, now.Add(30*time.Second))
	if err != nil {
		t.Fatalf("Touch error = %v", err)
	}
	persisted := repo.sessions[testSessionHash]
	if persisted.City != testStoredCity {
		t.Fatalf("persisted City = %q, want Москва kept (unknown must not erase)", persisted.City)
	}
	if persisted.LastIP != testPrivateIP {
		t.Fatalf("persisted LastIP = %q, want 10.1.2.3", persisted.LastIP)
	}
}

func TestSessionServiceTouch_RotatesOnRenewalWindow(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	seed := seededSession(now)
	seed.RotatedAt = now.Add(-domain.SessionRenewalInterval) // The renewal window has elapsed.
	repo := newTouchRepo(seed)
	geo := &countingGeo{cityByIP: map[string]string{testStoredIP: testStoredCity}}

	svc := newTouchService(repo, geo)
	got, rotatedToken, err := svc.Touch(context.Background(), repo.sessions[testSessionHash], testStoredIP, now)
	if err != nil {
		t.Fatalf("Touch error = %v", err)
	}
	if rotatedToken == "" {
		t.Fatal("rotatedToken is empty, want a fresh raw token")
	}
	if len(rotatedToken) != 64 {
		t.Fatalf("rotatedToken length = %d, want 64 hex chars", len(rotatedToken))
	}
	if len(repo.rotated) != 1 {
		t.Fatalf("repo.Rotate called %d times, want 1", len(repo.rotated))
	}
	if repo.lastNewHash != got.TokenHash {
		t.Fatalf("Rotate stored hash %q, want the hash of the returned session %q", repo.lastNewHash, got.TokenHash)
	}
	if got.PreviousTokenHash != "hash-1" {
		t.Fatalf("PreviousTokenHash = %q, want hash-1 (grace slot)", got.PreviousTokenHash)
	}
	if !got.RotatedAt.Equal(now) {
		t.Fatalf("RotatedAt = %v, want %v", got.RotatedAt, now)
	}
	if got.RenewalDue(now.Add(time.Second)) {
		t.Fatal("session is due again right after rotation")
	}
	if _, ok := repo.sessions[got.TokenHash]; !ok {
		t.Fatalf("rotated session not persisted under the new hash; map keys: %d", len(repo.sessions))
	}
	if _, ok := repo.sessions[testSessionHash]; ok {
		t.Fatal("old hash still present in the store after rotation")
	}
}

func TestSessionServiceTouch_RotationRaceLostKeepsOldToken(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	seed := seededSession(now)
	seed.RotatedAt = now.Add(-domain.SessionRenewalInterval)
	repo := newTouchRepo(seed)
	repo.rotateWon = false
	geo := &countingGeo{cityByIP: map[string]string{testStoredIP: testStoredCity}}

	svc := newTouchService(repo, geo)
	got, rotatedToken, err := svc.Touch(context.Background(), repo.sessions[testSessionHash], testStoredIP, now)
	if err != nil {
		t.Fatalf("Touch error = %v", err)
	}
	if rotatedToken != "" {
		t.Fatalf("rotatedToken = %q, want empty (another request won the rotation)", rotatedToken)
	}
	if got.TokenHash != testSessionHash {
		t.Fatalf("TokenHash = %q, want the original hash kept", got.TokenHash)
	}
}

func TestSessionServiceTouch_RotationErrorPropagates(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	seed := seededSession(now)
	seed.RotatedAt = now.Add(-domain.SessionRenewalInterval)
	repo := newTouchRepo(seed)
	repo.rotateErr = errors.New("boom")
	geo := &countingGeo{cityByIP: map[string]string{testStoredIP: testStoredCity}}

	svc := newTouchService(repo, geo)
	if _, _, err := svc.Touch(context.Background(), repo.sessions[testSessionHash], testStoredIP, now); err == nil {
		t.Fatal("Touch error = nil, want the rotation failure to propagate")
	}
}

func TestSessionServiceIssue_CapturesDeviceOnce(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	repo := &touchRepo{sessions: map[string]domain.Session{}, rotateWon: true}
	parser := &fixedParser{}
	geo := &countingGeo{cityByIP: map[string]string{testNewIP: testNewCity}}

	factory := NewTxStoreFactory(
		newFakeUserRepo(), newFakeCodeRepo(), newFakeAttemptRepo(),
		repo, newFakeGrantRepo(), nil, &fakeUoW{beginner: &fakeBeginner{}},
	)
	svc := NewSessionService(factory, SessionServiceConfig{Hasher: fakeHasher{}, Parser: parser, Geo: geo})

	phone, err := domain.NewPhone("+79001234567")
	if err != nil {
		t.Fatalf("NewPhone: %v", err)
	}
	email, err := domain.NewEmail("user@example.com")
	if err != nil {
		t.Fatalf("NewEmail: %v", err)
	}

	rawIssued, _, _, issueErr := svc.Issue(context.Background(), &txStores{
		users:    newFakeUserRepo(),
		sessions: repo,
	}, phone, email, DeviceContext{UserAgent: testUserAgent, IP: testNewIP}, now)
	if issueErr != nil {
		t.Fatalf("Issue error = %v", issueErr)
	}
	if len(rawIssued.Token) != 64 {
		t.Fatalf("issued token length = %d, want 64 hex chars", len(rawIssued.Token))
	}

	t.Run("parses the User-Agent exactly once", func(t *testing.T) {
		t.Parallel()
		if len(parser.calls) != 1 || parser.calls[0] != testUserAgent {
			t.Fatalf("parser calls = %v, want exactly the login UA", parser.calls)
		}
	})

	t.Run("persists the captured device description", func(t *testing.T) {
		t.Parallel()
		if len(repo.sessions) != 1 {
			t.Fatalf("sessions persisted = %d, want 1", len(repo.sessions))
		}
		for _, s := range repo.sessions {
			assertIssuedDevice(t, s)
		}
	})
}

// assertIssuedDevice pins the device fields of the single issued session.
func assertIssuedDevice(t *testing.T, s domain.Session) {
	t.Helper()
	if s.UserAgent != testUserAgent {
		t.Fatalf("UserAgent = %q, want the raw login UA", s.UserAgent)
	}
	if s.DeviceType != domain.DevicePhone || s.Browser != "TestBrowser" || s.BrowserMajor != 121 || s.OS != "TestOS" {
		t.Fatalf("device fields = %+v, want the parsed description", s)
	}
	if s.LastIP != testNewIP {
		t.Fatalf("LastIP = %q, want %s", s.LastIP, testNewIP)
	}
	if s.City != testNewCity {
		t.Fatalf("City = %q, want %s", s.City, testNewCity)
	}
}
