package application

import "errors"

var (
	ErrNotFound            = errors.New("not found")
	ErrUserBlocked         = errors.New("user is temporarily blocked")
	ErrCodeSentTooRecently = errors.New("code sent too recently")
	ErrEmailAlreadyTaken   = errors.New("email already taken")
	ErrEmailUnchanged      = errors.New("new email must differ from current email")
	// ErrEmailChangeGrantInvalid reports the one-time grant of the email-change
	// protocol is invalid, expired, or already used (#721).
	ErrEmailChangeGrantInvalid = errors.New("email change grant is invalid, expired, or already used")
	// ErrEmailChangeBudgetExhausted reports the per-user hourly budget on code
	// sends to NEW addresses is spent (decision #720-3).
	ErrEmailChangeBudgetExhausted = errors.New("email change send budget is exhausted")
	// ErrRecipientSendLimitExceeded reports the hourly cap on code sends
	// delivered to one email address is spent, across all flows and
	// initiators (#1210, OWASP per-account anti-flood).
	ErrRecipientSendLimitExceeded = errors.New("recipient code send limit is exhausted")
	// ErrInitiatorSendLimitExceeded reports the authenticated user's hourly
	// cap on code sends they initiate in the email-change flow is
	// spent (#1210).
	ErrInitiatorSendLimitExceeded = errors.New("initiator code send limit is exhausted")
	// ErrCurrentSession reports the requested session revocation targets the
	// caller's own live session; the devices list ends it through logout
	// instead (issue #728).
	ErrCurrentSession = errors.New("cannot revoke the current session")
)

// ErrPhotoNotFound marks a photo request for an entity that has no photo —
// the privacy-preserving 404 of the photo serving endpoints (ADR 0065).
var ErrPhotoNotFound = errors.New("identity: photo not found")
