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
	if r.err != nil {
		return domain.TenantContact{}, r.err
	}
	if contact.Phone != nil {
		for _, c := range r.contacts {
			if c.OwnerID == ownerID && c.ID != contact.ID && c.Phone != nil && *c.Phone == *contact.Phone {
				return domain.TenantContact{}, ErrDuplicatePhone
			}
		}
	}
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
	svc := NewTenantContactService(repo, nil, nil)

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
	svc := NewTenantContactService(repo, nil, nil)

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
	svc := NewTenantContactService(repo, nil, nil)

	_, err := svc.ListTenantContactsByIDs(context.Background(), ownerID, []uuid.UUID{uuid.New()})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func setupUpdateTenantContactService(ownerID, contactID uuid.UUID) (*fakeTenantContactRepo, *TenantContactService) {
	surname := "Ivanov"
	patronymic := "Ivanovich"
	phone := "+79161234567"
	email := "ivan@example.com"
	comment := "initial comment"
	repo := &fakeTenantContactRepo{
		contacts: []domain.TenantContact{
			{
				ID:         contactID,
				OwnerID:    ownerID,
				Name:       "Ivan",
				Surname:    &surname,
				Patronymic: &patronymic,
				Phone:      &phone,
				Email:      &email,
				Comment:    &comment,
			},
		},
	}
	svc := NewTenantContactService(repo, nil, nil)
	return repo, svc
}

func TestUpdateTenantContactClearsSurname(t *testing.T) {
	ownerID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a01")
	contactID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a02")
	_, svc := setupUpdateTenantContactService(ownerID, contactID)

	empty := ""
	updated, err := svc.UpdateTenantContact(context.Background(), ownerID, contactID, UpdateTenantContactCommand{
		Surname: &empty,
	})
	if err != nil {
		t.Fatalf("UpdateTenantContact failed: %v", err)
	}
	if updated.Surname != nil {
		t.Fatalf("expected surname to be cleared, got %q", *updated.Surname)
	}
}

func TestUpdateTenantContactClearsPatronymic(t *testing.T) {
	ownerID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a01")
	contactID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a02")
	_, svc := setupUpdateTenantContactService(ownerID, contactID)

	empty := ""
	updated, err := svc.UpdateTenantContact(context.Background(), ownerID, contactID, UpdateTenantContactCommand{
		Patronymic: &empty,
	})
	if err != nil {
		t.Fatalf("UpdateTenantContact failed: %v", err)
	}
	if updated.Patronymic != nil {
		t.Fatalf("expected patronymic to be cleared, got %q", *updated.Patronymic)
	}
}

func TestUpdateTenantContactClearsPhone(t *testing.T) {
	ownerID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a01")
	contactID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a02")
	_, svc := setupUpdateTenantContactService(ownerID, contactID)

	empty := ""
	updated, err := svc.UpdateTenantContact(context.Background(), ownerID, contactID, UpdateTenantContactCommand{
		Phone: &empty,
	})
	if err != nil {
		t.Fatalf("UpdateTenantContact failed: %v", err)
	}
	if updated.Phone != nil {
		t.Fatalf("expected phone to be cleared, got %q", *updated.Phone)
	}
}

func TestUpdateTenantContactClearsEmail(t *testing.T) {
	ownerID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a01")
	contactID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a02")
	_, svc := setupUpdateTenantContactService(ownerID, contactID)

	empty := ""
	updated, err := svc.UpdateTenantContact(context.Background(), ownerID, contactID, UpdateTenantContactCommand{
		Email: &empty,
	})
	if err != nil {
		t.Fatalf("UpdateTenantContact failed: %v", err)
	}
	if updated.Email != nil {
		t.Fatalf("expected email to be cleared, got %q", *updated.Email)
	}
}

func TestUpdateTenantContactClearsComment(t *testing.T) {
	ownerID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a01")
	contactID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a02")
	_, svc := setupUpdateTenantContactService(ownerID, contactID)

	empty := ""
	updated, err := svc.UpdateTenantContact(context.Background(), ownerID, contactID, UpdateTenantContactCommand{
		Comment: &empty,
	})
	if err != nil {
		t.Fatalf("UpdateTenantContact failed: %v", err)
	}
	if updated.Comment != nil {
		t.Fatalf("expected comment to be cleared, got %q", *updated.Comment)
	}
}

func TestUpdateTenantContactNormalizesPhone(t *testing.T) {
	ownerID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a01")
	contactID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a02")
	_, svc := setupUpdateTenantContactService(ownerID, contactID)

	phone := "89161234567"
	updated, err := svc.UpdateTenantContact(context.Background(), ownerID, contactID, UpdateTenantContactCommand{
		Phone: &phone,
	})
	if err != nil {
		t.Fatalf("UpdateTenantContact failed: %v", err)
	}
	if updated.Phone == nil {
		t.Fatal("expected phone to be set, got nil")
	}
	if *updated.Phone != "+79161234567" {
		t.Fatalf("expected phone %q, got %q", "+79161234567", *updated.Phone)
	}
}

