package httpsupport

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
)

// PaidSectionsGate is the billing port consumed by the paid-sections tariff
// gate: it resolves whether the authenticated user's subscription covers the
// platform's paid sections. It is declared here, next to its only consumer,
// per the consumer-side interface rule (ADR 0035); the billing module provides
// the adapter, and a user without a subscription yet counts as basic — the
// gate only blocks, never grants.
type PaidSectionsGate interface {
	CanUsePaidFeatures(ctx context.Context, userID uuid.UUID) (bool, error)
}

// WriteTariffRequired answers the canonical "needs a paid tariff" refusal:
// 402 PaymentRequired with the machine code "tariff_required" — the wire
// shape the route middleware and the handlers mapping a use-case-level
// gate's verdict (ADR 0064, ревизии #1462 и #1465) share.
func WriteTariffRequired(ctx context.Context, w http.ResponseWriter) {
	WriteProblem(ctx, w, http.StatusPaymentRequired,
		ProblemWithCode(ctx, "Paid tariff required",
			"Этот раздел доступен на платных тарифах", "tariff_required"))
}

// PaidSectionsMiddleware blocks requests to the paid platform sections when
// the authenticated user's subscription does not cover them (карта #997):
// the answer is 402 PaymentRequired with the machine code "tariff_required",
// the same canonical "needs a paid tariff" status as the property-limit 402.
// Mounted over the generated paid-section routes via r.With (the AdminOnly
// precedent). Since ревизия #1465 the mount covers only the global
// participants section; the per-object access group (ревизия #1462) and the
// history journal (ревизия #1465) are gated inside their use cases instead —
// their verdicts are role-aware, the middleware has no access to the role.
func PaidSectionsMiddleware(gate PaidSectionsGate, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := OwnerIDFromContext(r)
			if !ok {
				// Let downstream auth middleware handle the missing session.
				next.ServeHTTP(w, r)
				return
			}

			canUse, err := gate.CanUsePaidFeatures(r.Context(), userID)
			if err != nil {
				WriteProblem(r.Context(), w, http.StatusInternalServerError, InternalError(r.Context(), err))
				return
			}
			if !canUse {
				logger.WarnContext(r.Context(), "blocked paid-section request on the basic tariff",
					slog.String("user_id", userID.String()),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path))
				WriteTariffRequired(r.Context(), w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
