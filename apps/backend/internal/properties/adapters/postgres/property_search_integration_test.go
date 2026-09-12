package postgres

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// The GET /properties/search store contract (ticket #601): the keyset walk
// covers every match exactly once at any page size, the match runs over the
// name and the address with the ILIKE metacharacters inert, and the visible
// slice mirrors the main list's — own plus actively shared, archived
// excluded, the actor's role resolved per row.

// insertSearchProperties creates count properties with padded distinct names
// («Объект 000»…) sharing the search prefix, half of them address-matched
// only.
func insertSearchProperties(t *testing.T, repo *PropertyRepository, ownerID uuid.UUID, count int) []domain.Property {
	t.Helper()
	ctx := t.Context()
	created := make([]domain.Property, 0, count)
	for i := range count {
		property := sampleProperty(ownerID, nil)
		property.Name = fmt.Sprintf("Объект поиска %03d", i)
		if i%2 == 0 {
			property.Address = fmt.Sprintf("Съездsearch, %03d, Москва", i)
		}
		if _, err := repo.Create(ctx, ownerID, property); err != nil {
			t.Fatalf("create property %d: %v", i, err)
		}
		created = append(created, property)
	}
	return created
}

// walkAllPages is the honest single loop the endpoint's cursor drives: feed
// the last row back until the page comes short.
func walkAllPages(t *testing.T, repo *PropertyRepository, actor uuid.UUID, search string, limit int32) []domain.Property {
	t.Helper()
	ctx := t.Context()
	var all []domain.Property
	var afterName *string
	var afterID *uuid.UUID
	for {
		page, err := repo.SearchVisible(ctx, actor, application.PropertySearchQuery{
			Search:    search,
			Limit:     limit,
			AfterName: afterName,
			AfterID:   afterID,
		})
		if err != nil {
			t.Fatalf("search page: %v", err)
		}
		all = append(all, page...)
		if len(page) < int(limit) {
			return all
		}
		last := page[len(page)-1]
		afterName = &last.Name
		afterID = &last.ID
	}
}

// assertWalkCovers asserts the whole walk: every seeded match appears
// exactly once — no duplicates, no drops (the #600 walk proof, over
// objects).
func assertWalkCovers(t *testing.T, got, seeded []domain.Property) {
	t.Helper()
	seen := make(map[uuid.UUID]bool, len(got))
	for _, p := range got {
		if seen[p.ID] {
			t.Fatalf("duplicate row %s in the walk", p.ID)
		}
		seen[p.ID] = true
	}
	if len(seen) != len(seeded) {
		t.Fatalf("walk covered %d rows, want %d", len(seen), len(seeded))
	}
}

func TestPropertyRepository_SearchVisible_KeysetWalkCompleteness(t *testing.T) {
	t.Parallel()

	pool := setupPropertiesIntegrationDB(t)
	repo := NewPropertyRepository(pool)
	ownerID := createTestOwner(t, pool)
	seeded := insertSearchProperties(t, repo, ownerID, 150)

	for _, limit := range []int32{50, 100} {
		got := walkAllPages(t, repo, ownerID, "поиска", limit)
		if len(got) != 150 {
			t.Fatalf("limit %d: walked %d rows, want 150", limit, len(got))
		}
		assertWalkCovers(t, got, seeded)
	}
}

func TestPropertyRepository_SearchVisible_MatchesNameAndAddressOnly(t *testing.T) {
	t.Parallel()

	pool := setupPropertiesIntegrationDB(t)
	repo := NewPropertyRepository(pool)
	ownerID := createTestOwner(t, pool)
	insertSearchProperties(t, repo, ownerID, 20)

	// The padded names all carry the prefix; the odd rows carry it only in
	// the name, the even ones in both.
	byName := walkAllPages(t, repo, ownerID, "Поиск", 50)
	if len(byName) != 20 {
		t.Fatalf("name match walked %d rows, want 20", len(byName))
	}

	if got := walkAllPages(t, repo, ownerID, "Съездsearch", 50); len(got) != 10 {
		t.Fatalf("address match walked %d rows, want 10", len(got))
	}

	if got := walkAllPages(t, repo, ownerID, "несуществующий", 50); len(got) != 0 {
		t.Fatalf("non-matching search walked %d rows, want 0", len(got))
	}

	// The ILIKE metacharacters stay inert: a bare % must match nothing, not
	// everything.
	if got := walkAllPages(t, repo, ownerID, "%", 50); len(got) != 0 {
		t.Fatalf("%% search walked %d rows, want 0 (the escape is broken)", len(got))
	}
	if got := walkAllPages(t, repo, ownerID, "Объект поиска 00%", 50); len(got) != 0 {
		t.Fatalf("trailing %% walked %d rows, want 0 (the escape is broken)", len(got))
	}
}

