package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// recordingRecorder captures every Record call so a test can assert the audit
// entry was written with the right action and actor inside the transaction.
type recordingRecorder struct {
	auditapp.Noop
	entries []auditdomain.Entry
	err     error
}

func (r *recordingRecorder) Record(_ context.Context, entry auditdomain.Entry) error {
	if r.err != nil {
		return r.err
	}
	r.entries = append(r.entries, entry)
	return nil
}

func (r *recordingRecorder) WithTx(transaction.Tx) auditapp.Recorder { return r }

// fakeReminderRescheduler captures RescheduleForTimezoneChange calls so a test
// can prove the post-commit reschedule runs only on a timezone change.
type fakeReminderRescheduler struct {
	calls []rescheduleCall
	err   error
}

type rescheduleCall struct {
	userID      uuid.UUID
	oldTimezone string
	newTimezone string
}

func (r *fakeReminderRescheduler) RescheduleForTimezoneChange(_ context.Context, userID uuid.UUID, oldTimezone, newTimezone string) error {
	if r.err != nil {
		return r.err
	}
	r.calls = append(r.calls, rescheduleCall{userID: userID, oldTimezone: oldTimezone, newTimezone: newTimezone})
	return nil
}

// profileHarness wires a ProfileService to a fakeUoW + the shared identity
// fakes, returning every piece the tests need to assert behavior.
type profileHarness struct {
	*fakeStores
	svc         *ProfileService
	audit       *recordingRecorder
	rescheduler *fakeReminderRescheduler
}

func newProfileHarness(t *testing.T) *profileHarness {
	t.Helper()
	stores := newFakeStores()
	audit := &recordingRecorder{}
	rescheduler := &fakeReminderRescheduler{}
	svc := NewProfileService(
		stores.factory(audit),
		ProfileServiceConfig{
			ReminderRescheduler: rescheduler,
		},
	)
	return &profileHarness{fakeStores: stores, svc: svc, audit: audit, rescheduler: rescheduler}
}

// seedProfileUser creates a verified owner in the fake user repo and returns it.
func seedProfileUser(t *testing.T, users *fakeUserRepo) domain.User {
	t.Helper()
	phone := mustPhone(t, "+79160000001")
	user, err := domain.NewOwner(phone)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	user.Email = new(mustEmail(t, "owner@example.com"))
	user.EmailVerifiedAt = &testNow
	users.byPhone[phone.String()] = user
	return user
}

// TestProfileService_UpdateProfile_UpdatesPersonalData proves a name change is
// persisted and the transaction commits.
func TestProfileService_UpdateProfile_UpdatesPersonalData(t *testing.T) {
	t.Parallel()
	h := newProfileHarness(t)
	user := seedProfileUser(t, h.users)

	updated, err := h.svc.UpdateProfile(context.Background(), user.ID, UpdateProfileCommand{Name: new("Ivan")})
	if err != nil {
		t.Fatalf("UpdateProfile error = %v", err)
	}
	if updated.Name == nil || *updated.Name != "Ivan" {
		t.Fatalf("updated name = %v, want Ivan", updated.Name)
	}
	if h.beginner.committed != 1 {
		t.Errorf("committed = %d, want 1", h.beginner.committed)
	}
	if h.beginner.rolledBack != 0 {
		t.Errorf("rolledBack = %d, want 0", h.beginner.rolledBack)
	}
}

// TestProfileService_UpdateProfile_EmailChangeResetsVerified proves changing
// the email clears EmailVerifiedAt, while an unchanged email leaves it intact.
func TestProfileService_UpdateProfile_EmailChangeResetsVerified(t *testing.T) {
	t.Parallel()
	h := newProfileHarness(t)
	user := seedProfileUser(t, h.users)

	t.Run("new email clears verified", func(t *testing.T) {
		t.Parallel()
		updated, err := h.svc.UpdateProfile(context.Background(), user.ID, UpdateProfileCommand{Email: new("new@example.com")})
		if err != nil {
			t.Fatalf("UpdateProfile error = %v", err)
		}
		if updated.EmailVerifiedAt != nil {
			t.Fatalf("EmailVerifiedAt = %v, want nil after email change", updated.EmailVerifiedAt)
		}
		if updated.Email == nil || updated.Email.String() != "new@example.com" {
			t.Fatalf("updated email = %v, want new@example.com", updated.Email)
		}
	})

	t.Run("same email keeps verified", func(t *testing.T) {
		t.Parallel()
		// Re-seed: the first subtest already mutated the shared user.
		h := newProfileHarness(t)
		user := seedProfileUser(t, h.users)

		updated, err := h.svc.UpdateProfile(context.Background(), user.ID, UpdateProfileCommand{Email: new("owner@example.com")})
		if err != nil {
			t.Fatalf("UpdateProfile error = %v", err)
		}
		if updated.EmailVerifiedAt == nil {
			t.Fatal("EmailVerifiedAt = nil, want preserved when email is unchanged")
		}
	})
}

