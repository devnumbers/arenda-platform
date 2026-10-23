package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/notificationsjob"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The per-payment email flag's delivery-time gate (карта #1551, тикет
// #1554): the real PaymentEmailGate and the real DeliverEmailWorker over the
// real feed row, only the transports (contact, SMTP, provider budget, the
// account matrix) stubbed. These tests run against a real Postgres via
// TEST_DATABASE_URL (the shared harness of the notifications integration
// tests) and are skipped when it is unset.

// insertEmailGatePayment adds a payment rule row with the per-payment email
// flag spelled out (payments.notify_email, миграция 000163) — the flag's own
// fixture: the scan fixtures spell the auto-pay pair instead.
func insertEmailGatePayment(
	ctx context.Context, t *testing.T, pool *pgxpool.Pool, ownerID, propertyID uuid.UUID, notifyEmail bool,
) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := pool.Exec(ctx, `
		INSERT INTO payments (id, owner_id, property_id, type, title, amount_kopecks, recurrence,
			since, auto_pay, notify_auto_paid, notify_email, category_slug)
		VALUES ($1, $2, $3, 'expense', 'Обслуживание', 250000, '{"kind": "monthly"}',
			'2026-08-01', false, true, $4, 'maintenance')`,
		id, ownerID, propertyID, notifyEmail)
	require.NoError(t, err)
	return id
}

// gateContactStub resolves every scope to the same verified address.
type gateContactStub struct{}

func (gateContactStub) Resolve(ctx context.Context, scope uuid.UUID) (application.Contact, error) {
	return application.Contact{Email: "owner@example.ru"}, nil
}

// gateEmailerStub counts the worker's send attempts.
type gateEmailerStub struct {
	calls int
}

func (e *gateEmailerStub) SendTemplate(ctx context.Context, to, subject, template string, data map[string]any) error {
	e.calls++
	return nil
}

// gateLimiterStub always lets the send through — the budget is not this
// ticket's subject.
type gateLimiterStub struct{}

func (gateLimiterStub) Allow(key string) bool { return true }

// setupEmailGateWorker wires the real delivery-time gates (account matrix +
// per-payment flag) and the real feed repository into the real email worker,
// transports stubbed — the worker's full DB path with no SMTP at the end.
func setupEmailGateWorker(t *testing.T, pool *pgxpool.Pool) (*notificationsjob.DeliverEmailWorker, *gateEmailerStub) {
	t.Helper()
	metrics, err := notificationsjob.NewEmailMetrics()
	require.NoError(t, err)
	emailer := &gateEmailerStub{}
	worker := notificationsjob.NewDeliverEmailWorker(
		NewNotificationRepository(pool),
		gateContactStub{},
		emailer,
		gateLimiterStub{},
		metrics,
		"https://app.example",
		nil,
		NewDeliverySettingsGate(NewEmailPreferencesRepository(pool)),
		NewPaymentEmailGate(pool),
	)
	return worker, emailer
}

// emailGateFeedRow publishes a payment_due feed row whose payload links the
// given rule — the payment events' payload shape (решение #737) the gate
// reads.
func emailGateFeedRow(ctx context.Context, t *testing.T, pool *pgxpool.Pool, userID, paymentID uuid.UUID) domain.Notification {
	t.Helper()
	repo := NewNotificationRepository(pool)
	n, err := domain.NewNotification(
		uuid.Must(uuid.NewV7()), userID,
		domain.EventPaymentDue,
		"Оплатите платёж",
		"Платёж «Обслуживание» по объекту «Объект»: 250000. Срок оплаты: 1 октября",
		"Объект",
		domain.Payload{
			Property:  &domain.EntityRef{ID: uuid.Must(uuid.NewV7()), Name: "Объект"},
			PaymentID: &paymentID,
		},
		domain.DedupKey("payment_due:email-gate:"+uuid.Must(uuid.NewV7()).String()),
	)
	require.NoError(t, err)
	inserted, err := repo.Insert(ctx, *n)
	require.NoError(t, err)
	require.True(t, inserted)
	return *n
}

