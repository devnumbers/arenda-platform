package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

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

// MeEnricher augments a MeResponse with cross-cutting data (for example, the
// current subscription). Transport code may pass nil when no enrichment is
// required.
type MeEnricher func(ctx context.Context, userID uuid.UUID, resp *openapi.MeResponse) error

// AuthHandlers implements the generated non-strict ServerInterface.
type AuthHandlers struct {
	auth              application.Authenticator
	phoneChange       application.PhoneChanger
	profile           application.Profiler
	logout            application.Logout
	cookieSecure      bool
	logger            *slog.Logger
	emailSend         *RateLimiter
	emailVerify       *RateLimiter
	phoneChangeSend   *RateLimiter
	phoneChangeVerify *RateLimiter
	meEnricher        MeEnricher
}

// NewAuthHandlers creates HTTP handlers for the auth API.
func NewAuthHandlers(
	auth application.Authenticator,
	phoneChange application.PhoneChanger,
	profile application.Profiler,
	logout application.Logout,
	cookieSecure bool,
	logger *slog.Logger,
	emailSend *RateLimiter,
	emailVerify *RateLimiter,
	phoneChangeSend *RateLimiter,
	phoneChangeVerify *RateLimiter,
	meEnricher MeEnricher,
) *AuthHandlers {
	return &AuthHandlers{
		auth:              auth,
		phoneChange:       phoneChange,
		profile:           profile,
		logout:            logout,
		cookieSecure:      cookieSecure,
		logger:            logger,
		emailSend:         emailSend,
		emailVerify:       emailVerify,
		phoneChangeSend:   phoneChangeSend,
		phoneChangeVerify: phoneChangeVerify,
		meEnricher:        meEnricher,
	}
}

// authSendCodeResponse is the JSON body returned by POST /auth/send.
type authSendCodeResponse struct {
	Sent       bool `json:"sent"`
	RetryAfter *int `json:"retryAfter,omitempty"`
}

// SendCode implements POST /auth/send.
func (h *AuthHandlers) SendCode(w http.ResponseWriter, r *http.Request) {
	var body openapi.SendCodeRequest
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

	email, ok := parseOptionalEmail(w, r, body.Email)
	if !ok {
		return
	}

	// Phone-only and email requests use separate buckets; the service-level
	// minSendInterval still caps actual code sends.
	rateLimitKey := phone.String()
	if email != nil {
		rateLimitKey = email.String()
	}
	if h.emailSend != nil && !h.emailSend.Allow(rateLimitKey) {
		writeTooManyRequests(w, r, "Превышен лимит запросов")
		return
	}

	retry := retryAfterSeconds
	if email == nil {
		sent, err := h.auth.SendCodeByPhone(r.Context(), phone)
		if err != nil {
			switch {
			case errors.Is(err, application.ErrUserBlocked), errors.Is(err, application.ErrCodeSentTooRecently):
				writeTooManyRequests(w, r, userFacingDetailOrDefault(r.Context(), err, "Превышен лимит запросов"))
			default:
				writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
			}
			return
		}
		if !sent {
			writeJSON(r.Context(), w, http.StatusOK, authSendCodeResponse{Sent: false})
			return
		}
		writeJSON(r.Context(), w, http.StatusOK, authSendCodeResponse{Sent: true, RetryAfter: &retry})
		return
	}

	if err := h.auth.SendCode(r.Context(), phone, *email, domain.LoginCodePurposeLogin); err != nil {
		switch {
		case errors.Is(err, application.ErrUserBlocked), errors.Is(err, application.ErrCodeSentTooRecently):
			writeTooManyRequests(w, r, userFacingDetailOrDefault(r.Context(), err, "Превышен лимит запросов"))
		case errors.Is(err, application.ErrEmailDoesNotMatch):
			writeProblem(w, http.StatusConflict, problem(r.Context(), "Conflict", userFacingDetailOrDefault(r.Context(), err, "Некорректные учётные данные")))
		case errors.Is(err, application.ErrEmailAlreadyTaken):
			writeProblem(w, http.StatusConflict, problem(r.Context(), "Conflict", userFacingDetailOrDefault(r.Context(), err, "Этот email уже используется")))
		default:
			writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
		}
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, authSendCodeResponse{Sent: true, RetryAfter: &retry})
}

