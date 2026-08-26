package wire

import (
	paymentspg "github.com/nambers/arenda-planform/apps/backend/internal/payments/adapters/postgres"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
)

// Payments holds the payments module's services wired by WirePayments: the
// payment rule use cases now and the materialization tick service — the
// worker's single door to the tick (ticket #458 registers it in the
// scheduler).
type Payments struct {
	PaymentService *paymentsapp.PaymentService
	TickService    *paymentsapp.TickService
}

// WirePayments constructs the payments context (ADR 0049): the tick store,
// the payment and property stores, the owner calendar (ADR 0048), the
// single canonical txStoreFactory shared by every payments service
// (ADR 0033 γ-factory) and the tick service the future worker drives. The
// policy comes from the access module — payments is wired after it, so the
// membership-aware policy is already resolved.
func WirePayments(p platformDeps) *Payments {
	tickStore := paymentspg.NewTickStore(p.DB)
	paymentStore := paymentspg.NewPaymentStore(p.DB)
	propertyStore := paymentspg.NewPropertyStore(p.DB)
	calendar := paymentspg.NewOwnerCalendar(p.DB, p.Clock)

	factory := paymentsapp.NewTxStoreFactory(
		tickStore,
		paymentStore,
		propertyStore,
		p.AuditRecorder,
		p.UoW,
	)

	return &Payments{
		PaymentService: paymentsapp.NewPaymentService(factory, calendar, p.Policy),
		TickService:    paymentsapp.NewTickService(factory, calendar),
	}
}
