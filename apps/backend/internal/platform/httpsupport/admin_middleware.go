package httpsupport

import (
	"net/http"

	identitydomain "github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

// AdminOnlyMiddleware allows only authenticated users with the admin role.
func AdminOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFromContext(r.Context())
		if !ok {
			WriteProblem(w, http.StatusUnauthorized, Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
			return
		}
		if user.Role != identitydomain.RoleAdmin {
			WriteProblem(w, http.StatusForbidden, Problem(r.Context(), "Forbidden", "Требуются права администратора"))
			return
		}
		next.ServeHTTP(w, r)
	})
}
