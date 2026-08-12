package http

import (
	"errors"

	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
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
		return "Пользователь временно заблокирован", true
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
		return "Эта почта уже используется", true
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
