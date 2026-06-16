package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"

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

func fallbackRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Last resort: return zeros; this should never happen in practice.
		return hex.EncodeToString(b)
	}
	return hex.EncodeToString(b)
}
