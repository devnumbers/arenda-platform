package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

type fakeClock struct {
	now time.Time
}

func (c *fakeClock) Now() time.Time { return c.now }

type fakeTariffDB struct {
	calls int
	rows  []genpostgres.Tariff
}

func (db *fakeTariffDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (db *fakeTariffDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	db.calls++
	return &fakeRows{rows: db.rows}, nil
}

func (db *fakeTariffDB) QueryRow(context.Context, string, ...interface{}) pgx.Row {
	db.calls++
	return &fakeRow{row: db.rows[0]}
}

type fakeRow struct {
	row genpostgres.Tariff
}

func (r *fakeRow) Scan(dest ...any) error {
	for _, d := range dest {
		switch v := d.(type) {
		case *pgtype.UUID:
			*v = r.row.ID
		case *string:
			*v = r.row.Name
		case *int32:
			*v = r.row.ActivePropertyLimit
		case *int64:
			*v = r.row.MonthlyPriceKopecks
		case *pgtype.Timestamptz:
			*v = r.row.CreatedAt
		}
	}
	return nil
}

type fakeRows struct {
	rows  []genpostgres.Tariff
	index int
}

func (r *fakeRows) Close()                                       {}
func (r *fakeRows) Err() error                                   { return nil }
func (r *fakeRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeRows) Next() bool {
	if r.index >= len(r.rows) {
		return false
	}
	r.index++
	return true
}

func (r *fakeRows) Scan(dest ...any) error {
	row := r.rows[r.index-1]
	for _, d := range dest {
		switch v := d.(type) {
		case *pgtype.UUID:
			*v = row.ID
		case *string:
			*v = row.Name
		case *int32:
			*v = row.ActivePropertyLimit
		case *int64:
			*v = row.MonthlyPriceKopecks
		case *pgtype.Timestamptz:
			*v = row.CreatedAt
		}
	}
	return nil
}

func (r *fakeRows) Values() ([]any, error) { return nil, nil }
func (r *fakeRows) RawValues() [][]byte    { return nil }
func (r *fakeRows) Conn() *pgx.Conn        { return nil }

func TestTariffRepositoryCacheHits(t *testing.T) {
	id := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	db := &fakeTariffDB{
		rows: []genpostgres.Tariff{{
			ID:                  pgtype.UUID{Bytes: id, Valid: true},
			Name:                "basic",
			ActivePropertyLimit: 1,
			MonthlyPriceKopecks: 0,
			CreatedAt:           pgtype.Timestamptz{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
		}},
	}
	clk := &fakeClock{now: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)}
	repo := NewTariffRepository(db, time.Hour, clk)

	if _, err := repo.GetByID(context.Background(), id); err != nil {
		t.Fatalf("first GetByID failed: %v", err)
	}
	if _, err := repo.GetByID(context.Background(), id); err != nil {
		t.Fatalf("second GetByID failed: %v", err)
	}
	if db.calls != 1 {
		t.Fatalf("expected 1 DB call, got %d", db.calls)
	}
}

func TestTariffRepositoryCacheExpires(t *testing.T) {
	id := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	db := &fakeTariffDB{
		rows: []genpostgres.Tariff{{
			ID:                  pgtype.UUID{Bytes: id, Valid: true},
			Name:                "basic",
			ActivePropertyLimit: 1,
			MonthlyPriceKopecks: 0,
			CreatedAt:           pgtype.Timestamptz{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
		}},
	}
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	clk := &fakeClock{now: start}
	repo := NewTariffRepository(db, time.Minute, clk)

	if _, err := repo.GetByID(context.Background(), id); err != nil {
		t.Fatalf("first GetByID failed: %v", err)
	}

	clk.now = start.Add(2 * time.Minute)

	if _, err := repo.GetByID(context.Background(), id); err != nil {
		t.Fatalf("second GetByID after TTL failed: %v", err)
	}
	if db.calls != 2 {
		t.Fatalf("expected 2 DB calls after cache expiry, got %d", db.calls)
	}
}

func TestTariffRepositoryDefaultTTL(t *testing.T) {
	db := &fakeTariffDB{}
	repo := NewTariffRepository(db, 0, nil)
	if repo.ttl != 5*time.Minute {
		t.Fatalf("expected default TTL 5m, got %v", repo.ttl)
	}
}

func TestTariffRepositoryListCache(t *testing.T) {
	id := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	db := &fakeTariffDB{
		rows: []genpostgres.Tariff{{
			ID:                  pgtype.UUID{Bytes: id, Valid: true},
			Name:                "basic",
			ActivePropertyLimit: 1,
			MonthlyPriceKopecks: 0,
			CreatedAt:           pgtype.Timestamptz{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
		}},
	}
	clk := &fakeClock{now: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)}
	repo := NewTariffRepository(db, time.Hour, clk)

	if _, err := repo.List(context.Background()); err != nil {
		t.Fatalf("first List failed: %v", err)
	}
	if _, err := repo.List(context.Background()); err != nil {
		t.Fatalf("second List failed: %v", err)
	}
	if db.calls != 1 {
		t.Fatalf("expected 1 DB call for cached list, got %d", db.calls)
	}
}

func TestTariffRepositoryGetByNameCache(t *testing.T) {
	id := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	db := &fakeTariffDB{
		rows: []genpostgres.Tariff{{
			ID:                  pgtype.UUID{Bytes: id, Valid: true},
			Name:                "basic",
			ActivePropertyLimit: 1,
			MonthlyPriceKopecks: 0,
			CreatedAt:           pgtype.Timestamptz{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
		}},
	}
	clk := &fakeClock{now: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)}
	repo := NewTariffRepository(db, time.Hour, clk)

	if _, err := repo.GetByName(context.Background(), domain.TariffName("basic")); err != nil {
		t.Fatalf("first GetByName failed: %v", err)
	}
	if _, err := repo.GetByName(context.Background(), domain.TariffName("basic")); err != nil {
		t.Fatalf("second GetByName failed: %v", err)
	}
	if db.calls != 1 {
		t.Fatalf("expected 1 DB call for cached name, got %d", db.calls)
	}
}
