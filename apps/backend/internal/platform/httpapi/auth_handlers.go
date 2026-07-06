package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
)

const maxRequestBodySize = 16 << 10 // 16 KiB

// retryAfterSeconds is the cooldown clients should wait before retrying a
// rate-limited auth request. It matches the service-layer minSendInterval.
const retryAfterSeconds = 60

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

// AuthHandlers implements the generated non-strict ServerInterface.
type AuthHandlers struct {
	auth              *application.AuthService
	billing           *billingapp.BillingService
	cookieSecure      bool
	logger            *slog.Logger
	phoneSend         *RateLimiter
	phoneVerify       *RateLimiter
	emailSend         *RateLimiter
	emailVerify       *RateLimiter
	phoneChangeSend   *RateLimiter
	phoneChangeVerify *RateLimiter
}

// NewAuthHandlers creates HTTP handlers for the auth API.
func NewAuthHandlers(
	auth *application.AuthService,
	billing *billingapp.BillingService,
	cookieSecure bool,
	logger *slog.Logger,
	phoneSend *RateLimiter,
	phoneVerify *RateLimiter,
	emailSend *RateLimiter,
	emailVerify *RateLimiter,
	phoneChangeSend *RateLimiter,
	phoneChangeVerify *RateLimiter,
) *AuthHandlers {
	return &AuthHandlers{
		auth:              auth,
		billing:           billing,
		cookieSecure:      cookieSecure,
		logger:            logger,
		phoneSend:         phoneSend,
		phoneVerify:       phoneVerify,
		emailSend:         emailSend,
		emailVerify:       emailVerify,
		phoneChangeSend:   phoneChangeSend,
		phoneChangeVerify: phoneChangeVerify,
	}
}

