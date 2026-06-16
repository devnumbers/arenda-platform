package httpapi

import (
	"net/http"

	"github.com/google/uuid"
)

// ownerIDFromContext extracts the authenticated user ID from the request context.
// It returns false (without writing a response) when the user is not authenticated.
// Callers should write their own unauthorized response.
func ownerIDFromContext(r *http.Request) (uuid.UUID, bool) {
	userID, ok := UserIDFromContext(r.Context())
	return userID, ok
}
