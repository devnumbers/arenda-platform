package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

type fakeTenantContactRepo struct {
	contacts []domain.TenantContact
	err      error
}

func (r *fakeTenantContactRepo) Create(ctx context.Context, ownerID uuid.UUID, contact domain.TenantContact) (domain.TenantContact, error) {
	return contact, nil
}

func (r *fakeTenantContactRepo) GetByIDAndOwner(ctx context.Context, id, ownerID uuid.UUID) (domain.TenantContact, error) {
	for _, c := range r.contacts {
		if c.ID == id && c.OwnerID == ownerID {
			return c, nil
		}
	}
	return domain.TenantContact{}, ErrNotFound
}

func (r *fakeTenantContactRepo) ListByIDs(ctx context.Context, ownerID uuid.UUID, ids []uuid.UUID) ([]domain.TenantContact, error) {
	if r.err != nil {
		return nil, r.err
	}
	result := make([]domain.TenantContact, 0, len(ids))
	for _, c := range r.contacts {
		if c.OwnerID != ownerID {
			continue
		}
		for _, id := range ids {
			if c.ID == id {
				result = append(result, c)
			}
		}
	}
	return result, nil
}

func (r *fakeTenantContactRepo) Update(ctx context.Context, ownerID uuid.UUID, contact domain.TenantContact) (domain.TenantContact, error) {
	return contact, nil
}

func (r *fakeTenantContactRepo) ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]domain.TenantContact, error) {
	return r.contacts, nil
}

func (r *fakeTenantContactRepo) ListWithLeaseStatus(ctx context.Context, ownerID uuid.UUID) ([]domain.TenantContactWithLeases, error) {
	return nil, nil
}

func (r *fakeTenantContactRepo) WithTx(tx transaction.Tx) TenantContactRepository {
	return r
}

func TestListTenantContactsByIDs(t *testing.T) {
	ownerID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a01")
	id1 := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a02")
	id2 := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a03")
	otherOwner := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a04")

	repo := &fakeTenantContactRepo{
		contacts: []domain.TenantContact{
			{ID: id1, OwnerID: ownerID, Name: "One"},
			{ID: id2, OwnerID: ownerID, Name: "Two"},
			{ID: uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a05"), OwnerID: otherOwner, Name: "Other"},
		},
	}
	svc := NewTenantContactService(repo, nil)

	contacts, err := svc.ListTenantContactsByIDs(context.Background(), ownerID, []uuid.UUID{id1, id2})
	if err != nil {
		t.Fatalf("ListTenantContactsByIDs failed: %v", err)
	}
	if len(contacts) != 2 {
		t.Fatalf("expected 2 contacts, got %d", len(contacts))
	}
	if contacts[id1].Name != "One" {
		t.Fatalf("expected contact One, got %s", contacts[id1].Name)
	}
	if contacts[id2].Name != "Two" {
		t.Fatalf("expected contact Two, got %s", contacts[id2].Name)
	}
}

func TestListTenantContactsByIDsEmpty(t *testing.T) {
	ownerID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a01")
	repo := &fakeTenantContactRepo{}
	svc := NewTenantContactService(repo, nil)

	contacts, err := svc.ListTenantContactsByIDs(context.Background(), ownerID, nil)
	if err != nil {
		t.Fatalf("ListTenantContactsByIDs failed: %v", err)
	}
	if len(contacts) != 0 {
		t.Fatalf("expected empty map, got %d", len(contacts))
	}
}

func TestListTenantContactsByIDsRepositoryError(t *testing.T) {
	ownerID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a01")
	repo := &fakeTenantContactRepo{err: errors.New("boom")}
	svc := NewTenantContactService(repo, nil)

	_, err := svc.ListTenantContactsByIDs(context.Background(), ownerID, []uuid.UUID{uuid.New()})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