// SendPhoneCode implements POST /auth/phone/send.
func (h *AuthHandlers) SendPhoneCode(w http.ResponseWriter, r *http.Request) {
	var body openapi.SendPhoneCodeRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.WarnContext(r.Context(), "failed to decode request body", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	phone, err := domain.NewPhone(body.Phone)
	if err != nil {
		h.logger.WarnContext(r.Context(), "invalid phone in request body", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Invalid phone", "Некорректный номер телефона"))
		return
	}

	if h.phoneSend != nil && !h.phoneSend.Allow(phone.String()) {
		writeTooManyRequests(w, r, "Превышен лимит запросов")
		return
	}

	if err := h.auth.SendCode(r.Context(), phone); err != nil {
		switch {
		case errors.Is(err, application.ErrPhoneLoginDeprecated):
			detail, ok := UserFacingDetail(err)
			if !ok {
				writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
				return
			}
			writeProblem(w, http.StatusGone, problem(r.Context(), "Gone", detail))
		case errors.Is(err, application.ErrUserBlocked), errors.Is(err, application.ErrCodeSentTooRecently):
			writeTooManyRequests(w, r, userFacingDetailOrDefault(r.Context(), err, "Превышен лимит запросов"))
		default:
			writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// SendEmailCode implements POST /auth/email/send.
func (h *AuthHandlers) SendEmailCode(w http.ResponseWriter, r *http.Request) {
	var body openapi.SendEmailCodeRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.WarnContext(r.Context(), "failed to decode request body", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	phone, err := domain.NewPhone(body.Phone)
	if err != nil {
		h.logger.WarnContext(r.Context(), "invalid phone in request body", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Invalid phone", "Некорректный номер телефона"))
		return
	}

	email, err := domain.NewEmail(body.Email)
	if err != nil {
		h.logger.WarnContext(r.Context(), "invalid email in request body", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Invalid email", "Некорректная почта"))
		return
	}

	if h.emailSend != nil && !h.emailSend.Allow(email.String()) {
		writeTooManyRequests(w, r, "Превышен лимит запросов")
		return
	}

	if err := h.auth.SendEmailCode(r.Context(), phone, email); err != nil {
		switch {
		case errors.Is(err, application.ErrUserBlocked), errors.Is(err, application.ErrCodeSentTooRecently):
			writeTooManyRequests(w, r, userFacingDetailOrDefault(r.Context(), err, "Превышен лимит запросов"))
		default:
			writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// VerifyEmailCode implements POST /auth/email/verify.
func (h *AuthHandlers) VerifyEmailCode(w http.ResponseWriter, r *http.Request) {
	var body openapi.VerifyEmailCodeRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.WarnContext(r.Context(), "failed to decode request body", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	phone, err := domain.NewPhone(body.Phone)
	if err != nil {
		h.logger.WarnContext(r.Context(), "invalid phone in request body", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Invalid phone", "Некорректный номер телефона"))
		return
	}

	email, err := domain.NewEmail(body.Email)
	if err != nil {
		h.logger.WarnContext(r.Context(), "invalid email in request body", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Invalid email", "Некорректная почта"))
		return
	}

	if h.emailVerify != nil && !h.emailVerify.Allow(email.String()) {
		writeTooManyRequests(w, r, "Превышен лимит запросов")
		return
	}

	raw, user, err := h.auth.VerifyEmailCode(r.Context(), phone, email, body.Code)
	if err != nil {
		switch {
		case errors.Is(err, application.ErrUserBlocked),
			errors.Is(err, domain.ErrTooManyAttempts):
			writeTooManyRequests(w, r, userFacingDetailOrDefault(r.Context(), err, "Превышен лимит запросов"))
		case errors.Is(err, domain.ErrLoginCodeInvalid),
			errors.Is(err, application.ErrNotFound):
			writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Неверный телефон, почта или код"))
		case errors.Is(err, application.ErrEmailDoesNotMatch):
			writeProblem(w, http.StatusConflict, problem(r.Context(), "Conflict", userFacingDetailOrDefault(r.Context(), err, "Некорректные учётные данные")))
		default:
			writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
		}
		return
	}

	setSessionCookie(w, raw.Token, raw.Session.ExpiresAt, h.cookieSecure)
	writeJSON(r.Context(), w, http.StatusOK, meResponse(user))
}

// VerifyPhoneCode implements POST /auth/phone/verify.
func (h *AuthHandlers) VerifyPhoneCode(w http.ResponseWriter, r *http.Request) {
	var body openapi.VerifyPhoneCodeRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.WarnContext(r.Context(), "failed to decode request body", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	phone, err := domain.NewPhone(body.Phone)
	if err != nil {
		h.logger.WarnContext(r.Context(), "invalid phone in request body", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Invalid phone", "Некорректный номер телефона"))
		return
	}

	if h.phoneVerify != nil && !h.phoneVerify.Allow(phone.String()) {
		writeTooManyRequests(w, r, "Превышен лимит запросов")
		return
	}

	raw, user, err := h.auth.VerifyCode(r.Context(), phone, body.Code)
	if err != nil {
		switch {
		case errors.Is(err, application.ErrPhoneLoginDeprecated):
			detail, ok := UserFacingDetail(err)
			if !ok {
				writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
				return
			}
			writeProblem(w, http.StatusGone, problem(r.Context(), "Gone", detail))
		case errors.Is(err, application.ErrUserBlocked),
			errors.Is(err, domain.ErrTooManyAttempts):
			writeTooManyRequests(w, r, userFacingDetailOrDefault(r.Context(), err, "Превышен лимит запросов"))
		case errors.Is(err, domain.ErrLoginCodeInvalid),
			errors.Is(err, application.ErrNotFound):
			writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Неверный телефон или код"))
		default:
			writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
		}
		return
	}

	setSessionCookie(w, raw.Token, raw.Session.ExpiresAt, h.cookieSecure)
	writeJSON(r.Context(), w, http.StatusOK, meResponse(user))
}

// Logout implements POST /auth/logout.
func (h *AuthHandlers) Logout(w http.ResponseWriter, r *http.Request) {
	token := sessionTokenFromRequest(r, h.cookieSecure)
	if token == "" {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if err := h.auth.Logout(r.Context(), hashSessionToken(token)); err != nil {
		h.logger.ErrorContext(r.Context(), "logout failed", slog.String("error", sanitizeError(err)))
	}

	clearSessionCookie(w, h.cookieSecure)
	w.WriteHeader(http.StatusNoContent)
}

// LogoutAll implements POST /auth/logout-all.
func (h *AuthHandlers) LogoutAll(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if err := h.auth.LogoutAll(r.Context(), userID); err != nil {
		h.logger.ErrorContext(r.Context(), "logout all failed", slog.String("error", sanitizeError(err)))
		clearSessionCookie(w, h.cookieSecure)
		writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
		return
	}

	clearSessionCookie(w, h.cookieSecure)
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandlers) enrichSubscription(ctx context.Context, userID uuid.UUID, resp *openapi.MeResponse) error {
	if h.billing == nil {
		return nil
	}
	view, err := h.billing.GetSubscription(ctx, userID)
	if err != nil {
		if errors.Is(err, billingapp.ErrSubscriptionNotFound) {
			return nil
		}
		return err
	}
	s := subscriptionResponse(view)
	resp.Subscription = &s
	return nil
}

// GetMe implements GET /me.
func (h *AuthHandlers) GetMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	user, ok := UserFromContext(r.Context())
	if !ok {
		var err error
		user, err = h.auth.Me(r.Context(), userID)
		if err != nil {
			if errors.Is(err, application.ErrNotFound) {
				writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Сессия недействительна"))
				return
			}
			writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
			return
		}
	}

	resp := meResponse(user)
	if err := h.enrichSubscription(r.Context(), userID, &resp); err != nil {
		writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, resp)
}

func optionalStringFromPtr(s *string) domain.Optional[string] {
	if s == nil {
		return domain.Optional[string]{Set: false}
	}
	return domain.Optional[string]{Value: *s, Set: true}
}

// UpdateMe implements PATCH /me.
func (h *AuthHandlers) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.UserUpdateRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.WarnContext(r.Context(), "failed to decode request body", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	cmd := application.UpdateUserCommand{
		Email:      optionalStringFromPtr(body.Email),
		Name:       optionalStringFromPtr(body.Name),
		Surname:    optionalStringFromPtr(body.Surname),
		Patronymic: optionalStringFromPtr(body.Patronymic),
	}

	user, err := h.auth.UpdateUser(r.Context(), userID, cmd)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidEmail):
			writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Invalid email", "Некорректный формат email"))
		case errors.Is(err, application.ErrEmailAlreadyTaken):
			writeProblem(w, http.StatusConflict, problem(r.Context(), "Conflict", userFacingDetailOrDefault(r.Context(), err, "Этот email уже используется")))
		case errors.Is(err, application.ErrNotFound):
			writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Сессия недействительна"))
		default:
			writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
		}
		return
	}

	resp := meResponse(user)
	if err := h.enrichSubscription(r.Context(), userID, &resp); err != nil {
		writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, resp)
}

// SendPhoneChangeCode implements POST /me/phone/send-code.
func (h *AuthHandlers) SendPhoneChangeCode(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.SendPhoneChangeCodeRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.WarnContext(r.Context(), "failed to decode request body", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	phone, err := domain.NewPhone(body.Phone)
	if err != nil {
		h.logger.WarnContext(r.Context(), "invalid phone in request body", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Invalid phone", "Некорректный номер телефона"))
		return
	}

	if h.phoneChangeSend != nil && !h.phoneChangeSend.Allow(phone.String()) {
		writeTooManyRequests(w, r, "Превышен лимит запросов")
		return
	}

	if err := h.auth.SendPhoneChangeCode(r.Context(), userID, phone); err != nil {
		switch {
		case errors.Is(err, application.ErrPhoneUnchanged):
			writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Invalid phone", userFacingDetailOrDefault(r.Context(), err, "Новый номер должен отличаться от текущего")))
		case errors.Is(err, application.ErrPhoneAlreadyTaken):
			writeProblem(w, http.StatusConflict, problem(r.Context(), "Conflict", userFacingDetailOrDefault(r.Context(), err, "Этот номер телефона уже используется")))
		case errors.Is(err, application.ErrUserBlocked):
			writeTooManyRequests(w, r, userFacingDetailOrDefault(r.Context(), err, "Слишком много попыток"))
		case errors.Is(err, application.ErrCodeSentTooRecently):
			writeTooManyRequests(w, r, userFacingDetailOrDefault(r.Context(), err, "Код отправлен слишком недавно"))
		default:
			writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ChangePhone implements POST /me/phone/change.
func (h *AuthHandlers) ChangePhone(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	token := sessionTokenFromRequest(r, h.cookieSecure)
	if token == "" {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.ChangePhoneRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.WarnContext(r.Context(), "failed to decode request body", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	phone, err := domain.NewPhone(body.Phone)
	if err != nil {
		h.logger.WarnContext(r.Context(), "invalid phone in request body", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Invalid phone", "Некорректный номер телефона"))
		return
	}

	if h.phoneChangeVerify != nil && !h.phoneChangeVerify.Allow(phone.String()) {
		writeTooManyRequests(w, r, "Превышен лимит запросов")
		return
	}

	user, err := h.auth.ChangePhone(r.Context(), userID, phone, body.Code, hashSessionToken(token))
	if err != nil {
		switch {
		case errors.Is(err, application.ErrPhoneUnchanged):
			writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Invalid phone", userFacingDetailOrDefault(r.Context(), err, "Новый номер должен отличаться от текущего")))
		case errors.Is(err, application.ErrPhoneAlreadyTaken):
			writeProblem(w, http.StatusConflict, problem(r.Context(), "Conflict", userFacingDetailOrDefault(r.Context(), err, "Этот номер телефона уже используется")))
		case errors.Is(err, application.ErrUserBlocked),
			errors.Is(err, domain.ErrTooManyAttempts):
			writeTooManyRequests(w, r, userFacingDetailOrDefault(r.Context(), err, "Слишком много попыток"))
		case errors.Is(err, domain.ErrLoginCodeInvalid),
			errors.Is(err, application.ErrNotFound):
			writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Неверный код"))
		default:
			writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
		}
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, meResponse(user))
}

func meResponse(user domain.User) openapi.MeResponse {
	return openapi.MeResponse{
		Id:         user.ID,
		Phone:      user.Phone.String(),
		Role:       openapi.MeResponseRole(user.Role),
		Name:       user.Name,
		Surname:    user.Surname,
		Patronymic: user.Patronymic,
		Email:      user.Email,
	}
}

func writeJSON(ctx context.Context, w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		loggerFromContext(ctx).ErrorContext(ctx, "failed to encode JSON response", slog.String("error", sanitizeError(err)))
	}
}

// writeTooManyRequests writes a 429 RFC 7807 problem response with a
// Retry-After header so clients can back off before retrying.
func writeTooManyRequests(w http.ResponseWriter, r *http.Request, detail string) {
	w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
	writeProblem(w, http.StatusTooManyRequests, problem(r.Context(), "Too many requests", detail))
}

// userFacingDetailOrDefault returns a user-facing message for err if one is
// defined, otherwise returns the provided default.
func userFacingDetailOrDefault(ctx context.Context, err error, defaultDetail string) string {
	if detail, ok := UserFacingDetail(err); ok {
		return detail
	}
	return defaultDetail
}
