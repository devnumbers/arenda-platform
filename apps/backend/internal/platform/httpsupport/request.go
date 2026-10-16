package httpsupport

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"strconv"

	"github.com/google/uuid"
)

const MaxRequestBodySize = 16 << 10 // 16 KiB.

// RetryAfterSeconds is the cooldown clients should wait before retrying a
// rate-limited auth request. It matches the service-layer minSendInterval.
const RetryAfterSeconds = 60

// errContentTypeNotJSON reports a request whose Content-Type is not
// application/json. A cross-site HTML form can only send the simple content
// types, so the check closes the JSON-parsing CSRF side door that SameSite
// cannot reach (ADR 0056).
var errContentTypeNotJSON = errors.New("request Content-Type is not application/json")

// IsContentTypeNotJSON reports whether err is the Content-Type sentinel the
// JSON body decoder rejects with.
func IsContentTypeNotJSON(err error) bool { return errors.Is(err, errContentTypeNotJSON) }

func DecodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) error {
	if ct := r.Header.Get("Content-Type"); ct != "" {
		if mediaType, _, err := mime.ParseMediaType(ct); err != nil || mediaType != "application/json" {
			return errContentTypeNotJSON
		}
	} else if r.ContentLength != 0 {
		// No declared type: only an empty body (the optional-body endpoints)
		// is tolerated; anything else must declare application/json.
		return errContentTypeNotJSON
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodySize)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func WriteJSON(ctx context.Context, w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		LoggerFromContext(ctx).ErrorContext(ctx, "failed to encode JSON response", slog.String("error", SanitizeError(err)))
	}
}

// WriteTooManyRequests writes a 429 RFC 7807 problem response with a
// Retry-After header so clients can back off before retrying.
func WriteTooManyRequests(w http.ResponseWriter, r *http.Request, detail string) {
	w.Header().Set("Retry-After", strconv.Itoa(RetryAfterSeconds))
	WriteProblem(r.Context(), w, http.StatusTooManyRequests, Problem(r.Context(), "Too many requests", detail))
}

// UserFacingDetailOrDefault returns a user-facing message for err if one is
// defined, otherwise returns the provided default.
func UserFacingDetailOrDefault(err error, defaultDetail string) string {
	if detail, ok := UserFacingDetail(err); ok {
		return detail
	}
	return defaultDetail
}

// RequireUser resolves the acting user from the request context and writes
// the canonical 401 problem itself when the session middleware stored none.
// The bool verdict is the handler's early-return signal:
//
//	actor, ok := httpsupport.RequireUser(w, r)
//	if !ok { return }
//
// One home for the preamble every authenticated handler otherwise repeats.
func RequireUser(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, ok := UserIDFromContext(r.Context())
	if !ok {
		WriteProblem(r.Context(), w, http.StatusUnauthorized,
			Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return uuid.Nil, false
	}
	return id, true
}