// visibleSliceFixture carries the seeded visibility scenario: the owner's
// active, maintenance and archived rows, a stranger's row and the shared
// row the recipient sees through an active full-access membership.
type visibleSliceFixture struct {
	ownerID, recipientID uuid.UUID
	own, maintenance     domain.Property
	archived, shared     domain.Property
	stranger             domain.Property
}

// seedVisibleSlice plants the fixture rows and the recipient's membership.
func seedVisibleSlice(t *testing.T, pool *pgxpool.Pool, repo *PropertyRepository) visibleSliceFixture {
	t.Helper()
	f := visibleSliceFixture{
		ownerID:     createTestOwner(t, pool),
		recipientID: createTestOwner(t, pool),
	}
	strangerSeedID := createTestOwner(t, pool)

	seed := func(owner uuid.UUID, name string, status domain.PropertyStatus) domain.Property {
		t.Helper()
		p := sampleProperty(owner, nil)
		p.Name = name
		p.Status = status
		if _, err := repo.Create(t.Context(), owner, p); err != nil {
			t.Fatalf("create %q: %v", name, err)
		}
		return p
	}

	f.own = seed(f.ownerID, "Свой объект", domain.PropertyStatusActive)
	f.maintenance = seed(f.ownerID, "Объект на ремонте", domain.PropertyStatusMaintenance)
	f.archived = seed(f.ownerID, "Архивный объект", domain.PropertyStatusArchived)
	f.shared = seed(f.ownerID, "Общий объект", domain.PropertyStatusActive)
	f.stranger = seed(strangerSeedID, "Чужой объект", domain.PropertyStatusActive)

	// The recipient's active full-access membership makes the shared object
	// visible.
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO property_members (id, property_id, user_id, role, granted_by)
		 VALUES ($1, $2, $3, 'full_access', $4)`,
		uuid.Must(uuid.NewV7()), f.shared.ID, f.recipientID, f.ownerID,
	); err != nil {
		t.Fatalf("insert membership: %v", err)
	}
	return f
}

func TestPropertyRepository_SearchVisible_OwnerSlice(t *testing.T) {
	t.Parallel()

	pool := setupPropertiesIntegrationDB(t)
	repo := NewPropertyRepository(pool)
	f := seedVisibleSlice(t, pool, repo)

	got := walkAllPages(t, repo, f.ownerID, "объект", 50)
	ids := make(map[uuid.UUID]bool, len(got))
	for _, p := range got {
		ids[p.ID] = true
		if p.AccessRole != sharedpolicy.RoleOwner {
			t.Errorf("own row %s role = %q, want owner", p.ID, p.AccessRole)
		}
	}
	if !ids[f.own.ID] || !ids[f.maintenance.ID] {
		t.Error("expected own active and maintenance rows in the slice")
	}
	if ids[f.archived.ID] {
		t.Error("archived row leaked into the search slice")
	}
	if ids[f.stranger.ID] {
		t.Error("a stranger's row leaked into the search slice")
	}
}

func TestPropertyRepository_SearchVisible_RecipientSlice(t *testing.T) {
	t.Parallel()

	pool := setupPropertiesIntegrationDB(t)
	repo := NewPropertyRepository(pool)
	f := seedVisibleSlice(t, pool, repo)

	got := walkAllPages(t, repo, f.recipientID, "объект", 50)
	if len(got) != 1 || got[0].ID != f.shared.ID {
		t.Fatalf("expected the shared row only, got %+v", got)
	}
	if got[0].AccessRole != sharedpolicy.RoleFullAccess {
		t.Errorf("shared row role = %q, want full_access", got[0].AccessRole)
	}
}
