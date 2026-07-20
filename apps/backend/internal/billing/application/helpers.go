package application

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

const tkassaDescriptionLimit = 140

const (
	renewalBatchSize = 100
	graceBatchSize   = 100
	// maxRenewalChargeAttempts caps how many times a renewal charge may fail
	// with an unresolved (pending or unexpected) provider outcome before the
	// payment is marked failed and the subscription is moved to grace.
	maxRenewalChargeAttempts = 3
)

const (
	pendingUpgradeStalenessThreshold = 5 * time.Minute
	pendingPaymentStalenessThreshold = pendingUpgradeStalenessThreshold
)

func upgradePaymentDescription(name domain.TariffName, period domain.SubscriptionPeriod) string {
	return fmt.Sprintf("Upgrade to %s (%s)", name, period)
}

func renewalPaymentDescription(name domain.TariffName, period domain.SubscriptionPeriod) string {
	return fmt.Sprintf("Subscription renewal %s (%s)", name, period)
}

// providerError is implemented by provider-specific errors that expose a
// machine-readable error code (e.g. T-Kassa's ErrorCode).
type providerError interface {
	error
	ProviderErrorCode() string
}

func providerErrorCode(err error) *string {
	var coder providerError
	if errors.As(err, &coder) {
		code := coder.ProviderErrorCode()
		if code != "" {
			return &code
		}
	}
	return nil
}

// providerResultErrorCode converts the error code reported in a provider
// result (ChargeResult/StatusResult) to the nullable form persisted on failed
// payments, falling back to the code carried by the provider error cause when
// the result does not carry one.
func providerResultErrorCode(resultCode string, cause error) *string {
	if resultCode != "" {
		return &resultCode
	}
	return providerErrorCode(cause)
}

func truncateTkassaDescription(s string) string {
	if len(s) <= tkassaDescriptionLimit {
		return s
	}
	return s[:tkassaDescriptionLimit]
}

func tkassaCallbackURLs(baseURL string, paymentID uuid.UUID) (notification, success, fail string) {
	baseURL = strings.TrimRight(baseURL, "/")
	notification = tkassaNotificationURL(baseURL)
	success = fmt.Sprintf("%s/subscription/payments/%s/success", baseURL, paymentID.String())
	fail = fmt.Sprintf("%s/subscription/payments/%s/fail", baseURL, paymentID.String())
	return
}

// tkassaNotificationURL builds the webhook URL T-Kassa calls with payment and
// card-binding status notifications.
func tkassaNotificationURL(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/webhooks/payment/tkassa"
}

// tkassaAddCardReturnURLs builds the return URLs passed per-request to T-Kassa
// AddCard. The /api prefix is the public path: Caddy strips it on prod and the
// Next.js rewrite adds it locally, while the backend itself serves the routes
// without the prefix.
func tkassaAddCardReturnURLs(baseURL string) (success, fail string) {
	baseURL = strings.TrimRight(baseURL, "/")
	success = baseURL + "/api/subscription/payment-methods/add-card/success"
	fail = baseURL + "/api/subscription/payment-methods/add-card/fail"
	return
}

func maskToken(token string) string {
	if len(token) <= 4 {
		return "****"
	}
	return "****" + token[len(token)-4:]
}

func maskCardID(cardID string) string {
	if len(cardID) <= 4 {
		return "****"
	}
	return cardID[:2] + "****" + cardID[len(cardID)-4:]
}

func isAddCardNotificationType(notificationType string) bool {
	return notificationType == "NotificationAddCard" || notificationType == "AddCard"
}

// isSubscriptionRenewalApplied reports whether the subscription already reflects
// the given successful payment. The primary check is the last applied payment id
// stored on the subscription; a date-based fallback is kept for subscriptions
// created before the last_applied_payment_id column existed.
func isSubscriptionRenewalApplied(sub domain.Subscription, payment domain.SubscriptionPayment) bool {
	// Primary check: the subscription records exactly which payment was last
	// applied. A non-nil value that does not match means this payment has not
	// been applied yet.
	if sub.LastAppliedPaymentID != nil {
		return *sub.LastAppliedPaymentID == payment.ID
	}
	// Fallback for subscriptions created before the last_applied_payment_id column.
	if sub.TariffID != payment.TariffID {
		return false
	}
	base := payment.SucceededAt
	if base == nil {
		// Backward compatibility for payments created before the succeeded_at
		// column was introduced; use creation time as a conservative fallback.
		base = &payment.CreatedAt
	}
	expectedValidUntil := base.AddDate(0, 1, 0)
	if payment.Period == domain.PeriodYear {
		expectedValidUntil = base.AddDate(1, 0, 0)
	}
	return sub.ValidUntil != nil && !sub.ValidUntil.Before(expectedValidUntil)
}
