//go:build integration

package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/admin/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/testdb"
)

// The admin read of contacts switched to the contacts table (ADR 0051,
// ticket #506): the property's bound cards with the display name composed
// from the name fields; unbound contacts belong to no property card.

func TestAdminListPropertyContactsIntegration(t *testing.T) {
	t.Parallel()

	pool := testdb.Setup(t)
	ctx := t.Context()
	repo := NewAdminRepository(pool, nil, nil)

	owner := seedAdminContactUser(t, ctx, pool)
	property := seedAdminContactProperty(t, ctx, pool, owner)
	otherProperty := seedAdminContactProperty(t, ctx, pool, owner)

	seedAdminContact(t, ctx, pool, owner, property, "Пётр", "Иванов", "Петрович", "+79160000001")
	seedAdminContact(t, ctx, pool, owner, property, "Мария", "", "", "")
	seedAdminContact(t, ctx, pool, owner, otherProperty, "Чужой", "", "", "")
	seedAdminContact(t, ctx, pool, owner, uuid.Nil, "БезОбъекта", "", "", "")

	views, total, err := repo.ListPropertyContacts(ctx, application.AdminPropertyContactFilters{
		PropertyID: property,
		Limit:      20,
		Offset:     0,
	})
	if err != nil {
		t.Fatalf("list property contacts: %v", err)
	}
	if total != 2 {
		t.Fatalf("total: want 2, got %d", total)
	}
	if len(views) != 2 {
		t.Fatalf("views: want 2, got %d", len(views))
	}
	if views[0].Name != "Пётр Иванов Петрович" {
		t.Fatalf("name[0]: want composed full name, got %q", views[0].Name)
	}
	if views[0].Phone != "+79160000001" {
		t.Fatalf("phone[0]: want plaintext phone, got %q", views[0].Phone)
	}
	if views[0].PropertyID != property || views[0].OwnerID != owner {
		t.Fatalf("ids[0]: want property %s of owner %s, got %s/%s",
			property, owner, views[0].PropertyID, views[0].OwnerID)
	}
	if views[1].Name != "Мария" {
		t.Fatalf("name[1]: want first name only, got %q", views[1].Name)
	}

	// Pagination: the second page over the same property is empty.
	views, total, err = repo.ListPropertyContacts(ctx, application.AdminPropertyContactFilters{
		PropertyID: property,
		Limit:      20,
		Offset:     2,
	})
	if err != nil {
		t.Fatalf("list page 2: %v", err)
	}
	if total != 2 || len(views) != 0 {
		t.Fatalf("page 2: want total 2 and 0 views, got %d/%d", total, len(views))
	}
}

func seedAdminContactUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	phone := fmt.Sprintf("+7999%010d", time.Now().UnixNano()%10000000000)
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, phone, role, timezone) VALUES ($1, $2, 'owner', 'Europe/Moscow')`,
		id, phone,
	); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return id
}

func seedAdminContactProperty(t *testing.T, ctx context.Context, pool *pgxpool.Pool, owner uuid.UUID) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO properties (id, owner_id, name, type, address, status)
		 VALUES ($1, $2, 'Админский объект', 'apartment', 'Москва', 'active')`,
		id, owner,
	); err != nil {
		t.Fatalf("seed property: %v", err)
	}
	return id
}

func seedAdminContact(
	t *testing.T, ctx context.Context, pool *pgxpool.Pool,
	owner, property uuid.UUID,
	first, last, patronymic, phone string,
) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	var propertyArg *uuid.UUID
	if property != uuid.Nil {
		propertyArg = &property
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO contacts (id, owner_id, property_id, first_name, last_name, patronymic, phone)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''))`,
		id, owner, propertyArg, first, last, patronymic, phone,
	); err != nil {
		t.Fatalf("seed contact: %v", err)
	}
	return id
}
