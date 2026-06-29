package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

type fakeSessionRepo struct {
	deletedUserID uuid.UUID
}

func (f *fakeSessionRepo) Create(_ context.Context, _ domain.Session) error               { return nil }
func (f *fakeSessionRepo) GetByTokenHash(_ context.Context, _ string, _ time.Time) (domain.Session, domain.User, error) {
	return domain.Session{}, domain.User{}, ErrNotFound
}
func (f *fakeSessionRepo) Update(_ context.Context, _ domain.Session) error                { return nil }
func (f *fakeSessionRepo) DeleteByTokenHash(_ context.Context, _ string) error             { return nil }
func (f *fakeSessionRepo) DeleteByUserID(_ context.Context, userID uuid.UUID) error         {
	f.deletedUserID = userID
	return nil
}
func (f *fakeSessionRepo) DeleteByUserIDExcept(_ context.Context, _ uuid.UUID, _ string) error { return nil }
func (f *fakeSessionRepo) DeleteExpiredBefore(_ context.Context, _ time.Time) error        { return nil }
func (f *fakeSessionRepo) DeleteExpiredBeforeBatch(_ context.Context, _ time.Time, _ int32) (int64, error) {
	return 0, nil
}
func (f *fakeSessionRepo) WithTx(_ transaction.Tx) SessionRepository                      { return f }

func TestAuthServiceLogoutAll(t *testing.T) {
	repo := &fakeSessionRepo{}
	svc := NewAuthService(nil, nil, nil, repo, nil, nil, nil, nil, nil)

	userID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	if err := svc.LogoutAll(context.Background(), userID); err != nil {
		t.Fatalf("LogoutAll() error = %v", err)
	}

	if repo.deletedUserID != userID {
		t.Errorf("deleted user ID = %v, want %v", repo.deletedUserID, userID)
	}
}

type failingSessionRepo struct{}

func (failingSessionRepo) Create(_ context.Context, _ domain.Session) error               { return nil }
func (failingSessionRepo) GetByTokenHash(_ context.Context, _ string, _ time.Time) (domain.Session, domain.User, error) {
	return domain.Session{}, domain.User{}, ErrNotFound
}
func (failingSessionRepo) Update(_ context.Context, _ domain.Session) error                { return nil }
func (failingSessionRepo) DeleteByTokenHash(_ context.Context, _ string) error             { return nil }
func (failingSessionRepo) DeleteByUserID(_ context.Context, _ uuid.UUID) error             { return errors.New("db error") }
func (failingSessionRepo) DeleteByUserIDExcept(_ context.Context, _ uuid.UUID, _ string) error { return nil }
func (failingSessionRepo) DeleteExpiredBefore(_ context.Context, _ time.Time) error        { return nil }
func (failingSessionRepo) DeleteExpiredBeforeBatch(_ context.Context, _ time.Time, _ int32) (int64, error) {
	return 0, nil
}
func (failingSessionRepo) WithTx(_ transaction.Tx) SessionRepository                      { return failingSessionRepo{} }

func TestAuthServiceLogoutAllPropagatesError(t *testing.T) {
	svc := NewAuthService(nil, nil, nil, failingSessionRepo{}, nil, nil, nil, nil, nil)

	userID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	err := svc.LogoutAll(context.Background(), userID)
	if err == nil {
		t.Fatal("expected error from DeleteByUserID")
	}
}

type fakeUserRepo struct {
	user domain.User
}

func (f *fakeUserRepo) GetByID(_ context.Context, _ uuid.UUID) (domain.User, error)                 { return f.user, nil }
func (f *fakeUserRepo) GetByIDForUpdate(_ context.Context, _ uuid.UUID) (domain.User, error)         { return f.user, nil }
func (f *fakeUserRepo) GetByPhone(_ context.Context, _ domain.Phone) (domain.User, error)             { return domain.User{}, ErrNotFound }
func (f *fakeUserRepo) GetPhoneByID(_ context.Context, _ uuid.UUID) (string, error)                   { return f.user.Phone.String(), nil }
func (f *fakeUserRepo) Create(_ context.Context, user domain.User) (domain.User, error)               { return user, nil }
func (f *fakeUserRepo) Update(_ context.Context, user domain.User) (domain.User, error)               { f.user = user; return user, nil }
func (f *fakeUserRepo) UpdatePhone(_ context.Context, _ uuid.UUID, phone domain.Phone) (domain.User, error) {
	f.user.Phone = phone
	return f.user, nil
}
func (f *fakeUserRepo) WithTx(_ transaction.Tx) UserRepository { return f }

func TestUpdateUserClearsName(t *testing.T) {
	phone, err := domain.NewPhone("+79001234567")
	if err != nil {
		t.Fatalf("NewPhone() error = %v", err)
	}

	name := "Ivan"
	user, err := domain.NewOwner(phone)
	if err != nil {
		t.Fatalf("NewOwner() error = %v", err)
	}
	user.Name = &name

	repo := &fakeUserRepo{user: user}
	svc := NewAuthService(repo, nil, nil, nil, nil, nil, nil, nil, nil)

	updated, err := svc.UpdateUser(context.Background(), user.ID, UpdateUserCommand{
		Name: domain.Optional[string]{Set: true},
	})
	if err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}

	if updated.Name != nil {
		t.Errorf("Name = %v, want nil", updated.Name)
	}
}