// VerifyCode implements POST /auth/verify.
func (h *AuthHandlers) VerifyCode(w http.ResponseWriter, r *http.Request) {
	var body openapi.VerifyCodeRequest
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

	email, ok := parseOptionalEmail(w, r, body.Email)
	if !ok {
		return
	}

	// Phone-only verify requests are keyed by phone; brute force is
	// additionally capped by the service-level attempt window.
	rateLimitKey := phone.String()
	if email != nil {
		rateLimitKey = email.String()
	}
	if h.emailVerify != nil && !h.emailVerify.Allow(rateLimitKey) {
		writeTooManyRequests(w, r, "Превышен лимит запросов")
		return
	}

	raw, user, err := h.auth.VerifyCode(r.Context(), phone, email, body.Code)
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
		case errors.Is(err, application.ErrEmailAlreadyTaken):
			writeProblem(w, http.StatusConflict, problem(r.Context(), "Conflict", userFacingDetailOrDefault(r.Context(), err, "Этот email уже используется")))
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

	if err := h.logout.Logout(r.Context(), token); err != nil {
		if !errors.Is(err, application.ErrNotFound) {
			h.logger.ErrorContext(r.Context(), "logout failed", slog.String("error", sanitizeError(err)))
			clearSessionCookie(w, h.cookieSecure)
			writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
			return
		}
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

	if err := h.logout.LogoutAll(r.Context(), userID); err != nil {
		h.logger.ErrorContext(r.Context(), "logout all failed", slog.String("error", sanitizeError(err)))
		clearSessionCookie(w, h.cookieSecure)
		writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
		return
	}

	clearSessionCookie(w, h.cookieSecure)
	w.WriteHeader(http.StatusNoContent)
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
		user, err = h.profile.Me(r.Context(), userID)
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
	if h.meEnricher != nil {
		if err := h.meEnricher(r.Context(), userID, &resp); err != nil {
			writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
			return
		}
	}

	writeJSON(r.Context(), w, http.StatusOK, resp)
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

	cmd := application.UpdateProfileCommand{
		Email:      body.Email,
		Name:       body.Name,
		Surname:    body.Surname,
		Patronymic: body.Patronymic,
	}

	user, err := h.profile.UpdateProfile(r.Context(), userID, cmd)
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
	if h.meEnricher != nil {
		if err := h.meEnricher(r.Context(), userID, &resp); err != nil {
			writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
			return
		}
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

	if err := h.phoneChange.SendChangeCode(r.Context(), userID, phone); err != nil {
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

	user, err := h.phoneChange.ChangePhone(r.Context(), userID, phone, body.Code, token)
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
	var email *string
	if user.Email != nil {
		s := user.Email.String()
		email = &s
	}
	return openapi.MeResponse{
		Id:         user.ID,
		Phone:      user.Phone.String(),
		Role:       openapi.MeResponseRole(user.Role),
		Name:       user.Name,
		Surname:    user.Surname,
		Patronymic: user.Patronymic,
		Email:      email,
	}
}

// BillingMeEnricher returns a MeEnricher that adds the current billing
// subscription to a MeResponse. It keeps the billing-to-OpenAPI mapping in the
// HTTP layer so the application layer does not depend on openapi types.
func BillingMeEnricher(billing billingapp.Subscriber) MeEnricher {
	return func(ctx context.Context, userID uuid.UUID, resp *openapi.MeResponse) error {
		if billing == nil {
			return nil
		}
		view, err := billing.GetSubscription(ctx, userID)
		if err != nil {
			if errors.Is(err, billingapp.ErrSubscriptionNotFound) {
				return nil
			}
			return err
		}
		sub := subscriptionResponse(view)
		resp.Subscription = &sub
		return nil
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

// parseOptionalEmail parses an optional request email. It returns ok=false
// after writing a 400 problem response when the provided email is invalid.
// A nil or blank email is treated as absent and yields (nil, true).
func parseOptionalEmail(w http.ResponseWriter, r *http.Request, raw *string) (*domain.Email, bool) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, true
	}
	parsed, err := domain.NewEmail(*raw)
	if err != nil {
		loggerFromContext(r.Context()).WarnContext(r.Context(), "invalid email in request body", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Invalid email", "Некорректная почта"))
		return nil, false
	}
	return &parsed, true
}

// userFacingDetailOrDefault returns a user-facing message for err if one is
// defined, otherwise returns the provided default.
func userFacingDetailOrDefault(ctx context.Context, err error, defaultDetail string) string {
	if detail, ok := UserFacingDetail(err); ok {
		return detail
	}
	return defaultDetail
}
