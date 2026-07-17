package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/requestctx"
)

// RequestIDHeader is the header used to correlate requests and responses.
const RequestIDHeader = "X-Request-ID"

const maxRequestIDLength = 64

// requestIDPattern allows UUIDs or alphanumeric/hyphen identifiers.
var requestIDPattern = regexp.MustCompile(`^[0-9a-fA-F-]+$|^[a-zA-Z0-9-]+$`)

func isValidRequestID(id string) bool {
	if id == "" || len(id) > maxRequestIDLength {
		return false
	}
	return requestIDPattern.MatchString(id)
}

// RequestIDFromContext returns the request ID stored in the context, if any.
func RequestIDFromContext(ctx context.Context) string {
	return requestctx.RequestIDFromContext(ctx)
}

// TraceIDFromContext returns the local trace ID stored in the context, if any.
func TraceIDFromContext(ctx context.Context) string {
	return requestctx.TraceIDFromContext(ctx)
}

// RequestIDMiddleware ensures every request has a request ID.
// It preserves an incoming X-Request-ID when present and echoes it back in the response.
// Untrusted or malformed values are replaced with a generated ID.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if !isValidRequestID(id) {
			uid, err := uuid.NewV7()
			if err == nil {
				id = uid.String()
			} else {
				id = fallbackRequestID()
			}
		}
		w.Header().Set(RequestIDHeader, id)
		ctx := requestctx.WithRequestID(r.Context(), id)
		ctx = requestctx.WithTraceID(ctx, id)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

var fallbackCounter atomic.Uint64

func fallbackRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err == nil {
		return hex.EncodeToString(b)
	}

	ts := uint64(time.Now().UTC().UnixNano())
	n := fallbackCounter.Add(1)
	return fmt.Sprintf("%016x%016x", ts, n)
}
