package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Дефолт #1330: каждый ответ несёт no-store — эвристический кэш браузера
// больше не переиспользует устаревший 200 после мутаций (наблюдение на
// живой приёмке #1312: хаб показал снятый объект из кэша).
func TestNoStoreCache_DefaultHeader(t *testing.T) {
	t.Parallel()
	handler := noStoreCache()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/properties", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, r)

	if got := rr.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
}

// Хендлер, который хочет кэширование, перекрывает дефолт: фото-эндпоинты
// (ADR 0065) ставят свой private, max-age=300 — Set поверх Set выигрывает.
func TestNoStoreCache_HandlerOverrides(t *testing.T) {
	t.Parallel()
	handler := noStoreCache()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "private, max-age=300")
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/properties/id/photo", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, r)

	if got := rr.Header().Get("Cache-Control"); got != "private, max-age=300" {
		t.Fatalf("Cache-Control = %q, want the photo override", got)
	}
}
