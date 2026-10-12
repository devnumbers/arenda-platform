package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
	identitypg "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/encryption"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// Shared fixture of the access integration tests that run the real
// AccessService / InvitationService / SlotCoordinator over the real
// repositories on a test-transaction Postgres (issue #158 T4, #695: the
// lifecycle emails are cut, so no mailer is wired here).

// accessNoCommitTx wraps the test's pgx.Tx as a transaction.Tx whose
// commit/rollback are no-ops (the outer test transaction is rolled back in
// cleanup). The embedded pgx.Tx still satisfies postgres.DBTX, so real
// repositories bind to it via WithTx. Same pattern as the scheduler reminder
// integration tests.
type accessNoCommitTx struct{ pgx.Tx }

func (accessNoCommitTx) Commit(context.Context) error   { return nil }
func (accessNoCommitTx) Rollback(context.Context) error { return nil }

// accessBeginner always returns the test's already-open transaction.
type accessBeginner struct{ tx pgx.Tx }

func (b accessBeginner) Begin(context.Context) (transaction.Tx, error) {
	return accessNoCommitTx{b.tx}, nil
}

// accessUoW adapts accessBeginner to the transaction.UoW port so the
// services under test open their transactions through runInTx (ADR 0033); the
// no-commit tx keeps the outer test transaction in charge of cleanup.
type accessUoW struct{ beginner accessBeginner }

