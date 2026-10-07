package domain

import (
	"time"

	"github.com/google/uuid"
)

// EmailChangeGrantTTL is the lifetime of an email-change grant: the window the
// user has to bind the new address, receive the code on it, and confirm the
// change (issue #721, grilling decision #720-2 — about 10 minutes, no resume
// between sessions). The countdown starts at issuance, on the current-address
// code check (protocol #1202).
const EmailChangeGrantTTL = 10 * time.Minute

// EmailChangeGrant is the proof, issued by the server, that the user confirmed
// the code sent to their current email. It is born without an address, on the
// current-address code check (verify-current); the new address binds to the
// live grant at the next step (request-new-email-code). From the binding on,
// the grant pins the delivery address for the resend and the final change. It
// expires after EmailChangeGrantTTL and is consumed (deleted) by the change
// that succeeds with it.
type EmailChangeGrant struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Email     *Email
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// NewEmailChangeGrant builds an addressless grant for the user, valid for
// EmailChangeGrantTTL from now. The token arrives already hashed: the service
// stores only the hash and shows the plaintext token to the client once.
func NewEmailChangeGrant(userID uuid.UUID, tokenHash string, now time.Time) EmailChangeGrant {
	return EmailChangeGrant{
		ID:        uuid.Must(uuid.NewV7()),
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: now.Add(EmailChangeGrantTTL),
		CreatedAt: now,
	}
}

// Expired reports whether the grant is no longer valid at now.
func (g EmailChangeGrant) Expired(now time.Time) bool {
	return !now.Before(g.ExpiresAt)
}
