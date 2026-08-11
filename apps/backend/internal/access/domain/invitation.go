package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Invitation is a pending shared-access grant sent to an email that is not
// (yet) registered (issue #161, T5). A pending invitation never expires: it
// activates automatically when a user registers with the same email, applying
// the role current at that moment. Cancellation deletes the row, so the same
// email can be invited again freely afterwards.
type Invitation struct {
	ID         uuid.UUID
	PropertyID uuid.UUID
	Email      string // normalized (lowercase, trimmed); PII — never in audit context (ADR 0020)
	Role       Role
	InvitedBy  uuid.UUID
	LastSentAt time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// InvitationResendCooldown is the manual invite resend cooldown: the invite
// email may be re-sent at most once per 24 hours per invitation.
const InvitationResendCooldown = 24 * time.Hour

// Sentinel errors for the invitation lifecycle.
var (
	// ErrInvitationAlreadyExists is returned when a pending invitation already
	// exists for the (property, email) pair.
	ErrInvitationAlreadyExists = errors.New("invitation already exists")
	// ErrInvitationNotFound is returned when the invitation does not exist or
	// does not belong to the property (privacy-preserving not-found).
	ErrInvitationNotFound = errors.New("invitation not found")
	// ErrInvitationResendCooldown is matched (via errors.Is) by
	// ResendCooldownError when the 24h resend cooldown has not elapsed.
	ErrInvitationResendCooldown = errors.New("invitation resend cooldown")
	// ErrInvalidEmail is returned when an invitee email fails validation.
	ErrInvalidEmail = errors.New("invalid email")
	// ErrUserNotFound is returned by the user lookup port when no registered
	// user matches the given id or email.
	ErrUserNotFound = errors.New("user not found")
)

// ResendCooldownError reports a manual resend attempted before the 24h
// cooldown elapsed. RetryAfter is the remaining wait. It matches
// ErrInvitationResendCooldown via errors.Is.
type ResendCooldownError struct {
	RetryAfter time.Duration
}

// Error implements error.
func (e *ResendCooldownError) Error() string {
	return ErrInvitationResendCooldown.Error()
}

// Is makes errors.Is(err, ErrInvitationResendCooldown) true for this error.
func (e *ResendCooldownError) Is(target error) bool {
	return target == ErrInvitationResendCooldown
}

var invitationEmailRegex = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// NormalizeEmail lowercases and trims an invitee email and validates its
// shape. Invitations are stored and matched in this normalized form, so a
// registration with the same email in any casing activates them.
func NormalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" || !invitationEmailRegex.MatchString(email) {
		return "", ErrInvalidEmail
	}
	return email, nil
}
