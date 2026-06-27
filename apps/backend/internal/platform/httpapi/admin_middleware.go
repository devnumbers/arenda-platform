package httpapi

import (
	"net/http"

	identitydomain "github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

// AdminOnlyMiddleware allows only authenticated users with the admin role.
func AdminOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFromContext(r.Context())
		if !ok {
			writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
			return
		}
		if user.Role != identitydomain.RoleAdmin {
			writeProblem(w, http.StatusForbidden, problem(r.Context(), "Forbidden", "admin role required"))
			return
		}
		next.ServeHTTP(w, r)
	})
}
