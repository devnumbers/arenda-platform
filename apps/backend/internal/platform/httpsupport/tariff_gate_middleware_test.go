package httpsupport

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

// gateLogger swallows the middleware's block warnings in test output.
var gateLogger = slog.New(slog.DiscardHandler)

// fakePaidSectionsGate is the in-memory PaidSectionsGate for middleware tests.
type fakePaidSectionsGate struct {
	canUse bool
	err    error
	calls  int
}

func (g *fakePaidSectionsGate) CanUsePaidFeatures(context.Context, uuid.UUID) (bool, error) {
	g.calls++
	return g.canUse, g.err
}

func paidGateRequest(userID *uuid.UUID) *http.Request {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet,
		"/properties/00000000-0000-7000-8000-000000000001/access/members", nil)
	if userID != nil {
		r = r.WithContext(WithUserID(r.Context(), *userID))
	}
	return r
}

// TestPaidSectionsMiddleware_Allowed passes the request through and consults
// the gate once (карта #997: платный тариф идёт в платный раздел).
func TestPaidSectionsMiddleware_Allowed(t *testing.T) {
	t.Parallel()

	gate := &fakePaidSectionsGate{canUse: true}
	var passed bool
	handler := PaidSectionsMiddleware(gate, gateLogger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		passed = true
		w.WriteHeader(http.StatusOK)
	}))

	userID := uuid.Must(uuid.NewV7())
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, paidGateRequest(&userID))

	if !passed {
		t.Error("request did not reach the handler, want pass-through for a paid tariff")
	}
	if gate.calls != 1 {
		t.Errorf("gate calls = %d, want 1", gate.calls)
	}
}

// TestPaidSectionsMiddleware_BlockedWithoutSubscription proves the gate
// answer: 402 PaymentRequired with the machine code tariff_required — the
// same canonical "needs a paid tariff" status as the property-limit 402.
func TestPaidSectionsMiddleware_BlockedWithoutSubscription(t *testing.T) {
	t.Parallel()

	gate := &fakePaidSectionsGate{canUse: false}
	var passed bool
	handler := PaidSectionsMiddleware(gate, gateLogger)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		passed = true
	}))

	userID := uuid.Must(uuid.NewV7())
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, paidGateRequest(&userID))

	if passed {
		t.Error("request reached the handler, want 402 before it")
	}
	if rec.Code != http.StatusPaymentRequired {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusPaymentRequired)
	}
	var problem struct {
		Title  string  `json:"title"`
		Detail string  `json:"detail"`
		Code   *string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("problem body: %v", err)
	}
	if problem.Code == nil || *problem.Code != "tariff_required" {
		t.Errorf("code = %v, want tariff_required", problem.Code)
	}
	if problem.Title == "" || problem.Detail == "" {
		t.Errorf("title/detail empty: %q / %q", problem.Title, problem.Detail)
	}
}

// TestPaidSectionsMiddleware_GateError is a 500: the gate failing must not
// open the section.
func TestPaidSectionsMiddleware_GateError(t *testing.T) {
	t.Parallel()

	gate := &fakePaidSectionsGate{err: errors.New("billing down")}
	var passed bool
	handler := PaidSectionsMiddleware(gate, gateLogger)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		passed = true
	}))

	userID := uuid.Must(uuid.NewV7())
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, paidGateRequest(&userID))

	if passed {
		t.Error("request reached the handler, want 500 on gate failure")
	}
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// TestPaidSectionsMiddleware_UnauthenticatedSkipsGate leaves the 401 to the
// auth chain downstream: no user id — the gate is never consulted.
func TestPaidSectionsMiddleware_UnauthenticatedSkipsGate(t *testing.T) {
	t.Parallel()

	gate := &fakePaidSectionsGate{canUse: false}
	var passed bool
	handler := PaidSectionsMiddleware(gate, gateLogger)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		passed = true
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, paidGateRequest(nil))

	if !passed {
		t.Error("request did not reach the handler, want pass-through for the auth chain")
	}
	if gate.calls != 0 {
		t.Errorf("gate calls = %d, want 0 without a user id", gate.calls)
	}
}
