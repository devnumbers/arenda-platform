//go:build integration

package postgres_test

// The integration suite of the action journal store (карта #704, тикет #707,
// ADR 0061): the insert/read-back round trip with the database-generated
// search_tsv, the schema contracts the feed relies on — the property CASCADE
// («удаляется с объектом»), the user SET NULL with the snapshots surviving
// and the membership removal leaving the rows in place («переживает отзыв и
// выход участника») — and the actor display-name/email snapshot resolution.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	historypg "github.com/nambers/arenda-planform/apps/backend/internal/history/adapters/postgres"
	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/testdb"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/actor"
)

func newRecorder(pool *pgxpool.Pool) historyapp.Recorder {
	return historyapp.NewService(historypg.NewEntryStore(pool), historypg.NewActorStore(pool), nil)
}

func seedUser(t *testing.T, pool *pgxpool.Pool, name, surname, phone, email string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO users (id, phone, role, name, surname, email, timezone)
		 VALUES ($1, $2, $3, $4, $5, $6, 'Europe/Moscow')`,
		id, phone, actor.RoleOwner, name, surname, email,
	); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return id
}

func seedProperty(t *testing.T, pool *pgxpool.Pool, owner uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO properties (id, owner_id, name, type, address, status)
		 VALUES ($1, $2, $3, 'apartment', 'Москва, Тверская 1', 'active')`,
		id, owner, "Квартира на Тверской",
	); err != nil {
		t.Fatalf("seed property: %v", err)
	}
	return id
}

