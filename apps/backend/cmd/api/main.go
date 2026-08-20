package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/cmd/api/wire"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	emailnotifier "github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/email"
	notificationspg "github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/postgres"
	webpush "github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/webpush"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/config"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/events"
	platformgenerated "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpserver"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
)

func main() {
	fallback := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	if err := run(); err != nil {
		// The fallback logger exists before any context does, so the record is
		// anchored to context.Background() — the only ctx available in main.
		fallback.ErrorContext(context.Background(), "backend stopped", "error", err)
		os.Exit(1)
	}
}

// run is the backend's composition root. It wires platform resources and the
// per-module services (via the cmd/api/wire constructors) in dependency order,
// registers the cross-module event subscribers, builds the HTTP handler and
// runs the server with graceful shutdown. The construction order, event
// subscriptions, worker goroutines and server lifecycle are unchanged from the
// previous monolithic version — only the construction bodies moved into wire
// and the cross-module wiring steps into the named functions below.
func run() error {
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		return runMigrate()
	}

	// 1. Platform: config, logger, signal ctx, OTel, renderer, encryptor,
	//    auto-migrate, db pool, audit recorder, tz resolver.
	platform, ctx, err := wire.WirePlatform()
	if err != nil {
		return err
	}
	defer platform.Cleanup()
	p := platform.Deps

	// 2. Event dispatcher (shared by identity publisher and subscribers).
	eventDispatcher := events.NewInProcessDispatcher()

	// 3. Notifications: reminder/calendar/preference services +
	//    reminder scheduler. Built before identity because Profile depends on
	//    the reminder service, and before leases because the reminder scheduler
	//    feeds the property billing lifecycle and lease/operation services.
	notificationsMod := wire.WireNotifications(p)

	// 4. Identity: repos, session service, event publisher, email mailer
	//    switch, auth/phone-change/profile/logout services.
	identityMod, err := wire.WireIdentity(ctx, p, eventDispatcher, notificationsMod.ReminderService)
	if err != nil {
		return err
	}

	// 5. Billing module (rewritten core, issue #245): repositories, UoW
	//    factory, tariff/subscription/onboarding services, limiter and worker
	//    shells. Built before access and properties because their subscription
	//    limiters consume the billing repositories. The grace lifecycle events
	//    (issue #253) go through the shared event dispatcher.
	billingMod, err := wire.WireBilling(ctx, p, eventDispatcher)
	if err != nil {
		return err
	}

	// 6. Leases repos: operation/recurring/lease/category repos, category
	//    service, property billing lifecycle. Built before properties because
	//    PropertyService depends on the lifecycle and the shared lease repo.
	leasesRepos := wire.WireLeasesRepos(p, notificationsMod.ReminderScheduler)

	// 7. Access (T3, issue #156): membership repository, property owner
	//    resolver, membership-aware policy and access service. Built before
	//    properties because the policy replaces the T2 owner-only policy and is
	//    injected into every property/lease/operation service.
	accessMod, err := wire.WireAccess(ctx, p, billingMod, identityMod.EmailMailer)
	if err != nil {
		return err
	}
	p.Policy = accessMod.Policy
	injectAccessPolicy(notificationsMod, leasesRepos, accessMod)

	// 8. Properties: repos, subscription limiter, photo storage, property and
	//    property-contact services, dadata suggester.
	propertiesMod, err := wire.WireProperties(ctx, p, billingMod, leasesRepos)
	if err != nil {
		return err
	}
	injectPropertyServiceAccess(propertiesMod, accessMod)
	setBillingLifecycleBridges(billingMod, propertiesMod, accessMod)

	// 9. Cross-module user_registered subscribers.
	subscribeUserRegistered(eventDispatcher, billingMod, leasesRepos, accessMod)

	// 10. Grace notifications (issue #253): the billing grace events deliver
	//     through the notifications context over push and email, honouring the
	//     per-channel preferences (ADR 0030). Subscribers are registered before
	//     the workers start so no grace event fires unwired.
	pushSender, err := newPushSender(ctx, p.Cfg, p.Logger)
	if err != nil {
		return err
	}
	graceNotifier := newGraceNotifier(p.DB, p.Renderer, p.Cfg, p.Logger, notificationsMod, identityMod, pushSender)
	subscribeGraceEvents(eventDispatcher, graceNotifier)

	// 11. Admin service (depends on billing subscriptions + occupancy provider).
	adminMod := wire.WireAdmin(p, billingMod.Services.Subscriptions, propertiesMod.OccupancyProvider)

	// 12. Leases services: lease/operation/recurring/tenant-contact/export.
	leasesMod := wire.WireLeasesServices(p, leasesRepos, notificationsMod.ReminderScheduler, notificationsMod.ReminderService)
	injectCrossContextScopes(leasesMod, propertiesMod, notificationsMod, accessMod)

	// 13. Popups service.
	popupsMod := wire.WirePopups(p)

	// 14. Background workers (6 goroutines). Started before the HTTP server so
	//     they are live while serving. The Web Push sender was constructed in
	//     step 10 together with the grace notification delivery; without VAPID
	//     keys the reminder worker runs email-only and push dispatch is skipped
	//     (guard in reminder_worker).
	workers := wire.NewWorkers(
		ctx, p,
		leasesMod.LeaseService,
		leasesMod.OperationService,
		notificationsMod.ReminderRepo,
		identityMod.SessionRepo,
		identityMod.CodeRepo,
		identityMod.AttemptRepo,
		identityMod.EmailMailer,
		billingMod.Services.Workers,
		notificationsMod.PushSubscriptionRepo,
		pushSender,
	)

	// 15. HTTP rate limiters.
	limiters := wire.WireRateLimiters(p.Cfg)
	defer limiters.Stop()

	poolStats := newPoolStats(p.Cfg, p.Pool)

	// 16. HTTP handler + server.
	handler := httpserver.New(httpserver.Deps{
		Auth:                     identityMod.Authentication,
		PhoneChange:              identityMod.PhoneChange,
		Profile:                  identityMod.Profile,
		Logout:                   identityMod.Logout,
		Sessions:                 identityMod.SessionLoader,
		Audit:                    p.AuditRecorder,
		MeEnricher:               wire.BillingMeEnricher(billingMod.Services.Subscriptions),
		Tariffs:                  billingMod.Services.Tariffs,
		AdminTariffs:             billingMod.Services.Tariffs,
		Subscriptions:            billingMod.Services.Subscriptions,
		SubscriptionManagers:     billingMod.Services.Subscriptions,
		Payments:                 billingMod.Services.Payments,
		PaymentMethods:           billingMod.Services.PaymentMethods,
		Webhooks:                 billingMod.Services.Payments,
		AdminPayments:            billingMod.Services.Payments,
		AdminSubscriptions:       billingMod.Services.Subscriptions,
		BillingFakeConfirms:      billingMod.FakeConfirms,
		ReadonlyGate:             billingMod.MutationGate,
		Admin:                    adminMod.Service,
		Properties:               propertiesMod.PropertyService,
		PropertyContacts:         propertiesMod.PropertyContactService,
		AddressSuggester:         propertiesMod.DadataClient,
		Access:                   accessMod.AccessService,
		Invitations:              accessMod.InvitationService,
		Leases:                   leasesMod.LeaseService,
		TenantContacts:           leasesMod.TenantContactService,
		Operations:               leasesMod.OperationService,
		Export:                   leasesMod.ExportService,
		RecurringOperations:      leasesMod.RecurringOperationService,
		Categories:               leasesRepos.CategoryService,
		Reminders:                notificationsMod.ReminderService,
		Calendar:                 notificationsMod.CalendarService,
		NotificationPreferences:  notificationsMod.PreferenceService,
		PushSubscriptions:        notificationsMod.PushSubscriptionService,
		VAPIDPublicKey:           p.Cfg.VAPIDPublicKey,
		Popups:                   popupsMod.Service,
		AppBaseURL:               p.Cfg.AppBaseURL,
		CookieSecure:             p.Cfg.CookieSecure,
		Logger:                   p.Logger,
		Clock:                    p.Clock,
		TZResolver:               p.TZResolver,
		LogSuccessfulRequests:    p.Cfg.LogSuccessfulRequests,
		IPRateLimiter:            limiters.IPRateLimiter,
		EmailSendLimiter:         limiters.EmailSendLimiter,
		EmailVerifyLimiter:       limiters.EmailVerifyLimiter,
		PhoneChangeSendLimiter:   limiters.PhoneChangeSendLimiter,
		PhoneChangeVerifyLimiter: limiters.PhoneChangeVerifyLimiter,
		ClientErrorsLimiter:      limiters.ClientErrorsLimiter,
		DBPoolStats:              poolStats,
		TrustedProxies:           p.Cfg.TrustedProxies,
		AppVersion:               p.Cfg.AppVersion,
	})

	server := &http.Server{
		Addr:              p.Cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	return serveAndWait(ctx, server, workers, p.Logger, p.Cfg)
}

