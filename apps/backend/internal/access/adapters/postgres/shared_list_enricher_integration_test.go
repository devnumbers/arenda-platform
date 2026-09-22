package postgres

import (
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// Integration tests of the properties list reads' access projections (ticket
// #702): the member-name projection of the owner's cards and the suspended
// blur-card placeholders. Same TEST_DATABASE_URL convention as the rest of
// the access integration tests.

// addNamedUserWithEmail creates a fixture user with a display name and a
// verified email (the reason sheet's owner contact row).
func (f *accessLifecycleFixture) addNamedUserWithEmail(t *testing.T, name, surname, email string) uuid.UUID {
	t.Helper()
	id := f.addUserWithEmail(t, email)
	_, err := f.q.UpdateUser(f.bg(), genpostgres.UpdateUserParams{
		ID:              pgUUID(id),
		Name:            pgtype.Text{String: name, Valid: true},
		Surname:         pgtype.Text{String: surname, Valid: true},
		Patronymic:      pgtype.Text{},
		Email:           pgtype.Text{String: email, Valid: true},
		EmailVerifiedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		Timezone:        "UTC",
	})
	if err != nil {
		t.Fatalf("name user: %v", err)
	}
	return id
}

// addMember inserts a membership row with an explicit status.
func (f *accessLifecycleFixture) addMember(t *testing.T, propertyID, userID uuid.UUID, role, status string) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	_, err = f.q.CreatePropertyMemberWithStatus(f.bg(), genpostgres.CreatePropertyMemberWithStatusParams{
		ID:         pgUUID(id),
		PropertyID: pgUUID(propertyID),
		UserID:     pgUUID(userID),
		Role:       role,
		GrantedBy:  pgUUID(userID),
		Status:     status,
	})
	if err != nil {
		t.Fatalf("create member: %v", err)
	}
	return id
}

// backdateMember shifts a membership's created_at an hour back, so the
// membership-order assertion survives the equal tx timestamps.
func (f *accessLifecycleFixture) backdateMember(t *testing.T, id uuid.UUID) {
	t.Helper()
	const backdate = `UPDATE property_members SET created_at = now() - interval '1 hour' WHERE id = $1`
	if _, err := f.tx.Exec(f.bg(), backdate, pgUUID(id)); err != nil {
		t.Fatalf("backdate member: %v", err)
	}
}

// archiveProperty flips the property to archived (the enricher's predicate
// hides suspended legs behind the archive, issue #163).
func (f *accessLifecycleFixture) archiveProperty(t *testing.T, id uuid.UUID) {
	t.Helper()
	if _, err := f.tx.Exec(f.bg(), `UPDATE properties SET status = 'archived', pinned_at = NULL WHERE id = $1`, pgUUID(id)); err != nil {
		t.Fatalf("archive property: %v", err)
	}
}

// TestSharedListEnricher_SuspendedWith verifies the blur-cards (ticket
// #702): suspended memberships on non-archived properties only, FIFO order,
// each with the property's own card data (title and address — the card
// renders for real under the blur, Figma 2213-99113) and the owner's
// display name and account email.
func TestSharedListEnricher_SuspendedWith(t *testing.T) {
	t.Parallel()
	f := newAccessLifecycleFixture(t)

	recipient := createAccessTestUser(t, f.bg(), f.q)
	owner := f.addNamedUserWithEmail(t, "Максим", "Сергеевич", f.email("maksim"))

	propVisible := f.addProperty(t, owner, "Доступная")
	propArchived := f.addProperty(t, owner, "Архивная")

	if _, err := f.tx.Exec(f.bg(), `UPDATE properties SET address = 'Тестовый адрес, 1' WHERE id = $1`, pgUUID(propVisible)); err != nil {
		t.Fatalf("set address: %v", err)
	}

	first := f.addMember(t, propVisible, recipient, "viewer", "suspended")
	f.backdateMember(t, first)
	_ = f.addMember(t, propVisible, recipient, "full_access", "active")
	second := f.addMember(t, propArchived, recipient, "full_access", "suspended")
	f.backdateMember(t, second)
	f.archiveProperty(t, propArchived)

	enricher := NewSharedListEnricher(f.tx, f.userRepo)
	got, err := enricher.SuspendedWith(f.bg(), recipient)
	if err != nil {
		t.Fatalf("SuspendedWith: %v", err)
	}
	want := []propertiesapp.SharedSuspendedMembership{
		{
			PropertyID: propVisible,
			Role:       sharedpolicy.RoleViewer,
			Name:       "Доступная",
			Address:    "Тестовый адрес, 1",
			OwnerName:  "Максим Сергеевич",
			OwnerEmail: f.email("maksim"),
		},
	}
	if !slices.Equal(got, want) {
		t.Errorf("SuspendedWith = %v, want %v", got, want)
	}
}