// TestProfileService_UpdateProfile_TimezoneChangeReschedules proves a timezone
// change triggers the post-commit reminder reschedule, and an unchanged
// timezone does not.
func TestProfileService_UpdateProfile_TimezoneChangeReschedules(t *testing.T) {
	t.Parallel()
	t.Run("changed timezone reschedules post-commit", func(t *testing.T) {
		t.Parallel()
		h := newProfileHarness(t)
		user := seedProfileUser(t, h.users)

		updated, err := h.svc.UpdateProfile(context.Background(), user.ID, UpdateProfileCommand{Timezone: new("Europe/Moscow")})
		if err != nil {
			t.Fatalf("UpdateProfile error = %v", err)
		}
		if len(h.rescheduler.calls) != 1 {
			t.Fatalf("reschedule calls = %d, want 1", len(h.rescheduler.calls))
		}
		call := h.rescheduler.calls[0]
		if call.userID != user.ID {
			t.Errorf("reschedule userID = %s, want %s", call.userID, user.ID)
		}
		if call.oldTimezone != "" {
			t.Errorf("reschedule oldTimezone = %q, want empty (uninitialized user)", call.oldTimezone)
		}
		if call.newTimezone != "Europe/Moscow" {
			t.Errorf("reschedule newTimezone = %q, want Europe/Moscow", call.newTimezone)
		}
		if updated.Timezone.String() != "Europe/Moscow" {
			t.Fatalf("updated timezone = %q, want Europe/Moscow", updated.Timezone.String())
		}
		// The profile mutation is committed before the reschedule runs.
		if h.beginner.committed != 1 {
			t.Errorf("committed = %d, want 1", h.beginner.committed)
		}
	})

	t.Run("no timezone command does not reschedule", func(t *testing.T) {
		t.Parallel()
		h := newProfileHarness(t)
		user := seedProfileUser(t, h.users)

		if _, err := h.svc.UpdateProfile(context.Background(), user.ID, UpdateProfileCommand{Name: new("Ivan")}); err != nil {
			t.Fatalf("UpdateProfile error = %v", err)
		}
		if len(h.rescheduler.calls) != 0 {
			t.Fatalf("reschedule calls = %d, want 0 without a timezone change", len(h.rescheduler.calls))
		}
	})
}

// TestProfileService_UpdateProfile_RecordsAuditInTx proves the audit entry is
// recorded with the profile-updated action for the acting user, and the
// transaction commits — so audit shares the mutation's transaction (ADR 0020).
func TestProfileService_UpdateProfile_RecordsAuditInTx(t *testing.T) {
	t.Parallel()
	h := newProfileHarness(t)
	user := seedProfileUser(t, h.users)

	if _, err := h.svc.UpdateProfile(context.Background(), user.ID,
		UpdateProfileCommand{Name: new("Ivan"), Surname: new("Petrov")}); err != nil {
		t.Fatalf("UpdateProfile error = %v", err)
	}
	if len(h.audit.entries) != 1 {
		t.Fatalf("audit entries = %d, want 1", len(h.audit.entries))
	}
	entry := h.audit.entries[0]
	if entry.Action != auditdomain.ActionProfileUpdated {
		t.Errorf("audit action = %q, want %q", entry.Action, auditdomain.ActionProfileUpdated)
	}
	if entry.EntityType != auditdomain.EntityUser {
		t.Errorf("audit entityType = %q, want %q", entry.EntityType, auditdomain.EntityUser)
	}
	if entry.ActorID == nil || *entry.ActorID != user.ID {
		t.Errorf("audit actorID = %v, want %s", entry.ActorID, user.ID)
	}
	if entry.EntityID == nil || *entry.EntityID != user.ID {
		t.Errorf("audit entityID = %v, want %s", entry.EntityID, user.ID)
	}
	if h.beginner.committed != 1 {
		t.Errorf("committed = %d, want 1 (audit in-tx, not early-committed)", h.beginner.committed)
	}
}

// TestProfileService_UpdateProfile_RollsBackOnGetError proves a failure to load
// the user aborts the transaction and returns the error.
func TestProfileService_UpdateProfile_RollsBackOnGetError(t *testing.T) {
	t.Parallel()
	h := newProfileHarness(t)
	// No seeded user → GetByIDForUpdate returns ErrNotFound.

	_, err := h.svc.UpdateProfile(context.Background(), uuid.Must(uuid.NewV7()), UpdateProfileCommand{Name: new("Ivan")})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateProfile error = %v, want wrap of ErrNotFound", err)
	}
	if h.beginner.committed != 0 {
		t.Errorf("committed = %d, want 0 on error", h.beginner.committed)
	}
	if h.beginner.rolledBack != 1 {
		t.Errorf("rolledBack = %d, want 1 on error", h.beginner.rolledBack)
	}
	if len(h.audit.entries) != 0 {
		t.Errorf("audit entries = %d, want 0 on rollback", len(h.audit.entries))
	}
}