// injectAccessPolicy re-injects the membership-aware policy into the services
// built before the access module: they captured the T2 owner-only stub
// (notifications reminder service, issue #166, and the leases category
// service, which is needed for the user_registered subscriber, issue #157).
// The accessible-scopes adapter lands with the category service (issue #157).
func injectAccessPolicy(
	notificationsMod *wire.Notifications,
	leasesRepos *wire.LeasesRepos,
	accessMod *wire.Access,
) {
	notificationsMod.ReminderService.SetPolicy(accessMod.Policy)
	leasesRepos.CategoryService.SetPolicy(accessMod.Policy)
	leasesRepos.CategoryService.SetAccessibleScopes(accessMod.AccessibleScopes)
}

// injectPropertyServiceAccess wires the access-context adapters into the
// property service: shared memberships for list endpoints (issues #156 T3,
// T11), the owner display name for the sharing banner (T11), the recipient
// slot policy and suspended-shared counter for archive/unarchive/delete
// (issue #158, T4), and the shared-members delete mailer (issue #162, T6).
func injectPropertyServiceAccess(propertiesMod *wire.Properties, accessMod *wire.Access) {
	propertiesMod.PropertyService.SetSharedMemberships(accessMod.SharedProperties)
	propertiesMod.PropertyService.SetOwnerDisplayNameResolver(accessMod.AccessService)
	propertiesMod.PropertyService.SetRecipientSlotPolicy(accessMod.SlotCoordinator)
	propertiesMod.PropertyService.SetSuspendedSharedCounter(accessMod.SuspendedCounter)
	propertiesMod.PropertyService.SetSharedMembersDeleteMailer(accessMod.PropertyDeleteMailer)
}

