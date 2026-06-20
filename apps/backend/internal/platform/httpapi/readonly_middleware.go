package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

var mutatingMethods = map[string]struct{}{
	http.MethodPost:   {},
	http.MethodPut:    {},
	http.MethodPatch:  {},
	http.MethodDelete: {},
}

// readonlyExemptPrefixes are the path prefixes that remain writable even when
// the subscription does not allow data mutations. These are the recovery paths
// (auth, tariffs, subscription management, webhooks, internal dev tools).
var readonlyExemptPrefixes = []string{
	"/auth",
	"/tariffs",
	"/subscription/change",
	"/subscription/auto-renew",
	"/subscription/payment-methods",
	"/webhooks",
	"/internal",
	"/me",
}

func isReadonlyExempt(path string) bool {
	for _, prefix := range readonlyExemptPrefixes {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

type canMutateDataKey struct{}

// canMutateDataFromContext returns the cached per-request subscription mutation
// flag, if any middleware or handler has already computed it.
func canMutateDataFromContext(ctx context.Context) (bool, bool) {
	v, ok := ctx.Value(canMutateDataKey{}).(bool)
	return v, ok
}

// withCanMutateData caches the subscription mutation flag on the request context
// so that later middleware or handlers can reuse it without another DB lookup.
func withCanMutateData(ctx context.Context, canMutate bool) context.Context {
	return context.WithValue(ctx, canMutateDataKey{}, canMutate)
}

// readonlyMiddleware blocks mutating requests when the authenticated user's
// subscription does not allow data mutations. Read operations and the recovery
// paths listed above are always allowed.
func readonlyMiddleware(billing *billingapp.BillingService, logger *slog.Logger, clk clock.Clock) func(http.Handler) http.Handler {
	if clk == nil {
		clk = fallbackClock{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := mutatingMethods[r.Method]; !ok {
				next.ServeHTTP(w, r)
				return
			}

			path := r.URL.Path
			if isReadonlyExempt(path) {
				next.ServeHTTP(w, r)
				return
			}

			userID, ok := ownerIDFromContext(r)
			if !ok {
				// Let downstream auth middleware handle the missing session.
				next.ServeHTTP(w, r)
				return
			}

			canMutate, ok := canMutateDataFromContext(r.Context())
			if !ok {
				var err error
				canMutate, err = canMutateData(r.Context(), billing, userID, clk)
				if err != nil {
					writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
					return
				}
				r = r.WithContext(withCanMutateData(r.Context(), canMutate))
			}
			if !canMutate {
				logger.WarnContext(r.Context(), "blocked mutating request due to subscription status",
					slog.String("user_id", userID.String()),
					slog.String("method", r.Method),
					slog.String("path", path))
				writeProblem(w, http.StatusForbidden, problem(r.Context(), "Subscription blocked", "subscription is blocked; renew to continue"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func canMutateData(ctx context.Context, billing *billingapp.BillingService, userID uuid.UUID, clk clock.Clock) (bool, error) {
	view, err := billing.GetSubscription(ctx, userID)
	if err != nil {
		if errors.Is(err, billingapp.ErrSubscriptionNotFound) {
			return true, nil
		}
		return false, err
	}
	return view.Subscription.CanMutateData(clk.Now().UTC()), nil
}
