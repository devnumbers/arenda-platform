package application

import (
	"log/slog"

	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// Audit and log context keys shared by the billing use cases: the same keys
// appear in audit Entry.Context maps and in slog attribute lists.
const (
	auditKeyPaymentID      = "payment_id"
	auditKeyAmountKopecks  = "amount_kopecks"
	auditKeyProvider       = "provider"
	auditKeyReason         = "reason"
	auditKeyTariffName     = "tariff_name"
	auditKeyFromTariffID   = "from_tariff_id"
	auditKeyToTariffID     = "to_tariff_id"
	auditKeyValidUntil     = "valid_until"
	auditKeyKeepPropertyID = "keep_property_id"
	auditKeyShiftHours     = "shift_hours"
	auditKeyPreset         = "preset"
)

// Services is the billing module's public composition surface: the services
// every consumer (HTTP handlers, event subscribers, cross-context bridges,
// workers) is wired against.
type Services struct {
	Tariffs        *TariffService
	Subscriptions  *SubscriptionService
	Payments       *PaymentService
	PaymentMethods *PaymentMethodService
	Onboarding     *OnboardingService
	// Limiter computes the active-property limit from the user's subscription;
	// the properties and access contexts consume it through their bridge
	// adapters (issue #245 reconnect).
	Limiter *SubscriptionPropertyLimiter
	// Workers drives the lifecycle phases of the scheduler worker shells
	// (issues #245, #252).
	Workers *Workers
	// Config carries the module's operational parameters (issue #244).
	Config Config
}

// ServicesConfig carries the non-transactional dependencies of the billing
// services.
type ServicesConfig struct {
	Config Config
	Clock  clock.Clock
	Logger *slog.Logger
	// Provider is the single active payment provider (ADR 0038). The payment
	// flows of issues #250-#252 consume it; nil keeps the pre-#250 behaviour
	// where paid tariff changes answer ErrPaymentUnavailable.
	Provider PaymentProvider
	// Publisher emits the grace lifecycle events to the notifications context
	// (issue #253); nil keeps the pre-#253 behaviour of no grace
	// notifications.
	Publisher EventPublisher
	// Metrics records the stuck-payment gauges of the worker hygiene phase
	// (ticket #433); nil keeps the phases running without gauges.
	Metrics *Metrics
	// AdminPayments reads the cross-user payment rows behind the admin views
	// (issue #254); nil keeps the admin read methods answered by an explicit
	// wiring error.
	AdminPayments AdminPaymentListing
	// TimeTravelEnabled turns on the stand-only time-travel rig (issue #665):
	// the admin time-shift of the subscription lifecycle. The wiring mounts
	// the rig's endpoints only under the platform's BILLING_TIME_TRAVEL flag,
	// whose railguard keeps production off; the service-level flag is the
	// second layer — a service built without it refuses the operation even if
	// a future caller reaches it past the routes.
	TimeTravelEnabled bool
}

// NewServices builds every billing service over one shared txStoreFactory
// (ADR 0033 γ-factory). Nil clock and logger default to the real clock and the
// default logger, matching the identity module's constructor conventions.
func NewServices(factory txStoreFactory, cfg ServicesConfig) Services {
	if cfg.Clock == nil {
		cfg.Clock = clock.Real{}
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	payments := NewPaymentService(factory, cfg.Provider, PaymentServiceConfig{
		Clock:         cfg.Clock,
		Log:           cfg.Logger,
		Config:        cfg.Config,
		Publisher:     cfg.Publisher,
		AdminPayments: cfg.AdminPayments,
	})
	return Services{
		Tariffs: NewTariffService(factory, TariffServiceConfig{Log: cfg.Logger}),
		Subscriptions: NewSubscriptionService(factory, SubscriptionServiceConfig{
			Clock: cfg.Clock, Provider: cfg.Provider, Config: cfg.Config, Logger: cfg.Logger,
			TimeTravelEnabled: cfg.TimeTravelEnabled,
			Publisher:         cfg.Publisher,
		}),
		Payments: payments,
		PaymentMethods: NewPaymentMethodService(factory, cfg.Provider, PaymentMethodServiceConfig{
			Config: cfg.Config, Clock: cfg.Clock, Log: cfg.Logger,
		}),
		Onboarding: NewOnboardingService(factory, OnboardingServiceConfig{Logger: cfg.Logger}),
		Limiter:    NewSubscriptionPropertyLimiter(factory.subscriptions, factory.tariffs, cfg.Clock),
		Workers: NewWorkers(factory, WorkersConfig{
			Provider:  cfg.Provider,
			Payments:  payments,
			Clock:     cfg.Clock,
			Metrics:   cfg.Metrics,
			Config:    cfg.Config,
			Logger:    cfg.Logger,
			Publisher: cfg.Publisher,
		}),
		Config: cfg.Config,
	}
}
