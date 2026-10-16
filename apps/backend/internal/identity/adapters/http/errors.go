package http

import (
	"errors"
	"net/http"

	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
)

// writeSharedIdentityError maps the cross-cutting identity errors that recur
// across the auth handlers to their RFC 7807 responses and reports whether it
// wrote one. When it returns true the caller must stop handling the error.
//
// It collapses the ~5 duplicated switch arms (ErrUserBlocked ×5, ErrNotFound ×5,
// ErrEmailAlreadyTaken ×3, ErrCodeSentTooRecently ×3, ErrEmailDoesNotMatch ×2)
// that previously appeared in every auth handler (issue #222, grilling E2):
//
//   - ErrUserBlocked / domain.ErrTooManyAttempts / ErrCodeSentTooRecently /
//     ErrRecipientSendLimitExceeded / ErrInitiatorSendLimitExceeded → 429
//   - ErrEmailAlreadyTaken / ErrEmailDoesNotMatch → 409
//   - ErrNotFound → 401
//
// ErrNotFound is cross-cutting only in status code (401): each handler still
// owns its detail text, so notFoundDetail is threaded in rather than fixed
// here. Handler-specific errors (ErrInvalidEmail, ErrLoginCodeInvalid,
// ErrPhoneUnchanged, ErrPhoneAlreadyTaken, …) stay in each handler's local
// switch so the set of errors a handler can return stays readable. The
// anti-flood send limits (#1210) ride the 429 arm with the generic default
// text: one UX for every send refusal, and no address- or account-specific
// wording on a pre-auth-reachable path.
func writeSharedIdentityError(w http.ResponseWriter, r *http.Request, err error, notFoundDetail string) bool {
	switch {
	case errors.Is(err, application.ErrUserBlocked),
		errors.Is(err, domain.ErrTooManyAttempts),
		errors.Is(err, application.ErrCodeSentTooRecently),
		errors.Is(err, application.ErrRecipientSendLimitExceeded),
		errors.Is(err, application.ErrInitiatorSendLimitExceeded):
		httpsupport.WriteTooManyRequests(w, r, userFacingDetailOrDefault(err, "Превышен лимит запросов"))
		return true
	case errors.Is(err, application.ErrEmailAlreadyTaken),
		errors.Is(err, application.ErrEmailDoesNotMatch):
		httpsupport.WriteProblem(r.Context(), w, http.StatusConflict,
			httpsupport.Problem(r.Context(), "Conflict", userFacingDetailOrDefault(err, "Некорректные учётные данные")))
		return true
	case errors.Is(err, application.ErrNotFound):
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", notFoundDetail))
		return true
	}
	return false
}

// User-facing messages that recur outside this switch — the http tests assert
// them as fixtures — so they are named constants instead of inline literals.
const (
	detailUserBlocked = "Пользователь временно заблокирован"
	detailEmailTaken  = "Эта почта уже используется"
)

// userFacingDetail maps known identity domain/application errors to fixed,
// non-sensitive messages suitable for RFC 7807 problem details. It is the
// identity-local replacement for the identity cases that were removed from
// httpsupport.UserFacingDetail so that platform/httpsupport no longer imports
// identity/application (ADR 0034). The second result is false when err is not a
// recognized identity error.
func userFacingDetail(err error) (string, bool) {
	switch {
	case errors.Is(err, application.ErrUserBlocked):
		return detailUserBlocked, true
	case errors.Is(err, application.ErrCodeSentTooRecently):
		return "Код отправлен слишком недавно", true
	case errors.Is(err, application.ErrPhoneAlreadyTaken):
		return "Этот номер телефона уже используется", true
	case errors.Is(err, application.ErrPhoneUnchanged):
		return "Новый номер должен отличаться от текущего", true
	case errors.Is(err, domain.ErrTooManyAttempts):
		return "Слишком много попыток", true
	case errors.Is(err, application.ErrEmailDoesNotMatch):
		return "Некорректные учётные данные", true
	case errors.Is(err, application.ErrEmailAlreadyTaken):
		return detailEmailTaken, true
	}
	return "", false
}

// userFacingDetailOrDefault returns a user-facing message for err if one is
// defined, otherwise returns the provided default. It is the identity-local
// counterpart of httpsupport.UserFacingDetailOrDefault, consulted after the
// identity handler has already narrowed the error in its switch.
func userFacingDetailOrDefault(err error, defaultDetail string) string {
	if detail, ok := userFacingDetail(err); ok {
		return detail
	}
	return defaultDetail
}
