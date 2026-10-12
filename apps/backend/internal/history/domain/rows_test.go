package domain

// Единые change-строки (решение владельца 12.10, карта #1324; аменда D1):
// заголовок — «<Сущность> изменён(а): [значение-ссылка]» со снапшотом нового
// значения, диффы — по серой строке на каждое изменившееся поле, полных
// реальных значений (PII и суммы — аменда §5). Полевых заголовков больше нет:
// гранулярные билдеры объекта снесены, их идентификаторы остаются словарём
// легаси-строк. Точные формулировки каталога утверждает walkthrough экранов;
// здешние тесты закрепляют механику: ссылка в заголовке, диффы — в деталях,
// детали без ссылок.

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func at2(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// Константы повторяющихся ожиданий таблиц (goconst).
const (
	wantRentalHeaderCreated = "Добавлена аренда: 11 месяцев"
	detailLabelAddress      = "Адрес"
	detailLabelName         = "Название"
)

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

func TestPropertyUpdated_UnifiedHeaderAndDetails(t *testing.T) {
	t.Parallel()
	id := uuid.Must(uuid.NewV7())
	changes := []AttributeChange{
		{Key: ctxKeyName, Label: detailLabelName, Old: "Моя квартира", New: "Квартира №124"},
	}
	e := PropertyUpdated(id, "Квартира №124", changes)

	if got := e.Segments.PlainText(); got != "Объект изменён: Квартира №124" {
		t.Errorf("header = %q, want %q", got, "Объект изменён: Квартира №124")
	}
	if e.Action != ActionPropertyUpdated || e.BaseAction != BaseChanged {
		t.Errorf("action/base = %s/%s, want property.updated/changed", e.Action, e.BaseAction)
	}
	if len(e.Segments) != 2 || e.Segments[1].Link == nil ||
		e.Segments[1].Link.Kind != KindProperty || e.Segments[1].Link.ID != id {
		t.Errorf("segments = %+v, want the linked new-name run", e.Segments)
	}
	want := DetailLines{`Название: «Моя квартира» → «Квартира №124»`}
	if len(e.Details) != 1 || e.Details[0] != want[0] {
		t.Errorf("details = %q, want %q", e.Details, want)
	}
	assertNoLinksInDetails(t, e)
	if _, ok := e.Context["attributes"]; !ok {
		t.Error("context must keep the raw changes")
	}
}

func TestPropertyUpdated_LinePerField(t *testing.T) {
	t.Parallel()
	// Один ручной PATCH — одна строка: по пояснению на каждое изменившееся
	// поле, в порядке вызова (название, адрес, описание, тип, характеристики).
	id := uuid.Must(uuid.NewV7())
	changes := []AttributeChange{
		{Key: ctxKeyName, Label: detailLabelName, Old: "Старое", New: "Новое"},
		{Key: "address", Label: "Адрес", Old: "Ленина 5", New: "Ленина 7"},
		{Key: "description", Label: "Описание", Old: "было", New: "стало"},
		{Key: "type", Label: "Тип", Old: "Квартира", New: "Дом"},
		{Key: "area", Label: "Общая площадь", Old: "45,5 м²", New: "50 м²"},
	}
	e := PropertyUpdated(id, "Новое", changes)

	want := DetailLines{
		`Название: «Старое» → «Новое»`,
		`Адрес: «Ленина 5» → «Ленина 7»`,
		`Описание: «было» → «стало»`,
		`Тип: «Квартира» → «Дом»`,
		`Общая площадь: «45,5 м²» → «50 м²»`,
	}
	if len(e.Details) != len(want) {
		t.Fatalf("details = %q, want %q", e.Details, want)
	}
	for i := range want {
		if e.Details[i] != want[i] {
			t.Errorf("details[%d] = %q, want %q", i, e.Details[i], want[i])
		}
	}
	assertNoLinksInDetails(t, e)
}

func TestPropertyUpdated_AdditionsAndRemovals(t *testing.T) {
	t.Parallel()
	id := uuid.Must(uuid.NewV7())
	changes := []AttributeChange{
		{Key: "address", Label: detailLabelAddress, Old: nil, New: "Москва, Тверская 1"},
		{Key: "address2", Label: detailLabelAddress, Old: "Москва, Тверская 1", New: ""},
		{Key: "area", Label: "Общая площадь", Old: "45,5 м²", New: nil},
	}
	e := PropertyUpdated(id, "Квартира", changes)

	want := DetailLines{
		`Адрес: «Москва, Тверская 1»`,
		"Адрес: удалено",
		"Общая площадь: удалено",
	}
	if len(e.Details) != len(want) {
		t.Fatalf("details = %q, want %q", e.Details, want)
	}
	for i := range want {
		if e.Details[i] != want[i] {
			t.Errorf("details[%d] = %q, want %q", i, e.Details[i], want[i])
		}
	}
}

func TestPropertyPhotosChanged_FoldedIntoChangeRow(t *testing.T) {
	t.Parallel()
	// Фото свёрнуты в единую строку изменения (решение владельца 12.10):
	// «Объект изменён: [имя]» + «Фото: добавлено / удалено» — отдельных
	// фото-строк и их групп ➕/🗑 больше нет.
	id := uuid.Must(uuid.NewV7())

	added := PropertyPhotosChanged(id, "Квартира", true)
	if got := added.Segments.PlainText(); got != "Объект изменён: Квартира" {
		t.Errorf("added header = %q", got)
	}
	if added.Action != ActionPropertyUpdated || added.BaseAction != BaseChanged {
		t.Errorf("added action/base = %s/%s, want property.updated/changed", added.Action, added.BaseAction)
	}
	want := DetailLines{"Фото: добавлено"}
	if len(added.Details) != 1 || added.Details[0] != want[0] {
		t.Errorf("added details = %q, want %q", added.Details, want)
	}
	assertNoLinksInDetails(t, added)

	removed := PropertyPhotosChanged(id, "Квартира", false)
	if got := removed.Segments.PlainText(); got != "Объект изменён: Квартира" {
		t.Errorf("removed header = %q", got)
	}
	want = DetailLines{"Фото: удалено"}
	if len(removed.Details) != 1 || removed.Details[0] != want[0] {
		t.Errorf("removed details = %q, want %q", removed.Details, want)
	}
}

// Константы повторяющихся ожиданий таблиц аренды (goconst).
const (
	wantTenantLine = `Арендатор: «Максим»`
	wantPeriodLine = "Период: с 1 ноября 2026 по 30 сентября 2027"
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

func TestRentalUpdated_CarriesTheDiffs(t *testing.T) {
	t.Parallel()
	// Правка условий: заголовок читает НОВЫЙ срок, пояснения — диффы
	// арендатора и периода (период в кавычках, формат D3 внутри).
	id := uuid.Must(uuid.NewV7())
	e := RentalUpdated(id,
		"Иван", "Пётр",
		at2(2026, time.November, 1), at2(2027, time.September, 30),
		at2(2026, time.December, 1), at2(2027, time.November, 30),
	)

	if got := e.Segments.PlainText(); got != "Аренда изменена: 12 месяцев" {
		t.Errorf("header = %q, want %q", got, "Аренда изменена: 12 месяцев")
	}
	if e.Action != ActionRentalUpdated || e.BaseAction != BaseChanged {
		t.Errorf("action/base = %s/%s", e.Action, e.BaseAction)
	}
	want := DetailLines{
		`Арендатор: «Иван» → «Пётр»`,
		`Период: «с 1 ноября 2026 по 30 сентября 2027» → «с 1 декабря 2026 по 30 ноября 2027»`,
	}
	if len(e.Details) != len(want) {
		t.Fatalf("details = %q, want %q", e.Details, want)
	}
	for i := range want {
		if e.Details[i] != want[i] {
			t.Errorf("details[%d] = %q, want %q", i, e.Details[i], want[i])
		}
	}
	assertNoLinksInDetails(t, e)
}

func TestRentalUpdated_OnlyChangedFields(t *testing.T) {
	t.Parallel()
	// Неб changed — пояснения только по изменившимся полям: у периода
	// дифф, арендатор не пишется.
	id := uuid.Must(uuid.NewV7())
	from := at2(2026, time.November, 1)
	e := RentalUpdated(id, "Максим", "Максим", from, at2(2027, time.September, 30), from, time.Time{})

	if got := e.Segments.PlainText(); got != "Аренда изменена: 1 ноября" {
		t.Errorf("header = %q, want %q", got, "Аренда изменена: 1 ноября")
	}
	want := DetailLines{
		`Период: «с 1 ноября 2026 по 30 сентября 2027» → «с 1 ноября 2026»`,
	}
	if len(e.Details) != 1 || e.Details[0] != want[0] {
		t.Errorf("details = %q, want %q", e.Details, want)
	}
}

func TestRentalUpdated_NoDiffNoDetails(t *testing.T) {
	t.Parallel()
	// Правка мимо полей аренды (аренда не изменилась) — строка без
	// пояснений, заголовок читает текущий срок.
	id := uuid.Must(uuid.NewV7())
	from := at2(2026, time.November, 1)
	to := at2(2027, time.September, 30)
	e := RentalUpdated(id, "Максим", "Максим", from, to, from, to)

	if got := e.Segments.PlainText(); got != "Аренда изменена: 11 месяцев" {
		t.Errorf("header = %q, want %q", got, "Аренда изменена: 11 месяцев")
	}
	if e.Details != nil {
		t.Errorf("details = %q, want nil", e.Details)
	}
}

func TestPaymentUpdated_FieldDiffs(t *testing.T) {
	t.Parallel()
	// Пояснения несут все изменения, значения реальные: суммы форматом
	// «5 000 ₽» (аменда §5, слово владельца 12.10).
	id := uuid.Must(uuid.NewV7())
	changes := []AttributeChange{
		{Key: "title", Label: detailLabelName, Old: "Электричество", New: "Электричество и вода"},
		{Key: "amount_kopecks", Label: "Сумма", Old: "5 000 ₽", New: "6 000 ₽"},
	}
	e := PaymentUpdated(id, "Электричество и вода", changes)

	if got := e.Segments.PlainText(); got != "Платёж изменён: Электричество и вода" {
		t.Errorf("header = %q", got)
	}
	if len(e.Segments) != 2 || e.Segments[1].Link == nil || e.Segments[1].Link.Kind != KindPayment {
		t.Errorf("segments = %+v, want the linked new-title run", e.Segments)
	}
	want := DetailLines{
		`Название: «Электричество» → «Электричество и вода»`,
		`Сумма: «5 000 ₽» → «6 000 ₽»`,
	}
	if len(e.Details) != len(want) {
		t.Fatalf("details = %q, want %q", e.Details, want)
	}
	for i := range want {
		if e.Details[i] != want[i] {
			t.Errorf("details[%d] = %q, want %q", i, e.Details[i], want[i])
		}
	}
	assertNoLinksInDetails(t, e)
	if v, ok := e.Context["title"]; !ok || v != "Электричество и вода" {
		t.Errorf("context title = %v, want the snapshot", v)
	}
}

func TestContactUpdated_PerFieldDiffs(t *testing.T) {
	t.Parallel()
	// Пояснения несут полные значения, PII включительно (аменда §5, слово
	// владельца 12.10): телефон, почта, мессенджер — реальными значениями.
	id := uuid.Must(uuid.NewV7())
	changes := []AttributeChange{
		{Key: "full_name", Label: "ФИО", Old: "Пётр", New: "Пётр Иванов"},
		{Key: "phone", Label: "Телефон", Old: "+79123456789", New: "+79211112233"},
		{Key: ctxKeyEmail, Label: "Email", Old: nil, New: "petrov@mail.ru"},
	}
	e := ContactUpdated(id, "Пётр Иванов", changes)

	if got := e.Segments.PlainText(); got != "Контакт изменён: Пётр Иванов" {
		t.Errorf("header = %q", got)
	}
	if len(e.Segments) != 2 || e.Segments[1].Link == nil || e.Segments[1].Link.Kind != KindContact {
		t.Errorf("segments = %+v, want the linked new-ФИО run", e.Segments)
	}
	want := DetailLines{
		`ФИО: «Пётр» → «Пётр Иванов»`,
		`Телефон: «+79123456789» → «+79211112233»`,
		`Email: «petrov@mail.ru»`,
	}
	if len(e.Details) != len(want) {
		t.Fatalf("details = %q, want %q", e.Details, want)
	}
	for i := range want {
		if e.Details[i] != want[i] {
			t.Errorf("details[%d] = %q, want %q", i, e.Details[i], want[i])
		}
	}
	assertNoLinksInDetails(t, e)
}

func TestTaskRuleUpdated_DetailsCarryTitleAndDate(t *testing.T) {
	t.Parallel()
	// Макет 3258-77019: «Задача изменена: [название]» + «Название» и
	// «Дата» в пояснениях, точечные даты дд.мм.гггг.
	id := uuid.Must(uuid.NewV7())
	changes := []AttributeChange{
		{Key: "title", Label: detailLabelName, Old: "Моя квартира", New: "Квартира №124"},
		{Key: "due_date", Label: "Дата", Old: "19.02.2026", New: "20.03.2027"},
	}
	e := TaskRuleUpdated(id, "Квартира №124", changes)

	if got := e.Segments.PlainText(); got != "Задача изменена: Квартира №124" {
		t.Errorf("header = %q", got)
	}
	want := DetailLines{
		`Название: «Моя квартира» → «Квартира №124»`,
		`Дата: «19.02.2026» → «20.03.2027»`,
	}
	if len(e.Details) != len(want) {
		t.Fatalf("details = %q, want %q", e.Details, want)
	}
	for i := range want {
		if e.Details[i] != want[i] {
			t.Errorf("details[%d] = %q, want %q", i, e.Details[i], want[i])
		}
	}
	assertNoLinksInDetails(t, e)
}

func TestMemberRoleChanged_DetailsCarryTheDiff(t *testing.T) {
	t.Parallel()
	// «Участник изменён: [имя]» + роль в пояснениях (было «Роль участника
	// изменена» с диффом в заголовке).
	id := uuid.Must(uuid.NewV7())
	e := MemberRoleChanged(id, "Иван Иванов", ActorRoleViewer, ActorRoleFullAccess)

	if got := e.Segments.PlainText(); got != "Участник изменён: Иван Иванов" {
		t.Errorf("header = %q, want %q", got, "Участник изменён: Иван Иванов")
	}
	if e.Action != ActionMemberRoleChanged || e.BaseAction != BaseChanged {
		t.Errorf("action/base = %s/%s", e.Action, e.BaseAction)
	}
	want := DetailLines{`Роль: «Просмотр» → «Полный доступ»`}
	if len(e.Details) != 1 || e.Details[0] != want[0] {
		t.Errorf("details = %q, want %q", e.Details, want)
	}
	assertNoLinksInDetails(t, e)
}

func TestLegacyRows_KeepNoDetails(t *testing.T) {
	t.Parallel()
	// Actions outside the change-row family record no details — the wire
	// field stays absent and the screens render the header alone.
	withoutDetails := []Entry{
		PropertyCreated(uuid.Must(uuid.NewV7()), "Квартира"),
		PropertyArchived(uuid.Must(uuid.NewV7())),
		RentalCompleted(uuid.Must(uuid.NewV7()), "Максим"),
		RentalDeleted(uuid.Must(uuid.NewV7()), "Максим"),
		PaymentCreated(uuid.Must(uuid.NewV7()), "Электричество"),
		PaymentDeleted(uuid.Must(uuid.NewV7()), "Электричество"),
		OperationCreated(uuid.Must(uuid.NewV7()), "Аренда", at2(2026, time.November, 1)),
		ContactCreated(uuid.Must(uuid.NewV7()), "Пётр Иванов"),
		TaskRuleCreated(uuid.Must(uuid.NewV7()), "Задача"),
		TaskCompleted(uuid.Must(uuid.NewV7()), "Задача", at2(2026, time.November, 1)),
		MemberAdded(uuid.Must(uuid.NewV7()), "Иван Иванов"),
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
		{"created", RentalCreated(id, "", from, to), wantRentalHeaderCreated},
		{"updated", RentalUpdated(id, "", "", from, to, from, to), "Аренда изменена: 11 месяцев"},
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
	// snapshot and the name in the context.
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
