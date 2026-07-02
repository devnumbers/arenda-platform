package httpapi

import "net/http"

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(r.Context(), w, http.StatusOK, map[string]string{"status": "ok"})
}
