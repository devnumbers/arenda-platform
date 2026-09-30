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

// PaidSectionsMiddleware blocks requests to the paid platform sections when
// the authenticated user's subscription does not cover them (карта #997):
// the answer is 402 PaymentRequired with the machine code "tariff_required",
// the same canonical "needs a paid tariff" status as the property-limit 402.
// Mounted over the generated paid-section routes via r.With (the AdminOnly
// precedent); the self-leave route stays outside the mount.
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
				WriteProblem(r.Context(), w, http.StatusPaymentRequired,
					ProblemWithCode(r.Context(), "Paid tariff required",
						"Этот раздел доступен на платных тарифах", "tariff_required"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
