//go:build integration

package application_test

// The archive bridge (тикет #1342): the system auto_archive pause the
// properties context places on a property's live rules when the object
// enters the archive, and the reverse leg closing only the system pauses
// when the object returns. The matrix per the owner's resolution: every
// live rule pauses — rent-managed included (the pause rides the object's
// lifecycle, ADR 0053 §3), completed rules are terminal and get nothing,
// an open manual pause outranks the round-trip; the resume leg closes only
// its own auto_archive pauses and journal rows (both logs) carry the
// origin mark. The cycle archives and returns the same object twice.

import (
	"testing"

	"github.com/google/uuid"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pauseRows reads the rule's pause intervals through the service read — the
// assertion sees what any later read (the page, the tick) would see.
func pauseRows(t *testing.T, h *paymentsHarness, paymentID uuid.UUID) []domain.PauseInterval {
	t.Helper()
	payment, err := h.svc.GetPayment(h.ctx(), h.owner, h.propID, paymentID)
	require.NoError(t, err)
	return payment.Pauses
}

// openPauseOrigin reports the open pause's origin ("" when none).
func openPauseOrigin(t *testing.T, h *paymentsHarness, paymentID uuid.UUID) domain.PauseOrigin {
	t.Helper()
	active, has := domain.ActivePause(pauseRows(t, h, paymentID))
	if !has {
		return ""
	}
	return active.Origin
}

// changeRows reads the rule's change log through the service read.
func changeRows(t *testing.T, h *paymentsHarness, paymentID uuid.UUID) []domain.ChangeEntry {
	t.Helper()
	page, err := h.svc.PaymentChanges(h.ctx(), h.owner, h.propID, paymentID, paymentsapp.ChangesQuery{})
	require.NoError(t, err)
	return page.Items
}

// archivePause drives the bridge's pause leg the way the properties context
// does: inside its own transaction, the owner as the actor.
func archivePause(t *testing.T, h *paymentsHarness) {
	t.Helper()
	require.NoError(t, h.inTx(func(tx transaction.Tx) error {
		return h.bridge.PauseAllForArchive(h.ctx(), tx, h.owner, h.propID, h.owner, sharedpolicy.RoleOwner)
	}))
}

// archiveResume drives the bridge's resume leg the same way.
func archiveResume(t *testing.T, h *paymentsHarness) {
	t.Helper()
	require.NoError(t, h.inTx(func(tx transaction.Tx) error {
		return h.bridge.ResumeArchivePauses(h.ctx(), tx, h.owner, h.propID, h.owner, sharedpolicy.RoleOwner)
	}))
}

func TestArchivePauseBridge_PausesAllLiveRules(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	active := h.seedNamedPayment(t, "Электричество")
	manuallyPaused := h.seedNamedPayment(t, "Ручная пауза")
	h.seedPause(manuallyPaused, "2026-08-01")
	rentManaged := h.seedNamedPayment(t, "Аренда")
	rentalID, err := uuid.NewV7()
	require.NoError(t, err)
	_, err = h.pool.Exec(h.ctx(),
		`INSERT INTO rentals (id, owner_id, property_id, payment_id, start_date, utilities)
		 VALUES ($1, $2, $3, $4, '2026-08-01', 'included')`,
		rentalID, h.owner, h.propID, rentManaged)
	require.NoError(t, err)
	completed := h.seedNamedPayment(t, "Завершён")
	_, err = h.pool.Exec(h.ctx(), `UPDATE payments SET completed_at = now() WHERE id = $1`, completed)
	require.NoError(t, err)

	archivePause(t, h)

	// Активное и арендно-управляемое правила — системная пауза.
	assert.Equal(t, domain.PauseOriginAutoArchive, openPauseOrigin(t, h, active))
	assert.Equal(t, domain.PauseOriginAutoArchive, openPauseOrigin(t, h, rentManaged),
		"rent-managed rule pauses with the object (ADR 0053 §3)")
	// Ручная пауза остаётся ручной.
	assert.Equal(t, domain.PauseOriginManual, openPauseOrigin(t, h, manuallyPaused))
	// Завершённое правило паузы не получает.
	assert.Equal(t, domain.PauseOrigin(""), openPauseOrigin(t, h, completed))

	// Журнал изменений: системная строка с origin, у ручной паузы и
	// завершённого правила — ничего.
	entries := changeRows(t, h, active)
	require.Len(t, entries, 1)
	assert.Equal(t, domain.ChangePaused, entries[0].Action)
	assert.Equal(t, domain.PauseOriginAutoArchive, entries[0].Origin)
	assert.Empty(t, entries[0].Changes)
	assert.Empty(t, changeRows(t, h, manuallyPaused), "the manual pause writes no second row")
	assert.Empty(t, changeRows(t, h, completed))

	// «История действий»: строка с именованной причиной для каждого
	// поставленного на паузу правила, с отметкой происхождения в контексте.
	rows := h.actionJournalRows(t, "Электричество")
	require.Len(t, rows, 1)
	assert.Contains(t, rows[0].searchable, "Платёж поставлен на паузу (объект ушёл в архив): Электричество")
	assert.Equal(t, "auto_archive", rows[0].origin)
	assert.Empty(t, h.actionJournalRows(t, "Завершён"))
}

func TestArchivePauseBridge_ResumeClosesOnlyAuto(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	auto := h.seedNamedPayment(t, "Электричество")
	manual := h.seedNamedPayment(t, "Ручная пауза")
	h.seedPause(manual, "2026-08-01")

	archivePause(t, h)
	archiveResume(t, h)

	// Системная пауза закрыта сегодняшним (владельческим) днём.
	payment, err := h.svc.GetPayment(h.ctx(), h.owner, h.propID, auto)
	require.NoError(t, err)
	require.Len(t, payment.Pauses, 1)
	require.NotNil(t, payment.Pauses[0].To)
	assert.Equal(t, "2026-08-25", payment.Pauses[0].To.Format("2006-01-02"))
	assert.Equal(t, domain.PauseOriginAutoArchive, payment.Pauses[0].Origin)
	// Ручная пауза открыта и после возврата объекта.
	assert.Equal(t, domain.PauseOriginManual, openPauseOrigin(t, h, manual))

	// Журнал изменений: пара paused→resumed с origin у обеих.
	entries := changeRows(t, h, auto)
	require.Len(t, entries, 2)
	assert.Equal(t, domain.ChangeResumed, entries[0].Action, "newest first")
	assert.Equal(t, domain.PauseOriginAutoArchive, entries[0].Origin)
	assert.Equal(t, domain.ChangePaused, entries[1].Action)
	assert.Equal(t, domain.PauseOriginAutoArchive, entries[1].Origin)

	// «История действий»: строка возобновления с причиной.
	rows := h.actionJournalRows(t, "Электричество")
	require.Len(t, rows, 2)
	assert.Contains(t, rows[0].searchable, "Платёж возобновлён (объект вернулся из архива): Электричество")
	assert.Equal(t, "auto_archive", rows[0].origin)
}

func TestArchivePauseBridge_CycleTwice(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	pay := h.seedNamedPayment(t, "Электричество")

	archivePause(t, h)
	archiveResume(t, h)
	// Повторный цикл архивация/возврат не спотыкается об уже закрытые
	// интервалы и не плодит дублей: каждый цикл оставляет одну пару.
	archivePause(t, h)
	assert.Equal(t, domain.PauseOriginAutoArchive, openPauseOrigin(t, h, pay))
	archiveResume(t, h)

	payment, err := h.svc.GetPayment(h.ctx(), h.owner, h.propID, pay)
	require.NoError(t, err)
	assert.Len(t, payment.Pauses, 2, "каждый цикл оставляет один закрытый интервал")
	entries := changeRows(t, h, pay)
	assert.Len(t, entries, 4, "paused→resumed ×2")
	assert.Equal(t, domain.PauseOriginAutoArchive, entries[0].Origin)
}

func TestArchivePauseBridge_ResumeLegIsNoopWithoutAutoPauses(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	manual := h.seedNamedPayment(t, "Ручная пауза")
	h.seedPause(manual, "2026-08-01")

	// Возврат объекта без архивной автопаузы не пишет ничего: строки моста
	// — только для собственных auto_archive-пауз.
	archiveResume(t, h)
	assert.Equal(t, domain.PauseOriginManual, openPauseOrigin(t, h, manual))
	assert.Empty(t, changeRows(t, h, manual))
}
