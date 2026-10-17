package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
)

// stubAddCardSyncer records the sync calls the add-card return route makes.
type stubAddCardSyncer struct {
	calls   int
	userIDs []uuid.UUID
	err     error
}

func (s *stubAddCardSyncer) SyncPaymentMethods(_ context.Context, userID uuid.UUID) ([]domain.PaymentMethod, error) {
	s.calls++
	s.userIDs = append(s.userIDs, userID)
	return nil, s.err
}

// TestAddCardReturnHandler_SyncsBeforeRedirect proves the success return
// materializes the completed binding before the browser is sent to the
// frontend (issue #1514): the provider's notification retries run hourly, so
// the browser's own return is the earliest reliable completion moment. The
// redirect must happen either way.
func TestAddCardReturnHandler_SyncsBeforeRedirect(t *testing.T) {
	t.Parallel()
	syncer := &stubAddCardSyncer{}
	handler := addCardReturnHandler("https://app.example", syncer, slog.New(slog.DiscardHandler), true)

	userID := uuid.Must(uuid.NewV7())
	r := httptest.NewRequestWithContext(
		httpsupport.WithUserID(context.Background(), userID),
		http.MethodGet, "/api/subscription/payment-methods/add-card/success", nil,
	)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, r)

	if syncer.calls != 1 || len(syncer.userIDs) != 1 || syncer.userIDs[0] != userID {
		t.Fatalf("sync calls = %d user ids = %v, want one sync for %v", syncer.calls, syncer.userIDs, userID)
	}
	if rr.Code != http.StatusFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusFound)
	}
	if loc := rr.Header().Get("Location"); loc != httpsupport.AddCardReturnURL("https://app.example", httpsupport.AddCardResultSuccess) {
		t.Errorf("Location = %q, want the frontend payment-methods page", loc)
	}
}

// TestAddCardReturnHandler_AnonymousReturnSkipsSync proves a return without a
// session cookie redirects untouched: the bank's redirect may outlive the
// session, and the frontend mount-sync remains the fallback there.
func TestAddCardReturnHandler_AnonymousReturnSkipsSync(t *testing.T) {
	t.Parallel()
	syncer := &stubAddCardSyncer{}
	handler := addCardReturnHandler("https://app.example", syncer, slog.New(slog.DiscardHandler), true)

	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/subscription/payment-methods/add-card/success", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, r)

	if syncer.calls != 0 {
		t.Errorf("sync calls = %d, want 0 for an anonymous return", syncer.calls)
	}
	if rr.Code != http.StatusFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusFound)
	}
}

// TestAddCardReturnHandler_SyncFailureStillRedirects proves a failed sync
// cannot break the return: the redirect is the route's contract, the sync is
// opportunistic healing.
func TestAddCardReturnHandler_SyncFailureStillRedirects(t *testing.T) {
	t.Parallel()
	syncer := &stubAddCardSyncer{err: context.DeadlineExceeded}
	handler := addCardReturnHandler("https://app.example", syncer, slog.New(slog.DiscardHandler), true)

	userID := uuid.Must(uuid.NewV7())
	r := httptest.NewRequestWithContext(
		httpsupport.WithUserID(context.Background(), userID),
		http.MethodGet, "/api/subscription/payment-methods/add-card/success", nil,
	)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, r)

	if syncer.calls != 1 {
		t.Errorf("sync calls = %d, want 1", syncer.calls)
	}
	if rr.Code != http.StatusFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusFound)
	}
}

// TestAddCardReturnHandler_FailRouteNeverSyncs proves the fail return stays a
// pure redirect: a refused binding has nothing to materialize, the frontend
// mount-sync closes its session on the next screen visit.
func TestAddCardReturnHandler_FailRouteNeverSyncs(t *testing.T) {
	t.Parallel()
	syncer := &stubAddCardSyncer{}
	handler := addCardReturnHandler("https://app.example", syncer, slog.New(slog.DiscardHandler), false)

	userID := uuid.Must(uuid.NewV7())
	r := httptest.NewRequestWithContext(
		httpsupport.WithUserID(context.Background(), userID),
		http.MethodGet, "/api/subscription/payment-methods/add-card/fail", nil,
	)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, r)

	if syncer.calls != 0 {
		t.Errorf("sync calls = %d, want 0 on the fail route", syncer.calls)
	}
	if rr.Code != http.StatusFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusFound)
	}
	if loc := rr.Header().Get("Location"); loc != httpsupport.AddCardReturnURL("https://app.example", httpsupport.AddCardResultFailed) {
		t.Errorf("Location = %q, want the fail return", loc)
	}
}
