package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
)

const maxRequestBodySize = 16 << 10 // 16 KiB

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

// AuthHandlers implements the generated non-strict ServerInterface.
type AuthHandlers struct {
	auth         *application.AuthService
	billing      *billingapp.BillingService
	cookieSecure bool
	logger       *slog.Logger
	phoneSend    *RateLimiter
	phoneVerify  *RateLimiter
}

// NewAuthHandlers creates HTTP handlers for the auth API.
func NewAuthHandlers(
	auth *application.AuthService,
	billing *billingapp.BillingService,
	cookieSecure bool,
	logger *slog.Logger,
	phoneSend *RateLimiter,
	phoneVerify *RateLimiter,
) *AuthHandlers {
	return &AuthHandlers{
		auth:         auth,
		billing:      billing,
		cookieSecure: cookieSecure,
		logger:       logger,
		phoneSend:    phoneSend,
		phoneVerify:  phoneVerify,
	}
}

// SendPhoneCode implements POST /auth/phone/send.
func (h *AuthHandlers) SendPhoneCode(w http.ResponseWriter, r *http.Request) {
	var body openapi.SendPhoneCodeRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.WarnContext(r.Context(), "failed to decode request body", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "invalid request body"))
		return
	}

	phone, err := domain.NewPhone(body.Phone)
	if err != nil {
		h.logger.WarnContext(r.Context(), "invalid phone in request body", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Invalid phone", "invalid phone"))
		return
	}

	if h.phoneSend != nil && !h.phoneSend.Allow(phone.String()) {
		writeProblem(w, http.StatusTooManyRequests, problem(r.Context(), "Too many requests", "rate limit exceeded"))
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
			detail, ok := UserFacingDetail(err)
			if !ok {
				writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
				return
			}
			writeProblem(w, http.StatusTooManyRequests, problem(r.Context(), "Too many requests", detail))
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
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "invalid request body"))
		return
	}

	_ = body
	writeProblem(w, http.StatusNotImplemented, problem(r.Context(), "Not Implemented", "email login is not yet implemented"))
}

// VerifyEmailCode implements POST /auth/email/verify.
func (h *AuthHandlers) VerifyEmailCode(w http.ResponseWriter, r *http.Request) {
	var body openapi.VerifyEmailCodeRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.WarnContext(r.Context(), "failed to decode request body", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "invalid request body"))
		return
	}

	_ = body
	writeProblem(w, http.StatusNotImplemented, problem(r.Context(), "Not Implemented", "email login is not yet implemented"))
}

// VerifyPhoneCode implements POST /auth/phone/verify.
func (h *AuthHandlers) VerifyPhoneCode(w http.ResponseWriter, r *http.Request) {
	var body openapi.VerifyPhoneCodeRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.WarnContext(r.Context(), "failed to decode request body", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "invalid request body"))
		return
	}

	phone, err := domain.NewPhone(body.Phone)
	if err != nil {
		h.logger.WarnContext(r.Context(), "invalid phone in request body", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Invalid phone", "invalid phone"))
		return
	}

	if h.phoneVerify != nil && !h.phoneVerify.Allow(phone.String()) {
		writeProblem(w, http.StatusTooManyRequests, problem(r.Context(), "Too many requests", "rate limit exceeded"))
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
			detail, ok := UserFacingDetail(err)
			if !ok {
				writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
				return
			}
			writeProblem(w, http.StatusTooManyRequests, problem(r.Context(), "Too many requests", detail))
		case errors.Is(err, domain.ErrLoginCodeInvalid),
			errors.Is(err, application.ErrNotFound):
			writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "invalid phone or code"))
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
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
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
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	if err := h.auth.LogoutAll(r.Context(), userID); err != nil {
		h.logger.ErrorContext(r.Context(), "logout all failed", slog.String("error", sanitizeError(err)))
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
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	user, ok := UserFromContext(r.Context())
	if !ok {
		var err error
		user, err = h.auth.Me(r.Context(), userID)
		if err != nil {
			if errors.Is(err, application.ErrNotFound) {
				writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session invalid"))
				return
			}
			writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
			return
		}
	}

	resp := meResponse(user)
	if h.billing != nil {
		view, err := h.billing.GetSubscription(r.Context(), userID)
		if err != nil {
			if !errors.Is(err, billingapp.ErrSubscriptionNotFound) {
				writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
				return
			}
		} else {
			s := subscriptionResponse(view)
			resp.Subscription = &s
		}
	}

	writeJSON(r.Context(), w, http.StatusOK, resp)
}

