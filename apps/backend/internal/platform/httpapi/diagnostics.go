package httpapi

import "net/http"

type DBPoolSnapshot struct {
	Available              bool    `json:"available"`
	AcquiredConns          int32   `json:"acquired_conns"`
	IdleConns              int32   `json:"idle_conns"`
	TotalConns             int32   `json:"total_conns"`
	ConstructingConns      int32   `json:"constructing_conns"`
	MaxConns               int32   `json:"max_conns"`
	AcquireCount           int64   `json:"acquire_count"`
	AcquireDurationMS      float64 `json:"acquire_duration_ms"`
	CanceledAcquireCount   int64   `json:"canceled_acquire_count"`
	EmptyAcquireCount      int64   `json:"empty_acquire_count"`
	EmptyAcquireWaitTimeMS float64 `json:"empty_acquire_wait_time_ms"`
	NewConnsCount          int64   `json:"new_conns_count"`
}

func dbPoolDiagnosticsHandler(stats func() DBPoolSnapshot) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if stats == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		snapshot := stats()
		snapshot.Available = true
		writeJSON(r.Context(), w, http.StatusOK, snapshot)
	}
}
