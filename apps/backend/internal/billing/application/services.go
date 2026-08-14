package application

import (
	"log/slog"

	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
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
	payments := NewPaymentService(factory, cfg.Provider, PaymentServiceConfig{Clock: cfg.Clock, Log: cfg.Logger, Config: cfg.Config})
	return Services{
		Tariffs:        NewTariffService(factory),
		Subscriptions:  NewSubscriptionService(factory, SubscriptionServiceConfig{Clock: cfg.Clock, Provider: cfg.Provider, Config: cfg.Config, Logger: cfg.Logger}),
		Payments:       payments,
		PaymentMethods: NewPaymentMethodService(factory, cfg.Provider, PaymentMethodServiceConfig{Config: cfg.Config, Clock: cfg.Clock, Log: cfg.Logger}),
		Onboarding:     NewOnboardingService(factory, OnboardingServiceConfig{Logger: cfg.Logger}),
		Limiter:        NewSubscriptionPropertyLimiter(factory.subscriptions, factory.tariffs, cfg.Clock),
		Workers: NewWorkers(factory, WorkersConfig{
			Provider: cfg.Provider,
			Payments: payments,
			Clock:    cfg.Clock,
			Config:   cfg.Config,
			Logger:   cfg.Logger,
		}),
		Config: cfg.Config,
	}
}
