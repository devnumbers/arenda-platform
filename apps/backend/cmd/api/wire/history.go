package wire

import (
	billinghttp "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/http"
	historypg "github.com/nambers/arenda-planform/apps/backend/internal/history/adapters/postgres"
	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
)

// HistoryReadMod is the history module's reading side (карта #704, тикет
// #708, ADR 0061 §7): the feed page and the filter-sheet options.
type HistoryReadMod struct {
	Service *historyapp.HistoryReadService
}

// The billing paid-sections gate adapter doubles as the journal's
// tariff-gate port (ADR 0064, ревизия #1465): one adapter, three consumer-
// side ports — the middleware's, the access use cases' and the history
// reads'. This assertion lives in the wiring layer (the only place allowed
// to depend on several bounded contexts at once).
var _ historyapp.PaidSectionsGate = (*billinghttp.PaidSectionsGate)(nil)

// WireHistoryRead builds the action journal's reading service: the read
// store plus the membership policy and the role-aware tariff gate. Must run
// after the policy is installed (p.Policy = accessMod.Policy) — the
// property_ids scope proof resolves roles through it; on the owner-only
// fallback every member's scoped request would privacy-404.
func WireHistoryRead(p platformDeps, billing *Billing) HistoryReadMod {
	return HistoryReadMod{
		Service: historyapp.NewHistoryReadService(historypg.NewReadStore(p.DB), p.Policy, billing.PaidSectionsGate),
	}
}