// setBillingLifecycleBridges wires the billing worker's cross-context
// lifecycle bridges (issue #252): the expiry and downgrade phases archive
// excess properties (open leases force-completed) and suspend excess shared
// memberships in the same transaction as the subscription change. Billing is
// built before the properties and access modules, so the bridges land here.
// The workers, the payment service and the subscription service share the
// same bridges: the expiry and downgrade phases (issue #252), the refund's
// downgrade to basic (issue #254) and the admin operations that can lower a
// tariff limit (issue #255) all archive excess properties and suspend excess
// shared memberships in the same transaction as the subscription change.
func setBillingLifecycleBridges(billingMod *wire.Billing, propertiesMod *wire.Properties, accessMod *wire.Access) {
	archiverSource := wire.NewPropertyArchiverSource(propertiesMod.PropertyService)
	slotSource := wire.NewRecipientSlotSource(accessMod.SlotCoordinator)
	billingMod.Services.Workers.SetLifecycleBridges(archiverSource, slotSource)
	billingMod.Services.Payments.SetLifecycleBridges(archiverSource, slotSource)
	billingMod.Services.Subscriptions.SetLifecycleBridges(archiverSource, slotSource)
}

// subscribeUserRegistered registers the user_registered reactions: billing
// onboarding and default-category seeding, plus the email invitation
// activation (issue #161, T5; an empty email is skipped by the service). Kept
// here (not in wire) because the subscribers reference types from identity,
// billing and leases.
func subscribeUserRegistered(
	eventDispatcher *events.InProcessDispatcher,
	billingMod *wire.Billing,
	leasesRepos *wire.LeasesRepos,
	accessMod *wire.Access,
) {
	eventDispatcher.Subscribe(events.EventType("user_registered"), func(ctx context.Context, event any) error {
		e, ok := event.(identityapp.UserRegistered)
		if !ok {
			return fmt.Errorf("unexpected event type %T", event)
		}
		return billingMod.Services.Onboarding.OnUserRegistered(ctx, e.UserID)
	})
	eventDispatcher.Subscribe(events.EventType("user_registered"), func(ctx context.Context, event any) error {
		e, ok := event.(identityapp.UserRegistered)
		if !ok {
			return fmt.Errorf("unexpected event type %T", event)
		}
		return leasesRepos.CategoryService.SeedDefaultCategories(ctx, e.UserID)
	})
	eventDispatcher.Subscribe(events.EventType("user_registered"), func(ctx context.Context, event any) error {
		e, ok := event.(identityapp.UserRegistered)
		if !ok {
			return fmt.Errorf("unexpected event type %T", event)
		}
		return accessMod.InvitationService.ActivatePendingInvitations(ctx, e.UserID, e.Email.String())
	})
}

// newPushSender builds the Web Push sender when the VAPID keys are configured
// (RFC 8292); without them it returns a nil sender, push delivery is disabled
// and the reminder worker runs email-only.
func newPushSender(ctx context.Context, cfg *config.Config, logger *slog.Logger) (notificationsapp.PushSender, error) {
	var sender notificationsapp.PushSender
	if cfg.VAPIDPublicKey == "" || cfg.VAPIDPrivateKey == "" {
		logger.WarnContext(ctx, "VAPID keys not configured; web push delivery disabled (email-only)")
		return sender, nil
	}
	pushMetrics, err := webpush.NewMetrics()
	if err != nil {
		return nil, fmt.Errorf("wire push metrics: %w", err)
	}
	s, err := webpush.NewSender(cfg.VAPIDSubject, cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey, pushMetrics, logger)
	if err != nil {
		return nil, fmt.Errorf("wire webpush sender: %w", err)
	}
	logger.InfoContext(ctx, "web push delivery enabled")
	return s, nil
}

