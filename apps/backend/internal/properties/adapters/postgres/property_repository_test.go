package postgres

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
)

// attrRooms — код атрибута rooms, повторённый JSONB-тестами пакета.
const attrRooms = "rooms"

var (
	migrateOnce sync.Once
	errMigrate  error
)

func setupPropertiesIntegrationDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	migrateOnce.Do(func() {
		errMigrate = database.MigrateUp(databaseURL, "../../../../db/migrations")
	})
	if errMigrate != nil {
		t.Fatalf("migrate up: %v", errMigrate)
	}
	// Tests run in parallel and each holds its own pool. The production
	// DefaultPoolConfig keeps MinConns=16 warm per pool; across the parallel
	// fixtures of the package that is close to a hundred connections against
	// one PostgreSQL, so the test pools stay minimal (repo tests run
	// sequential statements, the policy test one rollback transaction).
	cfg := database.DefaultPoolConfig()
	cfg.MinConns = 0
	cfg.MaxConns = 2
	ctx := t.Context()
	pool, err := database.NewPoolWithConfig(ctx, databaseURL, cfg)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

// createTestOwner inserts a minimal users row to satisfy the properties.owner_id
// foreign key. All non-required user columns have database defaults. Deleting the
// owner cascades to its properties (ON DELETE CASCADE), so callers only need to
// delete the owner in cleanup.
func createTestOwner(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	// The phone comes from the UUID's random tail, not its leading bytes:
	// those are the UUIDv7 timestamp with millisecond precision, so parallel
	// tests creating owners in the same millisecond would collide on
	// users.phone (the flake #168 predicted). Value-unique phones keep the
	// fixture parallel-safe (the same fix as waves #374/#375; the sibling
	// createContactPolicyTestUser derives its phone the same way).
	phone := fmt.Sprintf("+7000%07d", binary.BigEndian.Uint32(id[12:])%10000000)
	ctx := t.Context()
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, phone, role) VALUES ($1, $2, 'owner')`,
		id, phone,
	); err != nil {
		t.Fatalf("insert test owner: %v", err)
	}
	t.Cleanup(func() {
		// Since t.Context() is canceled once the test ends, use a fresh
		// context for cleanup to ensure the owner (and its cascaded property)
		// is removed.
		if _, err := pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id); err != nil {
			t.Logf("cleanup delete owner %s: %v", id, err)
		}
	})
	return id
}

// sampleProperty builds a valid domain.Property ready for Create/Update.
func sampleProperty(ownerID uuid.UUID, attrs domain.Attributes) domain.Property {
	return domain.Property{
		ID:         uuid.Must(uuid.NewV7()),
		OwnerID:    ownerID,
		Name:       "Test Property",
		Type:       domain.PropertyTypeApartment,
		Address:    "ul. Testovaya 1",
		Attributes: attrs,
		Status:     domain.PropertyStatusActive,
	}
}

func TestPropertyRepository_Attributes_JSONBRoundTrip(t *testing.T) {
	t.Parallel()

	pool := setupPropertiesIntegrationDB(t)
	ctx := t.Context()
	repo := NewPropertyRepository(pool)
	ownerID := createTestOwner(t, pool)

	property := sampleProperty(ownerID, domain.Attributes{
		attrRooms:    "2",
		"area_total": 50.0,
		"floor":      float64(3),
	})

	created, err := repo.Create(ctx, ownerID, property)
	if err != nil {
		t.Fatalf("create property: %v", err)
	}

	got, err := repo.GetByIDAndOwner(ctx, created.ID, ownerID)
	if err != nil {
		t.Fatalf("get property: %v", err)
	}

	if len(got.Attributes) != 3 {
		t.Fatalf("expected 3 attributes, got %d (%v)", len(got.Attributes), got.Attributes)
	}

	rooms, ok := got.Attributes[attrRooms].(string)
	if !ok || rooms != "2" {
		t.Fatalf("expected rooms to be string %q, got %T %v", "2", got.Attributes[attrRooms], got.Attributes[attrRooms])
	}
	areaTotal, ok := got.Attributes["area_total"].(float64)
	if !ok || areaTotal != 50.0 {
		t.Fatalf("expected area_total to be float64 50.0, got %T %v", got.Attributes["area_total"], got.Attributes["area_total"])
	}
	floor, ok := got.Attributes["floor"].(float64)
	if !ok || floor != 3.0 {
		t.Fatalf("expected floor to be float64 3.0, got %T %v", got.Attributes["floor"], got.Attributes["floor"])
	}
}

func TestPropertyRepository_Attributes_EmptyObjectDefault(t *testing.T) {
	t.Parallel()

	pool := setupPropertiesIntegrationDB(t)
	ctx := t.Context()
	repo := NewPropertyRepository(pool)
	ownerID := createTestOwner(t, pool)

	property := sampleProperty(ownerID, domain.Attributes{})

	created, err := repo.Create(ctx, ownerID, property)
	if err != nil {
		t.Fatalf("create property: %v", err)
	}

	got, err := repo.GetByIDAndOwner(ctx, created.ID, ownerID)
	if err != nil {
		t.Fatalf("get property: %v", err)
	}

	if got.Attributes == nil {
		t.Fatal("expected non-nil attributes, got nil")
	}
	if len(got.Attributes) != 0 {
		t.Fatalf("expected 0 attributes, got %d (%v)", len(got.Attributes), got.Attributes)
	}
}

func TestPropertyRepository_Attributes_UpdateReplacesEntireBlob(t *testing.T) {
	t.Parallel()

	pool := setupPropertiesIntegrationDB(t)
	ctx := t.Context()
	repo := NewPropertyRepository(pool)
	ownerID := createTestOwner(t, pool)

	property := sampleProperty(ownerID, domain.Attributes{attrRooms: "2"})

	created, err := repo.Create(ctx, ownerID, property)
	if err != nil {
		t.Fatalf("create property: %v", err)
	}

	// Update with a completely different key set: the old single key must be gone.
	created.Attributes = domain.Attributes{
		attrRooms:    "3",
		"area_total": 60.0,
	}
	if _, err := repo.Update(ctx, ownerID, created); err != nil {
		t.Fatalf("update property: %v", err)
	}

	got, err := repo.GetByIDAndOwner(ctx, created.ID, ownerID)
	if err != nil {
		t.Fatalf("get property: %v", err)
	}

	if len(got.Attributes) != 2 {
		t.Fatalf("expected exactly 2 attributes after update, got %d (%v)", len(got.Attributes), got.Attributes)
	}

	rooms, ok := got.Attributes[attrRooms].(string)
	if !ok || rooms != "3" {
		t.Fatalf("expected rooms to be string %q, got %T %v", "3", got.Attributes[attrRooms], got.Attributes[attrRooms])
	}
	areaTotal, ok := got.Attributes["area_total"].(float64)
	if !ok || areaTotal != 60.0 {
		t.Fatalf("expected area_total to be float64 60.0, got %T %v", got.Attributes["area_total"], got.Attributes["area_total"])
	}
}

func TestPropertyRepository_Attributes_NilAttributesStoredAsEmpty(t *testing.T) {
	t.Parallel()

	pool := setupPropertiesIntegrationDB(t)
	ctx := t.Context()
	repo := NewPropertyRepository(pool)
	ownerID := createTestOwner(t, pool)

	property := sampleProperty(ownerID, nil)

	created, err := repo.Create(ctx, ownerID, property)
	if err != nil {
		t.Fatalf("create property: %v", err)
	}

	got, err := repo.GetByIDAndOwner(ctx, created.ID, ownerID)
	if err != nil {
		t.Fatalf("get property: %v", err)
	}

	if got.Attributes == nil {
		t.Fatal("expected non-nil attributes (column default '{}'), got nil")
	}
	if len(got.Attributes) != 0 {
		t.Fatalf("expected 0 attributes, got %d (%v)", len(got.Attributes), got.Attributes)
	}
}

// Pin tests (ticket #577): the pinned rise above the unpinned — among
// themselves by the pin time; archiving clears the pin (the schema CHECK
// keeps the invariant).

func TestPropertyRepository_Pin_OrderAndClear(t *testing.T) {
	t.Parallel()

	pool := setupPropertiesIntegrationDB(t)
	ctx := t.Context()
	repo := NewPropertyRepository(pool)
	ownerID := createTestOwner(t, pool)

	pinnedFirst := sampleProperty(ownerID, nil)
	pinnedSecond := sampleProperty(ownerID, nil)
	unpinned := sampleProperty(ownerID, nil)
	for _, p := range []domain.Property{pinnedFirst, pinnedSecond, unpinned} {
		if _, err := repo.Create(ctx, ownerID, p); err != nil {
			t.Fatalf("create property: %v", err)
		}
	}

	pinA := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	pinB := time.Date(2026, 9, 2, 8, 0, 0, 0, time.UTC)
	if _, err := repo.SetPin(ctx, pinnedSecond.ID, ownerID, &pinB); err != nil {
		t.Fatalf("set pin B: %v", err)
	}
	if _, err := repo.SetPin(ctx, pinnedFirst.ID, ownerID, &pinA); err != nil {
		t.Fatalf("set pin A: %v", err)
	}

	list, err := repo.ListActiveByOwner(ctx, ownerID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := []uuid.UUID{pinnedFirst.ID, pinnedSecond.ID, unpinned.ID}
	if len(list) != len(want) {
		t.Fatalf("list len = %d, want %d", len(list), len(want))
	}
	for i, id := range want {
		if list[i].ID != id {
			t.Errorf("list[%d] = %s, want %s", i, list[i].ID, id)
		}
	}

	// Unpinning returns the property to the unpinned tail (updated_at DESC).
	if _, err := repo.SetPin(ctx, pinnedFirst.ID, ownerID, nil); err != nil {
		t.Fatalf("clear pin: %v", err)
	}
	got, err := repo.GetByIDAndOwner(ctx, pinnedFirst.ID, ownerID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.PinnedAt != nil {
		t.Errorf("pinnedAt after clear = %v, want nil", got.PinnedAt)
	}
}

func TestPropertyRepository_ArchiveClearsPin(t *testing.T) {
	t.Parallel()

	pool := setupPropertiesIntegrationDB(t)
	ctx := t.Context()
	repo := NewPropertyRepository(pool)
	ownerID := createTestOwner(t, pool)

	property := sampleProperty(ownerID, nil)
	if _, err := repo.Create(ctx, ownerID, property); err != nil {
		t.Fatalf("create property: %v", err)
	}
	pin := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	if _, err := repo.SetPin(ctx, property.ID, ownerID, &pin); err != nil {
		t.Fatalf("set pin: %v", err)
	}

	if err := repo.Archive(ctx, property.ID, ownerID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	got, err := repo.GetByIDAndOwner(ctx, property.ID, ownerID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Status != domain.PropertyStatusArchived {
		t.Errorf("status = %q, want archived", got.Status)
	}
	if got.PinnedAt != nil {
		t.Errorf("pinnedAt after archive = %v, want nil (the schema CHECK invariant)", got.PinnedAt)
	}
}

func TestPropertyRepository_CountByOwnerAndType(t *testing.T) {
	t.Parallel()

	pool := setupPropertiesIntegrationDB(t)
	ctx := t.Context()
	repo := NewPropertyRepository(pool)
	ownerID := createTestOwner(t, pool)
	otherOwner := createTestOwner(t, pool)

	// The serial source counts the owner's properties of the type in every
	// status (archived too), never another owner's (ticket #1001).
	active := sampleProperty(ownerID, nil)
	if _, err := repo.Create(ctx, ownerID, active); err != nil {
		t.Fatalf("create active: %v", err)
	}
	archived := sampleProperty(ownerID, nil)
	archived.Type = domain.PropertyTypeGarage
	if _, err := repo.Create(ctx, ownerID, archived); err != nil {
		t.Fatalf("create archived candidate: %v", err)
	}
	if err := repo.Archive(ctx, archived.ID, ownerID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	foreign := sampleProperty(otherOwner, nil)
	if _, err := repo.Create(ctx, otherOwner, foreign); err != nil {
		t.Fatalf("create foreign: %v", err)
	}

	count, err := repo.CountByOwnerAndType(ctx, ownerID, domain.PropertyTypeApartment)
	if err != nil {
		t.Fatalf("count apartment: %v", err)
	}
	if count != 1 {
		t.Errorf("apartment count = %d, want 1 (the foreign owner's row stays out)", count)
	}

	count, err = repo.CountByOwnerAndType(ctx, ownerID, domain.PropertyTypeGarage)
	if err != nil {
		t.Fatalf("count garage: %v", err)
	}
	if count != 1 {
		t.Errorf("garage count = %d, want 1 (the archived row counts)", count)
	}

	count, err = repo.CountByOwnerAndType(ctx, ownerID, domain.PropertyTypeHouse)
	if err != nil {
		t.Fatalf("count house: %v", err)
	}
	if count != 0 {
		t.Errorf("house count = %d, want 0", count)
	}

	// Удалённые строки из счёта выпадают (hard delete, #1001).
	gone := sampleProperty(ownerID, nil)
	if _, err := repo.Create(ctx, ownerID, gone); err != nil {
		t.Fatalf("create doomed: %v", err)
	}
	if err := repo.Delete(ctx, gone.ID, ownerID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	count, err = repo.CountByOwnerAndType(ctx, ownerID, domain.PropertyTypeApartment)
	if err != nil {
		t.Fatalf("count after delete: %v", err)
	}
	if count != 1 {
		t.Errorf("apartment count after delete = %d, want 1 (the deleted row is gone)", count)
	}
}

// TestPropertyRepository_CountArchivedByOwner verifies the «Архив» button
// gate's count (issue #1233): only the owner's archived rows count — active
// and maintenance stay out, another owner's archived row never leaks in, and
// an owner without archived rows reads 0.
func TestPropertyRepository_CountArchivedByOwner(t *testing.T) {
	t.Parallel()

	pool := setupPropertiesIntegrationDB(t)
	ctx := t.Context()
	repo := NewPropertyRepository(pool)
	ownerID := createTestOwner(t, pool)
	otherOwner := createTestOwner(t, pool)
	emptyOwner := createTestOwner(t, pool)

	active := sampleProperty(ownerID, nil)
	if _, err := repo.Create(ctx, ownerID, active); err != nil {
		t.Fatalf("create active: %v", err)
	}
	maintenance := sampleProperty(ownerID, nil)
	if _, err := repo.Create(ctx, ownerID, maintenance); err != nil {
		t.Fatalf("create maintenance candidate: %v", err)
	}
	maintenance.Status = domain.PropertyStatusMaintenance
	if _, err := repo.Update(ctx, ownerID, maintenance); err != nil {
		t.Fatalf("update to maintenance: %v", err)
	}
	archived := sampleProperty(ownerID, nil)
	if _, err := repo.Create(ctx, ownerID, archived); err != nil {
		t.Fatalf("create archived candidate: %v", err)
	}
	if err := repo.Archive(ctx, archived.ID, ownerID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	foreign := sampleProperty(otherOwner, nil)
	if _, err := repo.Create(ctx, otherOwner, foreign); err != nil {
		t.Fatalf("create foreign: %v", err)
	}
	if err := repo.Archive(ctx, foreign.ID, otherOwner); err != nil {
		t.Fatalf("archive foreign: %v", err)
	}

	count, err := repo.CountArchivedByOwner(ctx, ownerID)
	if err != nil {
		t.Fatalf("count archived: %v", err)
	}
	if count != 1 {
		t.Errorf("archived count = %d, want 1 (active and maintenance stay out, the foreign archived row never leaks in)", count)
	}

	count, err = repo.CountArchivedByOwner(ctx, otherOwner)
	if err != nil {
		t.Fatalf("count archived (other owner): %v", err)
	}
	if count != 1 {
		t.Errorf("other owner's archived count = %d, want 1", count)
	}

	count, err = repo.CountArchivedByOwner(ctx, emptyOwner)
	if err != nil {
		t.Fatalf("count archived (empty): %v", err)
	}
	if count != 0 {
		t.Errorf("empty owner's archived count = %d, want 0", count)
	}
}

func TestPropertyRepository_StudioTypeAccepted(t *testing.T) {
	t.Parallel()

	// 000143: 'studio' проходит CHECK-констрейнт properties.type (#1003).
	pool := setupPropertiesIntegrationDB(t)
	ctx := t.Context()
	repo := NewPropertyRepository(pool)
	ownerID := createTestOwner(t, pool)

	studio := sampleProperty(ownerID, nil)
	studio.Type = domain.PropertyTypeStudio
	created, err := repo.Create(ctx, ownerID, studio)
	if err != nil {
		t.Fatalf("create studio property: %v", err)
	}
	if created.Type != domain.PropertyTypeStudio {
		t.Errorf("type = %q, want studio", created.Type)
	}
}
