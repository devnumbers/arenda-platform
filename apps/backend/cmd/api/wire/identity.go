package wire

import (
	"context"
	"fmt"

	identityemail "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/email"
	identityevents "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/events"
	identityhttp "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/http"
	identitypg "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/postgres"
	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/events"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
	mailerfake "github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer/fake"
	mailersmtp "github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer/smtp"
)

// Identity holds the identity module's services and event publisher wired by
// WireIdentity.
type Identity struct {
	UserRepo       *identitypg.UserRepository
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
	Profile        *identityapp.ProfileService
	Logout         *identityapp.LogoutService
	// EmailMailer is the configured mailer.Sender (smtp or fake). It is exposed
	// because the notification reminder worker reuses it to send emails.
	EmailMailer mailer.Sender
}

// WireIdentity constructs the identity repositories, session service, event
// publisher, selects the email mailer based on config, and builds the
// authentication, phone-change, profile and logout services. It takes the event
// dispatcher (for the publisher) and the reminder service (notifications module)
// that profile depends on.
func WireIdentity(
	_ context.Context,
	p platformDeps,
	eventDispatcher *events.InProcessDispatcher,
	reminderService *notificationsapp.ReminderService,
) (*Identity, error) {
	userRepo := identitypg.NewUserRepository(p.DB, p.Encryptor)
	codeRepo := identitypg.NewLoginCodeRepository(p.DB, p.Encryptor)
	attemptRepo := identitypg.NewAttemptRepository(p.DB, p.Encryptor)
	sessionRepo := identitypg.NewSessionRepository(p.DB, p.Encryptor)
	sessionService := identityapp.NewSessionService(
		userRepo, codeRepo, attemptRepo, sessionRepo, p.Encryptor,
		identityapp.SessionServiceConfig{Audit: p.AuditRecorder, UoW: p.UoW},
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
	case "fake":
		emailMailer = mailerfake.NewFakeSender(p.Logger)
	default:
		return nil, fmt.Errorf("unsupported EMAIL_SENDER: %s", p.Cfg.EmailSender)
	}

	emailSender := identityemail.NewSender(emailMailer, p.Renderer)

	// loginCodeService is the deep module for login-code issuance/verification
	// shared by AuthenticationService and PhoneChangeService (ADR 0033, step 4).
	loginCodeService := identityapp.NewLoginCodeService(
		userRepo, codeRepo, attemptRepo, sessionRepo,
		identityapp.LoginCodeServiceConfig{
			CodeSender: emailSender,
			Clock:      p.Clock,
			Hasher:     p.Encryptor,
			Logger:     p.Logger,
			Audit:      p.AuditRecorder,
			UoW:        p.UoW,
		},
	)

	authenticationService := identityapp.NewAuthenticationService(
		userRepo,
		codeRepo,
		attemptRepo,
		sessionRepo,
		identityapp.AuthenticationServiceConfig{
			LoginCodes: loginCodeService,
			Sessions:   sessionService,
			Publisher:  eventPublisher,
			Clock:      p.Clock,
			Logger:     p.Logger,
			Audit:      p.AuditRecorder,
			UoW:        p.UoW,
		},
	)

	phoneChangeService := identityapp.NewPhoneChangeService(
		userRepo,
		codeRepo,
		attemptRepo,
		sessionRepo,
		identityapp.PhoneChangeServiceConfig{
			LoginCodes: loginCodeService,
			Clock:      p.Clock,
			Hasher:     p.Encryptor,
			Logger:     p.Logger,
			Audit:      p.AuditRecorder,
			UoW:        p.UoW,
		},
	)

	profileService := identityapp.NewProfileService(
		userRepo,
		codeRepo,
		attemptRepo,
		sessionRepo,
		identityapp.ProfileServiceConfig{
			Audit:               p.AuditRecorder,
			UoW:                 p.UoW,
			ReminderRescheduler: reminderService,
		},
	)

	logoutService := identityapp.NewLogoutService(
		userRepo,
		codeRepo,
		attemptRepo,
		sessionRepo,
		identityapp.LogoutServiceConfig{
			Hasher: p.Encryptor,
			Audit:  p.AuditRecorder,
			UoW:    p.UoW,
		},
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
		Profile:        profileService,
		Logout:         logoutService,
		EmailMailer:    emailMailer,
	}, nil
}
