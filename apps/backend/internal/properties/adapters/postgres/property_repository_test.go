package postgres

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

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
	ctx := t.Context()
	pool, err := database.NewPool(ctx, databaseURL)
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
	// Derive a unique phone from the owner UUID's random bytes. Since the id is
	// unique, the phone is too — dodging the users.phone UNIQUE constraint.
	phone := fmt.Sprintf("+7000%012x", id[0:6])
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
