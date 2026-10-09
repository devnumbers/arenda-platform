// Package http holds the identity HTTP adapters: authentication and phone-change endpoints, profile updates,
// logout flows and session loading.
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
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/requestctx"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/photo"
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
	// stored user record for the phone. An optional timezone (browser-detected,
	// #451) applies only when the verify registers a new user. The device
	// context (raw User-Agent + client IP) is captured onto the created
	// session once (#728).
	VerifyCode(
		ctx context.Context, phone domain.Phone, email *domain.Email, code string, timezone *string, device application.DeviceContext,
	) (domain.RawSession, domain.User, error)
}

// SessionLister serves the devices list: the user's sessions with the current
// one flagged, single-session revocation, and revoke-everything-else.
type SessionLister interface {
	List(ctx context.Context, userID uuid.UUID, currentToken string) ([]domain.Session, uuid.UUID, error)
	Revoke(ctx context.Context, userID, sessionID uuid.UUID, currentToken string, actor auditdomain.Actor) error
	RevokeOthers(ctx context.Context, userID uuid.UUID, currentToken string, actor auditdomain.Actor) (int64, error)
}

// PhoneChanger handles phone-number change for authenticated users.
type PhoneChanger interface {
	SendChangeCode(ctx context.Context, userID uuid.UUID, newPhone domain.Phone) error
	ChangePhone(ctx context.Context, userID uuid.UUID, newPhone domain.Phone, code, currentToken string) (domain.User, error)
}

// EmailChanger handles confirmed email change for authenticated users
// (issue #721): a code to the current address, then — against the grant — a
// code to the new one, a resend of that code (#732), and the change itself.
type EmailChanger interface {
	SendCurrentEmailCode(ctx context.Context, userID uuid.UUID) error
	ConfirmCurrentEmail(ctx context.Context, userID uuid.UUID, code string, newEmail domain.Email) (string, error)
	ResendNewEmailCode(ctx context.Context, userID uuid.UUID, grant string) error
	ChangeEmail(ctx context.Context, userID uuid.UUID, code, grant string) (domain.User, error)
}

// Profiler provides the current user's profile and updates it.
type Profiler interface {
	Me(ctx context.Context, userID uuid.UUID) (domain.User, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, cmd application.UpdateProfileCommand) (domain.User, error)
}

// ProfilePhotos serves the profile photo flows (ADR 0065): one image per
// profile, uploaded and streamed through the backend. The viewer is gated in
// the service: the owner reads their own photo, a foreign read needs a
// readable shared property (решение #1286). The descriptor powers the serving
// ETag (the storage key hash) so a 304 is answered without a storage read.
type ProfilePhotos interface {
	PhotoDescriptor(ctx context.Context, viewerID, userID uuid.UUID) (key, contentType string, err error)
	OpenPhoto(ctx context.Context, viewerID, userID uuid.UUID) (application.OpenedPhoto, error)
	SetProfilePhoto(ctx context.Context, userID uuid.UUID, processed photo.Processed) (domain.User, error)
	DeleteProfilePhoto(ctx context.Context, userID uuid.UUID) (domain.User, error)
}

// Logout terminates the current session.
type Logout interface {
	Logout(ctx context.Context, rawToken string, actor auditdomain.Actor) error
}

// AuthHandlers implements the generated non-strict ServerInterface.
type AuthHandlers struct {
	auth         Authenticator
	phoneChange  PhoneChanger
	emailChange  EmailChanger
	profile      Profiler
	profilePhoto ProfilePhotos
	logout       Logout
	sessions     SessionLister
	cookieSecure bool
	logger       *slog.Logger
	limits       AuthRateLimits
	meEnricher   MeEnricher
}

