package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	identitypg "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/encryption"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// The fixture of the property lifecycle integration tests (issue #163): the
// real AccessService / InvitationService / SlotCoordinator over the real
// repositories bound to the test transaction, with in-memory fakes for the
// tariff limiter and the own-properties port. The lifecycle events (#751) are
// not subscribed here — these tests assert the slot and membership state, no
// event publication is part of them; a nil publisher keeps them silent.

// lifecycleNoCommitTx wraps the test's pgx.Tx as a transaction.Tx whose
// commit/rollback are no-ops (the outer test transaction is rolled back in
// cleanup). The embedded pgx.Tx still satisfies postgres.DBTX, so real
// repositories bind to it via WithTx. Same pattern as the scheduler reminder
// integration tests.
type lifecycleNoCommitTx struct{ pgx.Tx }

func (lifecycleNoCommitTx) Commit(context.Context) error   { return nil }
func (lifecycleNoCommitTx) Rollback(context.Context) error { return nil }

// lifecycleBeginner always returns the test's already-open transaction.
type lifecycleBeginner struct{ tx pgx.Tx }

func (b lifecycleBeginner) Begin(context.Context) (transaction.Tx, error) {
	return lifecycleNoCommitTx{b.tx}, nil
}

// lifecycleUoW adapts lifecycleBeginner to the transaction.UoW
// port so the services under test open their transactions through runInTx
// (ADR 0033); the no-commit tx keeps the outer test transaction in charge of
// cleanup.
type lifecycleUoW struct{ beginner lifecycleBeginner }

func (u lifecycleUoW) Do(ctx context.Context, work func(tx transaction.Tx) error) (err error) {
	tx, err := u.beginner.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		// Same defer shape as the production UoW (rollback, then recover and
		// re-panic). The wrapped lifecycleNoCommitTx rollback is a
		// no-op (the outer test transaction owns cleanup), so the fold never
		// fires.
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

// lifecycleFakeLimiter is an in-memory access RecipientLimiter.
type lifecycleFakeLimiter struct {
	limits map[uuid.UUID]int
}

func (f *lifecycleFakeLimiter) set(recipientID uuid.UUID, limit int) {
	f.limits[recipientID] = limit
}

func (f *lifecycleFakeLimiter) ActivePropertyLimit(_ context.Context, recipientID uuid.UUID) (int, error) {
	return f.limits[recipientID], nil
}

func (f *lifecycleFakeLimiter) WithTx(_ transaction.Tx) (accessapp.RecipientLimiter, error) {
	return f, nil
}

// lifecycleNoOwnedProps reports no own active properties.
type lifecycleNoOwnedProps struct{}

func (lifecycleNoOwnedProps) ListActiveWithMeta(context.Context, uuid.UUID) ([]accessapp.OwnedPropertyMeta, error) {
	return nil, nil
}

func (lifecycleNoOwnedProps) WithTx(_ transaction.Tx) (accessapp.OwnedActivePropertiesPort, error) {
	return lifecycleNoOwnedProps{}, nil
}

var (
	_ accessapp.RecipientLimiter          = (*lifecycleFakeLimiter)(nil)
	_ accessapp.OwnedActivePropertiesPort = lifecycleNoOwnedProps{}
)

// propertyLifecycleFixture bundles the real access services wired to the test
// transaction.
type propertyLifecycleFixture struct {
	tx      pgx.Tx
	q       *genpostgres.Queries
	limiter *lifecycleFakeLimiter
	nonce   string
	access  *accessapp.AccessService
	invites *accessapp.InvitationService
	slots   *accessapp.SlotCoordinator
}

// bg returns a background context (kept out of the struct per containedctx).
func (f *propertyLifecycleFixture) bg() context.Context { return context.Background() }

// email derives a fixture-unique address for the given local part: the tests
// run in parallel against one database, and users_email_lowercase_unique would
// otherwise make their uncommitted inserts block on each other's rollback
// transactions. Invitation-only addresses need no nonce — invitations are
// unique per (property_id, email) and every fixture uses fresh property ids.
func (f *propertyLifecycleFixture) email(local string) string {
	return local + "." + f.nonce + "@example.com"
}

func newLifecycleMailFixture(t *testing.T) *propertyLifecycleFixture {
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
	policy := accessapp.NewMembershipPolicy(ownerResolver, memberRepo)

	limiter := &lifecycleFakeLimiter{limits: map[uuid.UUID]int{}}
	beginner := lifecycleBeginner{tx: tx}

	slots := accessapp.NewSlotCoordinator(memberRepo, ownerResolver, limiter,
		lifecycleNoOwnedProps{}, nil, auditapp.Noop{}, beginner, nil, nil)
	factory := accessapp.NewTxStoreFactory(memberRepo, invitationRepo, auditapp.Noop{}, lifecycleUoW{beginner})
	access := accessapp.NewAccessService(memberRepo, ownerResolver, ownerResolver, userLookup, policy, slots, nil, factory, nil)
	invites := accessapp.NewInvitationService(access, memberRepo, invitationRepo, ownerResolver, ownerResolver,
		userLookup, policy, slots, nil, nil, ownerResolver, factory, nil, nil)

	return &propertyLifecycleFixture{
		tx: tx, q: genpostgres.New(tx),
		limiter: limiter, nonce: nonce.String(),
		access: access, invites: invites, slots: slots,
	}
}

// addUserWithEmail creates a user and sets their email.
func (f *propertyLifecycleFixture) addUserWithEmail(t *testing.T, email string) uuid.UUID {
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
func (f *propertyLifecycleFixture) addProperty(t *testing.T, owner uuid.UUID, name string) uuid.UUID {
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
