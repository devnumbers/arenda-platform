package domain

import (
	"testing"

	"github.com/google/uuid"
)

// Системные пауза/снятие архивного моста (тикет #1342): та же пара action-
// id, что у ручных строк (иконки и фильтры ленты не переучиваются), другой
// текст — причина системного флипа читается в самой строке, и отметка
// происхождения в контексте.
func TestPaymentArchivePauseRows(t *testing.T) {
	t.Parallel()

	id := uuid.Must(uuid.NewV7())

	paused := PaymentPausedByArchive(id, "Электричество")
	if paused.Action != ActionPaymentPaused {
		t.Errorf("action = %q, want %q", paused.Action, ActionPaymentPaused)
	}
	if got := paused.Segments.PlainText(); got != "Платёж поставлен на паузу (объект ушёл в архив): Электричество" {
		t.Errorf("paused header = %q", got)
	}
	if len(paused.Segments) != 2 || paused.Segments[1].Link == nil || paused.Segments[1].Link.Kind != KindPayment {
		t.Errorf("paused segments = %+v, want the linked title run", paused.Segments)
	}
	if v, ok := paused.Context["pause_origin"]; !ok || v != ctxPauseOriginAutoArchive {
		t.Errorf("paused context origin = %v, want auto_archive", v)
	}

	resumed := PaymentResumedByArchive(id, "Электричество")
	if resumed.Action != ActionPaymentResumed {
		t.Errorf("action = %q, want %q", resumed.Action, ActionPaymentResumed)
	}
	if got := resumed.Segments.PlainText(); got != "Платёж возобновлён (объект вернулся из архива): Электричество" {
		t.Errorf("resumed header = %q", got)
	}
	if v, ok := resumed.Context["pause_origin"]; !ok || v != ctxPauseOriginAutoArchive {
		t.Errorf("resumed context origin = %v, want auto_archive", v)
	}
}
