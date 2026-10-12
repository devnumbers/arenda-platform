//go:build integration

package application_test

// Завершение платежа (тикет #1340): явная мутация complete ставит хранимый
// признак completed_at, гейты доступности и read-only терминального
// состояния, скрытие из лент и подсчётов, пропуск тиком, живые чтения и
// удаление по общему праву редактирования (D9).

import (
	"testing"
	"time"

	"github.com/google/uuid"
	paymentspg "github.com/nambers/arenda-planform/apps/backend/internal/payments/adapters/postgres"
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

func TestCompletePayment_HappyPath(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	created := h.completableRule(t)

	completed, err := h.svc.CompletePayment(h.ctx(), h.owner, h.propID, created.ID)
	require.NoError(t, err)

	// The stored stamp rides the response and the direct read; the computed
	// settlement flag stays the read-side view.
	require.NotNil(t, completed.CompletedAt)
	got, err := h.svc.GetPayment(h.ctx(), h.owner, h.propID, created.ID)
	require.NoError(t, err)
	require.NotNil(t, got.CompletedAt)
	settled, err := h.svc.CompletedStatus(h.ctx(), h.owner, h.propID, created.ID)
	require.NoError(t, err)
	assert.True(t, settled)

	// Оплаченные операции не тронуты — «История платежа» жива.
	ops, err := h.ops.ListPaymentOperations(h.ctx(), h.owner, h.propID, created.ID,
		h.listCmd(nil, 50, 0, false))
	require.NoError(t, err)
	require.Len(t, ops, 1)
	assert.Equal(t, domain.StatusPaid, ops[0].Operation.Status)

	// Аудит: создание + завершение (оплата пишет своё action на операцию).
	actions := h.auditActions(t, created.ID)
	assert.Equal(t, []string{actionPaymentCreated, "payment.completed"}, actions)

	// «История действий»: строка завершения с базой «Завершение».
	rows := h.journalActions(t, "payment.completed")
	require.Len(t, rows, 1)

	// Журнал изменений: собственная строка с пустым дифом — форма паузы.
	entries := h.loadChangeLog(t, created.ID)
	require.Len(t, entries, 1)
	assert.Equal(t, "completed", entries[0].Action)
	assert.Empty(t, entries[0].Changes)

	// Realtime: фрейм ленты платежей, операций не трогали.
	assert.Contains(t, h.realtime.Pairs(), "payments:"+h.propID.String())
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

	t.Run("повторный complete — ErrAlreadyCompleted", func(t *testing.T) {
		t.Parallel()
		h := newPaymentsHarness(t).withOwner("Europe/Moscow")
		created := h.completableRule(t)
		_, err := h.svc.CompletePayment(h.ctx(), h.owner, h.propID, created.ID)
		require.NoError(t, err)
		_, err = h.svc.CompletePayment(h.ctx(), h.owner, h.propID, created.ID)
		assert.ErrorIs(t, err, paymentsapp.ErrAlreadyCompleted, "err = %v", err)
	})

	t.Run("открытая пауза — ErrAlreadyPaused, возобновление открывает путь", func(t *testing.T) {
		t.Parallel()
		h := newPaymentsHarness(t).withOwner("Europe/Moscow")
		created := h.completableRule(t)
		_, err := h.svc.PausePayment(h.ctx(), h.owner, h.propID, created.ID)
		require.NoError(t, err)
		_, err = h.svc.CompletePayment(h.ctx(), h.owner, h.propID, created.ID)
		require.ErrorIs(t, err, paymentsapp.ErrAlreadyPaused, "err = %v", err)
		_, err = h.svc.ResumePayment(h.ctx(), h.owner, h.propID, created.ID)
		require.NoError(t, err)
		_, err = h.svc.CompletePayment(h.ctx(), h.owner, h.propID, created.ID)
		require.NoError(t, err)
	})

	t.Run("есть planned — ErrPaymentNotCompletable", func(t *testing.T) {
		t.Parallel()
		h := newPaymentsHarness(t).withOwner("Europe/Moscow")
		created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.completeCmd())
		require.NoError(t, err)
		_, err = h.svc.CompletePayment(h.ctx(), h.owner, h.propID, created.ID)
		assert.ErrorIs(t, err, paymentsapp.ErrPaymentNotCompletable, "err = %v", err)
	})

	t.Run("endDate впереди при оплаченных вхождениях — ErrPaymentNotCompletable", func(t *testing.T) {
		t.Parallel()
		h := newPaymentsHarness(t).withOwner("Europe/Moscow")
		cmd := h.completeCmd()
		endDate := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
		cmd.EndDate = &endDate
		created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, cmd)
		require.NoError(t, err)
		// Вычисляемый флаг отвечает «завершён» уже сегодня — гейт мутации
		// ждёт конца правила.
		h.payAllOperations(t, created.ID)
		settled, err := h.svc.CompletedStatus(h.ctx(), h.owner, h.propID, created.ID)
		require.NoError(t, err)
		assert.True(t, settled)
		_, err = h.svc.CompletePayment(h.ctx(), h.owner, h.propID, created.ID)
		assert.ErrorIs(t, err, paymentsapp.ErrPaymentNotCompletable, "err = %v", err)
	})

	t.Run("бессрочное правило — ErrPaymentNotCompletable", func(t *testing.T) {
		t.Parallel()
		h := newPaymentsHarness(t).withOwner("Europe/Moscow")
		created, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
		require.NoError(t, err)
		h.payAllOperations(t, created.ID)
		_, err = h.svc.CompletePayment(h.ctx(), h.owner, h.propID, created.ID)
		assert.ErrorIs(t, err, paymentsapp.ErrPaymentNotCompletable, "err = %v", err)
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
		_, err = h.svc.CompletePayment(h.ctx(), h.owner, h.propID, payID)
		assert.ErrorIs(t, err, paymentsapp.ErrRentManagedPayment, "err = %v", err)
	})
}