func gateEmailJob(id uuid.UUID) *river.Job[notificationsjob.DeliverEmailArgs] {
	return &river.Job[notificationsjob.DeliverEmailArgs]{
		JobRow: &rivertype.JobRow{Attempt: 1},
		Args:   notificationsjob.DeliverEmailArgs{NotificationID: id},
	}
}

// The flag's three answers: off silences the rule, on lets the letter
// through, a vanished rule reads as allowed — «как сегодня» (тикет #1554).
func TestPaymentEmailGate_FlagSemantics(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()

	owner := createUserInZone(t, pool, "Europe/Moscow")
	prop := createLiveProperty(t, pool, owner)
	silent := insertEmailGatePayment(ctx, t, pool, owner, prop, false)
	loud := insertEmailGatePayment(ctx, t, pool, owner, prop, true)

	gate := NewPaymentEmailGate(pool)

	allowed, err := gate.EmailAllowed(ctx, silent)
	require.NoError(t, err)
	assert.False(t, allowed, "the flag off silences the rule's letters")

	allowed, err = gate.EmailAllowed(ctx, loud)
	require.NoError(t, err)
	assert.True(t, allowed, "the flag on delivers as today")

	allowed, err = gate.EmailAllowed(ctx, uuid.Must(uuid.NewV7()))
	require.NoError(t, err)
	assert.True(t, allowed, "a vanished rule leaves the in-flight delivery as it was")
}

// The flag off suppresses the letter only: the feed row the job serves stays
// intact — the gate is an email-leg outcome, not a retraction (лента/тосты
// не трогаются, решение владельца).
func TestDeliverEmailWorker_PaymentFlagOffSuppressesLetterOnly(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()

	owner := createUserInZone(t, pool, "Europe/Moscow")
	prop := createLiveProperty(t, pool, owner)
	rule := insertEmailGatePayment(ctx, t, pool, owner, prop, false)
	n := emailGateFeedRow(ctx, t, pool, owner, rule)

	worker, emailer := setupEmailGateWorker(t, pool)
	require.NoError(t, worker.Work(ctx, gateEmailJob(n.ID)))
	assert.Zero(t, emailer.calls, "the flag off suppresses the letter")

	feedRepo := NewNotificationRepository(pool)
	stored, err := feedRepo.GetByID(ctx, n.ID)
	require.NoError(t, err, "the feed row stays — the suppression is the email leg's outcome alone")
	assert.Equal(t, n.ID, stored.ID)
	assert.Equal(t, n.Title, stored.Title)
	assert.Nil(t, stored.DeletedAt)
	require.NotNil(t, stored.Payload.PaymentID)
	assert.Equal(t, rule, *stored.Payload.PaymentID)
}

// The flag on and a payload pointing at a vanished rule both deliver the
// letter — «как сегодня» (тикет #1554).
func TestDeliverEmailWorker_PaymentFlagOnAndMissingRuleDeliver(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()

	owner := createUserInZone(t, pool, "Europe/Moscow")
	prop := createLiveProperty(t, pool, owner)

	t.Run("flag on", func(t *testing.T) {
		t.Parallel()

		rule := insertEmailGatePayment(ctx, t, pool, owner, prop, true)
		n := emailGateFeedRow(ctx, t, pool, owner, rule)

		worker, emailer := setupEmailGateWorker(t, pool)
		require.NoError(t, worker.Work(ctx, gateEmailJob(n.ID)))
		assert.Equal(t, 1, emailer.calls, "the flag on delivers the letter as today")
	})

	t.Run("payment row gone", func(t *testing.T) {
		t.Parallel()

		n := emailGateFeedRow(ctx, t, pool, owner, uuid.Must(uuid.NewV7()))

		worker, emailer := setupEmailGateWorker(t, pool)
		require.NoError(t, worker.Work(ctx, gateEmailJob(n.ID)))
		assert.Equal(t, 1, emailer.calls, "a vanished rule leaves the letter as today")
	})
}