func seedMembership(t *testing.T, pool *pgxpool.Pool, propertyID, userID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO property_members (id, property_id, user_id, role, granted_by)
		 VALUES ($1, $2, $3, 'full_access', $4)`,
		id, propertyID, userID, userID,
	); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	return id
}

func recordEntry(t *testing.T, pool *pgxpool.Pool, actorID, propertyID uuid.UUID) {
	t.Helper()
	entry := domain.OperationPaid(uuid.Must(uuid.NewV7()), "Электричество", time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC))
	entry.PropertyID = propertyID
	entry.ActorID = &actorID
	entry.ActorRole = domain.ActorRoleOwner
	if err := newRecorder(pool).Record(context.Background(), entry); err != nil {
		t.Fatalf("Record: %v", err)
	}
}

func journalRows(t *testing.T, pool *pgxpool.Pool, propertyID uuid.UUID) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(t.Context(),
		`SELECT count(*) FROM action_journal WHERE property_id = $1`, propertyID,
	).Scan(&n); err != nil {
		t.Fatalf("count journal: %v", err)
	}
	return n
}

func TestStore_InsertRoundTrip(t *testing.T) {
	t.Parallel()
	pool := testdb.Setup(t)
	owner := seedUser(t, pool, "Иван", "Иванов", "+79990000001", "ivan@example.com")
	property := seedProperty(t, pool, owner)

	recordEntry(t, pool, owner, property)

	var (
		actorRole, actorName, actorEmail, kind, action, baseAction string
		segments, searchable, contextJSON                          string
		tsvPresent                                                 bool
	)
	if err := pool.QueryRow(t.Context(),
		`SELECT actor_role, actor_name, actor_email, kind, action, base_action,
		        segments::text, searchable, context::text,
		        search_tsv = to_tsvector('russian', searchable)
		 FROM action_journal WHERE property_id = $1`, property,
	).Scan(&actorRole, &actorName, &actorEmail, &kind, &action, &baseAction,
		&segments, &searchable, &contextJSON, &tsvPresent); err != nil {
		t.Fatalf("read journal row: %v", err)
	}

	if actorRole != "owner" || actorName != "Иван Иванов" || actorEmail != "ivan@example.com" {
		t.Errorf("actor columns = %s/%s/%s", actorRole, actorName, actorEmail)
	}
	if kind != "operation" || action != "operation.paid" || baseAction != "completed" {
		t.Errorf("vocabulary columns = %s/%s/%s", kind, action, baseAction)
	}
	wantSearchable := "Операция оплачена: Электричество (срок 15.09.2026) Иван Иванов ivan@example.com"
	if searchable != wantSearchable {
		t.Errorf("searchable = %q, want %q", searchable, wantSearchable)
	}
	if !tsvPresent {
		t.Error("the generated search_tsv must match its own searchable text")
	}
	// The search probes mirror the research #705 routing: the FTS leg for
	// the Russian word forms, the trgm leg (ILIKE) for the substrings and
	// the emails.
	const trgmSubstring = `searchable ILIKE '%' || $1::text || '%'`
	probes := []struct{ sql, term string }{
		{`search_tsv @@ plainto_tsquery('russian', $1::text)`, "оплачена"},
		{trgmSubstring, "лектричест"},
		{trgmSubstring, "Иванов"},
		{trgmSubstring, "ivan@example.com"},
	}
	for i, probe := range probes {
		var hit bool
		if err := pool.QueryRow(t.Context(),
			`SELECT `+probe.sql+` FROM action_journal WHERE property_id = $2`,
			probe.term, property,
		).Scan(&hit); err != nil {
			t.Fatalf("search probe %d: %v", i, err)
		}
		if !hit {
			t.Errorf("searchable must hit %q", probe.term)
		}
	}
	_ = segments
	_ = contextJSON
}

func TestStore_DeletedPropertyCascades(t *testing.T) {
	t.Parallel()
	pool := testdb.Setup(t)
	owner := seedUser(t, pool, "Иван", "Иванов", "+79990000002", "ivan2@example.com")
	property := seedProperty(t, pool, owner)
	recordEntry(t, pool, owner, property)

	if _, err := pool.Exec(t.Context(), `DELETE FROM properties WHERE id = $1`, property); err != nil {
		t.Fatalf("delete property: %v", err)
	}
	if n := journalRows(t, pool, property); n != 0 {
		t.Errorf("journal rows after the property delete = %d, want 0 (FK CASCADE)", n)
	}
}

func TestStore_DeletedUserNullsActorButKeepsSnapshots(t *testing.T) {
	t.Parallel()
	pool := testdb.Setup(t)
	owner := seedUser(t, pool, "Иван", "Иванов", "+79990000003", "ivan3@example.com")
	member := seedUser(t, pool, "Мария", "Петрова", "+79990000004", "maria@example.com")
	property := seedProperty(t, pool, owner)
	seedMembership(t, pool, property, member)

	// The member's action; the actor user then goes away. Deleting an OWNER
	// instead would cascade their property (owner_id ON DELETE CASCADE) and
	// the journal rows with it — the property outlives the actor here.
	entry := domain.PaymentCreated(property, "Интернет")
	entry.PropertyID = property
	entry.ActorID = &member
	entry.ActorRole = domain.ActorRoleFullAccess
	if err := newRecorder(pool).Record(context.Background(), entry); err != nil {
		t.Fatalf("Record: %v", err)
	}

	if _, err := pool.Exec(t.Context(), `DELETE FROM users WHERE id = $1`, member); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	var actorID *uuid.UUID
	var actorName, actorEmail string
	if err := pool.QueryRow(t.Context(),
		`SELECT actor_id, actor_name, actor_email FROM action_journal WHERE property_id = $1`, property,
	).Scan(&actorID, &actorName, &actorEmail); err != nil {
		t.Fatalf("read journal row: %v", err)
	}
	if actorID != nil {
		t.Errorf("actor_id = %v, want NULL (ON DELETE SET NULL)", *actorID)
	}
	if actorName != "Мария Петрова" || actorEmail != "maria@example.com" {
		t.Errorf("snapshots must survive the user delete, got %q/%q", actorName, actorEmail)
	}
}

func TestStore_SurvivesMemberRevokeAndLeave(t *testing.T) {
	t.Parallel()
	pool := testdb.Setup(t)
	owner := seedUser(t, pool, "Иван", "Иванов", "+79990000004", "ivan4@example.com")
	member := seedUser(t, pool, "Пётр", "Сидоров", "+79990000005", "petr@example.com")
	property := seedProperty(t, pool, owner)
	membership := seedMembership(t, pool, property, member)

	// The member's action recorded while their membership was alive.
	entry := domain.PaymentCreated(property, "Интернет")
	entry.PropertyID = property
	entry.ActorID = &member
	entry.ActorRole = domain.ActorRoleFullAccess
	if err := newRecorder(pool).Record(context.Background(), entry); err != nil {
		t.Fatalf("Record: %v", err)
	}

	// The revoke (a membership row delete) and the self-exit are the same
	// shape: neither may touch the member's journal rows.
	if _, err := pool.Exec(t.Context(), `DELETE FROM property_members WHERE id = $1`, membership); err != nil {
		t.Fatalf("revoke membership: %v", err)
	}

	var actorName, actorRole string
	var actorID *uuid.UUID
	if err := pool.QueryRow(t.Context(),
		`SELECT actor_id, actor_name, actor_role FROM action_journal WHERE property_id = $1`, property,
	).Scan(&actorID, &actorName, &actorRole); err != nil {
		t.Fatalf("read journal row: %v", err)
	}
	if actorName != "Пётр Сидоров" || actorRole != "full_access" {
		t.Errorf("the row must survive the revoke with its snapshots, got %q/%s", actorName, actorRole)
	}
}

func TestStore_ActorSnapshotMaskedPhoneFallback(t *testing.T) {
	t.Parallel()
	pool := testdb.Setup(t)
	// A user without name/surname: the display name degrades to the masked
	// phone — never the raw phone (the access canon, one place).
	anon := seedUser(t, pool, "", "", "+79991234567", "")
	snap, err := historypg.NewActorStore(pool).Snapshot(context.Background(), anon)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snap.Name == "+79991234567" || snap.Name == "" {
		t.Errorf("Name = %q, want the masked phone", snap.Name)
	}
	if len(snap.Name) != len("+79991234567") {
		t.Errorf("Name = %q, want the same length as the phone", snap.Name)
	}
	if snap.Email != "" {
		t.Errorf("Email = %q, want empty", snap.Email)
	}
}

var _ = fmt.Sprintf
