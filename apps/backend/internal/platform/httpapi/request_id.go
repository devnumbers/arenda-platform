package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// RequestIDHeader is the header used to correlate requests and responses.
const RequestIDHeader = "X-Request-ID"

type requestIDKey struct{}

// RequestIDFromContext returns the request ID stored in the context, if any.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// RequestIDMiddleware ensures every request has a request ID.
// It preserves an incoming X-Request-ID when present and echoes it back in the response.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if id == "" {
			uid, err := uuid.NewRandom()
			if err == nil {
				id = uid.String()
			} else {
				id = fallbackRequestID()
			}
		}
		w.Header().Set(RequestIDHeader, id)
		r = r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id))
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
