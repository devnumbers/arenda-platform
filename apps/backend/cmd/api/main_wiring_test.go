package main

// The realtime carrier's wiring guard (карта #714, тикет #861; ADR 0062;
// testing-strategy, layer 3): every late-bound SetRealtimePublisher of
// bindRealtimePublisher must land the carrier on its service — a nil carrier
// is the pre-#716 silence, and a forgotten binding would only surface on a
// live stage. The test replays the composition root's own binding over fresh
// service instances (zero-allocated wiring targets: the services' tx store
// factories are unexported, and no use case runs here, so only the type
// matters) and reads each unexported `realtime` field back through
// reflection. Removing any setter in bindRealtimePublisher turns this test
// red with the service's name — the ticket's acceptance demo
// (закомментировал — упал — вернул).

import (
	"reflect"
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/cmd/api/wire"
	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	contactsapp "github.com/nambers/arenda-planform/apps/backend/internal/contacts/application"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	realtimeapp "github.com/nambers/arenda-planform/apps/backend/internal/realtime/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/realtime/realtimetest"
	rentalsapp "github.com/nambers/arenda-planform/apps/backend/internal/rentals/application"
	tasksapp "github.com/nambers/arenda-planform/apps/backend/internal/tasks/application"
	"github.com/stretchr/testify/require"
)

func TestRealtimeCarrierWiring(t *testing.T) {
	t.Parallel()

	// The modules under test: fresh zero-allocated services behind the same
	// module structs the composition root binds through — the binder's
	// signature type-checks them, no dependency needs to exist.
	paymentsMod := &wire.Payments{
		PaymentService:   zeroService[paymentsapp.PaymentService](),
		OperationService: zeroService[paymentsapp.OperationService](),
		GlobalPayments:   zeroService[paymentsapp.GlobalPaymentService](),
	}
	tasksMod := &wire.Tasks{
		RuleService: zeroService[tasksapp.RuleService](),
		TaskService: zeroService[tasksapp.TaskService](),
	}
	contactsMod := &wire.Contacts{
		ContactService: zeroService[contactsapp.ContactService](),
	}
	rentalsMod := &wire.Rentals{
		RentalService: zeroService[rentalsapp.RentalService](),
	}
	propertiesMod := &wire.Properties{
		PropertyService: zeroService[propertiesapp.PropertyService](),
	}
	accessMod := &wire.Access{
		AccessService: zeroService[accessapp.AccessService](),
	}

	carrier := &realtimetest.RecordingPublisher{}
	bindRealtimePublisher(carrier, paymentsMod, tasksMod, contactsMod, rentalsMod, propertiesMod, accessMod)

	for _, target := range []struct {
		name string
		svc  any
	}{
		{"payments.PaymentService", paymentsMod.PaymentService},
		{"payments.OperationService", paymentsMod.OperationService},
		{"payments.GlobalPayments", paymentsMod.GlobalPayments},
		{"tasks.RuleService", tasksMod.RuleService},
		{"tasks.TaskService", tasksMod.TaskService},
		{"contacts.ContactService", contactsMod.ContactService},
		{"rentals.RentalService", rentalsMod.RentalService},
		{"properties.PropertyService", propertiesMod.PropertyService},
		{"access.AccessService", accessMod.AccessService},
	} {
		field := realtimeField(t, target.svc)
		require.False(t, field.IsNil(),
			"service %s lost its realtime carrier binding — the wiring in bindRealtimePublisher is incomplete", target.name)
		require.Equal(t, reflect.TypeFor[*realtimetest.RecordingPublisher](), field.Elem().Type(),
			"service %s holds something other than the carrier bindRealtimePublisher was given", target.name)
	}
}

// realtimeField reads the service's unexported `realtime` field. An invalid
// value means the field does not exist — the wiring test must follow a
// renamed binding loudly, not pass silently.
func realtimeField(t *testing.T, svc any) reflect.Value {
	t.Helper()
	field := reflect.ValueOf(svc).Elem().FieldByName("realtime")
	require.True(t, field.IsValid(),
		"service %T has no `realtime` field — the wiring test follows a renamed binding", svc)
	require.Equal(t, reflect.TypeFor[realtimeapp.Publisher](), field.Type(),
		"service %T's realtime field is not a realtimeapp.Publisher", svc)
	return field
}

// zeroService allocates a service of the concrete type — a wiring target the
// binder's setters can write their carrier into. No constructor, no deps:
// the setters only assign a field.
func zeroService[T any]() *T {
	svc, ok := reflect.New(reflect.TypeFor[T]()).Interface().(*T)
	if !ok {
		// Unreachable by construction: reflect.New of T's type holds *T.
		panic("zeroService: reflect.New did not yield *" + reflect.TypeFor[T]().String())
	}
	return svc
}
