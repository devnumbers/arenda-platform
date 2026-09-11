package tkassa

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
)

const (
	// Fixed acknowledgement body T-Kassa expects: HTTP 200 with the plain
	// text "OK" (uppercase, no tags).
	webhookOK = "OK"

	notificationTypeAddCard       = "NotificationAddCard"
	notificationTypeAddCardLegacy = "AddCard"
)

// WebhookAck returns the fixed success acknowledgement T-Kassa expects HTTP
// handlers to send back after receiving a webhook.
func (p *Provider) WebhookAck() []byte {
	return []byte(webhookOK)
}

// ParseWebhook parses and verifies a T-Kassa webhook payload into the
// provider-neutral webhook event.
func (p *Provider) ParseWebhook(ctx context.Context, payload []byte) (application.WebhookEvent, error) {
	_ = ctx

	if err := verifyToken(payload, p.password); err != nil {
		return application.WebhookEvent{}, err
	}

	data, err := unmarshalWebhook(payload)
	if err != nil {
		return application.WebhookEvent{}, err
	}

	if getString(data, fieldTerminalKey) != p.terminalKey {
		return application.WebhookEvent{}, errors.New("tkassa: webhook terminal key mismatch")
	}

	switch kind := notificationType(data); kind {
	case notificationTypeAddCard, notificationTypeAddCardLegacy:
		return parseAddCardNotification(data)
	case "", "NotificationPayment":
		// Payment notifications either omit NotificationType or explicitly set
		// it to NotificationPayment.
		return parsePaymentNotification(data)
	default:
		return application.WebhookEvent{}, fmt.Errorf("tkassa: unknown notification type %q", kind)
	}
}

// notificationType returns the payload's notification type. The official
// NotificationAddCard payload omits NotificationType and OrderId: discriminate
// by RequestKey (present on add-card notifications, absent on payment
// notifications) so it does not fall into the payment branch and fail OrderId
// parsing with a 500. The type is normalized so downstream handling is uniform.
func notificationType(data map[string]any) string {
	kind := getString(data, fieldNotificationType)
	if kind == "" && getString(data, fieldRequestKey) != "" && getString(data, fieldOrderID) == "" {
		return notificationTypeAddCard
	}
	return kind
}

// parseAddCardNotification maps an add-card notification to the neutral
// event. A completed binding becomes the method-bound event; a refused one
// (REJECTED — the failure terminal of the binding-status dictionary, ADR 0017)
// becomes the binding-failed event: a validly signed delivery the application
// closes the session on, so the handler answers 200 and the provider stops
// redelivering. Anything without a terminal outcome is an error so the
// application never resolves a session on an intermediate status.
func parseAddCardNotification(data map[string]any) (application.WebhookEvent, error) {
	if isAddCardSuccessful(data) {
		return application.WebhookEvent{
			MethodBound: &application.MethodBoundNotification{
				BindingID: getString(data, fieldRequestKey),
				Method:    savedMethodFromWebhook(data),
			},
		}, nil
	}
	if getString(data, fieldStatus) == statusRejected {
		return application.WebhookEvent{
			MethodBindingFailed: &application.MethodBindingFailedNotification{
				BindingID: getString(data, fieldRequestKey),
				ErrorCode: getString(data, fieldErrorCode),
			},
		}, nil
	}
	return application.WebhookEvent{}, errors.New("tkassa: add card webhook ignored: binding outcome not final")
}

// parsePaymentNotification maps a payment notification to the payment event,
// including the saved method of a save-method parent payment.
func parsePaymentNotification(data map[string]any) (application.WebhookEvent, error) {
	orderID := getString(data, fieldOrderID)
	internalPaymentID, err := uuid.Parse(orderID)
	if err != nil {
		return application.WebhookEvent{}, fmt.Errorf("tkassa: parse OrderId %q: %w", orderID, err)
	}

	amount, err := getAmount(data)
	if err != nil {
		return application.WebhookEvent{}, err
	}

	event := application.WebhookEvent{
		Payment: &application.PaymentNotification{
			InternalPaymentID: internalPaymentID,
			ProviderPaymentID: getString(data, fieldPaymentID),
			Status:            mapStatus(getString(data, "Status")),
			ErrorCode:         paymentErrorCode(data),
			AmountKopecks:     amount,
			// The masked card the charge ran on (issue #619): the payment
			// history's snapshot of the card used, on every payment
			// notification that carries it.
			CardMask: getString(data, "Pan"),
		},
	}
	// The RebillId of a save-method parent payment arrives in payment
	// notifications (typically AUTHORIZED): surface it as the saved method so
	// the application can store the charge token, exactly like a dedicated
	// binding notification would.
	if rebillID := getString(data, fieldRebillID); rebillID != "" {
		method := savedMethodFromWebhook(data)
		event.Payment.SavedMethod = &method
	}
	return event, nil
}

// paymentErrorCode returns the ErrorCode pointer of a payment notification:
// absent, empty and "0" codes carry no error and stay nil.
func paymentErrorCode(data map[string]any) *string {
	code := getString(data, fieldErrorCode)
	if code == "" || code == "0" {
		return nil
	}
	return &code
}

// savedMethodFromWebhook collects the neutral saved-method fields shared by
// payment and add-card notifications.
func savedMethodFromWebhook(data map[string]any) application.SavedMethod {
	return application.SavedMethod{
		ProviderMethodID: getString(data, fieldCardID),
		ChargeToken:      getString(data, fieldRebillID),
		MaskedPan:        getString(data, "Pan"),
		ExpDate:          getString(data, fieldExpDate),
		CustomerRef:      getString(data, "CustomerKey"),
	}
}

func getString(data map[string]any, key string) string {
	v, ok := data[key]
	if !ok {
		return ""
	}
	return stringifyValue(v)
}

func getAmount(data map[string]any) (int64, error) {
	v, ok := data[fieldAmount]
	if !ok {
		return 0, nil
	}
	s := stringifyValue(v)
	if s == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("tkassa: parse Amount=%q: %w", s, err)
	}
	return n, nil
}

// isAddCardSuccessful reports whether an add-card notification describes a
// completed binding: status COMPLETED and Success=true.
func isAddCardSuccessful(data map[string]any) bool {
	if status := getString(data, "Status"); status != statusCompleted {
		return false
	}
	success, ok := data[fieldSuccess].(bool)
	return ok && success
}
