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

	notificationType := getString(data, fieldNotificationType)
	// The official NotificationAddCard payload omits NotificationType and
	// OrderId: discriminate by RequestKey (present on add-card notifications,
	// absent on payment notifications) so it does not fall into the payment
	// branch and fail OrderId parsing with a 500. Normalize the type so
	// downstream handling is uniform.
	if notificationType == "" && getString(data, fieldRequestKey) != "" && getString(data, fieldOrderID) == "" {
		notificationType = notificationTypeAddCard
	}
	switch notificationType {
	case notificationTypeAddCard, notificationTypeAddCardLegacy:
		if !isAddCardSuccessful(data) {
			return application.WebhookEvent{}, errors.New("tkassa: add card webhook ignored: binding not successful")
		}
		return application.WebhookEvent{
			MethodBound: &application.MethodBoundNotification{
				BindingID: getString(data, fieldRequestKey),
				Method:    savedMethodFromWebhook(data),
			},
		}, nil
	case "", "NotificationPayment":
		// Payment notifications either omit NotificationType or explicitly set
		// it to NotificationPayment. Continue parsing as a payment webhook
		// below.
	default:
		return application.WebhookEvent{}, fmt.Errorf("tkassa: unknown notification type %q", notificationType)
	}

	orderID := getString(data, fieldOrderID)
	internalPaymentID, err := uuid.Parse(orderID)
	if err != nil {
		return application.WebhookEvent{}, fmt.Errorf("tkassa: parse OrderId %q: %w", orderID, err)
	}

	status := mapStatus(getString(data, "Status"))
	errorCode := getString(data, fieldErrorCode)
	var errorCodePtr *string
	if errorCode != "" && errorCode != "0" {
		errorCodePtr = &errorCode
	}

	amount, err := getAmount(data)
	if err != nil {
		return application.WebhookEvent{}, err
	}

	event := application.WebhookEvent{
		Payment: &application.PaymentNotification{
			InternalPaymentID: internalPaymentID,
			ProviderPaymentID: getString(data, fieldPaymentID),
			Status:            status,
			ErrorCode:         errorCodePtr,
			AmountKopecks:     amount,
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
