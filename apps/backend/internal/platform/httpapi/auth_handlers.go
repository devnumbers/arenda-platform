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

// VerifyPhoneCode implements POST /auth/phone/verify.
func (h *AuthHandlers) VerifyPhoneCode(w http.ResponseWriter, r *http.Request) {
	var body openapi.VerifyPhoneCodeRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode request body", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "invalid request body"))
		return
	}

	phone, err := domain.NewPhone(body.Phone)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "invalid phone in request body", slog.String("error", sanitizeError(err)))
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
		case errors.Is(err, application.ErrUserBlocked),
			errors.Is(err, domain.ErrTooManyAttempts):
			detail, ok := UserFacingDetail(err)
			if !ok {
				writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
				return
			}
			writeProblem(w, http.StatusTooManyRequests, problem(r.Context(), "Too many requests", detail))
		case errors.Is(err, domain.ErrSMSCodeInvalid),
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
