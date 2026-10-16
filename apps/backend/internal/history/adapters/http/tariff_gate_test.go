package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
)

// The wire contract of the role-aware tariff gate (ADR 0064, ревизия #1465):
// the handlers map the application's ErrTariffRequired onto 402 with the
// machine code "tariff_required" — the exact body the route middleware wrote
// before the gate moved into the use cases, so clients cannot tell the seam
// moved.
func TestHistoryHandlers_TariffRequiredIs402WithMachineCode(t *testing.T) {
	t.Parallel()
	h, _ := newHistoryTestHandlers()
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/history", http.NoBody)

	h.writeError(req, rec, historyapp.ErrTariffRequired)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402", rec.Code)
	}
	var problem struct {
		Title  string  `json:"title"`
		Detail string  `json:"detail"`
		Code   *string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if problem.Code == nil || *problem.Code != "tariff_required" {
		t.Errorf("code = %v, want tariff_required", problem.Code)
	}
	if problem.Detail != "Этот раздел доступен на платных тарифах" {
		t.Errorf("detail = %q, want the middleware's text", problem.Detail)
	}
}

// TestHistoryHandlers_WrappedTariffErrorIsMatched pins the errors.Is
// matching: the gate wraps storage failures, the sentinel verdict may arrive
// wrapped too.
func TestHistoryHandlers_WrappedTariffErrorIsMatched(t *testing.T) {
	t.Parallel()
	h, _ := newHistoryTestHandlers()
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/history", http.NoBody)

	h.writeError(req, rec, errors.Join(historyapp.ErrTariffRequired))

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402 for a wrapped sentinel", rec.Code)
	}
}
