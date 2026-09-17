package domain

import (
	"time"

	"github.com/google/uuid"
)

// EmailChangeGrantTTL is the lifetime of an email-change grant: the window the
// user has to receive the code on the new address and confirm it (issue #721,
// grilling decision #720-2 — about 10 minutes, no resume between sessions).
const EmailChangeGrantTTL = 10 * time.Minute

// EmailChangeGrant is the proof, issued by the server, that the user confirmed
// the code sent to their current email, so a code may be issued for the new
// one. It binds the confirmed new address to the user for one change: it
// expires after EmailChangeGrantTTL and is consumed (deleted) by the change
// that succeeds with it.
type EmailChangeGrant struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Email     Email
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// NewEmailChangeGrant builds a grant binding the new email to the user for
// EmailChangeGrantTTL from now. The token arrives already hashed: the service
// stores only the hash and shows the plaintext token to the client once.
func NewEmailChangeGrant(userID uuid.UUID, email Email, tokenHash string, now time.Time) EmailChangeGrant {
	return EmailChangeGrant{
		ID:        uuid.Must(uuid.NewV7()),
		UserID:    userID,
		Email:     email,
		TokenHash: tokenHash,
		ExpiresAt: now.Add(EmailChangeGrantTTL),
		CreatedAt: now,
	}
}

// Expired reports whether the grant is no longer valid at now.
func (g EmailChangeGrant) Expired(now time.Time) bool {
	return !now.Before(g.ExpiresAt)
}
