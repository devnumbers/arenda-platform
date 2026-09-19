package application

import (
	"context"

	"github.com/google/uuid"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// Имя и адрес стандартной фикстуры объекта, повторённые тест-файлами пакета.
const (
	testPropertyName    = "Test"
	testPropertyAddress = "Addr"
	// Короткое имя объекта access-тестов (деталь и исходы доступа).
	testObjName = "Obj"
)

// newPropertyTestFactory builds the txStoreFactory for the property service
// tests: the same collaborators the service receives for its transactional
// use cases, over the fakeUoW backed by fakePropertyTxBeginner so the
// in-memory repositories see the *fakePropertyTx their WithTx expects. A nil
// limiter matches the pre-factory fixtures: runInTx skips unwired optional
// stores, and the factory defaults a nil audit to Noop (no property service
// test asserts audit entries).
func newPropertyTestFactory(
	repo PropertyRepository,
	photos PropertyPhotoRepository,
	limiter SubscriptionLimiter,
) txStoreFactory {
	return NewTxStoreFactory(repo, photos, limiter, nil, fakeUoW{beginner: fakePropertyTxBeginner{}})
}

// testOwnerPolicy is the policy used by property service tests that pre-date
// the membership-aware policy. It treats the actor as the owner of every
// property (RoleOwner) so tests exercising owner-scoped behaviour keep working
// without wiring a real policy. It is intentionally test-only: production
// always injects the membership-aware policy from the access context.
type testOwnerPolicy struct{}

func (testOwnerPolicy) Role(_ context.Context, actor, scope uuid.UUID) (sharedpolicy.Role, error) {
	if actor == scope {
		return sharedpolicy.RoleOwner, nil
	}
	return sharedpolicy.RoleNone, nil
}

func (testOwnerPolicy) RoleForProperty(_ context.Context, _, _ uuid.UUID) (sharedpolicy.Role, error) {
	return sharedpolicy.RoleOwner, nil
}

// staticRolePolicy returns a fixed role for every RoleForProperty lookup. It
// backs privacy/outcome tests of the property service: how a given policy
// outcome (owner, member, none, suspended) maps to service errors (T9).
type staticRolePolicy struct {
	role sharedpolicy.Role
}

func (p staticRolePolicy) Role(_ context.Context, actor, scope uuid.UUID) (sharedpolicy.Role, error) {
	if actor == scope {
		return sharedpolicy.RoleOwner, nil
	}
	return sharedpolicy.RoleNone, nil
}

func (p staticRolePolicy) RoleForProperty(_ context.Context, _, _ uuid.UUID) (sharedpolicy.Role, error) {
	return p.role, nil
}

// fakeSharedMemberships returns a fixed set of active shared-access
// memberships (issue T11).
type fakeSharedMemberships struct {
	memberships []SharedMembership
}

func (f fakeSharedMemberships) MembershipsWith(_ context.Context, _ uuid.UUID) ([]SharedMembership, error) {
	return f.memberships, nil
}

// fakeOwnerNames resolves fixed owner display names, or fails when err is set
// (issue T11).
type fakeOwnerNames struct {
	names map[uuid.UUID]string
	err   error
}

func (f fakeOwnerNames) DisplayName(_ context.Context, userID uuid.UUID) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.names[userID], nil
}

// fakeOwnerEmails resolves fixed owner account emails, or fails when err is
// set (Figma 2200-97365, the detail's owner contact row).
type fakeOwnerEmails struct {
	emails map[uuid.UUID]string
	err    error
}

func (f fakeOwnerEmails) GetEmail(_ context.Context, userID uuid.UUID) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.emails[userID], nil
}

var (
	_ SharedMemberships        = fakeSharedMemberships{}
	_ OwnerDisplayNameResolver = fakeOwnerNames{}
	_ OwnerEmailResolver       = fakeOwnerEmails{}
)

// fakeSuspendedShared serves the SuspendedSharedMemberships port with a
// canned placeholder list (ticket #702).
type fakeSuspendedShared struct {
	items []SharedSuspendedMembership
}

func (f fakeSuspendedShared) SuspendedWith(context.Context, uuid.UUID) ([]SharedSuspendedMembership, error) {
	return f.items, nil
}

var _ SuspendedSharedMemberships = fakeSuspendedShared{}
