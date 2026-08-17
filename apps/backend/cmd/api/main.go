package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

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
)

func main() {
	fallback := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	if err := run(); err != nil {
		fallback.Error("backend stopped", "error", err) //nolint:sloglint // fallback logger before any context exists in main
		os.Exit(1)
	}
}

// run is the backend's composition root. It wires platform resources and the
// per-module services (via the cmd/api/wire constructors) in dependency order,
// registers the cross-module event subscribers, builds the HTTP handler and
// runs the server with graceful shutdown. The construction order, event
// subscriptions, worker goroutines and server lifecycle are unchanged from the
// previous monolithic version — only the construction bodies moved into wire.
func run() error {
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		return runMigrate()
	}

	// 1. Platform: config, logger, signal ctx, OTel, renderer, encryptor,
	//    auto-migrate, db pool, audit recorder, tz resolver.
	platform, err := wire.WirePlatform()
	if err != nil {
		return err
	}
	defer platform.Cleanup()
	ctx := platform.Ctx
	p := platform.Deps

	// 2. Event dispatcher (shared by identity publisher and subscribers).
	eventDispatcher := events.NewInProcessDispatcher()

	// 3. Notifications: reminder/free-reminder/calendar/preference services +
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

	// Wire the membership-aware policy into the notifications services so
	// reminder/free-reminder write gates resolve the actor's role correctly.
	// These services are built before the access module (they feed identity and
	// leases), so they captured the T2 owner-only stub and must be re-injected
	// here (issue #166, mirrors CategoryService.SetPolicy).
	notificationsMod.FreeReminderService.SetPolicy(accessMod.Policy)
	notificationsMod.ReminderService.SetPolicy(accessMod.Policy)

	// The category service is built before the access module (it is needed for
	// the user_registered subscriber), so wire the membership-aware policy and
	// the accessible-scopes adapter into it now that both exist (issue #157).
	leasesRepos.CategoryService.SetPolicy(accessMod.Policy)
	leasesRepos.CategoryService.SetAccessibleScopes(accessMod.AccessibleScopes)

	// 8. Properties: repos, subscription limiter, photo storage, property and
	//    property-contact services, dadata suggester.
	propertiesMod, err := wire.WireProperties(ctx, p, billingMod, leasesRepos)
	if err != nil {
		return err
	}
	// Wire the shared-memberships adapter from the access context into the
	// property service so list endpoints include properties shared with the
	// actor and expose the actor's access role (issues #156 T3, T11).
	propertiesMod.PropertyService.SetSharedMemberships(accessMod.SharedProperties)
	// Wire the owner display-name resolver so the property detail response can
	// carry the owner's public name for the sharing banner (issue T11).
	propertiesMod.PropertyService.SetOwnerDisplayNameResolver(accessMod.AccessService)
	// Wire the recipient slot policy (access SlotCoordinator) into the property
	// service so archive/unarchive/delete recover or suspend shared memberships
	// of recipients (issue #158, T4).
	propertiesMod.PropertyService.SetRecipientSlotPolicy(accessMod.SlotCoordinator)
	// Wire the suspended-shared counter so the active properties list can report
	// how many shared objects are hidden from the recipient by a slot shortage
	// (issue #158, T4).
	propertiesMod.PropertyService.SetSuspendedSharedCounter(accessMod.SuspendedCounter)
	// Wire the shared-members delete mailer so deleting a shared object emails
	// its former members (issue #162, T6).
	propertiesMod.PropertyService.SetSharedMembersDeleteMailer(accessMod.PropertyDeleteMailer)

	// Wire the billing worker's cross-context lifecycle bridges (issue #252):
	// the expiry and downgrade phases archive excess properties (open leases
	// force-completed) and suspend excess shared memberships in the same
	// transaction as the subscription change. Billing is built before the
	// properties and access modules, so the bridges land here.
	// The workers, the payment service and the subscription service share the
	// same bridges: the expiry and downgrade phases (issue #252), the refund's
	// downgrade to basic (issue #254) and the admin operations that can lower a
	// tariff limit (issue #255) all archive excess properties and suspend excess
	// shared memberships in the same transaction as the subscription change.
	archiverSource := wire.NewPropertyArchiverSource(propertiesMod.PropertyService)
	slotSource := wire.NewRecipientSlotSource(accessMod.SlotCoordinator)
	billingMod.Services.Workers.SetLifecycleBridges(archiverSource, slotSource)
	billingMod.Services.Payments.SetLifecycleBridges(archiverSource, slotSource)
	billingMod.Services.Subscriptions.SetLifecycleBridges(archiverSource, slotSource)

	// 9. Cross-module event subscribers: billing onboarding and default-category
	//    seeding both react to user_registered. Kept here (not in wire) because
	//    they reference types from identity, billing and leases.
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
	// Email invitation activation (issue #161, T5): a registration with an
	// invited email activates the pending invitations FIFO. An empty email is
	// skipped by the service.
	eventDispatcher.Subscribe(events.EventType("user_registered"), func(ctx context.Context, event any) error {
		e, ok := event.(identityapp.UserRegistered)
		if !ok {
			return fmt.Errorf("unexpected event type %T", event)
		}
		return accessMod.InvitationService.ActivatePendingInvitations(ctx, e.UserID, e.Email.String())
	})

	// 10. Grace notifications (issue #253): the billing grace events deliver
	//    through the notifications context over push and email, honouring the
	//    per-channel preferences (ADR 0030). The Web Push sender is built when
	//    VAPID keys are configured (RFC 8292); without them delivery is
	//    email-only (same guard as the reminder worker below). Subscribers are
	//    registered before the workers start so no grace event fires unwired.
	var pushSender notificationsapp.PushSender
	if p.Cfg.VAPIDPublicKey != "" && p.Cfg.VAPIDPrivateKey != "" {
		pushMetrics, err := webpush.NewMetrics()
		if err != nil {
			return fmt.Errorf("wire push metrics: %w", err)
		}
		s, err := webpush.NewSender(p.Cfg.VAPIDSubject, p.Cfg.VAPIDPublicKey, p.Cfg.VAPIDPrivateKey, pushMetrics, p.Logger)
		if err != nil {
			return fmt.Errorf("wire webpush sender: %w", err)
		}
		pushSender = s
		p.Logger.InfoContext(ctx, "web push delivery enabled")
	} else {
		p.Logger.WarnContext(ctx, "VAPID keys not configured; web push delivery disabled (email-only)")
	}
	queries := platformgenerated.New(p.DB)
	graceNotifier := notificationsapp.NewDirectNotificationService(
		notificationsMod.ReminderRepo,
		notificationspg.NewContactResolver(queries),
		emailnotifier.NewNotifier(identityMod.EmailMailer, p.Renderer),
		pushSender,
		notificationsMod.PushSubscriptionRepo,
		p.Cfg.AppBaseURL,
		p.Logger,
	)
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

	// 11. Admin service (depends on billing subscriptions + occupancy provider).
	adminMod := wire.WireAdmin(p, billingMod.Services.Subscriptions, propertiesMod.OccupancyProvider)

	// 12. Leases services: lease/operation/recurring/tenant-contact/export.
	leasesMod := wire.WireLeasesServices(p, leasesRepos, notificationsMod.ReminderScheduler, notificationsMod.ReminderService)
	// Wire the membership-aware policy and the accessible-scopes adapter into
	// the tenant contact service so list endpoints include owner-wide data the
	// actor may read via property memberships (issue #157, T2a).
	leasesMod.TenantContactService.SetPolicy(accessMod.Policy)
	leasesMod.TenantContactService.SetAccessibleScopes(accessMod.AccessibleScopes)
	// Wire the membership-aware policy into the property contact service so
	// members with the view/edit capability read and write the contacts of the
	// property's data owner (Property Sharing follow-up).
	propertiesMod.PropertyContactService.SetPolicy(accessMod.Policy)
	// Wire the shared-property-ids adapter into the operation service so the
	// finance report includes the actor's own operations plus operations of
	// properties shared with the actor via property membership (issue #157, T3).
	leasesMod.OperationService.SetSharedPropertyIDs(accessMod.SharedProperties)
	// Wire the same adapter into the calendar service so the reminders agenda
	// includes the actor's own reminders plus reminders of properties shared
	// with the actor (issue #157, T3), excluding archived shared properties.
	notificationsMod.CalendarService.SetSharedPropertyIDs(accessMod.SharedProperties)
	// Wire the same adapter into the lease service so the lease payment schedule
	// (overdue / next payment, shown on the lease card and property card) is
	// computed from the actor's own rent operations plus those of properties
	// shared with the actor (issue #157, T3).
	leasesMod.LeaseService.SetSharedPropertyIDs(accessMod.SharedProperties)
	// Wire the same adapter into the remaining aggregate-read services so the
	// recurring-operations list, the reminders list, and the free-reminders
	// list all include the actor's shared-property data (issue #157, T3).
	leasesMod.RecurringOperationService.SetSharedPropertyIDs(accessMod.SharedProperties)
	notificationsMod.ReminderService.SetSharedPropertyIDs(accessMod.SharedProperties)
	notificationsMod.FreeReminderService.SetSharedPropertyIDs(accessMod.SharedProperties)

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

	var poolStats func() httpsupport.DBPoolSnapshot
	if p.Cfg.AppEnv == "local" {
		poolStats = wire.DBPoolStats(p.Pool)
	}

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
		FreeReminders:            notificationsMod.FreeReminderService,
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

	errCh := make(chan error, 1)
	go func() {
		p.Logger.InfoContext(ctx, "backend listening", "addr", p.Cfg.HTTPAddr, "env", p.Cfg.AppEnv)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
	if err := database.MigrateUp(cfg.DatabaseURL, cfg.MigrationsDir); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	slog.Info("migrations applied") //nolint:sloglint // migrate step runs before the app logger is configured
	return nil
}
