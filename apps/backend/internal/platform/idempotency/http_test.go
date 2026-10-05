package idempotency

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
)

var testLogger = func() *slog.Logger { return slog.New(slog.DiscardHandler) }

const (
	testBody        = `{"id":"p1"}`
	testContentType = "application/json"
)

// Юнит-тест мидлвари на фейковом хранилище (node-канон: без БД) — сам
// сервис идемпотентности покрыт интеграционно. Сессия подставляется в
// контекст тем же WithUserID, что ставит SessionMiddleware.

type fakeStorage struct {
	reserve     func() (Outcome, error)
	completions []completion
	cleanups    int
}

type completion struct {
	key         string
	statusCode  int
	contentType string
	body        string
}

func (f *fakeStorage) Reserve(context.Context, uuid.UUID, string, string, string) (Outcome, error) {
	return f.reserve()
}

func (f *fakeStorage) Complete(_ context.Context, _ uuid.UUID, key string, statusCode int, contentType string, body []byte) error {
	f.completions = append(f.completions, completion{key, statusCode, contentType, string(body)})
	return nil
}

func (f *fakeStorage) Cleanup(context.Context) error {
	f.cleanups++
	return nil
}

func idempotencyRequest(key, body string) *http.Request {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost,
		"/properties/11111111-1111-4111-8111-111111111111/payments", strings.NewReader(body))
	r.Header.Set("Content-Type", testContentType)
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	return r.WithContext(httpsupport.WithUserID(r.Context(), uuid.Must(uuid.NewV7())))
}

func TestMiddleware_NoHeaderPassesThrough(t *testing.T) {
	t.Parallel()
	store := &fakeStorage{}
	called := false
	handler := Middleware(store, testLogger())
	handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	})).ServeHTTP(httptest.NewRecorder(), idempotencyRequest("", `{}`))

	if !called {
		t.Fatal("без заголовка запрос обязан пройти мимо идемпотентности")
	}
	if len(store.completions) != 0 {
		t.Fatalf("Complete не должен зваться без ключа: %+v", store.completions)
	}
}

func TestMiddleware_WinnerExecutesAndCompletes(t *testing.T) {
	t.Parallel()
	store := &fakeStorage{reserve: func() (Outcome, error) { return Outcome{Won: true}, nil }}
	rec := httptest.NewRecorder()
	handler := Middleware(store, testLogger())
	handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", testContentType)
		w.WriteHeader(http.StatusCreated)
		if _, err := w.Write([]byte(testBody)); err != nil {
			t.Errorf("write winner body: %v", err)
		}
	})).ServeHTTP(rec, idempotencyRequest("key-1", `{}`))

	if rec.Code != http.StatusCreated || rec.Body.String() != testBody {
		t.Fatalf("winner response = %d %q", rec.Code, rec.Body.String())
	}
	if len(store.completions) != 1 {
		t.Fatalf("expected exactly one completion, got %+v", store.completions)
	}
	c := store.completions[0]
	if c.statusCode != http.StatusCreated || c.contentType != testContentType || c.body != testBody {
		t.Fatalf("completion = %+v", c)
	}
}

func TestMiddleware_ReplayWithoutExecuting(t *testing.T) {
	t.Parallel()
	store := &fakeStorage{reserve: func() (Outcome, error) {
		return Outcome{Replay: Replay{StatusCode: http.StatusCreated, ContentType: testContentType, Body: []byte(`{"id":"first"}`)}}, nil
	}}
	called := false
	rec := httptest.NewRecorder()
	handler := Middleware(store, testLogger())
	handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
		if _, err := w.Write([]byte(`{"id":"second"}`)); err != nil {
			t.Errorf("write second body: %v", err)
		}
	})).ServeHTTP(rec, idempotencyRequest("key-1", `{}`))

	if called {
		t.Fatal("переигрывание не должно доходить до хендлера — иначе второе создание")
	}
	if rec.Code != http.StatusCreated || rec.Body.String() != `{"id":"first"}` {
		t.Fatalf("replay = %d %q", rec.Code, rec.Body.String())
	}
	if len(store.completions) != 0 {
		t.Fatalf("replay не должен звать Complete: %+v", store.completions)
	}
}

func TestMiddleware_Conflicts(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		reserve func() (Outcome, error)
	}{
		{"бронь в полёте", func() (Outcome, error) { return Outcome{}, ErrKeyInProgress }},
		{"тот же ключ с другим телом", func() (Outcome, error) { return Outcome{}, ErrKeyBodyMismatch }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store := &fakeStorage{reserve: tc.reserve}
			called := false
			rec := httptest.NewRecorder()
			handler := Middleware(store, testLogger())
			handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				called = true
			})).ServeHTTP(rec, idempotencyRequest("key-1", `{}`))

			if called {
				t.Fatal("409 не должен доходить до хендлера")
			}
			if rec.Code != http.StatusConflict {
				t.Fatalf("status = %d, want 409", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), `"status"`) {
				t.Fatalf("ответ не problem+json: %q", rec.Body.String())
			}
		})
	}
}

func TestMiddleware_OversizedKeyRejected(t *testing.T) {
	t.Parallel()
	store := &fakeStorage{}
	called := false
	rec := httptest.NewRecorder()
	handler := Middleware(store, testLogger())
	handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	})).ServeHTTP(rec, idempotencyRequest(strings.Repeat("k", 256), `{}`))

	if called {
		t.Fatal("перекаченный ключ не должен доходить до хендлера")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestMiddleware_BodyRereadableByHandler(t *testing.T) {
	t.Parallel()
	store := &fakeStorage{reserve: func() (Outcome, error) { return Outcome{Won: true}, nil }}
	seen := ""
	handler := Middleware(store, testLogger())
	handler(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 64)
		n, err := r.Body.Read(buf)
		if err != nil && !errors.Is(err, io.EOF) {
			t.Errorf("handler body read: %v", err)
		}
		seen = string(buf[:n])
	})).ServeHTTP(httptest.NewRecorder(), idempotencyRequest("key-1", `{"amount":1500}`))

	if seen != `{"amount":1500}` {
		t.Fatalf("хендлер прочитал тело %q — мидлварь обязан вернуть NopCloser", seen)
	}
}

func TestMiddleware_ReserveFailureIs500(t *testing.T) {
	t.Parallel()
	store := &fakeStorage{reserve: func() (Outcome, error) {
		return Outcome{}, errors.New("db down")
	}}
	called := false
	rec := httptest.NewRecorder()
	handler := Middleware(store, testLogger())
	handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	})).ServeHTTP(rec, idempotencyRequest("key-1", `{}`))

	if called || rec.Code != http.StatusInternalServerError {
		t.Fatalf("reserve failure: called=%v status=%d", called, rec.Code)
	}
}
