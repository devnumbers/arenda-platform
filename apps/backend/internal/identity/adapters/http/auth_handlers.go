package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
)

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
	emailSend         *httpsupport.RateLimiter
	emailVerify       *httpsupport.RateLimiter
	phoneChangeSend   *httpsupport.RateLimiter
	phoneChangeVerify *httpsupport.RateLimiter
	meEnricher        MeEnricher
	audit             auditapp.Recorder
}

// NewAuthHandlers creates HTTP handlers for the auth API.
func NewAuthHandlers(
	auth application.Authenticator,
	phoneChange application.PhoneChanger,
	profile application.Profiler,
	logout application.Logout,
	cookieSecure bool,
	logger *slog.Logger,
	emailSend *httpsupport.RateLimiter,
	emailVerify *httpsupport.RateLimiter,
	phoneChangeSend *httpsupport.RateLimiter,
	phoneChangeVerify *httpsupport.RateLimiter,
	meEnricher MeEnricher,
	audit auditapp.Recorder,
) *AuthHandlers {
	if audit == nil {
		audit = auditapp.Noop{}
	}
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
		audit:             audit,
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
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.WarnContext(r.Context(), "failed to decode request body", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	phone, err := domain.NewPhone(body.Phone)
	if err != nil {
		h.logger.WarnContext(r.Context(), "invalid phone in request body", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Invalid phone", "Некорректный номер телефона"))
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
		httpsupport.WriteTooManyRequests(w, r, "Превышен лимит запросов")
		return
	}

	retry := httpsupport.RetryAfterSeconds
	if email == nil {
		sent, err := h.auth.SendCodeByPhone(r.Context(), phone)
		if err != nil {
			switch {
			case errors.Is(err, application.ErrUserBlocked), errors.Is(err, application.ErrCodeSentTooRecently):
				httpsupport.WriteTooManyRequests(w, r, userFacingDetailOrDefault(err, "Превышен лимит запросов"))
			default:
				httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
			}
			return
		}
		if !sent {
			httpsupport.WriteJSON(r.Context(), w, http.StatusOK, authSendCodeResponse{Sent: false})
			return
		}
		httpsupport.WriteJSON(r.Context(), w, http.StatusOK, authSendCodeResponse{Sent: true, RetryAfter: &retry})
		return
	}

	if err := h.auth.SendCode(r.Context(), phone, *email, domain.LoginCodePurposeLogin); err != nil {
		switch {
		case errors.Is(err, application.ErrUserBlocked), errors.Is(err, application.ErrCodeSentTooRecently):
			httpsupport.WriteTooManyRequests(w, r, userFacingDetailOrDefault(err, "Превышен лимит запросов"))
		case errors.Is(err, application.ErrEmailDoesNotMatch):
			httpsupport.WriteProblem(w, http.StatusConflict, httpsupport.Problem(r.Context(), "Conflict", userFacingDetailOrDefault(err, "Некорректные учётные данные")))
		case errors.Is(err, application.ErrEmailAlreadyTaken):
			httpsupport.WriteProblem(w, http.StatusConflict, httpsupport.Problem(r.Context(), "Conflict", userFacingDetailOrDefault(err, "Этот email уже используется")))
		default:
			httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		}
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, authSendCodeResponse{Sent: true, RetryAfter: &retry})
}

// VerifyCode implements POST /auth/verify.
func (h *AuthHandlers) VerifyCode(w http.ResponseWriter, r *http.Request) {
	var body openapi.VerifyCodeRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.WarnContext(r.Context(), "failed to decode request body", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	phone, err := domain.NewPhone(body.Phone)
	if err != nil {
		h.logger.WarnContext(r.Context(), "invalid phone in request body", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Invalid phone", "Некорректный номер телефона"))
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
		httpsupport.WriteTooManyRequests(w, r, "Превышен лимит запросов")
		return
	}

	raw, user, err := h.auth.VerifyCode(r.Context(), phone, email, body.Code)
	if err != nil {
		switch {
		case errors.Is(err, application.ErrUserBlocked),
			errors.Is(err, domain.ErrTooManyAttempts):
			httpsupport.WriteTooManyRequests(w, r, userFacingDetailOrDefault(err, "Превышен лимит запросов"))
		case errors.Is(err, domain.ErrLoginCodeInvalid),
			errors.Is(err, application.ErrNotFound):
			httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Неверный телефон, почта или код"))
		case errors.Is(err, application.ErrEmailDoesNotMatch):
			httpsupport.WriteProblem(w, http.StatusConflict, httpsupport.Problem(r.Context(), "Conflict", userFacingDetailOrDefault(err, "Некорректные учётные данные")))
		case errors.Is(err, application.ErrEmailAlreadyTaken):
			httpsupport.WriteProblem(w, http.StatusConflict, httpsupport.Problem(r.Context(), "Conflict", userFacingDetailOrDefault(err, "Этот email уже используется")))
		default:
			httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		}
		return
	}

	httpsupport.SetSessionCookie(w, raw.Token, raw.Session.ExpiresAt, h.cookieSecure)
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, meResponse(user))
}

// Logout implements POST /auth/logout.
func (h *AuthHandlers) Logout(w http.ResponseWriter, r *http.Request) {
	token := httpsupport.SessionTokenFromRequest(r, h.cookieSecure)
	if token == "" {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if err := h.logout.Logout(r.Context(), token); err != nil {
		if !errors.Is(err, application.ErrNotFound) {
			h.logger.ErrorContext(r.Context(), "logout failed", slog.String("error", httpsupport.SanitizeError(err)))
			httpsupport.ClearSessionCookie(w, h.cookieSecure)
			httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
			return
		}
	}

	h.recordAuthAudit(r.Context(), auditdomain.ActionAuthLogout)

	httpsupport.ClearSessionCookie(w, h.cookieSecure)
	w.WriteHeader(http.StatusNoContent)
}

// LogoutAll implements POST /auth/logout-all.
func (h *AuthHandlers) LogoutAll(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if err := h.logout.LogoutAll(r.Context(), userID); err != nil {
		h.logger.ErrorContext(r.Context(), "logout all failed", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.ClearSessionCookie(w, h.cookieSecure)
		httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}

	h.recordAuthAudit(r.Context(), auditdomain.ActionAuthLogoutAll)

	httpsupport.ClearSessionCookie(w, h.cookieSecure)
	w.WriteHeader(http.StatusNoContent)
}

// recordAuthAudit writes an audit entry for a completed logout. Record errors
// are logged but never fail the request: the session is already deleted, so
// the response must not depend on the audit write (the documented fail-open
// exception to the service-layer fail-safe rule).
func (h *AuthHandlers) recordAuthAudit(ctx context.Context, action auditdomain.Action) {
	userID, ok := httpsupport.UserIDFromContext(ctx)
	if !ok {
		return
	}
	actorRole := auditdomain.ActorRoleOwner
	if _, role, ok := httpsupport.ActorFromContext(ctx); ok {
		actorRole = application.AuditActorRole(role)
	}
	if err := h.audit.Record(ctx, auditdomain.Entry{
		ActorID:    &userID,
		ActorRole:  actorRole,
		Action:     action,
		EntityType: auditdomain.EntityUser,
		EntityID:   &userID,
	}); err != nil {
		h.logger.ErrorContext(ctx, "failed to record audit entry", slog.String("error", httpsupport.SanitizeError(err)))
	}
}

// GetMe implements GET /me.
func (h *AuthHandlers) GetMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	// The full profile is always fetched from the identity service: the request
	// context only carries the actor identity (userID + role), not the cached
	// aggregate (ADR 0034). This is one extra read per /me request, accepted in
	// exchange for removing the platform → identity/domain dependency and the
	// stale-cache risk of a context-cached User.
	user, err := h.profile.Me(r.Context(), userID)
	if err != nil {
		if errors.Is(err, application.ErrNotFound) {
			httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Сессия недействительна"))
			return
		}
		httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}

	resp := meResponse(user)
	if h.meEnricher != nil {
		if err := h.meEnricher(r.Context(), userID, &resp); err != nil {
			httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
			return
		}
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, resp)
}

// UpdateMe implements PATCH /me.
func (h *AuthHandlers) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.UserUpdateRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.WarnContext(r.Context(), "failed to decode request body", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	cmd := application.UpdateProfileCommand{
		Email:      body.Email,
		Name:       body.Name,
		Surname:    body.Surname,
		Patronymic: body.Patronymic,
		Timezone:   body.Timezone,
	}

	user, err := h.profile.UpdateProfile(r.Context(), userID, cmd)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidEmail):
			httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Invalid email", "Некорректный формат email"))
		case errors.Is(err, domain.ErrInvalidTimezone):
			httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Invalid timezone", "Некорректный часовой пояс"))
		case errors.Is(err, application.ErrEmailAlreadyTaken):
			httpsupport.WriteProblem(w, http.StatusConflict, httpsupport.Problem(r.Context(), "Conflict", userFacingDetailOrDefault(err, "Этот email уже используется")))
		case errors.Is(err, application.ErrNotFound):
			httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Сессия недействительна"))
		default:
			httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		}
		return
	}

	resp := meResponse(user)
	if h.meEnricher != nil {
		if err := h.meEnricher(r.Context(), userID, &resp); err != nil {
			httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
			return
		}
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, resp)
}

// SendPhoneChangeCode implements POST /me/phone/send-code.
func (h *AuthHandlers) SendPhoneChangeCode(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.SendPhoneChangeCodeRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.WarnContext(r.Context(), "failed to decode request body", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	phone, err := domain.NewPhone(body.Phone)
	if err != nil {
		h.logger.WarnContext(r.Context(), "invalid phone in request body", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Invalid phone", "Некорректный номер телефона"))
		return
	}

	if h.phoneChangeSend != nil && !h.phoneChangeSend.Allow(phone.String()) {
		httpsupport.WriteTooManyRequests(w, r, "Превышен лимит запросов")
		return
	}

	if err := h.phoneChange.SendChangeCode(r.Context(), userID, phone); err != nil {
		switch {
		case errors.Is(err, application.ErrPhoneUnchanged):
			httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Invalid phone", userFacingDetailOrDefault(err, "Новый номер должен отличаться от текущего")))
		case errors.Is(err, application.ErrPhoneAlreadyTaken):
			httpsupport.WriteProblem(w, http.StatusConflict, httpsupport.Problem(r.Context(), "Conflict", userFacingDetailOrDefault(err, "Этот номер телефона уже используется")))
		case errors.Is(err, application.ErrUserBlocked):
			httpsupport.WriteTooManyRequests(w, r, userFacingDetailOrDefault(err, "Слишком много попыток"))
		case errors.Is(err, application.ErrCodeSentTooRecently):
			httpsupport.WriteTooManyRequests(w, r, userFacingDetailOrDefault(err, "Код отправлен слишком недавно"))
		default:
			httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ChangePhone implements POST /me/phone/change.
func (h *AuthHandlers) ChangePhone(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	token := httpsupport.SessionTokenFromRequest(r, h.cookieSecure)
	if token == "" {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.ChangePhoneRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.WarnContext(r.Context(), "failed to decode request body", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	phone, err := domain.NewPhone(body.Phone)
	if err != nil {
		h.logger.WarnContext(r.Context(), "invalid phone in request body", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Invalid phone", "Некорректный номер телефона"))
		return
	}

	if h.phoneChangeVerify != nil && !h.phoneChangeVerify.Allow(phone.String()) {
		httpsupport.WriteTooManyRequests(w, r, "Превышен лимит запросов")
		return
	}

	user, err := h.phoneChange.ChangePhone(r.Context(), userID, phone, body.Code, token)
	if err != nil {
		switch {
		case errors.Is(err, application.ErrPhoneUnchanged):
			httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Invalid phone", userFacingDetailOrDefault(err, "Новый номер должен отличаться от текущего")))
		case errors.Is(err, application.ErrPhoneAlreadyTaken):
			httpsupport.WriteProblem(w, http.StatusConflict, httpsupport.Problem(r.Context(), "Conflict", userFacingDetailOrDefault(err, "Этот номер телефона уже используется")))
		case errors.Is(err, application.ErrUserBlocked),
			errors.Is(err, domain.ErrTooManyAttempts):
			httpsupport.WriteTooManyRequests(w, r, userFacingDetailOrDefault(err, "Слишком много попыток"))
		case errors.Is(err, domain.ErrLoginCodeInvalid),
			errors.Is(err, application.ErrNotFound):
			httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Неверный код"))
		default:
			httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		}
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, meResponse(user))
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
		Timezone:   timezonePtrFromUser(user.Timezone),
	}
}

func timezonePtrFromUser(tz domain.Timezone) *string {
	return new(tz.String())
}
