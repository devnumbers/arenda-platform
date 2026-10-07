package wire

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	identityemail "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/email"
	identityevents "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/events"
	identitygeoip "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/geoip"
	identityhttp "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/http"
	identitypg "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/postgres"
	identityuaparse "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/uaparse"
	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/events"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
	mailerfake "github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer/fake"
	mailersmtp "github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer/smtp"
	storageshared "github.com/nambers/arenda-planform/apps/backend/internal/shared/storage"
)

// Identity holds the identity module's services and event publisher wired by
// WireIdentity.
type Identity struct {
	UserRepo *identitypg.UserRepository
	// PhotoStorage is the process-wide private-photos object storage (ADR
	// 0065), shared with the properties and contacts services downstream.
	PhotoStorage   storageshared.PhotoStorage
	CodeRepo       *identitypg.LoginCodeRepository
	AttemptRepo    *identitypg.AttemptRepository
	SessionRepo    *identitypg.SessionRepository
	SessionService identityapp.SessionService
	// SessionLoader is the platform-neutral adapter over SessionService that the
	// HTTP session middleware consumes. It keeps platform/httpsupport free of any
	// identity/domain import (ADR 0034).
	SessionLoader  httpsupport.SessionLoader
	EventPublisher *identityevents.Publisher
	Authentication *identityapp.AuthenticationService
	PhoneChange    *identityapp.PhoneChangeService
	EmailChange    *identityapp.EmailChangeService
	Profile        *identityapp.ProfileService
	Logout         *identityapp.LogoutService
	// Sessions serves the devices list (list/revoke/revoke-others, #728).
	Sessions *identityapp.SessionsService
	// EmailMailer is the configured mailer.Sender (smtp or fake). It is exposed
	// because other modules reuse it: the access email sender (invite and
	// sharing lifecycle emails) and the notifications email notifier (grace
	// letters).
	EmailMailer mailer.Sender
}

// WireIdentity constructs the identity repositories, session service, event
// publisher, selects the email mailer based on config, and builds the
// authentication, phone-change, email-change, profile and logout services.
// It takes the event dispatcher (for the publisher) and the shared rate
// limiters (the email-change send budget).
func WireIdentity(
	ctx context.Context,
	p platformDeps,
	eventDispatcher *events.InProcessDispatcher,
	rateLimits *RateLimiters,
) (*Identity, error) {
	// The one private-photos object storage of the process (ADR 0065): the
	// profile service is its first consumer, properties and contacts reuse
	// the same instance through the Identity handle.
	photoStorage, err := WirePhotoStorage(ctx, p)
	if err != nil {
		return nil, err
	}

	userRepo := identitypg.NewUserRepository(p.DB, p.Encryptor)
	codeRepo := identitypg.NewLoginCodeRepository(p.DB, p.Encryptor)
	attemptRepo := identitypg.NewAttemptRepository(p.DB, p.Encryptor)
	sessionRepo := identitypg.NewSessionRepository(p.DB, p.Encryptor)
	emailChangeGrantRepo := identitypg.NewEmailChangeGrantRepository(p.DB)

	// The single canonical txStoreFactory bundles the four identity
	// repositories, the audit recorder, and the UoW (ADR 0033 γ-factory). It is
	// passed to every identity service so adding an Nth repository is a change
	// here, not in six constructors.
	factory := identityapp.NewTxStoreFactory(
		userRepo, codeRepo, attemptRepo, sessionRepo, emailChangeGrantRepo,
		p.AuditRecorder, p.UoW,
	)

	// Device enrichment for sessions: the User-Agent parser runs once per
	// login, the offline GeoIP base answers city lookups (#728). A missing
	// database (GEOIP_DB_PATH unset — local development) degrades to "no city".
	sessionService := identityapp.NewSessionService(
		factory,
		identityapp.SessionServiceConfig{
			Hasher: p.Encryptor,
			Parser: identityuaparse.NewParser(),
			Geo:    identitygeoip.NewResolver(ctx, p.Cfg.GeoIPDBPath, p.Logger),
		},
	)
	sessionLoader := identityhttp.NewSessionLoader(sessionService)

	eventPublisher := identityevents.NewPublisher(eventDispatcher)

	var emailMailer mailer.Sender
	switch p.Cfg.EmailSender {
	case "smtp":
		emailMailer = mailersmtp.NewSender(mailersmtp.Config{
			Host:     p.Cfg.SMTPHost,
			Port:     p.Cfg.SMTPPort,
			Username: p.Cfg.SMTPUser,
			Password: p.Cfg.SMTPPass,
			From:     p.Cfg.SMTPFrom,
			FromName: p.Cfg.SMTPFromName,
			Timeout:  p.Cfg.SMTPTimeout,
		})
	case providerFake:
		emailMailer = mailerfake.NewFakeSender(p.Logger)
	default:
		return nil, fmt.Errorf("unsupported EMAIL_SENDER: %s", p.Cfg.EmailSender)
	}

	emailSender := identityemail.NewSender(emailMailer, p.Renderer)

	// Login-code issuance/verification is a deep module shared by
	// AuthenticationService and PhoneChangeService (ADR 0033, step 4).
	loginCodeService := identityapp.NewLoginCodeService(
		factory,
		identityapp.LoginCodeServiceConfig{
			CodeSender: emailSender,
			Clock:      p.Clock,
			Hasher:     p.Encryptor,
			Logger:     p.Logger,
		},
	)

	authenticationService := identityapp.NewAuthenticationService(
		factory,
		identityapp.AuthenticationServiceConfig{
			LoginCodes: loginCodeService,
			Sessions:   sessionService,
			Publisher:  eventPublisher,
			Clock:      p.Clock,
			Logger:     p.Logger,
		},
	)

	phoneChangeService := identityapp.NewPhoneChangeService(
		factory,
		identityapp.PhoneChangeServiceConfig{
			LoginCodes: loginCodeService,
			Clock:      p.Clock,
			Hasher:     p.Encryptor,
			Logger:     p.Logger,
		},
	)

	// The 5/hour budget lives with the transport limiter (wire/ratelimits.go)
	// and is consulted by the service on each actual send to a new address.
	emailChangeService := identityapp.NewEmailChangeService(
		factory,
		identityapp.EmailChangeServiceConfig{
			LoginCodes: loginCodeService,
			Clock:      p.Clock,
			Hasher:     p.Encryptor,
			Logger:     p.Logger,
			AllowNewAddressSend: func(userID uuid.UUID) bool {
				return rateLimits.EmailChangeSendLimiter.Allow(userID.String())
			},
		},
	)

	profileService := identityapp.NewProfileService(factory, photoStorage, p.Logger)

	logoutService := identityapp.NewLogoutService(
		factory,
		identityapp.LogoutServiceConfig{
			Hasher: p.Encryptor,
			Logger: p.Logger,
		},
	)

	sessionsService := identityapp.NewSessionsService(
		factory,
		identityapp.SessionsServiceConfig{Hasher: p.Encryptor, Clock: p.Clock},
	)

	return &Identity{
		UserRepo:       userRepo,
		CodeRepo:       codeRepo,
		AttemptRepo:    attemptRepo,
		SessionRepo:    sessionRepo,
		SessionService: sessionService,
		SessionLoader:  sessionLoader,
		EventPublisher: eventPublisher,
		Authentication: authenticationService,
		PhoneChange:    phoneChangeService,
		EmailChange:    emailChangeService,
		Profile:        profileService,
		Logout:         logoutService,
		Sessions:       sessionsService,
		EmailMailer:    emailMailer,
	}, nil
}
