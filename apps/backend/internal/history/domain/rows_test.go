package domain

// The new row template (карта #1324, решение #1317 §2): the header is the
// action with the linked entity («Название объекта изменено: [новое]»), the
// diffs moved from the header into the details — the ready gray lines built
// at write time and rendered by the frontend verbatim. Rows recorded before
// the template carry no details (nil) — the frontend renders them as the
// header alone. Точные формулировки каталога утверждает walkthrough экранов;
// здешние тесты закрепляют механику: ссылка в заголовке, диффы и периоды —
// в деталях, детали без ссылок и без сумм (§5).

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func at2(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// assertNoLinksInDetails pins the detail canon: the gray lines render
// verbatim — no links inside, the row's single link lives in the header.
func assertNoLinksInDetails(t *testing.T, e Entry) {
	t.Helper()
	for _, line := range e.Details {
		if strings.Contains(line, "http") || strings.Contains(line, string(byte(0))) {
			t.Errorf("detail line %q must not carry a link", line)
		}
	}
}

func TestPropertyRenamed_DetailsCarryTheDiff(t *testing.T) {
	t.Parallel()
	id := uuid.Must(uuid.NewV7())
	e := PropertyRenamed(id, "Old Name", "Новое имя")

	want := "Название объекта изменено: Новое имя"
	if got := e.Segments.PlainText(); got != want {
		t.Errorf("header = %q, want %q", got, want)
	}
	if len(e.Segments) != 2 || e.Segments[1].Link == nil ||
		e.Segments[1].Link.Kind != KindProperty || e.Segments[1].Link.ID != id {
		t.Errorf("segments = %+v, want the linked new-name run", e.Segments)
	}
	wantDetails := DetailLines{`Название: «Old Name» → «Новое имя»`}
	if len(e.Details) != 1 || e.Details[0] != wantDetails[0] {
		t.Errorf("details = %q, want %q", e.Details, wantDetails)
	}
	assertNoLinksInDetails(t, e)
	if v, ok := e.Context["old_name"]; !ok || v != "Old Name" {
		t.Errorf("context old_name = %v, want the snapshot", v)
	}
}

func TestPropertyAddressChanged_DetailsByBranch(t *testing.T) {
	t.Parallel()
	id := uuid.Must(uuid.NewV7())

	t.Run("addition", func(t *testing.T) {
		t.Parallel()
		e := PropertyAddressChanged(id, "", "Москва, Тверская 1")
		if got := e.Segments.PlainText(); got != "Добавлен адрес объекта: Москва, Тверская 1" {
			t.Errorf("header = %q", got)
		}
		want := DetailLines{`Адрес: «Москва, Тверская 1»`}
		if len(e.Details) != 1 || e.Details[0] != want[0] {
			t.Errorf("details = %q, want %q", e.Details, want)
		}
		assertNoLinksInDetails(t, e)
	})
	t.Run("removal", func(t *testing.T) {
		t.Parallel()
		e := PropertyAddressChanged(id, "Москва, Тверская 1", "")
		if got := e.Segments.PlainText(); got != "Удалён адрес объекта" {
			t.Errorf("header = %q", got)
		}
		want := DetailLines{"Адрес удалён"}
		if len(e.Details) != 1 || e.Details[0] != want[0] {
			t.Errorf("details = %q, want %q", e.Details, want)
		}
	})
	t.Run("change", func(t *testing.T) {
		t.Parallel()
		e := PropertyAddressChanged(id, "Старая", "Новая")
		if got := e.Segments.PlainText(); got != "Адрес объекта изменён: Новая" {
			t.Errorf("header = %q", got)
		}
		want := DetailLines{`Адрес: «Старая» → «Новая»`}
		if len(e.Details) != 1 || e.Details[0] != want[0] {
			t.Errorf("details = %q, want %q", e.Details, want)
		}
		assertNoLinksInDetails(t, e)
	})
}

func TestPropertyAttributesChanged_LinePerAttribute(t *testing.T) {
	t.Parallel()
	id := uuid.Must(uuid.NewV7())
	changes := []AttributeChange{
		{Key: "balcony", Label: "Балкон", Old: "none", New: "balcony"},
		{Key: "floor", Label: "Этаж", Old: nil, New: 5.0},
	}
	e := PropertyAttributesChanged(id, changes)

	if got := e.Segments.PlainText(); got != "Изменены характеристики объекта" {
		t.Errorf("header = %q", got)
	}
	want := DetailLines{`Балкон: «none» → «balcony»`, `Этаж: «5»`}
	if len(e.Details) != 2 || e.Details[0] != want[0] || e.Details[1] != want[1] {
		t.Errorf("details = %q, want %q", e.Details, want)
	}
	assertNoLinksInDetails(t, e)
	// The context keeps the raw changes (the future read model), not the
	// detail lines.
	if _, ok := e.Context["attributes"]; !ok {
		t.Error("context must keep the raw attribute changes")
	}
}

func TestPropertyMaintenanceRows(t *testing.T) {
	t.Parallel()
	id := uuid.Must(uuid.NewV7())

	started := PropertyMaintenanceStarted(id)
	if got := started.Segments.PlainText(); got != "Объект на ремонте" {
		t.Errorf("started header = %q", got)
	}
	if started.Action != ActionPropertyMaintenanceStarted || started.BaseAction != BaseChanged {
		t.Errorf("started action/base = %s/%s, want maintenance_started/changed", started.Action, started.BaseAction)
	}
	if len(started.Segments) != 1 || started.Segments[0].Link == nil ||
		started.Segments[0].Link.Kind != KindProperty || started.Segments[0].Link.ID != id {
		t.Errorf("started segments = %+v, want the single linked run", started.Segments)
	}
	if started.Details != nil {
		t.Errorf("started details = %q, want nil", started.Details)
	}

	finished := PropertyMaintenanceFinished(id)
	if got := finished.Segments.PlainText(); got != "Ремонт завершён" {
		t.Errorf("finished header = %q", got)
	}
	if finished.Action != ActionPropertyMaintenanceFinished || finished.BaseAction != BaseCompleted {
		t.Errorf("finished action/base = %s/%s, want maintenance_finished/completed", finished.Action, finished.BaseAction)
	}
	if finished.Details != nil {
		t.Errorf("finished details = %q, want nil", finished.Details)
	}
}

// Константы повторяющихся ожиданий таблиц аренды (goconst).
const (
	wantRentalHeaderCreated = "Добавлена аренда: 11 месяцев"
	wantTenantLine          = `Арендатор: «Максим»`
	wantPeriodLine          = "Период: с 1 ноября 2026 по 30 сентября 2027"
)

func TestRentalCreated_OpenEndedReadsTheDate(t *testing.T) {
	t.Parallel()
	id := uuid.Must(uuid.NewV7())
	e := RentalCreated(id, "Максим", at2(2026, time.November, 1), time.Time{})

	if got := e.Segments.PlainText(); got != "Добавлена аренда: 1 ноября" {
		t.Errorf("header = %q, want %q", got, "Добавлена аренда: 1 ноября")
	}
	if len(e.Segments) != 2 || e.Segments[1].Link == nil ||
		e.Segments[1].Link.Kind != KindRental || e.Segments[1].Link.ID != id {
		t.Errorf("segments = %+v, want the linked date run", e.Segments)
	}
	want := DetailLines{wantTenantLine, "Период: с 1 ноября 2026"}
	if len(e.Details) != 2 || e.Details[0] != want[0] || e.Details[1] != want[1] {
		t.Errorf("details = %q, want %q", e.Details, want)
	}
	assertNoLinksInDetails(t, e)
	if v, ok := e.Context["period_from"]; !ok || v != "2026-11-01T00:00:00Z" {
		t.Errorf("context period_from = %v, want the RFC3339 snapshot", v)
	}
}

func TestRentalCreated_DatedReadsTheTerm(t *testing.T) {
	t.Parallel()
	id := uuid.Must(uuid.NewV7())
	e := RentalCreated(id, "Максим", at2(2026, time.November, 1), at2(2027, time.September, 30))

	if got := e.Segments.PlainText(); got != wantRentalHeaderCreated {
		t.Errorf("header = %q, want %q", got, wantRentalHeaderCreated)
	}
	want := DetailLines{wantTenantLine, wantPeriodLine}
	if len(e.Details) != 2 || e.Details[0] != want[0] || e.Details[1] != want[1] {
		t.Errorf("details = %q, want %q", e.Details, want)
	}
	assertNoLinksInDetails(t, e)
}

func TestRentalCreated_UnnamedTenantKeepsHeader(t *testing.T) {
	t.Parallel()
	// Безымянная аренда не влияет на заголовок — строка «Арендатор» просто
	// не пишется.
	id := uuid.Must(uuid.NewV7())
	e := RentalCreated(id, "", at2(2026, time.November, 1), at2(2027, time.September, 30))

	if got := e.Segments.PlainText(); got != wantRentalHeaderCreated {
		t.Errorf("header = %q, want %q", got, wantRentalHeaderCreated)
	}
	want := DetailLines{wantPeriodLine}
	if len(e.Details) != 1 || e.Details[0] != want[0] {
		t.Errorf("details = %q, want %q", e.Details, want)
	}
	// The context keeps the snapshot as is — the empty label included.
	if v, ok := e.Context["tenant"]; !ok || v != "" {
		t.Errorf("context tenant = %v, want the empty snapshot", v)
	}
}

func TestRentalUpdated_SymmetricToCreated(t *testing.T) {
	t.Parallel()
	id := uuid.Must(uuid.NewV7())
	e := RentalUpdated(id, "Максим", at2(2026, time.November, 1), at2(2027, time.September, 30))
	if got := e.Segments.PlainText(); got != "Условия аренды изменены: 11 месяцев" {
		t.Errorf("header = %q", got)
	}
	if e.BaseAction != BaseChanged || e.Action != ActionRentalUpdated {
		t.Errorf("action/base = %s/%s", e.Action, e.BaseAction)
	}
	want := DetailLines{wantTenantLine, wantPeriodLine}
	if len(e.Details) != 2 || e.Details[0] != want[0] || e.Details[1] != want[1] {
		t.Errorf("details = %q, want %q", e.Details, want)
	}
}

func TestPaymentUpdated_DetailsCarryTheTitleDiff(t *testing.T) {
	t.Parallel()
	id := uuid.Must(uuid.NewV7())
	e := PaymentUpdated(id, "Электричество", "Электричество и вода")

	if got := e.Segments.PlainText(); got != "Платёж изменён: Электричество и вода" {
		t.Errorf("header = %q", got)
	}
	if len(e.Segments) != 2 || e.Segments[1].Link == nil || e.Segments[1].Link.Kind != KindPayment {
		t.Errorf("segments = %+v, want the linked new-title run", e.Segments)
	}
	want := DetailLines{`Название: «Электричество» → «Электричество и вода»`}
	if len(e.Details) != 1 || e.Details[0] != want[0] {
		t.Errorf("details = %q, want %q", e.Details, want)
	}
	assertNoLinksInDetails(t, e)
	if v, ok := e.Context["old_title"]; !ok || v != "Электричество" {
		t.Errorf("context old_title = %v, want the snapshot", v)
	}
}

func TestLegacyRows_KeepNoDetails(t *testing.T) {
	t.Parallel()
	// Actions outside the new template's first wave record no details —
	// the wire field stays absent and the screens render the header alone.
	withoutDetails := []Entry{
		PropertyCreated(uuid.Must(uuid.NewV7()), "Квартира"),
		PropertyDescriptionChanged(uuid.Must(uuid.NewV7()), "было", "стало"),
		PropertyUpdated(uuid.Must(uuid.NewV7()), []string{"название", "адрес"}),
		PropertyPhotoAdded(uuid.Must(uuid.NewV7())),
		PropertyPinned(uuid.Must(uuid.NewV7())),
		PropertyArchived(uuid.Must(uuid.NewV7())),
		RentalCompleted(uuid.Must(uuid.NewV7()), "Максим"),
		RentalDeleted(uuid.Must(uuid.NewV7()), "Максим"),
		PaymentCreated(uuid.Must(uuid.NewV7()), "Электричество"),
		PaymentDeleted(uuid.Must(uuid.NewV7()), "Электричество"),
		OperationCreated(uuid.Must(uuid.NewV7()), "Аренда", at2(2026, time.November, 1)),
		ContactCreated(uuid.Must(uuid.NewV7()), "Пётр Иванов"),
		ContactUpdated(uuid.Must(uuid.NewV7()), "Пётр", "Пётр Иванов"),
		TaskRuleCreated(uuid.Must(uuid.NewV7()), "Задача"),
		TaskCompleted(uuid.Must(uuid.NewV7()), "Задача", at2(2026, time.November, 1)),
		MemberAdded(uuid.Must(uuid.NewV7()), "Иван Иванов"),
		MemberRoleChanged(uuid.Must(uuid.NewV7()), "Иван Иванов", ActorRoleViewer, ActorRoleFullAccess),
		MemberRemoved("Иван Иванов"),
	}
	for _, e := range withoutDetails {
		if e.Details != nil {
			t.Errorf("%s details = %q, want nil", e.Action, e.Details)
		}
	}
}

func TestRentalRows_EmptyTenantLabel(t *testing.T) {
	t.Parallel()
	// Новый шаблон: заголовок читает дату/срок и не зависит от имени
	// арендатора — безымянная аренда не оставляет висячей двоеточия и не
	// роняет ссылку (она стоит на значении заголовка).
	id := uuid.Must(uuid.NewV7())
	from := at2(2026, time.November, 1)
	to := at2(2027, time.September, 30)

	for _, tc := range []struct {
		name string
		e    Entry
		text string
	}{
		{"created", RentalCreated(id, "", from, to), "Добавлена аренда: 11 месяцев"},
		{"updated", RentalUpdated(id, "", from, to), "Условия аренды изменены: 11 месяцев"},
		{"completed", RentalCompleted(id, ""), "Аренда завершена"},
		{"deleted", RentalDeleted(id, ""), "Аренда удалена"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.e.Segments.PlainText()
			if got != tc.text {
				t.Errorf("row text = %q, want %q", got, tc.text)
			}
			if strings.Contains(got, "  ") {
				t.Errorf("row text %q has a double space", got)
			}
			if v, ok := tc.e.Context["tenant"]; !ok || v != "" {
				t.Errorf("context tenant = %v, want the empty snapshot", v)
			}
		})
	}
}

