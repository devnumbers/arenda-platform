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
	accessevents "github.com/nambers/arenda-planform/apps/backend/internal/access/adapters/events"
	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	billingevents "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/events"
	billinghttp "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/http"
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
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/sse"
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

	// 3.5. HTTP rate limiters (built early: the email-change service's
	//      new-address send budget consumes the email-change limiter).
	limiters := wire.WireRateLimiters(p.Cfg)
	defer limiters.Stop()

	// 4. Identity: repos, session service, event publisher, email mailer
	//    switch, auth/phone-change/email-change/profile/logout services.
	identityMod, err := wire.WireIdentity(ctx, p, eventDispatcher, limiters)
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

	// 6. Access (T3, issue #156): membership repository, property owner
	//    resolver, membership-aware policy and access service. Built before
	//    properties because the policy replaces the T2 owner-only policy and is
	//    injected into every property service.
	accessMod, err := wire.WireAccess(ctx, p, billingMod, identityMod.EmailMailer, eventDispatcher)
	if err != nil {
		return err
	}
	p.Policy = accessMod.Policy

	// 6.5. Notifications (issue #438): the push-subscription service, the
	//      stored feed repository and the feed reading service; the delivery
	//      queue and the grace publisher build on them below. Wired after the
	//      membership policy is installed: the feed's live actions resolve the
	//      reader's role through the policy (FeedLiveState, #743) — on the
	//      owner-only fallback every property-scoped button (Продлить/
	//      Завершить, Принять) would silently never stand (находка приёмки
	//      #745).
	notificationsMod := wire.WireNotifications(p)

	// 7. Properties: repos, subscription limiter, photo storage, the property
	//    service and the dadata suggester.
	propertiesMod, err := wire.WireProperties(ctx, p, billingMod)
	if err != nil {
		return err
	}
	injectPropertyServiceAccess(propertiesMod, accessMod)
	setBillingLifecycleBridges(billingMod, propertiesMod, accessMod)

	// 7.5 Payments (ADR 0049): the payment rule CRUD service with its tick
	//     stores, the owner calendar and the tick zone sweep with its
	//     heartbeat metrics; wired after access so the membership-aware
	//     policy resolves the actor/scope matrix (ADR 0028).
	paymentsMod, err := wire.WirePayments(p)
	if err != nil {
		return err
	}

	// 7.6 Tasks (ADR 0051): the task rule CRUD service, the task use cases
	//     and the materialization tick service with its own hourly worker
	//     loop; wired after access so the membership-aware policy resolves
	//     the actor/scope matrix (ADR 0028).
	tasksMod, err := wire.WireTasks(p)
	if err != nil {
		return err
	}

	// 7.7 Contacts (ADR 0054): the owner's contact book CRUD with the
	//     property-scope role gates; wired after access so the
	//     membership-aware policy resolves the actor/scope matrix (ADR 0028).
	contactsMod, err := wire.WireContacts(p)
	if err != nil {
		return err
	}

	// 7.8 Rentals (ADR 0053): the rental use cases with the composite
	//     transaction factory over the payments stores and the wiring-bound
	//     RentPaymentGateway; wired after access and payments — the policy
	//     resolves the actor/scope matrix, and the seam joins the payments
	//     stores inside the rentals transactions.
	rentalsMod, err := wire.WireRentals(p)
	if err != nil {
		return err
	}
	injectPropertyListProjections(propertiesMod, paymentsMod, rentalsMod)

	// 8. Cross-module user_registered subscribers.
	subscribeUserRegistered(eventDispatcher, billingMod, accessMod)

	// 9. Notifications channel adapters: the Web Push sender (nil without
	//    VAPID keys — email-only local mode) and the shared contact resolver
	//    + email notifier. Both the grace subscribers (step 11.6) and the
	//    delivery queue (step 11.5) consume them.
	pushSender, err := newPushSender(ctx, p.Cfg, p.Logger)
	if err != nil {
		return err
	}
	delivery := newNotificationDelivery(p.DB, p.Renderer, identityMod.EmailMailer)

	// 10. Admin service (depends on billing subscriptions + occupancy provider).
	adminMod := wire.WireAdmin(p, billingMod.Services.Subscriptions)

	// 11. Popups service.
	popupsMod := wire.WirePopups(p)

	// 11.5 Delivery queue (карта #734, #740): the River client with the
	//     email/push delivery workers and the notification publisher. The
	//     client starts in the workers phase below; the publisher is the
	//     post-commit seam the pipeline publishers call (#741, #748–#752).
	//     The grace subscribers (step 11.6) are the first to call it. The
	//     publisher also pushes the live SSE frames through the stream hub
	//     (#742, ADR 0060), which the HTTP server serves below. The tasks
	//     (#750), payments (#776) and rentals (#777) scan publishers wire
	//     here as well — their booking legs schedule the boundary jobs
	//     through the same client.
	taskScanStore := notificationspg.NewTaskScanStore(p.DB)
	paymentScanStore := notificationspg.NewPaymentScanStore(p.DB)
	rentalScanStore := notificationspg.NewRentalScanStore(p.DB)
	riverMod, err := wire.WireRiverQueue(ctx, p, notificationsMod,
		delivery.resolver, delivery.emailer, pushSender, taskScanStore, paymentScanStore, rentalScanStore)
	if err != nil {
		return err
	}
	defer riverMod.ProviderLimiter.Stop()
	notificationsStream := riverMod.Stream

	// 11.5.1 The tasks scheduling seam (issue #775): the rule create/edit
	//     flows hand their standing tasks' ids over post-commit — a live
	//     timed task books its due-minute job at once (the term before the
	//     next hourly pass is exactly the delay this removes), a task born
	//     overdue publishes immediately. Late-bound: the tasks module builds
	//     earlier than the delivery queue (the grace-events canon —
	//     best-effort, a broken seam never fails the committed rule).
	tasksMod.RuleService.SetOverdueSeam(riverMod.TasksSeam)

	// 11.6 Grace notifications (issue #253, #741): the billing grace events
	//     publish to the stored feed + delivery queue through the pipeline
	//     publisher (the always-on Тариф category, ADR 0058). Subscribers
	//     are registered before the workers start (step 12), so no grace
	//     event fires unwired.
	subscribeGraceEvents(eventDispatcher, notificationsapp.NewGracePublisher(riverMod.Publisher))

	// 11.8 Access notifications (карта #734, #751): the access lifecycle
	//     events — the invitation's activation, the revoke, the slot pause
	//     and recovery, the member's self-exit — publish to the stored feed
	//     + delivery queue through the same pipeline publisher (the
	//     Совместный доступ category, gated per-channel by the settings
	//     matrix at delivery time). The dispatchers are synchronous, so the
	//     subscription must exist before any transition can fire it.
	accessEventViews := notificationspg.NewAccessViewStore(
		p.DB,
		notificationspg.NewAccessEventUserReader(identityMod.UserRepo),
	)
	subscribeAccessEvents(eventDispatcher, notificationsapp.NewAccessPublisher(riverMod.Publisher, accessEventViews))

	// 11.9 Tariff notifications (карта #734, #752): the billing tariff
	//     events — the applied subscription payment (№13 «Оплата прошла»),
	//     the upgrade an applied payment activated and the downgrade
	//     assigned for the period's end (№14 «Тариф изменён») — publish to
	//     the stored feed + delivery queue through the same pipeline
	//     publisher (the always-on Тариф category, ADR 0058). The dispatchers
	//     are synchronous, so the subscription must exist before any applied
	//     payment can fire it.
	tariffEventViews := notificationspg.NewTariffViewStore(p.DB)
	subscribeTariffEvents(eventDispatcher, notificationsapp.NewTariffPublisher(riverMod.Publisher, tariffEventViews))

	// 11.7 The notifications scan (карта #734, #748–#750, #776, #777): the
	//     hourly zone sweep publishes the scan-driven catalog events — the
	//     rentals that moved to «Ожидает действия», the payments that came
	//     due or overdue, and the tasks whose boundary has passed — through
	//     the same pipeline publisher; the Аренда, Платежи и операции and
	//     Задачи categories are gated per-channel by the settings matrix at
	//     delivery time. The tasks publisher's scheduled leg (#750),
	//     the payments publisher's booking leg (#776) and the rentals
	//     publisher's booking leg (#777) book the boundary jobs on the way —
	//     the exact trigger moments the hourly cadence cannot give; the
	//     sweeps stay the retrospectives after a downtime.
	notificationsScan := notificationsapp.NewScanGroup(
		riverMod.RentalsPublisher,
		riverMod.PaymentsPublisher,
		riverMod.TasksPublisher,
	)

	// 12. Background workers (6 goroutines + the delivery queue client).
	//     Started before the HTTP server so they are live while serving. The
	//     Web Push sender was constructed in step 9 and shared with the
	//     delivery queue.
	workers := wire.NewWorkers(
		ctx, p,
		identityMod.SessionRepo,
		identityMod.CodeRepo,
		identityMod.AttemptRepo,
		billingMod.Services.Workers,
		paymentsMod.TickService,
		tasksMod.TickService,
		notificationsScan,
		riverMod.Client,
	)

	// 12a. The stand-only time-travel rig (issue #665): the admin time-shift
	//      and tick endpoints exist only when the BILLING_TIME_TRAVEL
	//      railguard is on — nil keeps the routes unmounted.
	var billingTimeTravel *billinghttp.TimeTravelHandlers
	if p.Cfg.BillingTimeTravel {
		billingTimeTravel = billinghttp.NewTimeTravelHandlers(
			billingMod.Services.Subscriptions, workers.Billing, p.Logger)
	}

	poolStats := newPoolStats(p.Cfg, p.Pool)

	// 14. HTTP handler + server.
	handler := httpserver.New(httpserver.Deps{
		Auth:                     identityMod.Authentication,
		PhoneChange:              identityMod.PhoneChange,
		EmailChange:              identityMod.EmailChange,
		Profile:                  identityMod.Profile,
		Logout:                   identityMod.Logout,
		Sessions:                 identityMod.Sessions,
		SessionLoader:            identityMod.SessionLoader,
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
		BillingTimeTravel:        billingTimeTravel,
		ReadonlyGate:             billingMod.MutationGate,
		Admin:                    adminMod.Service,
		Properties:               propertiesMod.PropertyService,
		Contacts:                 contactsMod.ContactService,
		AddressSuggester:         propertiesMod.DadataClient,
		PropertyPayments:         paymentsMod.PaymentService,
		PropertyOperations:       paymentsMod.OperationService,
		GlobalPayments:           paymentsMod.GlobalPayments,
		PropertyRentals:          rentalsMod.RentalService,
		PropertyTaskRules:        tasksMod.RuleService,
		PropertyTasks:            tasksMod.TaskService,
		Access:                   accessMod.AccessService,
		Invitations:              accessMod.InvitationService,
		PushSubscriptions:        notificationsMod.PushSubscriptionService,
		NotificationsFeed:        notificationsMod.FeedService,
		NotificationSettings:     notificationsMod.SettingsService,
		NotificationsStreamHub:   notificationsStream,
		VAPIDPublicKey:           p.Cfg.VAPIDPublicKey,
		Popups:                   popupsMod.Service,
		AppBaseURL:               p.Cfg.AppBaseURL,
		CookieSecure:             p.Cfg.CookieSecure,
		Logger:                   p.Logger,
		Clock:                    p.Clock,
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

	// WriteTimeout is 0: the event stream (/notifications/stream, #742) is a
	// long-lived response the per-request write deadline would kill after 30s
	// in every environment. The deadline could not be lifted per-connection —
	// the otelhttp wrapper in the middleware chain hides the server's
	// SetWriteDeadline from http.ResponseController — so the protection moves
	// to the read deadlines (slow-request vector) and the stream's own
	// heartbeat + hourly TTL (ADR 0060). Responses are bounded API payloads
	// behind a buffering Caddy, so a client stalling a write is not a
	// resource leak.
	server := &http.Server{
		Addr:              p.Cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	return serveAndWait(ctx, server, workers, notificationsStream, p.Logger, p.Cfg)
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

// injectPropertyListProjections wires the list read projections of the
// property service (ticket #585): the rentals occupancy reader and the
// payments overdue reader behind the list endpoints' occupancy and red-dot
// data. Rentals and payments are wired after properties, so this lands once
// both modules exist.
func injectPropertyListProjections(propertiesMod *wire.Properties, paymentsMod *wire.Payments, rentalsMod *wire.Rentals) {
	propertiesMod.PropertyService.SetRentalOccupancyReader(rentalsMod.OccupancyReader)
	propertiesMod.PropertyService.SetOverdueOperationsReader(paymentsMod.OverdueOperations)
	// Deletion guard (issue #632): the property delete consults the rentals
	// context in its own transaction — the unfinished rental conflicts, the
	// completed ones are torn down before the property row.
	propertiesMod.PropertyService.SetRentalDeletionGuard(rentalsMod.DeletionGuard)
}

// setBillingLifecycleBridges wires the billing worker's cross-context
// lifecycle bridges (issue #252): the expiry and downgrade phases archive
// excess properties and suspend excess shared
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
// onboarding plus the email invitation activation (issue #161, T5; an empty
// email is skipped by the service). Kept here (not in wire) because the
// subscribers reference types from identity, billing and access.
func subscribeUserRegistered(
	eventDispatcher *events.InProcessDispatcher,
	billingMod *wire.Billing,
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
		return accessMod.InvitationService.ActivatePendingInvitations(ctx, e.UserID, e.Email.String())
	})
}

// newPushSender builds the Web Push sender when the VAPID keys are configured
// (RFC 8292); without them it returns a nil sender and push delivery is
// disabled.
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

// notificationDelivery bundles the delivery queue's channel adapters: the
// contact resolver and the email notifier the email worker resolves and
// sends through.
type notificationDelivery struct {
	resolver notificationsapp.ContactResolver
	emailer  notificationsapp.TemplateEmailSender
}

// newNotificationDelivery builds the shared channel adapters.
func newNotificationDelivery(db *database.InstrumentedPool, renderer *mailer.Renderer, emailMailer mailer.Sender) notificationDelivery {
	queries := platformgenerated.New(db)
	return notificationDelivery{
		resolver: notificationspg.NewContactResolver(queries),
		emailer:  emailnotifier.NewNotifier(emailMailer, renderer),
	}
}

// subscribeGraceEvents registers the grace-entered and grace-expiring
// subscribers on the shared event dispatcher: each billing event becomes a
// stored feed notification with queued email/push delivery (#741). The
// publisher is notifications' GracePublisher over the pipeline Publisher;
// billing never imports notifications — the composition root owns the seam.
func subscribeGraceEvents(
	eventDispatcher *events.InProcessDispatcher,
	gracePublisher *notificationsapp.GracePublisher,
) {
	eventDispatcher.Subscribe(billingevents.EventGraceEntered, func(ctx context.Context, event any) error {
		e, ok := event.(billingapp.GraceEntered)
		if !ok {
			return fmt.Errorf("unexpected event type %T", event)
		}
		return gracePublisher.NotifyGraceEntered(ctx, e.UserID, e.SubscriptionID, e.GraceUntil)
	})
	eventDispatcher.Subscribe(billingevents.EventGraceExpiring, func(ctx context.Context, event any) error {
		e, ok := event.(billingapp.GraceExpiring)
		if !ok {
			return fmt.Errorf("unexpected event type %T", event)
		}
		return gracePublisher.NotifyGraceExpiring(ctx, e.UserID, e.SubscriptionID, e.GraceUntil)
	})
}

// subscribeTariffEvents registers the tariff events' subscribers on the
// shared event dispatcher: an applied subscription payment and a tariff
// change become the Тариф catalog rows (карта #734, решение #737 №13–№14,
// #752). The publisher is notifications' TariffPublisher over the pipeline
// Publisher; billing never imports notifications — the composition root owns
// the seam.
func subscribeTariffEvents(
	eventDispatcher *events.InProcessDispatcher,
	tariffPublisher *notificationsapp.TariffPublisher,
) {
	eventDispatcher.Subscribe(billingevents.EventPaymentSucceeded, func(ctx context.Context, event any) error {
		e, ok := event.(billingapp.PaymentSucceeded)
		if !ok {
			return fmt.Errorf("unexpected event type %T", event)
		}
		return tariffPublisher.NotifyPaymentSucceeded(
			ctx, e.UserID, e.PaymentID, e.TariffID,
			e.AmountKopecks, string(e.Period), e.ActiveUntil,
		)
	})
	eventDispatcher.Subscribe(billingevents.EventPlanUpgraded, func(ctx context.Context, event any) error {
		e, ok := event.(billingapp.PlanUpgraded)
		if !ok {
			return fmt.Errorf("unexpected event type %T", event)
		}
		return tariffPublisher.NotifyPlanUpgraded(
			ctx, e.UserID, e.TransitionID, e.TariffID,
			string(e.Period), e.AmountKopecks, e.ActiveUntil,
		)
	})
	eventDispatcher.Subscribe(billingevents.EventPlanDowngradeScheduled, func(ctx context.Context, event any) error {
		e, ok := event.(billingapp.PlanDowngradeScheduled)
		if !ok {
			return fmt.Errorf("unexpected event type %T", event)
		}
		return tariffPublisher.NotifyPlanDowngradeScheduled(
			ctx, e.UserID, e.TransitionID, e.TariffID,
			string(e.Period), e.EffectiveAt,
		)
	})
}

// subscribeAccessEvents registers the access lifecycle events' subscribers on
// the shared event dispatcher: each access transition becomes the stored feed
// notification(s) of the Совместный доступ catalog (карта #734, #751). The
// publisher is notifications' AccessPublisher over the pipeline Publisher;
// access never imports notifications — the composition root owns the seam.
func subscribeAccessEvents(
	eventDispatcher *events.InProcessDispatcher,
	accessPublisher *notificationsapp.AccessPublisher,
) {
	eventDispatcher.Subscribe(accessevents.EventInvitationActivated, func(ctx context.Context, event any) error {
		e, ok := event.(accessapp.InvitationActivated)
		if !ok {
			return fmt.Errorf("unexpected event type %T", event)
		}
		return accessPublisher.NotifyInvitationActivated(
			ctx, e.MembershipID, e.PropertyID, e.InviterID, e.InviteeID,
			e.Suspended, e.At,
		)
	})
	eventDispatcher.Subscribe(accessevents.EventMembershipSuspended, func(ctx context.Context, event any) error {
		e, ok := event.(accessapp.MembershipSuspended)
		if !ok {
			return fmt.Errorf("unexpected event type %T", event)
		}
		return accessPublisher.NotifyMembershipSuspended(
			ctx, e.MembershipID, e.PropertyID, e.RecipientID, e.ActorID, e.SuspendedAt,
		)
	})
	eventDispatcher.Subscribe(accessevents.EventMembershipResumed, func(ctx context.Context, event any) error {
		e, ok := event.(accessapp.MembershipResumed)
		if !ok {
			return fmt.Errorf("unexpected event type %T", event)
		}
		return accessPublisher.NotifyMembershipResumed(
			ctx, e.MembershipID, e.PropertyID, e.RecipientID, e.ActorID, e.ResumedAt,
		)
	})
	eventDispatcher.Subscribe(accessevents.EventMembershipRevoked, func(ctx context.Context, event any) error {
		e, ok := event.(accessapp.MembershipRevoked)
		if !ok {
			return fmt.Errorf("unexpected event type %T", event)
		}
		return accessPublisher.NotifyAccessRevoked(
			ctx, e.MembershipID, e.PropertyID, e.RecipientID, e.ActorID,
		)
	})
	eventDispatcher.Subscribe(accessevents.EventMemberLeft, func(ctx context.Context, event any) error {
		e, ok := event.(accessapp.MemberLeft)
		if !ok {
			return fmt.Errorf("unexpected event type %T", event)
		}
		return accessPublisher.NotifyMemberLeft(
			ctx, e.MembershipID, e.PropertyID, e.OwnerID, e.MemberID,
		)
	})
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
	notificationsStream *sse.Hub,
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
		// The stream hub closes first: every SSE handler returns at once, so
		// the graceful HTTP shutdown below is not held up by long-lived
		// streams inside its 5-second window.
		notificationsStream.Close(ctx)
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
	// The River queue owns its own schema chain (rivermigrate, research
	// #735 §2): applied after ours, idempotent, never vendored into
	// db/migrations.
	riverPool, err := database.NewPool(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("river migrate pool: %w", err)
	}
	defer riverPool.Close()
	if err := database.MigrateRiverSchema(context.Background(), riverPool); err != nil {
		return fmt.Errorf("river migrate: %w", err)
	}
	appLogger.InfoContext(context.Background(), "migrations applied")
	return nil
}
