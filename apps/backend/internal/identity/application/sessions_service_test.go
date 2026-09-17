package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

// sessionsHarness wires a SessionsService over the shared identity fakes.
type sessionsHarness struct {
	repo    *fakeSessionRepo
	audit   *recordingRecorder
	beginer *fakeBeginner
	svc     *SessionsService
}

func newSessionsHarness() *sessionsHarness {
	repo := newFakeSessionRepo()
	audit := &recordingRecorder{}
	beginner := &fakeBeginner{}
	factory := NewTxStoreFactory(
		newFakeUserRepo(), newFakeCodeRepo(), newFakeAttemptRepo(), repo,
		newFakeGrantRepo(), audit, &fakeUoW{beginner: beginner},
	)
	return &sessionsHarness{
		repo:    repo,
		audit:   audit,
		beginer: beginner,
		svc:     NewSessionsService(factory, SessionsServiceConfig{Hasher: fakeHasher{}}),
	}
}

// seedSession stores a session for userID and returns its id.
func (h *sessionsHarness) seedSession(userID uuid.UUID, hash string) uuid.UUID {
	id := uuid.Must(uuid.NewV7())
	h.repo.sessions[hash] = domain.Session{ID: id, UserID: userID, TokenHash: hash}
	return id
}

func TestSessionsService_List_MarksCurrentSession(t *testing.T) {
	t.Parallel()
	h := newSessionsHarness()
	userID := uuid.Must(uuid.NewV7())
	other := uuid.Must(uuid.NewV7())

	currentID := h.seedSession(userID, fakeHasher{}.HashToken("current-token"))
	h.seedSession(userID, fakeHasher{}.HashToken("other-token"))
	h.seedSession(other, fakeHasher{}.HashToken("someone-else"))

	sessions, current, err := h.svc.List(context.Background(), userID, "current-token")
	if err != nil {
		t.Fatalf("List error = %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("sessions = %d, want 2 (only the caller's own)", len(sessions))
	}
	if current != currentID {
		t.Fatalf("current = %s, want %s", current, currentID)
	}
}

func TestSessionsService_Revoke_RemovesOwnForeignSession(t *testing.T) {
	t.Parallel()
	h := newSessionsHarness()
	userID := uuid.Must(uuid.NewV7())
	targetID := h.seedSession(userID, "target-hash")
	h.seedSession(userID, fakeHasher{}.HashToken("current-token"))
	actor := auditdomain.Actor{ID: userID, Role: auditdomain.ActorRoleOwner}

	err := h.svc.Revoke(context.Background(), userID, targetID, "current-token", actor)
	if err != nil {
		t.Fatalf("Revoke error = %v", err)
	}
	if _, ok := h.repo.sessions["target-hash"]; ok {
		t.Fatal("target session still stored after revoke")
	}
	if _, ok := h.repo.sessions[fakeHasher{}.HashToken("current-token")]; !ok {
		t.Fatal("current session was removed by a single-session revoke")
	}
	if len(h.audit.entries) != 1 {
		t.Fatalf("audit entries = %d, want 1", len(h.audit.entries))
	}
	entry := h.audit.entries[0]
	if entry.Action != auditdomain.ActionAuthSessionRevoked {
		t.Errorf("audit action = %s, want %s", entry.Action, auditdomain.ActionAuthSessionRevoked)
	}
	if entry.EntityID == nil || *entry.EntityID != userID {
		t.Errorf("audit entity = %v, want the acting user", entry.EntityID)
	}
	if entry.Context["session_id"] != targetID.String() {
		t.Errorf("audit context session_id = %v, want %s", entry.Context["session_id"], targetID)
	}
	if h.beginer.committed != 1 {
		t.Errorf("committed = %d, want 1 (audit and delete share one tx)", h.beginer.committed)
	}
}

func TestSessionsService_Revoke_CurrentSessionRejected(t *testing.T) {
	t.Parallel()
	h := newSessionsHarness()
	userID := uuid.Must(uuid.NewV7())
	currentID := h.seedSession(userID, fakeHasher{}.HashToken("current-token"))
	actor := auditdomain.Actor{ID: userID, Role: auditdomain.ActorRoleOwner}

	err := h.svc.Revoke(context.Background(), userID, currentID, "current-token", actor)
	if !errors.Is(err, ErrCurrentSession) {
		t.Fatalf("Revoke error = %v, want ErrCurrentSession", err)
	}
	if _, ok := h.repo.sessions[fakeHasher{}.HashToken("current-token")]; !ok {
		t.Fatal("current session was removed")
	}
	if len(h.audit.entries) != 0 {
		t.Fatalf("audit entries = %d, want 0 on rejection", len(h.audit.entries))
	}
}

func TestSessionsService_Revoke_UnknownOrForeignSessionNotFound(t *testing.T) {
	t.Parallel()
	h := newSessionsHarness()
	userID := uuid.Must(uuid.NewV7())
	foreignUserID := uuid.Must(uuid.NewV7())
	foreignID := h.seedSession(foreignUserID, "foreign-hash")
	actor := auditdomain.Actor{ID: userID, Role: auditdomain.ActorRoleOwner}

	if err := h.svc.Revoke(context.Background(), userID, uuid.Must(uuid.NewV7()), "current-token", actor); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Revoke(unknown) error = %v, want ErrNotFound", err)
	}
	if err := h.svc.Revoke(context.Background(), userID, foreignID, "current-token", actor); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Revoke(foreign) error = %v, want ErrNotFound (ownership hidden)", err)
	}
	if _, ok := h.repo.sessions["foreign-hash"]; !ok {
		t.Fatal("foreign session was removed")
	}
	if len(h.audit.entries) != 0 {
		t.Fatalf("audit entries = %d, want 0 on not-found", len(h.audit.entries))
	}
}