// UpdateMe implements PATCH /me.
func (h *AuthHandlers) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	type updateMeRequest struct {
		Name       domain.Optional[string] `json:"name"`
		Surname    domain.Optional[string] `json:"surname"`
		Patronymic domain.Optional[string] `json:"patronymic"`
		Email      domain.Optional[string] `json:"email"`
	}

	var body updateMeRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.WarnContext(r.Context(), "failed to decode request body", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "invalid request body"))
		return
	}

	user, err := h.auth.UpdateUser(r.Context(), userID, application.UpdateUserCommand{
		Name:       body.Name,
		Surname:    body.Surname,
		Patronymic: body.Patronymic,
		Email:      body.Email,
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidEmail):
			writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Invalid email", "invalid email format"))
		case errors.Is(err, application.ErrEmailAlreadyTaken):
			writeProblem(w, http.StatusConflict, problem(r.Context(), "Conflict", "Email is already in use"))
		case errors.Is(err, application.ErrNotFound):
			writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session invalid"))
		default:
			writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
		}
		return
	}

	resp := meResponse(user)
	if h.billing != nil {
		view, err := h.billing.GetSubscription(r.Context(), userID)
		if err != nil {
			if !errors.Is(err, billingapp.ErrSubscriptionNotFound) {
				writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
				return
			}
		} else {
			s := subscriptionResponse(view)
			resp.Subscription = &s
		}
	}

	writeJSON(r.Context(), w, http.StatusOK, resp)
}

// SendPhoneChangeCode implements POST /me/phone/send-code.
func (h *AuthHandlers) SendPhoneChangeCode(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	var body openapi.SendPhoneChangeCodeRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.WarnContext(r.Context(), "failed to decode request body", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "invalid request body"))
		return
	}

	phone, err := domain.NewPhone(body.Phone)
	if err != nil {
		h.logger.WarnContext(r.Context(), "invalid phone in request body", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Invalid phone", "invalid phone"))
		return
	}

	if h.phoneSend != nil && !h.phoneSend.Allow(phone.String()) {
		writeProblem(w, http.StatusTooManyRequests, problem(r.Context(), "Too many requests", "rate limit exceeded"))
		return
	}

	if err := h.auth.SendPhoneChangeCode(r.Context(), userID, phone); err != nil {
		switch {
		case errors.Is(err, application.ErrPhoneUnchanged):
			writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Invalid phone", "Новый номер не должен совпадать с текущим"))
		case errors.Is(err, application.ErrPhoneAlreadyTaken):
			writeProblem(w, http.StatusConflict, problem(r.Context(), "Conflict", "Номер телефона уже используется"))
		case errors.Is(err, application.ErrUserBlocked):
			writeProblem(w, http.StatusTooManyRequests, problem(r.Context(), "Too many requests", "Превышен лимит попыток, попробуйте позже"))
		case errors.Is(err, application.ErrCodeSentTooRecently):
			writeProblem(w, http.StatusTooManyRequests, problem(r.Context(), "Too many requests", "Код отправлен слишком часто, подождите немного"))
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
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	token := sessionTokenFromRequest(r, h.cookieSecure)
	if token == "" {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	var body openapi.ChangePhoneRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.WarnContext(r.Context(), "failed to decode request body", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "invalid request body"))
		return
	}

	phone, err := domain.NewPhone(body.Phone)
	if err != nil {
		h.logger.WarnContext(r.Context(), "invalid phone in request body", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Invalid phone", "invalid phone"))
		return
	}

	if h.phoneVerify != nil && !h.phoneVerify.Allow(phone.String()) {
		writeProblem(w, http.StatusTooManyRequests, problem(r.Context(), "Too many requests", "rate limit exceeded"))
		return
	}

	user, err := h.auth.ChangePhone(r.Context(), userID, phone, body.Code, hashSessionToken(token))
	if err != nil {
		switch {
		case errors.Is(err, application.ErrPhoneUnchanged):
			writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Invalid phone", "Новый номер не должен совпадать с текущим"))
		case errors.Is(err, application.ErrPhoneAlreadyTaken):
			writeProblem(w, http.StatusConflict, problem(r.Context(), "Conflict", "Номер телефона уже используется"))
		case errors.Is(err, application.ErrUserBlocked),
			errors.Is(err, domain.ErrTooManyAttempts):
			writeProblem(w, http.StatusTooManyRequests, problem(r.Context(), "Too many requests", "Превышен лимит попыток, попробуйте позже"))
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