func TestUpdateTenantContactPreservesEmail(t *testing.T) {
	ownerID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a01")
	contactID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a02")
	_, svc := setupUpdateTenantContactService(ownerID, contactID)

	email := "new.email+tag@example.ru"
	updated, err := svc.UpdateTenantContact(context.Background(), ownerID, contactID, UpdateTenantContactCommand{
		Email: &email,
	})
	if err != nil {
		t.Fatalf("UpdateTenantContact failed: %v", err)
	}
	if updated.Email == nil {
		t.Fatal("expected email to be set, got nil")
	}
	if *updated.Email != "new.email+tag@example.ru" {
		t.Fatalf("expected email %q, got %q", "new.email+tag@example.ru", *updated.Email)
	}
}

func TestUpdateTenantContactRejectsInvalidPhone(t *testing.T) {
	ownerID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a01")
	contactID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a02")
	_, svc := setupUpdateTenantContactService(ownerID, contactID)

	phone := "not-a-phone"
	_, err := svc.UpdateTenantContact(context.Background(), ownerID, contactID, UpdateTenantContactCommand{
		Phone: &phone,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestUpdateTenantContactRejectsInvalidEmail(t *testing.T) {
	ownerID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a01")
	contactID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a02")
	_, svc := setupUpdateTenantContactService(ownerID, contactID)

	email := "not-an-email"
	_, err := svc.UpdateTenantContact(context.Background(), ownerID, contactID, UpdateTenantContactCommand{
		Email: &email,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestUpdateTenantContactUpdatesName(t *testing.T) {
	ownerID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a01")
	contactID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a02")
	_, svc := setupUpdateTenantContactService(ownerID, contactID)

	newName := "Petr"
	updated, err := svc.UpdateTenantContact(context.Background(), ownerID, contactID, UpdateTenantContactCommand{
		Name: &newName,
	})
	if err != nil {
		t.Fatalf("UpdateTenantContact failed: %v", err)
	}
	if updated.Name != "Petr" {
		t.Fatalf("expected name %q, got %q", "Petr", updated.Name)
	}
}

func TestUpdateTenantContactRejectsEmptyName(t *testing.T) {
	ownerID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a01")
	contactID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a02")
	_, svc := setupUpdateTenantContactService(ownerID, contactID)

	empty := "   "
	_, err := svc.UpdateTenantContact(context.Background(), ownerID, contactID, UpdateTenantContactCommand{
		Name: &empty,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestUpdateTenantContactNoOpPreservesValues(t *testing.T) {
	ownerID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a01")
	contactID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a02")
	_, svc := setupUpdateTenantContactService(ownerID, contactID)

	updated, err := svc.UpdateTenantContact(context.Background(), ownerID, contactID, UpdateTenantContactCommand{})
	if err != nil {
		t.Fatalf("UpdateTenantContact failed: %v", err)
	}
	if updated.Name != "Ivan" {
		t.Fatalf("expected name %q, got %q", "Ivan", updated.Name)
	}
	if updated.Surname == nil || *updated.Surname != "Ivanov" {
		t.Fatalf("expected surname %q, got %v", "Ivanov", updated.Surname)
	}
	if updated.Patronymic == nil || *updated.Patronymic != "Ivanovich" {
		t.Fatalf("expected patronymic %q, got %v", "Ivanovich", updated.Patronymic)
	}
	if updated.Phone == nil || *updated.Phone != "+79161234567" {
		t.Fatalf("expected phone %q, got %v", "+79161234567", updated.Phone)
	}
	if updated.Email == nil || *updated.Email != "ivan@example.com" {
		t.Fatalf("expected email %q, got %v", "ivan@example.com", updated.Email)
	}
	if updated.Comment == nil || *updated.Comment != "initial comment" {
		t.Fatalf("expected comment %q, got %v", "initial comment", updated.Comment)
	}
}

func TestUpdateTenantContactNotFound(t *testing.T) {
	ownerID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a01")
	contactID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a02")
	_, svc := setupUpdateTenantContactService(ownerID, contactID)

	otherID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a99")
	newName := "Petr"
	_, err := svc.UpdateTenantContact(context.Background(), ownerID, otherID, UpdateTenantContactCommand{
		Name: &newName,
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdateTenantContactDuplicatePhone(t *testing.T) {
	ownerID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a01")
	contactID1 := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a02")
	contactID2 := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a03")
	phone1 := "+79161234567"
	phone2 := "+79169876543"
	surname := "Ivanov"
	patronymic := "Ivanovich"
	email := "ivan@example.com"
	comment := "initial comment"
	repo := &fakeTenantContactRepo{
		contacts: []domain.TenantContact{
			{
				ID:         contactID1,
				OwnerID:    ownerID,
				Name:       "Ivan",
				Surname:    &surname,
				Patronymic: &patronymic,
				Phone:      &phone1,
				Email:      &email,
				Comment:    &comment,
			},
			{
				ID:         contactID2,
				OwnerID:    ownerID,
				Name:       "Petr",
				Surname:    &surname,
				Patronymic: &patronymic,
				Phone:      &phone2,
				Email:      &email,
				Comment:    &comment,
			},
		},
	}
	svc := NewTenantContactService(repo, nil, nil)

	_, err := svc.UpdateTenantContact(context.Background(), ownerID, contactID2, UpdateTenantContactCommand{
		Phone: &phone1,
	})
	if !errors.Is(err, ErrDuplicatePhone) {
		t.Fatalf("expected ErrDuplicatePhone, got %v", err)
	}
}
