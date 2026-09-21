package application

import (
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
)

// deeplinkPropertyName is the property snapshot's name in the deeplink
// tests' payloads.
const deeplinkPropertyName = "Дом у парка"

// The grace events (issue #253, #741) route to the payment-methods screen —
// the one place the user fixes the failed renewal charge. The path is the
// legacy direct channel's path, preserved verbatim.
func TestDeeplinkForGrace(t *testing.T) {
	t.Parallel()

	for _, eventType := range []domain.EventType{
		domain.EventSubscriptionGraceEntered,
		domain.EventSubscriptionGraceExpiring,
	} {
		n := domain.Notification{EventType: eventType}
		if got := DeepLinkFor(n); got != graceNotificationPath {
			t.Errorf("DeepLinkFor(%q) = %q, want %s", eventType, got, graceNotificationPath)
		}
	}

	// An event without an entry delivers without a link.
	if got := DeepLinkFor(domain.Notification{EventType: domain.EventSystemMaintenance}); got != "" {
		t.Errorf("DeepLinkFor(system_maintenance) = %q, want empty", got)
	}
}

// The rental-completed event (#748) routes to the rental's screen, whose
// path carries both the property and the rental — the ids come from the
// row's payload, not from a static path.
func TestDeeplinkForRentalCompleted(t *testing.T) {
	t.Parallel()

	propertyID := uuid.Must(uuid.NewV7())
	rentalID := uuid.Must(uuid.NewV7())
	n := domain.Notification{
		EventType: domain.EventRentalCompleted,
		Payload: domain.Payload{
			Property: &domain.EntityRef{ID: propertyID, Name: deeplinkPropertyName},
			RentalID: &rentalID,
		},
	}
	want := "/properties/" + propertyID.String() + "/rentals/" + rentalID.String()
	if got := DeepLinkFor(n); got != want {
		t.Errorf("DeepLinkFor(rental_completed) = %q, want %q", got, want)
	}

	// A payload without the ids (a hand-edited row) delivers without a link
	// rather than a broken one.
	half := domain.Notification{
		EventType: domain.EventRentalCompleted,
		Payload:   domain.Payload{Property: &domain.EntityRef{ID: propertyID, Name: deeplinkPropertyName}},
	}
	if got := DeepLinkFor(half); got != "" {
		t.Errorf("DeepLinkFor(rental_completed without rental id) = %q, want empty", got)
	}
	if got := DeepLinkFor(domain.Notification{EventType: domain.EventRentalCompleted}); got != "" {
		t.Errorf("DeepLinkFor(rental_completed without payload) = %q, want empty", got)
	}
}

// The payment events (#749) route to the payment's page, whose path carries
// both the property and the rule — the ids come from the row's payload, not
// from a static path.
func TestDeeplinkForPayment(t *testing.T) {
	t.Parallel()

	propertyID := uuid.Must(uuid.NewV7())
	ruleID := uuid.Must(uuid.NewV7())
	for _, eventType := range []domain.EventType{domain.EventPaymentDue, domain.EventPaymentOverdue} {
		n := domain.Notification{
			EventType: eventType,
			Payload: domain.Payload{
				Property:  &domain.EntityRef{ID: propertyID, Name: deeplinkPropertyName},
				PaymentID: &ruleID,
			},
		}
		want := "/properties/" + propertyID.String() + "/payments/" + ruleID.String()
		if got := DeepLinkFor(n); got != want {
			t.Errorf("DeepLinkFor(%q) = %q, want %q", eventType, got, want)
		}
	}

	// A payload without the ids (a hand-edited row) delivers without a link
	// rather than a broken one.
	half := domain.Notification{
		EventType: domain.EventPaymentDue,
		Payload:   domain.Payload{Property: &domain.EntityRef{ID: propertyID, Name: deeplinkPropertyName}},
	}
	if got := DeepLinkFor(half); got != "" {
		t.Errorf("DeepLinkFor(payment_due without rule id) = %q, want empty", got)
	}
	if got := DeepLinkFor(domain.Notification{EventType: domain.EventPaymentOverdue}); got != "" {
		t.Errorf("DeepLinkFor(payment_overdue without payload) = %q, want empty", got)
	}
}

// The task-overdue event (#750) routes to the task's edit screen — its
// rule's screen: a property-bound task navigates through the property, the
// task without a property — through the flat tasks route (ADR 0052).
func TestDeeplinkForTaskOverdue(t *testing.T) {
	t.Parallel()

	propertyID := uuid.Must(uuid.NewV7())
	ruleID := uuid.Must(uuid.NewV7())
	bound := domain.Notification{
		EventType: domain.EventTaskOverdue,
		Payload: domain.Payload{
			Property:   &domain.EntityRef{ID: propertyID, Name: deeplinkPropertyName},
			TaskRuleID: &ruleID,
		},
	}
	want := "/properties/" + propertyID.String() + "/tasks/" + ruleID.String() + "/edit"
	if got := DeepLinkFor(bound); got != want {
		t.Errorf("DeepLinkFor(bound task_overdue) = %q, want %q", got, want)
	}

	propertyless := domain.Notification{
		EventType: domain.EventTaskOverdue,
		Payload:   domain.Payload{TaskRuleID: &ruleID},
	}
	wantFlat := "/tasks/" + ruleID.String() + "/edit"
	if got := DeepLinkFor(propertyless); got != wantFlat {
		t.Errorf("DeepLinkFor(propertyless task_overdue) = %q, want %q", got, wantFlat)
	}

	// A payload without the rule id (a hand-edited row) delivers without a
	// link rather than a broken one.
	if got := DeepLinkFor(domain.Notification{EventType: domain.EventTaskOverdue}); got != "" {
		t.Errorf("DeepLinkFor(task_overdue without rule id) = %q, want empty", got)
	}
}
