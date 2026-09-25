package application

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	accessdomain "github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// This file covers the property-deletion seam of the sharing lifecycle emails
// (issue #162, T6): PropertyService.DeleteProperty collects the former shared
// members' emails inside the delete transaction (before the memberships are
// dropped by the slot policy) and sends the "object deleted" email after
// commit.

// fakeSharedDeleteFlow records the ordered calls of the RecipientSlotPolicy and
// SharedMembersDeleteMailer ports so the test can assert the collect-before-
// drop ordering.
type fakeSharedDeleteFlow struct {
	events   []string
	emails   []string
	sentTo   []string
	sentWith []string
}

func (f *fakeSharedDeleteFlow) RecoverSuspendedForProperty(context.Context, transaction.Tx, uuid.UUID) ([]accessdomain.Membership, error) {
	return nil, nil
}

func (f *fakeSharedDeleteFlow) EnforceOnUnarchiveForProperty(context.Context, transaction.Tx, uuid.UUID) ([]accessdomain.Membership, error) {
	return nil, nil
}

func (f *fakeSharedDeleteFlow) RecoverAfterPropertyDelete(context.Context, transaction.Tx, uuid.UUID) ([]accessdomain.Membership, error) {
	f.events = append(f.events, "recover_after_delete")
	return nil, nil
}

func (f *fakeSharedDeleteFlow) RecoverSuspended(context.Context, transaction.Tx, uuid.UUID) ([]accessdomain.Membership, error) {
	return nil, nil
}

func (f *fakeSharedDeleteFlow) CollectFormerMemberEmails(_ context.Context, _ transaction.Tx, propertyID uuid.UUID) ([]string, error) {
	f.events = append(f.events, "collect")
	return f.emails, nil
}

func (f *fakeSharedDeleteFlow) SendPropertyDeleted(_ context.Context, to, propertyTitle string) error {
	f.events = append(f.events, "send")
	f.sentTo = append(f.sentTo, to)
	f.sentWith = append(f.sentWith, propertyTitle)
	return nil
}

var (
	_ RecipientSlotPolicy       = (*fakeSharedDeleteFlow)(nil)
	_ SharedMembersDeleteMailer = (*fakeSharedDeleteFlow)(nil)
)

func TestPropertyService_DeleteProperty_NotifiesFormerSharedMembers(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	repo := newFakePropertyRepo(
		domain.Property{
			ID: propertyID, OwnerID: ownerID, Name: "Квартира на Невском",
			Address: testPropertyAddress, Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive,
		},
	)
	shared := &fakeSharedDeleteFlow{emails: []string{"a@example.com", "b@example.com"}}
	svc := NewPropertyService(
		repo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		newPropertyTestFactory(repo, fakePropertyPhotoRepo{}, nil),
		fakePropertyClock{now: time.Now()},
		testOwnerPolicy{},
		nil,
	)
	svc.SetRecipientSlotPolicy(shared)
	svc.SetSharedMembersDeleteMailer(shared)

	if err := svc.DeleteProperty(ctx, ownerID, propertyID); err != nil {
		t.Fatalf("DeleteProperty: %v", err)
	}

	// The emails are collected before the slot policy drops the memberships.
	collectIdx := slices.Index(shared.events, "collect")
	recoverIdx := slices.Index(shared.events, "recover_after_delete")
	if collectIdx == -1 || recoverIdx == -1 || collectIdx > recoverIdx {
		t.Errorf("collect must run before recover_after_delete, events = %v", shared.events)
	}

	// Both former members are notified after the delete, with the property name.
	if !slices.Equal(shared.sentTo, []string{"a@example.com", "b@example.com"}) {
		t.Errorf("sent to = %v", shared.sentTo)
	}
	for _, title := range shared.sentWith {
		if title != "Квартира на Невском" {
			t.Errorf("sent with title = %q", title)
		}
	}
	// Sends happen after the collect/recover (post-commit): the last two
	// events are the two sends.
	if len(shared.events) < 2 || shared.events[len(shared.events)-2] != "send" || shared.events[len(shared.events)-1] != "send" {
		t.Errorf("sends must come last (post-commit), events = %v", shared.events)
	}

	if _, err := repo.GetByID(ctx, propertyID); err == nil {
		t.Error("property must be deleted")
	}
}