// NewAuthHandlers creates HTTP handlers for the auth API.
func NewAuthHandlers(
	auth Authenticator,
	phoneChange PhoneChanger,
	emailChange EmailChanger,
	profile Profiler,
	profilePhoto ProfilePhotos,
	logout Logout,
	sessions SessionLister,
	cookieSecure bool,
	logger *slog.Logger,
	limits AuthRateLimits,
	meEnricher MeEnricher,
) *AuthHandlers {
	return &AuthHandlers{
		auth:         auth,
		phoneChange:  phoneChange,
		emailChange:  emailChange,
		profile:      profile,
		profilePhoto: profilePhoto,
		logout:       logout,
		sessions:     sessions,
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

	device := application.DeviceContext{
		UserAgent: r.Header.Get("User-Agent"),
		IP:        requestctx.ClientIPFromContext(r.Context()),
	}
	raw, user, err := h.auth.VerifyCode(r.Context(), phone, email, body.Code, body.Timezone, device)
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
	// No Clear-Site-Data here, deliberately (#1098 added "cache", the
	// 2026-10-06 slow-logout diagnosis removed it): Chrome blocks delivering
	// the 204 until the origin HTTP-cache purge finishes — 11–14 s on real
	// profiles (Caddy logged the POST at 6–7 ms while the client-side fetch
	// stayed pending the whole purge; POST duration grows with cache entries:
	// 0 → 15–26 ms, 2000 → 1.1 s, 5000 → 2.8 s). The purge protected nothing:
	// API responses carry no Cache-Control/validators so the browser never
	// reuses them from cache, and static assets are immutable and public.
	// The auth boundary stays on the frontend hard navigation (#1098), the
	// bfcache guard, the session deletion above and the proxy /me gate.
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

// detailEmailTakenForChange is the conflict text pinned by grilling decision
// #720-4 for the email-change endpoints. The login flow asserts its own
// fixture text, so the two wording families stay separate.
const detailEmailTakenForChange = "Эта электронная почта уже используется"

// writeEmailChangeError maps the email-change-specific errors ahead of the
// shared identity mapping: these texts are pinned by the grilling decisions
// (#720-4, #720-2, #720 Q9) and differ from the login-flow fixtures. Returns
// false when err is not one of them.
func writeEmailChangeError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, application.ErrEmailUnchanged):
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Invalid email", "Новая электронная почта должна отличаться от текущей"))
	case errors.Is(err, application.ErrEmailAlreadyTaken):
		httpsupport.WriteProblem(r.Context(), w, http.StatusConflict,
			httpsupport.Problem(r.Context(), "Conflict", detailEmailTakenForChange))
	case errors.Is(err, application.ErrEmailDoesNotMatch):
		httpsupport.WriteProblem(r.Context(), w, http.StatusConflict,
			httpsupport.Problem(r.Context(), "Conflict", "У аккаунта нет электронной почты"))
	case errors.Is(err, application.ErrEmailChangeGrantInvalid):
		httpsupport.WriteProblem(r.Context(), w, http.StatusConflict,
			httpsupport.Problem(r.Context(), "Conflict", "Подтверждение истекло, начните смену почты заново"))
	case errors.Is(err, domain.ErrLoginCodeInvalid):
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Неверный код"))
	case errors.Is(err, application.ErrEmailChangeBudgetExhausted):
		httpsupport.WriteTooManyRequests(w, r, "Превышен лимит запросов")
	default:
		return false
	}
	return true
}

// SendEmailChangeCode implements POST /me/email/send-code — step 1: a code to
// the user's current email.
func (h *AuthHandlers) SendEmailChangeCode(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	// No send budget here: step 1 mails the user's current address. The
	// per-user 5/hour budget (#720-3) guards sends to NEW addresses — step 2;
	// this request is covered by the domain's 1-minute throttle and the global
	// IP limiter.
	if err := h.emailChange.SendCurrentEmailCode(r.Context(), userID); err != nil {
		if writeEmailChangeError(w, r, err) {
			return
		}
		if writeSharedIdentityError(w, r, err, "") {
			return
		}
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ConfirmCurrentEmail implements POST /me/email/confirm-current — step 2:
// verify the code from the current email, get the one-time grant and the code
// for the new address. The per-user 5/hour budget (#720-3) is spent inside the
// service, on the actual send to the new address.
func (h *AuthHandlers) ConfirmCurrentEmail(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.ConfirmCurrentEmailRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.WarnContext(r.Context(), "failed to decode request body", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	newEmail, err := domain.NewEmail(body.NewEmail)
	if err != nil {
		h.logger.WarnContext(r.Context(), "invalid email in request body", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Invalid email", "Некорректный формат электронной почты"))
		return
	}

	grant, err := h.emailChange.ConfirmCurrentEmail(r.Context(), userID, body.Code, newEmail)
	if err != nil {
		if writeEmailChangeError(w, r, err) {
			return
		}
		if writeSharedIdentityError(w, r, err, "") {
			return
		}
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.EmailChangeGrantResponse{Grant: grant})
}

// ResendEmailCode implements POST /me/email/resend-code — a fresh code for the
// pending new address, anchored on the still-live grant (#732): the step-1
// code is burned by ConfirmCurrentEmail, so the resend cannot re-run step 2
// and must ride the grant instead. The budget (#720-3) and the send throttle
// are spent inside the service, on the actual re-issuance.
func (h *AuthHandlers) ResendEmailCode(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.ResendEmailCodeRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.WarnContext(r.Context(), "failed to decode request body", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	if err := h.emailChange.ResendNewEmailCode(r.Context(), userID, body.Grant); err != nil {
		if writeEmailChangeError(w, r, err) {
			return
		}
		if writeSharedIdentityError(w, r, err, "") {
			return
		}
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ChangeEmail implements POST /me/email/change — step 3: verify the code from
// the new email against the grant and apply the change.
func (h *AuthHandlers) ChangeEmail(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.ChangeEmailRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.WarnContext(r.Context(), "failed to decode request body", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	user, err := h.emailChange.ChangeEmail(r.Context(), userID, body.Code, body.Grant)
	if err != nil {
		if writeEmailChangeError(w, r, err) {
			return
		}
		if writeSharedIdentityError(w, r, err, "") {
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
		PhotoUrl:   mePhotoURL(user),
	}
}

// mePhotoURL is the profile photo's same-origin streaming path (ADR 0065):
// nil without a photo. The self path is constant — /me is always the
// session's own profile.
func mePhotoURL(user domain.User) *string {
	if user.PhotoKey == nil {
		return nil
	}
	path := "/api/v1/me/photo"
	return &path
}

func timezonePtrFromUser(tz domain.Timezone) *string {
	return new(tz.String())
}