func (u accessUoW) Do(ctx context.Context, work func(tx transaction.Tx) error) (err error) {
	tx, err := u.beginner.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		// Same defer shape as the production UoW (rollback, then recover and
		// re-panic). Unlike prod's `_ =` discard, the rollback error is folded
		// into the named return — and only when work succeeded, so it never
		// masks the work error or the re-panicked value. The wrapped
		// accessNoCommitTx rollback is a no-op (the outer test transaction
		// owns cleanup), so the fold never fires.
		rollbackErr := tx.Rollback(ctx)
		if r := recover(); r != nil {
			panic(r)
		}
		if rollbackErr != nil && err == nil {
			err = rollbackErr
		}
	}()
	if err := work(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// accessFakeLimiter is an in-memory recipient limit map.
type accessFakeLimiter struct {
	limits map[uuid.UUID]int
}

func (f *accessFakeLimiter) set(recipientID uuid.UUID, limit int) {
	f.limits[recipientID] = limit
}

func (f *accessFakeLimiter) ActivePropertyLimit(_ context.Context, recipientID uuid.UUID) (int, error) {
	return f.limits[recipientID], nil
}

func (f *accessFakeLimiter) WithTx(_ transaction.Tx) (accessapp.RecipientLimiter, error) {
	return f, nil
}

// accessOwnedProps is an in-memory OwnedActivePropertiesPort: each owner maps
// to their own active properties with the UpdatedAt the eviction comparator
// reads. Seeded empty by the fixture — the pre-D13 tests keep seeing no own
// properties; the D13 scenarios (issue #1297) add the recipient's own objects
// as the coordinator's own-pool view of the transaction state.
type accessOwnedProps struct {
	byOwner map[uuid.UUID][]accessapp.OwnedPropertyMeta
}

func (f *accessOwnedProps) add(ownerID, propertyID uuid.UUID, updatedAt time.Time) {
	f.byOwner[ownerID] = append(f.byOwner[ownerID], accessapp.OwnedPropertyMeta{ID: propertyID, UpdatedAt: updatedAt})
}

func (f *accessOwnedProps) drop(ownerID, propertyID uuid.UUID) {
	kept := f.byOwner[ownerID][:0]
	for _, m := range f.byOwner[ownerID] {
		if m.ID != propertyID {
			kept = append(kept, m)
		}
	}
	f.byOwner[ownerID] = kept
}

func (f *accessOwnedProps) ListActiveWithMeta(_ context.Context, ownerID uuid.UUID) ([]accessapp.OwnedPropertyMeta, error) {
	out := make([]accessapp.OwnedPropertyMeta, len(f.byOwner[ownerID]))
	copy(out, f.byOwner[ownerID])
	return out, nil
}

func (f *accessOwnedProps) WithTx(_ transaction.Tx) (accessapp.OwnedActivePropertiesPort, error) {
	return f, nil
}

var _ accessapp.OwnedActivePropertiesPort = (*accessOwnedProps)(nil)

// accessLifecycleFixture bundles the real access services wired to the test
// transaction.
type accessLifecycleFixture struct {
	tx         pgx.Tx
	q          *genpostgres.Queries
	limiter    *accessFakeLimiter
	ownedProps *accessOwnedProps
	nonce      string
	access     *accessapp.AccessService
	invites    *accessapp.InvitationService
	slots      *accessapp.SlotCoordinator
	mutations  *accessapp.ParticipantMutationService
	userRepo   *identitypg.UserRepository
}

// bg returns a background context (kept out of the struct per containedctx).
func (f *accessLifecycleFixture) bg() context.Context { return context.Background() }

// email derives a fixture-unique address for the given local part: the tests
// run in parallel against one database, and users_email_lowercase_unique would
// otherwise make their uncommitted inserts block on each other's rollback
// transactions.
func (f *accessLifecycleFixture) email(local string) string {
	return local + "." + f.nonce + "@example.com"
}

func newAccessLifecycleFixture(t *testing.T) *accessLifecycleFixture {
	t.Helper()
	pool := setupAccessDB(t)
	_, tx, cleanup := beginAccessTx(t, pool)
	t.Cleanup(cleanup)

	nonce, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	encryptor, err := encryption.NewEncryptor("")
	if err != nil {
		t.Fatalf("noop encryptor: %v", err)
	}

	memberRepo := NewMembershipRepository(tx)
	invitationRepo := NewInvitationRepository(tx)
	ownerResolver := NewOwnerResolver(tx)
	userRepo := identitypg.NewUserRepository(tx, encryptor)
	userLookup := NewUserLookup(userRepo)
	emailResolver := NewUserEmailResolver(userRepo)
	policy := accessapp.NewMembershipPolicy(ownerResolver, memberRepo)

	limiter := &accessFakeLimiter{limits: map[uuid.UUID]int{}}
	ownedProps := &accessOwnedProps{byOwner: map[uuid.UUID][]accessapp.OwnedPropertyMeta{}}
	beginner := accessBeginner{tx: tx}

	slots := accessapp.NewSlotCoordinator(memberRepo, ownerResolver, limiter,
		ownedProps, nil, auditapp.Noop{}, beginner, nil, nil)
	factory := accessapp.NewTxStoreFactory(memberRepo, invitationRepo, auditapp.Noop{}, historyapp.Noop{}, accessUoW{beginner})
	access := accessapp.NewAccessService(memberRepo, ownerResolver, ownerResolver, userLookup, policy, slots, nil, factory, nil)
	invites := accessapp.NewInvitationService(access, memberRepo, invitationRepo, ownerResolver, ownerResolver,
		userLookup, policy, slots, nil, nil, ownerResolver, factory, nil, nil, emailResolver)
	// The mutation side with the real SQL revoke-scope gate (issue #1310);
	// no mailer/events — the integration tests assert state, not publications.
	mutations := accessapp.NewParticipantMutationService(access, ownerResolver, ownerResolver, userLookup,
		emailResolver, policy, NewRevokeAllScope(tx), slots, nil, nil, ownerResolver, factory, nil, nil)

	return &accessLifecycleFixture{
		tx: tx, q: genpostgres.New(tx),
		limiter: limiter, ownedProps: ownedProps, nonce: nonce.String(),
		access: access, invites: invites, slots: slots, mutations: mutations,
		userRepo: userRepo,
	}
}

// addUserWithEmail creates a user and sets their email.
func (f *accessLifecycleFixture) addUserWithEmail(t *testing.T, email string) uuid.UUID {
	t.Helper()
	id := createAccessTestUser(t, f.bg(), f.q)
	_, err := f.q.UpdateUserEmailVerified(f.bg(), genpostgres.UpdateUserEmailVerifiedParams{
		ID:    pgUUID(id),
		Email: pgtype.Text{String: email, Valid: true},
	})
	if err != nil {
		t.Fatalf("set user email: %v", err)
	}
	return id
}

// addProperty creates an active property with the given display name.
func (f *accessLifecycleFixture) addProperty(t *testing.T, owner uuid.UUID, name string) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	_, err = f.q.CreateProperty(f.bg(), genpostgres.CreatePropertyParams{
		ID:          pgUUID(id),
		OwnerID:     pgUUID(owner),
		Name:        name,
		Type:        propertyTypeApartment,
		Address:     "",
		Description: pgtype.Text{},
		Attributes:  []byte("{}"),
		Status:      statusActive,
	})
	if err != nil {
		t.Fatalf("create property: %v", err)
	}
	return id
}
