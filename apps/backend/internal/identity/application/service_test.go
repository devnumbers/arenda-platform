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