// newGraceNotifier builds the direct notification service the billing grace
// events deliver through; push delivery is included only when the sender
// could be built.
func newGraceNotifier(
	db *database.InstrumentedPool,
	renderer *mailer.Renderer,
	cfg *config.Config,
	logger *slog.Logger,
	notificationsMod *wire.Notifications,
	identityMod *wire.Identity,
	pushSender notificationsapp.PushSender,
) *notificationsapp.DirectNotificationService {
	queries := platformgenerated.New(db)
	return notificationsapp.NewDirectNotificationService(
		notificationsMod.ReminderRepo,
		notificationspg.NewContactResolver(queries),
		emailnotifier.NewNotifier(identityMod.EmailMailer, renderer),
		pushSender,
		notificationsMod.PushSubscriptionRepo,
		cfg.AppBaseURL,
		logger,
	)
}

// subscribeGraceEvents registers the grace-entered and grace-expiring
// subscribers on the shared event dispatcher.
func subscribeGraceEvents(
	eventDispatcher *events.InProcessDispatcher,
	graceNotifier *notificationsapp.DirectNotificationService,
) {
	eventDispatcher.Subscribe(events.EventType("subscription_grace_entered"), func(ctx context.Context, event any) error {
		e, ok := event.(billingapp.GraceEntered)
		if !ok {
			return fmt.Errorf("unexpected event type %T", event)
		}
		return graceNotifier.NotifyGraceEntered(ctx, e.UserID, e.GraceUntil)
	})
	eventDispatcher.Subscribe(events.EventType("subscription_grace_expiring"), func(ctx context.Context, event any) error {
		e, ok := event.(billingapp.GraceExpiring)
		if !ok {
			return fmt.Errorf("unexpected event type %T", event)
		}
		return graceNotifier.NotifyGraceExpiring(ctx, e.UserID, e.GraceUntil)
	})
}

// injectCrossContextScopes wires the membership-aware policy and the
// shared-property adapters into the aggregate-read services so lists and
// schedules include the actor's shared-property data (issue #157, T2a/T3).
func injectCrossContextScopes(
	leasesMod *wire.Leases,
	propertiesMod *wire.Properties,
	notificationsMod *wire.Notifications,
	accessMod *wire.Access,
) {
	leasesMod.TenantContactService.SetPolicy(accessMod.Policy)
	leasesMod.TenantContactService.SetAccessibleScopes(accessMod.AccessibleScopes)
	propertiesMod.PropertyContactService.SetPolicy(accessMod.Policy)
	leasesMod.OperationService.SetSharedPropertyIDs(accessMod.SharedProperties)
	notificationsMod.CalendarService.SetSharedPropertyIDs(accessMod.SharedProperties)
	leasesMod.LeaseService.SetSharedPropertyIDs(accessMod.SharedProperties)
	leasesMod.RecurringOperationService.SetSharedPropertyIDs(accessMod.SharedProperties)
	notificationsMod.ReminderService.SetSharedPropertyIDs(accessMod.SharedProperties)
}

// newPoolStats exposes live pool statistics only in the local environment.
func newPoolStats(cfg *config.Config, pool *pgxpool.Pool) func() httpsupport.DBPoolSnapshot {
	if cfg.AppEnv != "local" {
		return nil
	}
	return wire.DBPoolStats(pool)
}

// serveAndWait runs the HTTP server until the lifecycle context is cancelled
// or the listener fails, then shuts down gracefully and waits for the
// background workers.
func serveAndWait(
	ctx context.Context,
	server *http.Server,
	workers *wire.Workers,
	logger *slog.Logger,
	cfg *config.Config,
) error {
	errCh := make(chan error, 1)
	go func() {
		logger.InfoContext(ctx, "backend listening", "addr", cfg.HTTPAddr, "env", cfg.AppEnv)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		// The lifecycle context is already cancelled, so the shutdown window
		// derives from its value-bearing, cancellation-stripped view (same
		// pattern as the platform cleanup in wire).
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		workers.Wait()
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// runMigrate applies database migrations and returns; used by the deploy
// pipeline as a separate step (`arenda-api migrate`) instead of startup
// auto-migration.
func runMigrate() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	// The migrate step runs without the full platform wiring, but the log
	// format and level still follow the app config; the record is anchored to
	// context.Background() because no request or signal ctx exists here.
	appLogger, err := wire.NewAppLogger(&cfg)
	if err != nil {
		return err
	}
	if err := database.MigrateUp(cfg.DatabaseURL, cfg.MigrationsDir); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	appLogger.InfoContext(context.Background(), "migrations applied")
	return nil
}
