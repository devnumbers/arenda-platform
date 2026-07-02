package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

type fakeSessionRepoForHandlers struct{}

func (fakeSessionRepoForHandlers) Create(_ context.Context, _ domain.Session) error { return nil }
func (fakeSessionRepoForHandlers) GetByTokenHash(_ context.Context, _ string, _ time.Time) (domain.Session, domain.User, error) {
	return domain.Session{}, domain.User{}, application.ErrNotFound
}
func (fakeSessionRepoForHandlers) Update(_ context.Context, _ domain.Session) error    { return nil }
func (fakeSessionRepoForHandlers) DeleteByTokenHash(_ context.Context, _ string) error { return nil }
func (fakeSessionRepoForHandlers) DeleteByUserID(_ context.Context, _ uuid.UUID) error { return nil }
func (fakeSessionRepoForHandlers) DeleteByUserIDExcept(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}
func (fakeSessionRepoForHandlers) DeleteExpiredBefore(_ context.Context, _ time.Time) error {
	return nil
}
func (fakeSessionRepoForHandlers) DeleteExpiredBeforeBatch(_ context.Context, _ time.Time, _ int32) (int64, error) {
	return 0, nil
}
func (fakeSessionRepoForHandlers) WithTx(_ transaction.Tx) application.SessionRepository {
	return fakeSessionRepoForHandlers{}
}

type fakeUserRepoForHandlers struct {
	user    domain.User
	byEmail domain.User
}

func (f *fakeUserRepoForHandlers) GetByID(_ context.Context, _ uuid.UUID) (domain.User, error) {
	return f.user, nil
}
func (f *fakeUserRepoForHandlers) GetByIDForUpdate(_ context.Context, _ uuid.UUID) (domain.User, error) {
	return f.user, nil
}
func (f *fakeUserRepoForHandlers) GetByPhone(_ context.Context, _ domain.Phone) (domain.User, error) {
	return domain.User{}, application.ErrNotFound
}
func (f *fakeUserRepoForHandlers) GetByEmail(_ context.Context, email string) (domain.User, error) {
	if f.byEmail.Email != nil && *f.byEmail.Email == email {
		return f.byEmail, nil
	}
	return domain.User{}, application.ErrNotFound
}
func (f *fakeUserRepoForHandlers) GetPhoneByID(_ context.Context, _ uuid.UUID) (string, error) {
	return "", nil
}
func (f *fakeUserRepoForHandlers) Create(_ context.Context, user domain.User) (domain.User, error) {
	return user, nil
}
func (f *fakeUserRepoForHandlers) Update(_ context.Context, user domain.User) (domain.User, error) {
	return user, nil
}
func (f *fakeUserRepoForHandlers) UpdatePhone(_ context.Context, _ uuid.UUID, _ domain.Phone) (domain.User, error) {
	return domain.User{}, nil
}
func (f *fakeUserRepoForHandlers) UpdateEmailVerified(_ context.Context, _ uuid.UUID, email string, verifiedAt *time.Time) (domain.User, error) {
	f.user.Email = &email
	f.user.EmailVerifiedAt = verifiedAt
	return f.user, nil
}
func (f *fakeUserRepoForHandlers) WithTx(_ transaction.Tx) application.UserRepository { return f }

func TestAuthHandlers_UpdateMe_Returns409ForDuplicateEmail(t *testing.T) {
	phone, _ := domain.NewPhone("+79001234567")
	otherPhone, _ := domain.NewPhone("+79007654321")

	current, _ := domain.NewOwner(phone)
	other, _ := domain.NewOwner(otherPhone)
	email := "user@example.com"
	other.Email = &email

	userRepo := &fakeUserRepoForHandlers{user: current, byEmail: other}
	authSvc := application.NewAuthService(userRepo, nil, nil, fakeSessionRepoForHandlers{}, nil, nil, nil, nil, nil, slog.Default())
	handlers := NewAuthHandlers(authSvc, nil, false, slog.Default(), nil, nil, nil, nil)

	body := []byte(`{"email":"user@example.com"}`)
	ctx := context.WithValue(context.Background(), userIDKey{}, current.ID)
	req := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/me", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handlers.UpdateMe(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", rr.Code)
	}

	var p openapi.Problem
	if err := json.NewDecoder(rr.Body).Decode(&p); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if p.Title != "Conflict" || p.Detail == nil || *p.Detail != "Этот email уже используется" || p.Status != http.StatusConflict {
		t.Fatalf("unexpected problem response: %+v", p)
	}
}