func TestCompletePayment_ReadOnlyTerminalState(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	created := h.completableRule(t)
	_, err := h.svc.CompletePayment(h.ctx(), h.owner, h.propID, created.ID)
	require.NoError(t, err)

	// Pause/resume/update — 409; чтения и удаление живы (D9).
	_, err = h.svc.PausePayment(h.ctx(), h.owner, h.propID, created.ID)
	require.ErrorIs(t, err, paymentsapp.ErrPaymentCompleted, "pause err = %v", err)
	_, err = h.svc.ResumePayment(h.ctx(), h.owner, h.propID, created.ID)
	require.ErrorIs(t, err, paymentsapp.ErrPaymentCompleted, "resume err = %v", err)
	title := "Переименован"
	_, err = h.svc.UpdatePayment(h.ctx(), h.owner, h.propID, created.ID,
		paymentsapp.UpdatePaymentCommand{Title: &title})
	require.ErrorIs(t, err, paymentsapp.ErrPaymentCompleted, "update err = %v", err)

	// Прямые чтения живы: деталь несёт штамп, история изменений читается.
	got, err := h.svc.GetPayment(h.ctx(), h.owner, h.propID, created.ID)
	require.NoError(t, err)
	require.NotNil(t, got.CompletedAt)
	page, err := h.svc.PaymentChanges(h.ctx(), h.owner, h.propID, created.ID, paymentsapp.ChangesQuery{})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	assert.Equal(t, domain.ChangeCompleted, page.Items[0].Action)

	// Чтения не ломаются звездой читателя: звезда ставится, чтения отвечают.
	_, err = h.svc.SetPaymentFavorite(h.ctx(), h.owner, h.propID, created.ID, true)
	require.NoError(t, err)
	_, err = h.svc.GetPayment(h.ctx(), h.owner, h.propID, created.ID)
	require.NoError(t, err)

	// Удаление — по общему праву редактирования; после удаления правило
	// исчезает и из прямого чтения.
	require.NoError(t, h.svc.DeletePayment(h.ctx(), h.owner, h.propID, created.ID, true))
	_, err = h.svc.GetPayment(h.ctx(), h.owner, h.propID, created.ID)
	assert.ErrorIs(t, err, paymentsapp.ErrNotFound, "err = %v", err)
}

func TestCompletePayment_TickSkipsCompleted(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	// Завершённое бессрочное правило, засеянное напрямую: тик его пропускает
	// — операций не порождает (гейт SQL, тикет #1340).
	payID := h.seedPayment("2026-08-01", `{"kind":"daily"}`, false)
	_, err := h.pool.Exec(h.ctx(),
		`UPDATE payments SET completed_at = now() WHERE id = $1`, payID)
	require.NoError(t, err)

	h.runTick()

	assert.Empty(t, h.operationsOf(payID))

	// Владелец, чьи правила все завершены, не является целью часового свипа.
	zones, err := paymentspg.NewTickZoneDirectory(h.pool).ListTickZones(h.ctx())
	require.NoError(t, err)
	for _, zone := range zones {
		for _, owner := range zone.Owners {
			if owner == h.owner {
				t.Fatalf("owner %s with only completed rules is a sweep target", h.owner)
			}
		}
	}
}

func TestCompletePayment_HiddenFromFeedsAndCounts(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	alive, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, h.createCmd())
	require.NoError(t, err)
	// Завершённому правилу — чужая категория: по чипам поиска видно, что
	// категория невидимого правила из агрегатов выпала.
	doomedCmd := h.completeCmd()
	doomedCmd.CategorySlug = testIntegrationSlugUtilities
	doomed, err := h.svc.CreatePayment(h.ctx(), h.owner, h.propID, doomedCmd)
	require.NoError(t, err)
	h.payAllOperations(t, doomed.ID)
	_, err = h.svc.CompletePayment(h.ctx(), h.owner, h.propID, doomed.ID)
	require.NoError(t, err)
	// Личная звезда читателя на завершённом правиле не ломает чтений — и не
	// считается в счётчике хаба.
	_, err = h.svc.SetPaymentFavorite(h.ctx(), h.owner, h.propID, doomed.ID, true)
	require.NoError(t, err)

	// Объектный список показывает только живые правила.
	list, err := h.svc.ListPayments(h.ctx(), h.owner, h.propID, "")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, alive.ID, list[0].Payment.ID)

	// Поиск по названию объекта: завершённое не в ленте, не в «найдено N».
	feed, err := h.global.ListGlobalPayments(h.ctx(), h.owner)
	require.NoError(t, err)
	require.Len(t, feed.Items, 1)
	assert.Equal(t, alive.ID, feed.Items[0].ID)
	assert.EqualValues(t, 0, feed.FavoriteCount,
		"the star on a completed rule never lands in the hub counter")
	assert.EqualValues(t, 0, feed.OverdueOperationsCount)

	search, err := h.global.SearchGlobalPayments(h.ctx(), h.owner, "", paymentsapp.GlobalPaymentSearchPage{})
	require.NoError(t, err)
	assert.EqualValues(t, 1, search.Total)
	require.Len(t, search.Items, 1)
	assert.Equal(t, alive.ID, search.Items[0].ID)
	// Чипы категорий: «ЖКУ» завершённого правила в агрегате не живёт.
	require.Len(t, search.MatchedCategories, 1)
	assert.NotNil(t, search.MatchedCategories[0].Slug)
	assert.Equal(t, "rent", *search.MatchedCategories[0].Slug)

	// Флаги списка: завершённое правило в списках не живёт — карта флагов
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
