package tkassa

import (
	"errors"
	"fmt"
	"slices"

	"github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/payment"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
)

// ProviderError is returned when T-Kassa responds with Success=false and a
// non-zero ErrorCode.
type ProviderError struct {
	Method    string
	ErrorCode string
	Message   string
	Details   string
}

func (e *ProviderError) Error() string {
	msg := fmt.Sprintf("tkassa: %s failed with error code %s", e.Method, e.ErrorCode)
	if e.Message != "" {
		msg += ": " + e.Message
	}
	if e.Details != "" {
		msg += " (" + e.Details + ")"
	}
	return msg
}

// ProviderErrorCode exposes the T-Kassa ErrorCode so application-layer error
// handlers can persist it without importing this package.
func (e *ProviderError) ProviderErrorCode() string { return e.ErrorCode }

// metricStatus maps an adapter error to the RED "status" metric label.
//
// RED convention: status is "error" ONLY for genuine operational failures —
// transport errors, timeouts, context cancellation, request-build / marshal
// errors, non-2xx HTTP status, response-unmarshal errors, and
// broken-integration signals. A definitive provider response is a completed
// operation and reports "ok", even when it carries a business outcome such as
// a declined payment or a parsed provider error-code (surfaced as
// *ProviderError). The exceptions are application.ErrProviderAuthRejected
// (error codes 204/205 — the terminal rejected our request signature) and
// application.ErrProviderInvalidOperation (error codes 9/12/1125/1126 — the
// provider rejected our request parameters): both mean the integration itself
// is broken and must count as errors for alerting, even though they carry a
// *ProviderError. This matches the fake adapter (which returns nil err on
// decline) and keeps payment.provider.errors comparable across providers.
func metricStatus(err error) string {
	if err == nil {
		return payment.StatusOK
	}
	if errors.Is(err, application.ErrProviderAuthRejected) ||
		errors.Is(err, application.ErrProviderInvalidOperation) {
		return payment.StatusError
	}
	var providerErr *ProviderError
	if errors.As(err, &providerErr) ||
		errors.Is(err, application.ErrProviderMethodNotFound) ||
		errors.Is(err, application.ErrProviderCustomerNotFound) ||
		errors.Is(err, application.ErrProviderAccountNotFound) ||
		errors.Is(err, application.ErrProviderChargeBlocked) ||
		errors.Is(err, application.ErrProviderPaymentNotFound) ||
		errors.Is(err, application.ErrProviderBindingNotFound) {
		return payment.StatusOK
	}
	return payment.StatusError
}

// isProviderErrorCode reports whether err carries a *ProviderError with one of
// the given T-Kassa error codes.
func isProviderErrorCode(err error, codes ...string) bool {
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) {
		return false
	}
	return slices.Contains(codes, providerErr.ErrorCode)
}

// isMethodNotFoundError reports whether the provider error means the saved
// card does not exist at T-Kassa (error codes 107/231).
func isMethodNotFoundError(err error) bool {
	return isProviderErrorCode(err, "107", "231")
}

// isCustomerNotFoundError reports whether the provider error means the customer
// does not exist at T-Kassa. Customers are created lazily (by Init with a
// CustomerKey, AddCustomer, or AddCard), so GetCardList for a user who never
// paid or bound a card returns ErrorCode 7 ("customer not found"). For
// GetCardList that means the user has no cards, not a failure. Note that code 7
// carries a different meaning for AddCustomer ("customer already exists"), so
// this helper is only valid in the GetCardList flow.
func isCustomerNotFoundError(err error) bool {
	return isProviderErrorCode(err, "7")
}

// isAccountNotFoundError reports whether the provider error means the
// configured terminal does not exist at T-Kassa (error code 501). For
// GetCardList this is treated as an empty card list, not a failure.
func isAccountNotFoundError(err error) bool {
	return isProviderErrorCode(err, "501")
}

// classifyProviderError maps well-known T-Kassa error codes to the
// application-layer sentinels, keeping the *ProviderError in the chain so
// errors.As on it still works for callers and metricStatus. Errors without a
// known code are returned unchanged.
//
// Code catalogue (verified against the official T-Kassa error-code page,
// issue #246):
//   - 7: customer not found (flow-specific, handled by callers)
//   - 9: redirect URL empty — our callback configuration is broken
//   - 10: charge method blocked for the terminal (COF not enabled)
//   - 12: invalid RedirectDueDate — our deadline format regressed
//   - 100: duplicate cancel/confirm — the operation already happened
//   - 103/116/1051: insufficient funds
//   - 104: recurring charge failed
//   - 107/231: card not found
//   - 204/205: token invalid / terminal not found by signature
//   - 255: payment not found
//   - 262: parent payment of the saved card expired — re-binding required
//   - 501: terminal not found
//   - 502: request key not found — the binding session expired provider-side
//   - 1125/1126: inconsistent operation parameters (OperationInitiatorType)
func classifyProviderError(err error) error {
	switch {
	case isProviderErrorCode(err, "10"):
		return fmt.Errorf("%w: %w", application.ErrProviderChargeBlocked, err)
	case isProviderErrorCode(err, "204", "205"):
		return fmt.Errorf("%w: %w", application.ErrProviderAuthRejected, err)
	case isProviderErrorCode(err, "255"):
		return fmt.Errorf("%w: %w", application.ErrProviderPaymentNotFound, err)
	case isProviderErrorCode(err, "502"):
		// GetAddCardState for a request key the provider no longer tracks: the
		// binding session expired provider-side.
		return fmt.Errorf("%w: %w", application.ErrProviderBindingNotFound, err)
	case isProviderErrorCode(err, "9", "12", "1125", "1126"):
		return fmt.Errorf("%w: %w", application.ErrProviderInvalidOperation, err)
	case isProviderErrorCode(err, "103", "116", "1051"):
		return fmt.Errorf("%w: %w", application.ErrProviderInsufficientFunds, err)
	case isProviderErrorCode(err, "104"):
		return fmt.Errorf("%w: %w", application.ErrProviderRecurringFailed, err)
	case isProviderErrorCode(err, "262"):
		return fmt.Errorf("%w: %w", application.ErrProviderSavedMethodExpired, err)
	case isProviderErrorCode(err, "100"):
		return fmt.Errorf("%w: %w", application.ErrProviderDuplicateOperation, err)
	}
	return err
}
