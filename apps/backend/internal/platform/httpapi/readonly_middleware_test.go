package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

func newReadonlyMiddleware(t *testing.T, status domain.SubscriptionStatus) http.Handler {
	t.Helper()
	d := newHandlerTestDeps(t)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	basicID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	d.tariffs.add(domain.Tariff{ID: basicID, Name: domain.TariffBasic, ActivePropertyLimit: 5})
	if status != "" {
		sub := domain.Subscription{
			ID:       uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"),
			UserID:   userID,
			TariffID: basicID,
			Source:   domain.SubscriptionSourcePaid,
			Status:   status,
		}
		if status == domain.SubscriptionStatusGrace {
			graceUntil := time.Now().Add(24 * time.Hour)
			sub.ValidUntil = &graceUntil
		}
		d.subscriptions.add(sub)
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("next"))
	})

	return readonlyMiddleware(d.service, slog.New(slog.DiscardHandler))(next)
}

func TestReadonlyMiddleware_AllowsReadWhenBlocked(t *testing.T) {
	h := newReadonlyMiddleware(t, domain.SubscriptionStatusBlocked)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	req := withUserID(httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/properties", nil), userID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusOK)
}

func TestReadonlyMiddleware_BlocksMutationWhenBlocked(t *testing.T) {
	h := newReadonlyMiddleware(t, domain.SubscriptionStatusBlocked)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	req := withUserID(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/properties", nil), userID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusForbidden)
}

func TestReadonlyMiddleware_AllowsMutationWhenActive(t *testing.T) {
	h := newReadonlyMiddleware(t, domain.SubscriptionStatusActive)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	req := withUserID(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/properties", nil), userID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusOK)
}

func TestReadonlyMiddleware_AllowsMutationWhenNoSubscription(t *testing.T) {
	h := newReadonlyMiddleware(t, "")
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	req := withUserID(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/properties", nil), userID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusOK)
}

func TestReadonlyMiddleware_ExemptsSubscriptionPaths(t *testing.T) {
	h := newReadonlyMiddleware(t, domain.SubscriptionStatusBlocked)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	req := withUserID(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/subscription/change", nil), userID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusOK)
}

func TestReadonlyMiddleware_ExemptsAuthPaths(t *testing.T) {
	h := newReadonlyMiddleware(t, domain.SubscriptionStatusBlocked)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	req := withUserID(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/auth/logout", nil), userID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusOK)
}


func TestReadonlyMiddleware_BlocksMutationWhenCancelled(t *testing.T) {
	h := newReadonlyMiddleware(t, domain.SubscriptionStatusCancelled)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	req := withUserID(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/properties", nil), userID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusForbidden)
}

func TestReadonlyMiddleware_AllowsMutationInGrace(t *testing.T) {
	h := newReadonlyMiddleware(t, domain.SubscriptionStatusGrace)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	req := withUserID(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/properties", nil), userID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusOK)
}

func TestReadonlyMiddleware_ExemptsPaymentMethods(t *testing.T) {
	h := newReadonlyMiddleware(t, domain.SubscriptionStatusBlocked)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	req := withUserID(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/subscription/payment-methods", nil), userID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusOK)
}


func TestReadonlyMiddleware_DoesNotExemptPrefixSubpaths(t *testing.T) {
	h := newReadonlyMiddleware(t, domain.SubscriptionStatusBlocked)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	req := withUserID(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/subscription/change-plan", nil), userID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusForbidden)
}