func TestSessionsService_RevokeOthers_KeepsCurrentAndCounts(t *testing.T) {
	t.Parallel()
	h := newSessionsHarness()
	userID := uuid.Must(uuid.NewV7())
	h.seedSession(userID, fakeHasher{}.HashToken("current-token"))
	h.seedSession(userID, "other-1")
	h.seedSession(userID, "other-2")
	actor := auditdomain.Actor{ID: userID, Role: auditdomain.ActorRoleOwner}

	removed, err := h.svc.RevokeOthers(context.Background(), userID, "current-token", actor)
	if err != nil {
		t.Fatalf("RevokeOthers error = %v", err)
	}
	if removed != 2 {
		t.Fatalf("removed = %d, want 2", removed)
	}
	if _, ok := h.repo.sessions[fakeHasher{}.HashToken("current-token")]; !ok {
		t.Fatal("current session was removed by revoke-others")
	}
	if len(h.audit.entries) != 1 {
		t.Fatalf("audit entries = %d, want 1", len(h.audit.entries))
	}
	if h.audit.entries[0].Action != auditdomain.ActionAuthOtherSessionsRevoked {
		t.Errorf("audit action = %s, want %s", h.audit.entries[0].Action, auditdomain.ActionAuthOtherSessionsRevoked)
	}
}

func TestSessionsService_Revoke_AuditFailureRollsBack(t *testing.T) {
	t.Parallel()
	h := newSessionsHarness()
	h.audit.err = errors.New("audit down")
	userID := uuid.Must(uuid.NewV7())
	targetID := h.seedSession(userID, "target-hash")
	actor := auditdomain.Actor{ID: userID, Role: auditdomain.ActorRoleOwner}

	err := h.svc.Revoke(context.Background(), userID, targetID, "current-token", actor)
	if err == nil {
		t.Fatal("Revoke error = nil, want audit failure to propagate (fail-safe default)")
	}
	if h.beginer.committed != 0 {
		t.Errorf("committed = %d, want 0 — revocation must not commit without its audit entry", h.beginer.committed)
	}
	if h.beginer.rolledBack == 0 {
		t.Errorf("rolledBack = %d, want at least 1", h.beginer.rolledBack)
	}
}
