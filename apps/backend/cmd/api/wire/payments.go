package wire

import (
	"fmt"

	paymentspg "github.com/nambers/arenda-planform/apps/backend/internal/payments/adapters/postgres"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
)

// Payments holds the payments module's services wired by WirePayments: the
// payment rule use cases, the operation use cases (pay now and the listings,
// ticket #461), the global payment rules reads (ticket #575) and the
// materialization tick service — the worker's single door to the tick
// (ticket #458 registers it in the scheduler).
type Payments struct {
	PaymentService   *paymentsapp.PaymentService
	OperationService *paymentsapp.OperationService
	GlobalPayments   *paymentsapp.GlobalPaymentService
	TickService      *paymentsapp.TickService
}

// WirePayments constructs the payments context (ADR 0049): the tick store,
// the payment and property stores, the owner calendar and the tick zone
// directory (ADR 0048), the single canonical txStoreFactory shared by every
// payments service (ADR 0033 γ-factory), the heartbeat metrics of the tick
// sweep (ticket #458) and the tick service the hourly worker drives. The
// policy comes from the access module — payments is wired after it, so the
// membership-aware policy is already resolved.
func WirePayments(p platformDeps) (*Payments, error) {
	tickStore := paymentspg.NewTickStore(p.DB)
	paymentStore := paymentspg.NewPaymentStore(p.DB)
	operationStore := paymentspg.NewOperationStore(p.DB)
	propertyStore := paymentspg.NewPropertyStore(p.DB)
	calendar := paymentspg.NewOwnerCalendar(p.DB, p.Clock)
	zones := paymentspg.NewTickZoneDirectory(p.DB)
	globalPayments := paymentspg.NewGlobalPaymentStore(p.DB)

	factory := paymentsapp.NewTxStoreFactory(
		tickStore,
		paymentStore,
		operationStore,
		propertyStore,
		globalPayments,
		p.AuditRecorder,
		p.UoW,
	)

	metrics, err := paymentsapp.NewMetrics()
	if err != nil {
		return nil, fmt.Errorf("payments tick metrics: %w", err)
	}

	return &Payments{
		PaymentService:   paymentsapp.NewPaymentService(factory, calendar, p.Policy),
		OperationService: paymentsapp.NewOperationService(factory, calendar, p.Policy),
		GlobalPayments:   paymentsapp.NewGlobalPaymentService(globalPayments, calendar, factory),
		TickService:      paymentsapp.NewTickService(factory, zones, calendar, metrics),
	}, nil
}
