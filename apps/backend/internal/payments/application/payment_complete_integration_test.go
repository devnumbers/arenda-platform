//go:build integration

package application_test

// Завершение платежа (тикет #1377, ревизия #1340; ревизия 2 — #1426):
// завершение удаляет правило с сохранением оплаченных операций — хвост
// payment_id NULL по FK, origin 'payment' (тот же исход, что у завершения
// аренды #1161 и удаления #446). Гейт доступности один — арендный (ревизия
// 2 сняла endDate, «всё оплачено» и открытую паузу: доступность как у
// удаления #446), повторное завершение — приватный 404: правила больше нет.
// Скрытие из лент, списков и подсчётов хаба — фактом удаления; звезда
// читателя падает вместе с правилом (FK CASCADE). Журнал правок каскадит —
// строки о завершении в него не пишем; «История действий» несёт «Платёж
// завершён: …», audit — payment.completed.

import (
	"testing"
	"time"

	"github.com/google/uuid"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// completeCmd is the completable create fixture: a daily rule ending on the
// owner's today — the «в тот же день» canon makes it completable as soon as
// every occurrence is paid.
func (h *paymentsHarness) completeCmd() paymentsapp.CreatePaymentCommand {
	cmd := h.createCmd()
	endDate := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	cmd.EndDate = &endDate
	return cmd
}

// payAllOperations pays every planned occurrence of the rule — the manual
// «Оплатить сейчас» facts.
func (h *paymentsHarness) payAllOperations(t *testing.T, paymentID uuid.UUID) {
	t.Helper()
	for {
		ops, err := h.ops.ListPaymentOperations(h.ctx(), h.owner, h.propID, paymentID,
			h.listCmd(nil, 50, 0, false))
		require.NoError(t, err)
		var paid bool
		for _, op := range ops {
			if op.Operation.Status == domain.StatusPlanned {
				_, err := h.ops.PayOperation(h.ctx(), h.owner, h.propID, op.Operation.ID)
				require.NoError(t, err)
				paid = true
				break // The listing is stale after a pay — re-read.
			}
		}
		if !paid {
			return
		}
	}
}

// completableRule creates a rule through the service and pays it whole — the
// state the completion gate admits.
func (h *paymentsHarness) completableRule(t *testing.T) domain.Payment {
	t.Helper()
	created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.completeCmd())
	require.NoError(t, err)
	h.payAllOperations(t, created.ID)
	return created
}

// ruleOperationIDs returns the rule's stored operation ids straight from the
// table — the ids survive the completion's deletion, the rows do too (the
// tail), only their payment_id breaks.
func (h *paymentsHarness) ruleOperationIDs(t *testing.T, paymentID uuid.UUID) []uuid.UUID {
	t.Helper()
	rows, err := h.pool.Query(h.ctx(),
		`SELECT id FROM operations WHERE payment_id = $1 ORDER BY date`, paymentID)
	require.NoError(t, err)
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		require.NoError(t, rows.Scan(&id))
		out = append(out, id)
	}
	require.NoError(t, rows.Err())
	return out
}