// TestProfileService_UpdateProfile_RollsBackOnAuditError proves a recorder
// failure rolls the transaction back and returns the wrapped error, so audit
// failure cannot leave a committed mutation without its audit entry.
func TestProfileService_UpdateProfile_RollsBackOnAuditError(t *testing.T) {
	t.Parallel()
	h := newProfileHarness(t)
	user := seedProfileUser(t, h.users)
	auditErr := errors.New("audit writer down")
	h.audit.err = auditErr

	_, err := h.svc.UpdateProfile(context.Background(), user.ID, UpdateProfileCommand{Name: new("Ivan")})
	if !errors.Is(err, auditErr) {
		t.Fatalf("UpdateProfile error = %v, want wrap of %v", err, auditErr)
	}
	if h.beginner.committed != 0 {
		t.Errorf("committed = %d, want 0 on audit error", h.beginner.committed)
	}
	if h.beginner.rolledBack != 1 {
		t.Errorf("rolledBack = %d, want 1 on audit error", h.beginner.rolledBack)
	}
}

// TestProfileService_UpdateProfile_RescheduleErrorAfterCommit proves a
// post-commit reschedule failure is returned even though the profile mutation
// was committed. This documents the contract: the reschedule is best-effort
// synchronous and its error is surfaced to the caller.
func TestProfileService_UpdateProfile_RescheduleErrorAfterCommit(t *testing.T) {
	t.Parallel()
	h := newProfileHarness(t)
	user := seedProfileUser(t, h.users)
	rescheduleErr := errors.New("notifications unavailable")
	h.rescheduler.err = rescheduleErr

	_, err := h.svc.UpdateProfile(context.Background(), user.ID, UpdateProfileCommand{Timezone: new("Europe/Moscow")})
	if !errors.Is(err, rescheduleErr) {
		t.Fatalf("UpdateProfile error = %v, want wrap of %v", err, rescheduleErr)
	}
	// The identity transaction committed before the reschedule was attempted.
	if h.beginner.committed != 1 {
		t.Errorf("committed = %d, want 1 (mutation commits before reschedule)", h.beginner.committed)
	}
}

// TestProfileService_UsesRunInTx proves UpdateProfile goes through the UoW seam:
// the user repository is bound to the transaction exactly once and the UoW
// commits. This is the core ADR 0033 assertion for the Profile migration.
func TestProfileService_UsesRunInTx(t *testing.T) {
	t.Parallel()
	users := &countingUserRepo{fakeUserRepo: newFakeUserRepo()}
	codes := newFakeCodeRepo()
	attempts := newFakeAttemptRepo()
	sessions := newFakeSessionRepo()
	beginner := &fakeBeginner{}
	factory := NewTxStoreFactory(users, codes, attempts, sessions, &recordingRecorder{}, &fakeUoW{beginner: beginner})
	svc := NewProfileService(
		factory,
		ProfileServiceConfig{
			ReminderRescheduler: &fakeReminderRescheduler{},
		},
	)
	seedProfileUser(t, users.fakeUserRepo)

	if _, err := svc.UpdateProfile(context.Background(),
		users.fakeUserRepo.byPhone["+79160000001"].ID, UpdateProfileCommand{Name: new("Ivan")}); err != nil {
		t.Fatalf("UpdateProfile error = %v", err)
	}
	if users.withTxCalls != 1 {
		t.Errorf("users.WithTx calls = %d, want 1 (must bind inside runInTx)", users.withTxCalls)
	}
	if beginner.committed != 1 {
		t.Errorf("committed = %d, want 1", beginner.committed)
	}
}

// TestProfileService_Me_ReadsOutsideTx proves Me is a plain read that does not
// open a transaction (ADR 0033: reads stay outside runInTx).
func TestProfileService_Me_ReadsOutsideTx(t *testing.T) {
	t.Parallel()
	h := newProfileHarness(t)
	user := seedProfileUser(t, h.users)

	got, err := h.svc.Me(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("Me error = %v", err)
	}
	if got.ID != user.ID {
		t.Fatalf("Me user ID = %s, want %s", got.ID, user.ID)
	}
	if h.beginner.begun != 0 {
		t.Errorf("tx begun = %d, want 0 for a read", h.beginner.begun)
	}
}