func TestRentalRows_PeriodLivesInContextOnly(t *testing.T) {
	t.Parallel()
	// Аудит #876 (переиздан D3): период в заголовок не пишется голой датой —
	// заголовок читает срок словами, период с годами живёт в деталях, а
	// структурный снимок — в context (RFC3339).
	from := at2(2026, time.September, 1)
	e := RentalCreated(uuid.Must(uuid.NewV7()), "Иван Иванов", from, time.Time{})
	want := "Добавлена аренда: 1 сентября"
	if got := e.Segments.PlainText(); got != want {
		t.Errorf("row text = %q, want %q", got, want)
	}
	if v, ok := e.Context["period_from"]; !ok || v != "2026-09-01T00:00:00Z" {
		t.Errorf("context period_from = %v, want the RFC3339 snapshot", v)
	}
	// Детали несут период с годом — готовой строкой.
	wantPeriod := "Период: с 1 сентября 2026"
	if len(e.Details) != 2 || e.Details[1] != wantPeriod {
		t.Errorf("details = %q, want the tenant line plus %q", e.Details, wantPeriod)
	}
}

func TestContactRebindRows(t *testing.T) {
	t.Parallel()
	// The rebind family of the dictionary (ADR 0061 §4, тикет #856): a
	// cross-property move speaks of both ends — «на другой объект» from the
	// source, «с другого объекта» at the destination — the bind and the
	// unbind own a single end. All four are «changed», carry the linked ФИО
	// snapshot (never the phone, §5) and the name in the context.
	id := uuid.Must(uuid.NewV7())
	cases := []struct {
		name string
		e    Entry
		text string
	}{
		{"moved from the source", ContactMovedFrom(id, "Пётр Иванов"), "Контакт перенесён на другой объект: Пётр Иванов"},
		{"moved to the destination", ContactMovedTo(id, "Пётр Иванов"), "Контакт перенесён с другого объекта: Пётр Иванов"},
		{"bound", ContactBound(id, "Пётр Иванов"), "Контакт привязан к объекту: Пётр Иванов"},
		{"unbound", ContactUnbound(id, "Пётр Иванов"), "Контакт отвязан от объекта: Пётр Иванов"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.e.Segments.PlainText(); got != tc.text {
				t.Errorf("row text = %q, want %q", got, tc.text)
			}
			if tc.e.Kind != KindContact || tc.e.BaseAction != BaseChanged {
				t.Errorf("kind/base = %s/%s, want contact/changed", tc.e.Kind, tc.e.BaseAction)
			}
			if len(tc.e.Segments) != 2 || tc.e.Segments[1].Link == nil ||
				tc.e.Segments[1].Link.Kind != KindContact || tc.e.Segments[1].Link.ID != id {
				t.Errorf("segments = %+v, want the linked card run", tc.e.Segments)
			}
			if v, ok := tc.e.Context["name"]; !ok || v != "Пётр Иванов" {
				t.Errorf("context name = %v, want the ФИО snapshot", v)
			}
		})
	}
}
