package wire

import (
	historypg "github.com/nambers/arenda-planform/apps/backend/internal/history/adapters/postgres"
	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
)

// HistoryReadMod is the history module's reading side (карта #704, тикет
// #708, ADR 0061 §7): the feed page and the filter-sheet options.
type HistoryReadMod struct {
	Service *historyapp.HistoryReadService
}

// WireHistoryRead builds the action journal's reading service: the read
// store plus the membership policy. Must run after the policy is installed
// (p.Policy = accessMod.Policy) — the property_ids scope proof resolves
// roles through it; on the owner-only fallback every member's scoped
// request would privacy-404.
func WireHistoryRead(p platformDeps) HistoryReadMod {
	return HistoryReadMod{
		Service: historyapp.NewHistoryReadService(historypg.NewReadStore(p.DB), p.Policy),
	}
}