func TestCompletePayment_DeletesRuleKeepsPaidTail(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	created := h.completableRule(t)
	opIDs := h.ruleOperationIDs(t, created.ID)
	require.NotEmpty(t, opIDs)

	require.NoError(t, h.svc.CompletePayment(h.ctx(), h.owner, h.propID, created.ID))

	// Правило удалено: прямое чтение — приватный 404.
	_, err := h.svc.GetPayment(h.ctx(), h.owner, h.propID, created.ID)
	require.ErrorIs(t, err, paymentsapp.ErrNotFound, "err = %v", err)

	// Хвост правила: оплаченные операции живы с payment_id NULL и origin
	// 'payment' — тот же исход, что при завершении аренды (#1161).
	for _, opID := range opIDs {
		var paymentID *string
		var status, origin string
		err := h.pool.QueryRow(h.ctx(),
			`SELECT payment_id::text, status, origin FROM operations WHERE id = $1`, opID,
		).Scan(&paymentID, &status, &origin)
		require.NoError(t, err)
		assert.Nil(t, paymentID, "operation %s keeps no payment link", opID)
		assert.Equal(t, domain.StatusPaid, domain.OperationStatus(status))
		assert.Equal(t, domain.OriginPayment, domain.OperationOrigin(origin))
	}

	// Лента операций объекта несёт хвост: оплаченный факт читается.
	feed, err := h.ops.ListPropertyOperations(h.ctx(), h.owner, h.propID,
		h.listCmd(nil, 50, 0, false))
	require.NoError(t, err)
	feedIDs := make([]uuid.UUID, 0, len(feed))
	for _, item := range feed {
		feedIDs = append(feedIDs, item.Operation.ID)
	}
	for _, opID := range opIDs {
		assert.Contains(t, feedIDs, opID, "paid tail rides the property's operations feed")
	}

	// Аудит: создание + завершение (оплата пишет своё action на операцию).
	actions := h.auditActions(t, created.ID)
	assert.Equal(t, []string{actionPaymentCreated, "payment.completed"}, actions)

	// «История действий»: строка завершения с базой «Завершение».
	rows := h.journalActions(t, "payment.completed")
	require.Len(t, rows, 1)

	// Журнал правок каскадит вместе с правилом — строки о завершении нет.
	assert.Empty(t, h.loadChangeLog(t, created.ID))

	// Realtime: фреймы лент платежей и операций — хвост пришёл в ленту объекта.
	assert.Contains(t, h.realtime.Pairs(), "payments:"+h.propID.String())
	assert.Contains(t, h.realtime.Pairs(), "operations:"+h.propID.String())
}

// journalActions reads the action journal's rows of one action kind on the
// harness property.
func (h *paymentsHarness) journalActions(t *testing.T, action string) []uuid.UUID {
	t.Helper()
	rows, err := h.pool.Query(h.ctx(),
		`SELECT id FROM action_journal WHERE property_id = $1 AND action = $2`,
		h.propID, action)
	require.NoError(t, err)
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		require.NoError(t, rows.Scan(&id))
		out = append(out, id)
	}
	require.NoError(t, rows.Err())
	return out
}

