package httpsupport

import "net/http"

func HealthHandler(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		WriteJSON(r.Context(), w, http.StatusOK, map[string]string{"status": "ok", "version": version})
	}
}
