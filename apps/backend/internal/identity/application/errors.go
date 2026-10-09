package application

import "errors"

var (
	ErrNotFound                = errors.New("not found")
	ErrUserBlocked             = errors.New("user is temporarily blocked")
	ErrCodeSentTooRecently     = errors.New("code sent too recently")
	ErrPhoneAlreadyTaken       = errors.New("phone already taken")
	ErrPhoneUnchanged          = errors.New("new phone must differ from current phone")
	ErrEmailAlreadyTaken       = errors.New("email already taken")
	ErrEmailUnchanged          = errors.New("new email must differ from current email")
	ErrEmailDoesNotMatch       = errors.New("email does not match the phone number")
	ErrEmailChangeGrantInvalid = errors.New("email change grant is invalid, expired, or already used")
	// ErrEmailChangeBudgetExhausted reports the per-user hourly budget on code
	// sends to NEW addresses is spent (decision #720-3).
	ErrEmailChangeBudgetExhausted = errors.New("email change send budget is exhausted")
	// ErrCurrentSession reports the requested session revocation targets the
	// caller's own live session; the devices list ends it through logout
	// instead (issue #728).
	ErrCurrentSession = errors.New("cannot revoke the current session")
)

// ErrPhotoNotFound marks a photo request for an entity that has no photo —
// the privacy-preserving 404 of the photo serving endpoints (ADR 0065).
var ErrPhotoNotFound = errors.New("identity: photo not found")