func TestCompletePayment_Gates(t *testing.T) {
	t.Parallel()

	t.Run("повторный complete — приватный 404, правила больше нет", func(t *testing.T) {
		t.Parallel()
		h := newPaymentsHarness(t).withOwner("Europe/Moscow")
		created := h.completableRule(t)
		require.NoError(t, h.svc.CompletePayment(h.ctx(), h.owner, h.propID, created.ID))
		err := h.svc.CompletePayment(h.ctx(), h.owner, h.propID, created.ID)
		assert.ErrorIs(t, err, paymentsapp.ErrNotFound, "err = %v", err)
	})

	t.Run("открытая пауза не мешает — доступность как у удаления (#1426)", func(t *testing.T) {
		t.Parallel()
		h := newPaymentsHarness(t).withOwner("Europe/Moscow")
		created := h.completableRule(t)
		_, err := h.svc.PausePayment(h.ctx(), h.owner, h.propID, created.ID)
		require.NoError(t, err)
		require.NoError(t, h.svc.CompletePayment(h.ctx(), h.owner, h.propID, created.ID))
		_, err = h.svc.GetPayment(h.ctx(), h.owner, h.propID, created.ID)
		assert.ErrorIs(t, err, paymentsapp.ErrNotFound, "err = %v", err)
	})

	t.Run("endDate впереди при оплаченных вхождениях — завершается (#1426)", func(t *testing.T) {
		t.Parallel()
		h := newPaymentsHarness(t).withOwner("Europe/Moscow")
		cmd := h.completeCmd()
		endDate := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
		cmd.EndDate = &endDate
		created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, cmd)
		require.NoError(t, err)
		// Вычисляемый флаг отвечает «завершён» уже сегодня — мутация больше
		// не ждёт конца правила (ревизия 2 сняла endDate-гейт).
		h.payAllOperations(t, created.ID)
		settled, err := h.svc.CompletedStatus(h.ctx(), h.owner, h.propID, created.ID)
		require.NoError(t, err)
		assert.True(t, settled)
		require.NoError(t, h.svc.CompletePayment(h.ctx(), h.owner, h.propID, created.ID))
		_, err = h.svc.GetPayment(h.ctx(), h.owner, h.propID, created.ID)
		assert.ErrorIs(t, err, paymentsapp.ErrNotFound, "err = %v", err)
	})

	t.Run("бессрочное правило — завершается (#1426)", func(t *testing.T) {
		t.Parallel()
		h := newPaymentsHarness(t).withOwner("Europe/Moscow")
		created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
		require.NoError(t, err)
		h.payAllOperations(t, created.ID)
		require.NoError(t, h.svc.CompletePayment(h.ctx(), h.owner, h.propID, created.ID))
		_, err = h.svc.GetPayment(h.ctx(), h.owner, h.propID, created.ID)
		assert.ErrorIs(t, err, paymentsapp.ErrNotFound, "err = %v", err)
	})

	t.Run("частично оплаченное правило — плановые сносятся, оплаченные остаются хвостом", func(t *testing.T) {
		t.Parallel()
		h := newPaymentsHarness(t).withOwner("Europe/Moscow")
		created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
		require.NoError(t, err)
		// Двое суток тика: у бессрочного ежедневного правила копятся
		// непогашенные плановые — просрочка включена.
		h.clock.now = tickBaseTime.AddDate(0, 0, 2)
		h.runTick()
		ops, err := h.ops.ListPaymentOperations(h.ctx(), h.owner, h.propID, created.ID,
			h.listCmd(nil, 50, 0, false))
		require.NoError(t, err)
		var paidID uuid.UUID
		for _, op := range ops {
			if op.Operation.Status == domain.StatusPlanned {
				_, err := h.ops.PayOperation(h.ctx(), h.owner, h.propID, op.Operation.ID)
				require.NoError(t, err)
				paidID = op.Operation.ID
				break // Одно оплаченное — и достаточно: остальное остаётся долгом.
			}
		}
		require.NotEqual(t, uuid.Nil, paidID, "fixture: a planned occurrence got paid")
		allIDs := h.ruleOperationIDs(t, created.ID)
		require.Greater(t, len(allIDs), 1, "fixture: unpaid planned occurrences remain")

		require.NoError(t, h.svc.CompletePayment(h.ctx(), h.owner, h.propID, created.ID))

		_, err = h.svc.GetPayment(h.ctx(), h.owner, h.propID, created.ID)
		require.ErrorIs(t, err, paymentsapp.ErrNotFound, "err = %v", err)
		// Оплаченное — хвост: payment_id NULL, origin 'payment'.
		var paymentID *string
		var status, origin string
		err = h.pool.QueryRow(h.ctx(),
			`SELECT payment_id::text, status, origin FROM operations WHERE id = $1`, paidID,
		).Scan(&paymentID, &status, &origin)
		require.NoError(t, err)
		assert.Nil(t, paymentID)
		assert.Equal(t, domain.StatusPaid, domain.OperationStatus(status))
		assert.Equal(t, domain.OriginPayment, domain.OperationOrigin(origin))
		// Непогашенные плановые снесены без вопросов — ревизия 2 не спрашивает
		// о судьбе просрочки (шторка 3249:75723 дословно).
		for _, opID := range allIDs {
			if opID == paidID {
				continue
			}
			var n int
			require.NoError(t, h.pool.QueryRow(h.ctx(),
				`SELECT COUNT(*) FROM operations WHERE id = $1`, opID).Scan(&n))
			assert.Equal(t, 0, n, "unpaid planned %s is gone", opID)
		}
	})

	t.Run("арендно-управляемое правило — ErrRentManagedPayment", func(t *testing.T) {
		t.Parallel()
		h := newPaymentsHarness(t).withOwner("Europe/Moscow")
		payID := h.seedPayment("2026-08-01", `{"kind":"monthly","daysOfMonth":[15]}`, false)
		rentalID, err := uuid.NewV7()
		require.NoError(t, err)
		_, err = h.pool.Exec(h.ctx(),
			`INSERT INTO rentals (id, owner_id, property_id, payment_id, start_date, utilities)
			 VALUES ($1, $2, $3, $4, '2026-08-01', 'included')`,
			rentalID, h.owner, h.propID, payID)
		require.NoError(t, err)
		err = h.svc.CompletePayment(h.ctx(), h.owner, h.propID, payID)
		assert.ErrorIs(t, err, paymentsapp.ErrRentManagedPayment, "err = %v", err)
	})
}

