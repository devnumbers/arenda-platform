package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
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

// AuthRateLimits bundles the per-action auth rate limiters. A nil limiter
// means "allow" (no limit), so the zero-value AuthRateLimits{} permits
// everything — used by tests.
type AuthRateLimits struct {
	Send              *httpsupport.RateLimiter
	Verify            *httpsupport.RateLimiter
	PhoneChangeSend   *httpsupport.RateLimiter
	PhoneChangeVerify *httpsupport.RateLimiter
}

// AllowSend reports whether a send-code request for key is allowed. A nil
// Send limiter permits all requests.
func (l AuthRateLimits) AllowSend(key string) bool {
	if l.Send == nil {
		return true
	}
	return l.Send.Allow(key)
}

// AllowVerify reports whether a verify-code request for key is allowed.
func (l AuthRateLimits) AllowVerify(key string) bool {
	if l.Verify == nil {
		return true
	}
	return l.Verify.Allow(key)
}

// AllowPhoneChangeSend reports whether a phone-change send-code request for
// key is allowed.
func (l AuthRateLimits) AllowPhoneChangeSend(key string) bool {
	if l.PhoneChangeSend == nil {
		return true
	}
	return l.PhoneChangeSend.Allow(key)
}

// AllowPhoneChangeVerify reports whether a phone-change verify request for
// key is allowed.
func (l AuthRateLimits) AllowPhoneChangeVerify(key string) bool {
	if l.PhoneChangeVerify == nil {
		return true
	}
	return l.PhoneChangeVerify.Allow(key)
}

// rateLimitKey returns the bucket key shared by the send/verify handlers:
// email wins when present, otherwise phone.
func rateLimitKey(phone string, email *domain.Email) string {
	if email != nil {
		return email.String()
	}
	return phone
}

// The four service interfaces below are consumed only by this package, so per
// the Go idiom "accept interfaces at the consumer" (ADR 0035) they live here
// next to AuthHandlers rather than in the application package.

// Authenticator issues and verifies login codes.
type Authenticator interface {
	SendCode(ctx context.Context, phone domain.Phone, email domain.Email, purpose domain.LoginCodePurpose) error
	// SendCodeByPhone sends a login code to the email stored for the given
	// phone. It returns sent=false when the user does not exist or has no
	// email on file; the caller should then ask the user for an email.
	SendCodeByPhone(ctx context.Context, phone domain.Phone) (sent bool, err error)
	// VerifyCode accepts an optional email; nil resolves the email from the
	// stored user record for the phone.
	VerifyCode(ctx context.Context, phone domain.Phone, email *domain.Email, code string) (domain.RawSession, domain.User, error)
}

// PhoneChanger handles phone-number change for authenticated users.
type PhoneChanger interface {
	SendChangeCode(ctx context.Context, userID uuid.UUID, newPhone domain.Phone) error
	ChangePhone(ctx context.Context, userID uuid.UUID, newPhone domain.Phone, code, currentToken string) (domain.User, error)
}

// Profiler provides the current user's profile and updates it.
type Profiler interface {
	Me(ctx context.Context, userID uuid.UUID) (domain.User, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, cmd application.UpdateProfileCommand) (domain.User, error)
}

// Logout terminates sessions.
type Logout interface {
	Logout(ctx context.Context, rawToken string, actor auditdomain.Actor) error
	LogoutAll(ctx context.Context, userID uuid.UUID, actor auditdomain.Actor) error
}

// AuthHandlers implements the generated non-strict ServerInterface.
type AuthHandlers struct {
	auth         Authenticator
	phoneChange  PhoneChanger
	profile      Profiler
	logout       Logout
	cookieSecure bool
	logger       *slog.Logger
	limits       AuthRateLimits
	meEnricher   MeEnricher
}

