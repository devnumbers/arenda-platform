package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
)

// The admin time-travel endpoints of issue #665: the stand-only rig that
// walks the subscription lifecycle through its acceptance scenarios without
// waiting out real hours. They exist only when the platform's
// BILLING_TIME_TRAVEL railguard is on: the composition root constructs these
// handlers only then, so a production build mounts no such routes at all —
// the same shape the local fake-confirm endpoints use (issue #287). The
// routes sit on the admin paths with the AdminOnlyMiddleware, one row beside
// the #255 admin levers.

// TimeShiftRoute is POST /admin/users/{id}/subscription/time-shift — the
// coherent shift of a subscription's temporal boundaries, a raw signed shift
// or a named preset.
const TimeShiftRoute = "/admin/users/{id}/subscription/time-shift"

// BillingTickRoute is POST /admin/billing/tick — one synchronous pass of the
// billing worker phases, so a shifted boundary is picked up immediately.
const BillingTickRoute = "/admin/billing/tick"

// TimeShiftBackend is the application slice the time-shift endpoint needs —
// declared here, at the consumer, per ADR 0035.
type TimeShiftBackend interface {
	ShiftSubscriptionTime(ctx context.Context, adminID, userID uuid.UUID, req billingapp.TimeShiftRequest) error
}

// BillingTickRunner is the scheduler slice the admin tick endpoint triggers —
// the same leader-elected pass the worker loop runs on its interval.
type BillingTickRunner interface {
	TickOnce(ctx context.Context) error
}

// TimeTravelHandlers implements the admin time-shift and tick endpoints.
type TimeTravelHandlers struct {
	subscriptions TimeShiftBackend
	tick          BillingTickRunner
	logger        *slog.Logger
}

// NewTimeTravelHandlers creates the rig's handlers over the subscription
// service and the billing worker shell.
func NewTimeTravelHandlers(subscriptions TimeShiftBackend, tick BillingTickRunner, logger *slog.Logger) *TimeTravelHandlers {
	if logger == nil {
		logger = slog.Default()
	}
	return &TimeTravelHandlers{subscriptions: subscriptions, tick: tick, logger: logger}
}

// MountRoutes registers the rig's endpoints under the admin middleware — the
// single source of truth the HTTP wiring uses, called only when the rig is
// enabled.
func (h *TimeTravelHandlers) MountRoutes(r chi.Router) {
	r.With(httpsupport.AdminOnlyMiddleware).Post(TimeShiftRoute, h.ShiftTime)
	r.With(httpsupport.AdminOnlyMiddleware).Post(BillingTickRoute, h.RunTick)
}

// timeShiftBody is the wire shape of a shift request: shiftHours for the raw
// signed shift or preset for a named acceptance wrapper — exactly one.
type timeShiftBody struct {
	ShiftHours *int   `json:"shiftHours"`
	Preset     string `json:"preset"`
}

// maxShiftHours bounds the wire shift far above any legitimate use (the
// application cap is ±90 days = 2160 h) and far below where the int-hours →
// duration multiplication could overflow past the cap.
const maxShiftHours = 24 * 365

// ShiftTime implements POST /admin/users/{id}/subscription/time-shift
// (issue #665): the subscription's temporal boundaries move by one signed
// delta, the transition log and the audit record the move. Responds 204; a
// state or request defect answers a 400/409 problem.
func (h *TimeTravelHandlers) ShiftTime(w http.ResponseWriter, r *http.Request) {
	adminID, _, ok := httpsupport.ActorFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректный идентификатор пользователя"))
		return
	}
	var body timeShiftBody
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.WarnContext(r.Context(), "failed to decode time shift request",
			slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}
	req := billingapp.TimeShiftRequest{Preset: billingapp.TimeShiftPreset(body.Preset)}
	if body.ShiftHours != nil {
		if *body.ShiftHours > maxShiftHours || *body.ShiftHours < -maxShiftHours {
			httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
				httpsupport.Problem(r.Context(), "Bad request", "Сдвиг выходит за допустимый диапазон"))
			return
		}
		req.Shift = time.Duration(*body.ShiftHours) * time.Hour
	}
	if err := h.subscriptions.ShiftSubscriptionTime(r.Context(), adminID, userID, req); err != nil {
		writeBillingError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RunTick implements POST /admin/billing/tick (issue #665): one synchronous
// leader-elected pass of the billing worker phases under the same advisory
// lock the periodic loop holds. Responds 204 when every phase ran.
func (h *TimeTravelHandlers) RunTick(w http.ResponseWriter, r *http.Request) {
	adminID, _, ok := httpsupport.ActorFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}
	if err := h.tick.TickOnce(r.Context()); err != nil {
		h.logger.ErrorContext(r.Context(), "admin billing tick failed",
			slog.String("admin_id", adminID.String()),
			slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError,
			httpsupport.InternalError(r.Context(), errors.New("billing tick failed")))
		return
	}
	h.logger.InfoContext(r.Context(), "admin billing tick ran",
		slog.String("admin_id", adminID.String()))
	w.WriteHeader(http.StatusNoContent)
}