func TestCompletePayment_HiddenByDeletion(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	alive, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
	require.NoError(t, err)
	// Завершаемому правилу — чужая категория: по чипам поиска видно, что
	// категория исчезнувшего правила из агрегатов выпала.
	doomedCmd := h.completeCmd()
	doomedCmd.CategorySlug = testIntegrationSlugUtilities
	doomed, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, doomedCmd)
	require.NoError(t, err)
	h.payAllOperations(t, doomed.ID)
	// Личная звезда читателя на завершаемом правиле: падает вместе с правилом
	// (FK CASCADE) и не считается в счётчике хаба после завершения.
	_, err = h.svc.SetPaymentFavorite(h.ctx(), h.owner, h.propID, doomed.ID, true)
	require.NoError(t, err)

	require.NoError(t, h.svc.CompletePayment(h.ctx(), h.owner, h.propID, doomed.ID))

	// Звезда пала вместе с правилом.
	var stars int
	require.NoError(t, h.pool.QueryRow(h.ctx(),
		`SELECT COUNT(*) FROM user_payment_favorites WHERE payment_id = $1`, doomed.ID,
	).Scan(&stars))
	assert.Equal(t, 0, stars, "the favorite row cascades with the rule")

	// Объектный список показывает только живые правила.
	list, err := h.svc.ListPayments(h.ctx(), h.owner, h.propID, "")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, alive.ID, list[0].Payment.ID)

	// Поиск по названию объекта: завершённого нет в ленте, не в «найдено N».
	feed, err := h.global.ListGlobalPayments(h.ctx(), h.owner)
	require.NoError(t, err)
	require.Len(t, feed.Items, 1)
	assert.Equal(t, alive.ID, feed.Items[0].ID)
	assert.EqualValues(t, 0, feed.FavoriteCount,
		"the star fell with the completed rule — the hub counter does not count it")
	assert.EqualValues(t, 0, feed.OverdueOperationsCount)

	search, err := h.global.SearchGlobalPayments(h.ctx(), h.owner, "", paymentsapp.GlobalPaymentSearchPage{})
	require.NoError(t, err)
	assert.EqualValues(t, 1, search.Total)
	require.Len(t, search.Items, 1)
	assert.Equal(t, alive.ID, search.Items[0].ID)
	// Чипы категорий: «ЖКУ» исчезнувшего правила в агрегате не живёт.
	require.Len(t, search.MatchedCategories, 1)
	assert.NotNil(t, search.MatchedCategories[0].Slug)
	assert.Equal(t, "rent", *search.MatchedCategories[0].Slug)

	// Флаги списка: исчезнувшее правило в списках не живёт — карта флагов
	// отвечает только по живым, у живого флаги сняты.
	completed, managed, rentalCompleted, err := h.svc.PaymentFlags(h.ctx(), h.owner, h.propID)
	require.NoError(t, err)
	assert.Len(t, completed, 1)
	assert.Contains(t, completed, alive.ID)
	assert.False(t, completed[alive.ID])
	assert.Len(t, managed, 1)
	assert.False(t, managed[alive.ID])
	assert.Len(t, rentalCompleted, 1)
	assert.False(t, rentalCompleted[alive.ID])
}
