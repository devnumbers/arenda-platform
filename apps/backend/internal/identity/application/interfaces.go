package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

// Authenticator issues and verifies login codes.
type Authenticator interface {
	SendCode(ctx context.Context, phone domain.Phone, email domain.Email, purpose domain.LoginCodePurpose) error
	// SendCodeByPhone sends a login code to the email stored for the given
	// phone. It returns sent=false when the user does not exist or has no
	// email on file; the caller should then ask the user for an email.
	SendCodeByPhone(ctx context.Context, phone domain.Phone) (sent bool, err error)
	// VerifyCode accepts an optional email; nil resolves the email from the
	// stored user record for the phone.
	VerifyCode(ctx context.Context, phone domain.Phone, email *domain.Email, code string) (domain.RawSession, domain.User, error)
}

// PhoneChanger handles phone-number change for authenticated users.
type PhoneChanger interface {
	SendChangeCode(ctx context.Context, userID uuid.UUID, newPhone domain.Phone) error
	ChangePhone(ctx context.Context, userID uuid.UUID, newPhone domain.Phone, code, currentToken string) (domain.User, error)
}

// Profiler provides the current user's profile and updates it.
type Profiler interface {
	Me(ctx context.Context, userID uuid.UUID) (domain.User, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, cmd UpdateProfileCommand) (domain.User, error)
}

// Logout terminates sessions.
type Logout interface {
	Logout(ctx context.Context, rawToken string) error
	LogoutAll(ctx context.Context, userID uuid.UUID) error
}