// NewAuthHandlers creates HTTP handlers for the auth API.
func NewAuthHandlers(
	auth Authenticator,
	phoneChange PhoneChanger,
	profile Profiler,
	logout Logout,
	cookieSecure bool,
	logger *slog.Logger,
	limits AuthRateLimits,
	meEnricher MeEnricher,
) *AuthHandlers {
	return &AuthHandlers{
		auth:         auth,
		phoneChange:  phoneChange,
		profile:      profile,
		logout:       logout,
		cookieSecure: cookieSecure,
		logger:       logger,
		limits:       limits,
		meEnricher:   meEnricher,
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
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	phone, err := domain.NewPhone(body.Phone)
	if err != nil {
		h.logger.WarnContext(r.Context(), "invalid phone in request body", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Invalid phone", "Некорректный номер телефона"))
		return
	}

	email, ok := parseOptionalEmail(w, r, body.Email)
	if !ok {
		return
	}

	// Phone-only and email requests use separate buckets; the service-level
	// minSendInterval still caps actual code sends.
	if !h.limits.AllowSend(rateLimitKey(phone.String(), email)) {
		httpsupport.WriteTooManyRequests(w, r, "Превышен лимит запросов")
		return
	}

	retry := httpsupport.RetryAfterSeconds
	if email == nil {
		sent, err := h.auth.SendCodeByPhone(r.Context(), phone)
		if err != nil {
			if !writeSharedIdentityError(w, r, err, "") {
				httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
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
		if !writeSharedIdentityError(w, r, err, "") {
			httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
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
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	phone, err := domain.NewPhone(body.Phone)
	if err != nil {
		h.logger.WarnContext(r.Context(), "invalid phone in request body", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Invalid phone", "Некорректный номер телефона"))
		return
	}

	email, ok := parseOptionalEmail(w, r, body.Email)
	if !ok {
		return
	}

	// Phone-only verify requests are keyed by phone; brute force is
	// additionally capped by the service-level attempt window.
	if !h.limits.AllowVerify(rateLimitKey(phone.String(), email)) {
		httpsupport.WriteTooManyRequests(w, r, "Превышен лимит запросов")
		return
	}

	raw, user, err := h.auth.VerifyCode(r.Context(), phone, email, body.Code)
	if err != nil {
		if writeSharedIdentityError(w, r, err, "Неверный телефон, почта или код") {
			return
		}
		switch {
		case errors.Is(err, domain.ErrLoginCodeInvalid):
			httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
				httpsupport.Problem(r.Context(), "Unauthorized", "Неверный телефон, почта или код"))
		default:
			httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
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
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if err := h.logout.Logout(r.Context(), token, actorFromContext(r.Context())); err != nil {
		if !errors.Is(err, application.ErrNotFound) {
			h.logger.ErrorContext(r.Context(), "logout failed", slog.String("error", httpsupport.SanitizeError(err)))
			httpsupport.ClearSessionCookie(w, h.cookieSecure)
			httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
			return
		}
	}

	httpsupport.ClearSessionCookie(w, h.cookieSecure)
	w.WriteHeader(http.StatusNoContent)
}

// LogoutAll implements POST /auth/logout-all.
func (h *AuthHandlers) LogoutAll(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if err := h.logout.LogoutAll(r.Context(), userID, actorFromContext(r.Context())); err != nil {
		h.logger.ErrorContext(r.Context(), "logout all failed", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.ClearSessionCookie(w, h.cookieSecure)
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}

	httpsupport.ClearSessionCookie(w, h.cookieSecure)
	w.WriteHeader(http.StatusNoContent)
}

// actorFromContext builds an audit Actor from the request context. The ID
// defaults to uuid.Nil and the role to Anonymous when the context carries no
// authenticated identity; the service records the audit with whatever it gets.
func actorFromContext(ctx context.Context) auditdomain.Actor {
	actor := auditdomain.Actor{Role: auditdomain.ActorRoleAnonymous}
	if uid, ok := httpsupport.UserIDFromContext(ctx); ok {
		actor.ID = uid
	}
	if _, role, ok := httpsupport.ActorFromContext(ctx); ok {
		actor.Role = auditdomain.ActorRoleFromRole(role)
	}
	return actor
}

// GetMe implements GET /me.
func (h *AuthHandlers) GetMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
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
			httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
				httpsupport.Problem(r.Context(), "Unauthorized", "Сессия недействительна"))
			return
		}
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}

	resp := meResponse(user)
	if h.meEnricher != nil {
		if err := h.meEnricher(r.Context(), userID, &resp); err != nil {
			httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
			return
		}
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, resp)
}

// UpdateMe implements PATCH /me.
func (h *AuthHandlers) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.UserUpdateRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.WarnContext(r.Context(), "failed to decode request body", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
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
		if writeSharedIdentityError(w, r, err, "Сессия недействительна") {
			return
		}
		switch {
		case errors.Is(err, domain.ErrInvalidEmail):
			httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
				httpsupport.Problem(r.Context(), "Invalid email", "Некорректный формат email"))
		case errors.Is(err, domain.ErrInvalidTimezone):
			httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
				httpsupport.Problem(r.Context(), "Invalid timezone", "Некорректный часовой пояс"))
		default:
			httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		}
		return
	}

	resp := meResponse(user)
	if h.meEnricher != nil {
		if err := h.meEnricher(r.Context(), userID, &resp); err != nil {
			httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
			return
		}
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, resp)
}

// SendPhoneChangeCode implements POST /me/phone/send-code.
func (h *AuthHandlers) SendPhoneChangeCode(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.SendPhoneChangeCodeRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.WarnContext(r.Context(), "failed to decode request body", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	phone, err := domain.NewPhone(body.Phone)
	if err != nil {
		h.logger.WarnContext(r.Context(), "invalid phone in request body", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Invalid phone", "Некорректный номер телефона"))
		return
	}

	if !h.limits.AllowPhoneChangeSend(phone.String()) {
		httpsupport.WriteTooManyRequests(w, r, "Превышен лимит запросов")
		return
	}

	if err := h.phoneChange.SendChangeCode(r.Context(), userID, phone); err != nil {
		if writeSharedIdentityError(w, r, err, "") {
			return
		}
		switch {
		case errors.Is(err, application.ErrPhoneUnchanged):
			httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
				httpsupport.Problem(r.Context(), "Invalid phone",
					userFacingDetailOrDefault(err, "Новый номер должен отличаться от текущего")))
		case errors.Is(err, application.ErrPhoneAlreadyTaken):
			httpsupport.WriteProblem(r.Context(), w, http.StatusConflict,
				httpsupport.Problem(r.Context(), "Conflict",
					userFacingDetailOrDefault(err, "Этот номер телефона уже используется")))
		default:
			httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ChangePhone implements POST /me/phone/change.
func (h *AuthHandlers) ChangePhone(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	token := httpsupport.SessionTokenFromRequest(r, h.cookieSecure)
	if token == "" {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.ChangePhoneRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.WarnContext(r.Context(), "failed to decode request body", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	phone, err := domain.NewPhone(body.Phone)
	if err != nil {
		h.logger.WarnContext(r.Context(), "invalid phone in request body", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Invalid phone", "Некорректный номер телефона"))
		return
	}

	if !h.limits.AllowPhoneChangeVerify(phone.String()) {
		httpsupport.WriteTooManyRequests(w, r, "Превышен лимит запросов")
		return
	}

	user, err := h.phoneChange.ChangePhone(r.Context(), userID, phone, body.Code, token)
	if err != nil {
		if writeSharedIdentityError(w, r, err, "Неверный код") {
			return
		}
		switch {
		case errors.Is(err, application.ErrPhoneUnchanged):
			httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
				httpsupport.Problem(r.Context(), "Invalid phone",
					userFacingDetailOrDefault(err, "Новый номер должен отличаться от текущего")))
		case errors.Is(err, application.ErrPhoneAlreadyTaken):
			httpsupport.WriteProblem(r.Context(), w, http.StatusConflict,
				httpsupport.Problem(r.Context(), "Conflict",
					userFacingDetailOrDefault(err, "Этот номер телефона уже используется")))
		case errors.Is(err, domain.ErrLoginCodeInvalid):
			httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Неверный код"))
		default:
			httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
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
