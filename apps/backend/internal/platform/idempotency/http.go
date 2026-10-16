package idempotency

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
)

// maxKeyLength mirrors the de-facto header convention (Stripe): a key is at
// most 255 bytes — beyond that the client is broken, not the storage.
const maxKeyLength = 255

// cleanupProbability gates the TTL sweep: ~1% of keyed requests run the
// 24h delete — no dedicated scheduler, the storage stays bounded. The draw
// is crypto/rand not math/rand: one off-the-shelf CSPRNG instead of a
// gosec G404 exception for a number that does not need to be fast.
const cleanupProbability = 0.01

// randomFloat01 draws a uniform [0,1) float from crypto/rand.
func randomFloat01() (float64, error) {
	var b [2]byte
	if _, err := cryptorand.Read(b[:]); err != nil {
		return 0, err
	}
	return float64(binary.BigEndian.Uint16(b[:])) / 65536.0, nil
}

// storage is the seam the middleware needs; *Service implements it.
type storage interface {
	Reserve(ctx context.Context, ownerID uuid.UUID, key, endpoint, requestHash string) (Outcome, error)
	Complete(ctx context.Context, ownerID uuid.UUID, key string, statusCode int, contentType string, body []byte) error
}

// Middleware applies idempotency-key semantics to a creation route. A request
// without the header passes through untouched (the key is optional — the
// convention keeps non-keyed clients working). A keyed request reserves its
// key first: a replay returns the stored response, the parallel-same-key race
// and the «same key, different body» client bug answer 409. Keys are scoped
// per owner — one user's key never replays another user's result.
func Middleware(store storage, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}
			serveKeyed(next, store, logger, w, r, key)
		})
	}
}

// serveKeyed runs the reserve-execute-or-replay flow for a keyed request.
func serveKeyed(next http.Handler, store storage, logger *slog.Logger, w http.ResponseWriter, r *http.Request, key string) {
	if len(key) > maxKeyLength {
		writeProblem(w, r, http.StatusBadRequest, "Заголовок Idempotency-Key длиннее 255 символов")
		return
	}
	ownerID, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		// Unreachable behind the session middleware; the safe fallback
		// is executing without idempotency rather than a 500.
		next.ServeHTTP(w, r)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Не удалось прочитать тело запроса")
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	hash := sha256.Sum256(body)

	outcome, err := store.Reserve(r.Context(), ownerID, key, routePattern(r), hex.EncodeToString(hash[:]))
	if err != nil {
		if !writeReserveConflict(w, r, err) {
			logger.ErrorContext(r.Context(), "idempotency reserve failed", slog.String("error", err.Error()))
			writeProblem(w, r, http.StatusInternalServerError, "Внутренняя ошибка сервера")
		}
		return
	}
	if outcome.Replay.StatusCode != 0 {
		// Переигрывание не доходит до хендлера — второе создание исключено.
		writeReplay(r.Context(), w, outcome.Replay)
		return
	}

	capture := &captureResponseWriter{ResponseWriter: w}
	next.ServeHTTP(capture, r)
	completeAndSweep(store, logger, r, ownerID, key, capture)
}

// writeReserveConflict maps the two domain reserve failures to 409 problem
// responses; anything else is infrastructural (reported false — caller logs
// and answers 500).
func writeReserveConflict(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, ErrKeyInProgress):
		writeProblem(w, r, http.StatusConflict, "Запрос с этим ключом уже выполняется")
		return true
	case errors.Is(err, ErrKeyBodyMismatch):
		writeProblem(w, r, http.StatusConflict, "Ключ идемпотентности уже использован с другим телом запроса")
		return true
	default:
		return false
	}
}

// completeAndSweep stores the winner's response and, with ~1% probability,
// runs the 24h TTL sweep. Complete is best-effort: the request already
// executed, a failed store only means this key replays 409 instead of the
// result — the client generates a fresh key per attempt, so log-worthy only.
func completeAndSweep(store storage, logger *slog.Logger, r *http.Request, ownerID uuid.UUID, key string, capture *captureResponseWriter) {
	contentType := capture.Header().Get("Content-Type")
	if err := store.Complete(r.Context(), ownerID, key, capture.status, contentType, capture.body.Bytes()); err != nil {
		logger.ErrorContext(r.Context(), "idempotency complete failed", slog.String("error", err.Error()))
	}
	sweepExpired(store, logger, r)
}

// sweepExpired runs the 24h TTL delete with ~1% probability per keyed
// request; a failed draw skips the sweep (log-worthy only).
func sweepExpired(store storage, logger *slog.Logger, r *http.Request) {
	draw, err := randomFloat01()
	if err != nil || draw >= cleanupProbability {
		if err != nil {
			logger.ErrorContext(r.Context(), "idempotency cleanup draw failed", slog.String("error", err.Error()))
		}
		return
	}
	cleaner, ok := store.(interface {
		Cleanup(ctx context.Context) error
	})
	if !ok {
		return
	}
	if err := cleaner.Cleanup(r.Context()); err != nil {
		logger.ErrorContext(r.Context(), "idempotency cleanup failed", slog.String("error", err.Error()))
	}
}

// routePattern prefers the chi route template («/properties/{propertyId}/
// payments») over the concrete path — the endpoint column stays stable
// across property ids.
func routePattern(r *http.Request) string {
	if routeCtx := chi.RouteContext(r.Context()); routeCtx != nil {
		if pattern := routeCtx.RoutePattern(); pattern != "" {
			return pattern
		}
	}
	return r.URL.Path
}

func writeReplay(ctx context.Context, w http.ResponseWriter, replay Replay) {
	w.Header().Set("Content-Type", replay.ContentType)
	w.WriteHeader(replay.StatusCode)
	if _, err := w.Write(replay.Body); err != nil {
		// Канон WriteJSON: сбой записи клиенту (обрыв соединения) логируется.
		httpsupport.LoggerFromContext(ctx).ErrorContext(ctx, "failed to write replay response",
			slog.String("error", httpsupport.SanitizeError(err)))
	}
}

func writeProblem(w http.ResponseWriter, r *http.Request, status int, detail string) {
	problem := httpsupport.Problem(r.Context(), http.StatusText(status), detail)
	httpsupport.WriteProblem(r.Context(), w, status, problem)
}

// captureResponseWriter records status and body of the executed handler so
// Complete can store them; bytes still stream to the client write-through.
type captureResponseWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (w *captureResponseWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *captureResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	_, _ = w.body.Write(p)
	return w.ResponseWriter.Write(p)
}
