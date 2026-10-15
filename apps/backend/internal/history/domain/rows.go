package domain

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// The row-text catalog (ADR 0061 §6): every recorded action gets its row
// copy here, in one place — the server is the single source of the row text,
// and the searchable column is built from the same runs. The exact Russian
// wording is approved at the screen walkthroughs; the templates are passive
// voice («Операция оплачена», never «Платёж оплачен» for operations).
//
// Единые change-строки (решение владельца 12.10, карта #1324; аменда D1):
// любая правка сущности — одна строка уровня сущности «<Сущность>
// изменён(а): [значение-ссылка]» со снапшотом нового значения; диффы едут в
// details — по серой строке на каждое изменившееся поле, реальными
// значениями (PII и суммы — аменда §5). Полевых заголовков больше нет:
// гранулярные билдеры объекта снесены, их идентификаторы (property.renamed,
// address_changed, description_changed, attributes_changed, photo_added,
// photo_deleted) остаются словарём легаси-строк.
//
// A builder returns an Entry with the kind/action/base/segments/context
// already set; the calling module adds PropertyID, ActorID and ActorRole and
// records it in its transaction. A deleted entity leaves a row without a
// link — delete-builders omit the Link.

const (
	roleLabelFullAccess = "Управление"
	roleLabelViewer     = "Просмотр"

	// The context jsonb keys (ADR 0061 §5: the structured extras).
	ctxKeyName    = "name"
	ctxKeyEmail   = "email"
	ctxKeyTitle   = "title"
	ctxKeyTenant  = "tenant"
	ctxKeyLabel   = "label"
	ctxKeyDueDate = "due_date"
	// The ctxKeyChanges key carries the raw AttributeChange list of the
	// unified change rows (the future read model).
	ctxKeyChanges = "changes"
	// The ctxKeyPauseOrigin key marks the archive bridge's system
	// pause/resume rows (тикет #1342): the value is the payments domain's
	// auto_archive origin.
	ctxKeyPauseOrigin = "pause_origin"
	// The archive bridge's origin mark — the key's only value today; the
	// system rows carry it in the context jsonb.
	ctxPauseOriginAutoArchive = "auto_archive"
)

// Общее.

// RoleLabel is the human-readable membership role for the role-change rows.
func RoleLabel(r ActorRole) string {
	switch r {
	case ActorRoleFullAccess:
		return roleLabelFullAccess
	case ActorRoleViewer:
		return roleLabelViewer
	default:
		return string(r)
	}
}

// AttributeChange is one changed field of an entity: the catalog key plus
// the old and new values. Label is the RU display label the calling module
// fills from its own catalog (properties/domain owns the property catalog,
// payments/domain — the schedule and category wordings; history cannot
// import them); an empty label falls back to the raw key in the detail
// line. The values travel raw in the context for the future read model.
// Values arrive display-ready (FormatRubles, FormatDateDotted, the schedule
// labels) — the details are ready lines, the reading path carries no
// formatting catalog.
type AttributeChange struct {
	Key   string
	Label string
	Old   any
	New   any
	// Removed is the custom removal wording («Напоминание: выключено»);
	// empty falls back to «Label: удалено».
	Removed string
}

// PropertyUpdated builds the unified change row (решение 12.10): «Объект
// изменён: [новое имя]» with one gray diff line per changed field — the
// name, address, description, type and attribute edits ride one row, the
// photo edits ride PropertyPhotosChanged.
func PropertyUpdated(entityID uuid.UUID, name string, changes []AttributeChange) Entry {
	return Entry{
		Kind:       KindProperty,
		Action:     ActionPropertyUpdated,
		BaseAction: BaseChanged,
		Segments:   changeHeader("Объект изменён: ", name, entityLink(KindProperty, entityID)),
		Details:    changeDetails(changes),
		Context:    map[string]any{ctxKeyName: name, "attributes": changes},
	}
}

// PropertyPhotosChanged builds the photo leg of the unified change row
// (решение 12.10): «Объект изменён: [имя]» + «Фото: добавлено / удалено» —
// the former photo_added/photo_deleted rows and their ➕/🗑 tones are gone.
func PropertyPhotosChanged(entityID uuid.UUID, name string, added bool) Entry {
	line, key := "Фото: удалено", "removed"
	if added {
		line, key = "Фото: добавлено", "added"
	}
	return Entry{
		Kind:       KindProperty,
		Action:     ActionPropertyUpdated,
		BaseAction: BaseChanged,
		Segments:   changeHeader("Объект изменён: ", name, entityLink(KindProperty, entityID)),
		Details:    DetailLines{line},
		Context:    map[string]any{ctxKeyName: name, "photo": key},
	}
}

// attributeLabel picks the detail line's label: the calling module's RU
// label, falling back to the raw catalog key.
func attributeLabel(c AttributeChange) string {
	if c.Label != "" {
		return c.Label
	}
	return c.Key
}

// changeDetails builds the unified change row's gray lines: one per
// change, in the caller's order. The removal wording is the change's own,
// falling back to «Label: удалено».
func changeDetails(changes []AttributeChange) DetailLines {
	if len(changes) == 0 {
		return nil
	}
	details := make(DetailLines, 0, len(changes))
	for _, c := range changes {
		label := attributeLabel(c)
		removed := c.Removed
		if removed == "" {
			removed = label + ": удалено"
		}
		details = append(details, detailChange(label, c.Old, c.New, removed))
	}
	return details
}

// detailChange renders one gray diff line: «Название: «X» → «Y»» for the
// replacement, «Название: «Y»» for the addition (no old value), the caller's
// removal wording for the removal — род страдательного причастия знает
// только вызывающий билдер («Адрес удалён», «Название удалено»). Values
// read through detailValue.
func detailChange(label string, old, newValue any, removed string) string {
	switch {
	case old == nil || detailValue(old) == "":
		return label + ": «" + detailValue(newValue) + "»"
	case newValue == nil || detailValue(newValue) == "":
		return removed
	default:
		return label + ": «" + detailValue(old) + "» → «" + detailValue(newValue) + "»"
	}
}

// detailValue formats a detail line's value: strings verbatim, numbers
// without trailing zeros, booleans in Russian, nil as the empty string.
// Денежные значения и даты приходят готовыми строками (FormatRubles,
// FormatDateDotted) — вызывающий модуль форматирует их до билдера.
func detailValue(v any) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case int:
		return strconv.Itoa(val)
	case bool:
		if val {
			return "да"
		}
		return "нет"
	default:
		return fmt.Sprint(val)
	}
}

// changeHeader builds the unified change row's header: the action prefix
// plus the linked new-value snapshot; an empty value reads without the
// dangling colon and drops the link.
func changeHeader(prefix, value string, link *Link) Segments {
	if value == "" {
		return Segments{{Text: strings.TrimSuffix(prefix, ": ")}}
	}
	return Segments{{Text: prefix}, {Text: value, Link: link}}
}

func entityLink(kind Kind, id uuid.UUID) *Link {
	if id == uuid.Nil {
		return nil
	}
	return &Link{Kind: kind, ID: id}
}

// Объект.

// PropertyCreated builds the object-created row: the name snapshot with the
// link to the object page.
func PropertyCreated(entityID uuid.UUID, name string) Entry {
	return Entry{
		Kind:       KindProperty,
		Action:     ActionPropertyCreated,
		BaseAction: BaseAdded,
		Segments: Segments{
			{Text: "Добавлен объект: "},
			{Text: name, Link: entityLink(KindProperty, entityID)},
		},
		Context: map[string]any{ctxKeyName: name},
	}
}

// PropertyUpdated поглотил PropertyRenamed, PropertyAddressChanged,
// PropertyDescriptionChanged и PropertyAttributesChanged — решение 12.10;
// их идентификаторы живут в entry.go как словарь легаси-строк.

// PropertyArchived builds the archive row.
func PropertyArchived(entityID uuid.UUID) Entry {
	return Entry{
		Kind:       KindProperty,
		Action:     ActionPropertyArchived,
		BaseAction: BaseChanged,
		Segments:   Segments{{Text: "Объект отправлен в архив", Link: entityLink(KindProperty, entityID)}},
		Context:    map[string]any{},
	}
}

// PropertyUnarchived builds the unarchive row.
func PropertyUnarchived(entityID uuid.UUID) Entry {
	return Entry{
		Kind:       KindProperty,
		Action:     ActionPropertyUnarchived,
		BaseAction: BaseChanged,
		Segments:   Segments{{Text: "Объект восстановлен из архива", Link: entityLink(KindProperty, entityID)}},
		Context:    map[string]any{},
	}
}

// PropertyMaintenanceStarted builds the maintenance-start row (протокол D2,
// карта #1324): «Объект на ремонте: [объект]», группа «Изменение». Без
// деталей — диффа нет, статус сам себе заголовок.
func PropertyMaintenanceStarted(entityID uuid.UUID) Entry {
	return Entry{
		Kind:       KindProperty,
		Action:     ActionPropertyMaintenanceStarted,
		BaseAction: BaseChanged,
		Segments:   Segments{{Text: "Объект на ремонте", Link: entityLink(KindProperty, entityID)}},
		Context:    map[string]any{},
	}
}

// PropertyMaintenanceFinished builds the maintenance-finish row: «Ремонт
// завершён: [объект]», группа «Выполнение».
func PropertyMaintenanceFinished(entityID uuid.UUID) Entry {
	return Entry{
		Kind:       KindProperty,
		Action:     ActionPropertyMaintenanceFinished,
		BaseAction: BaseCompleted,
		Segments:   Segments{{Text: "Ремонт завершён", Link: entityLink(KindProperty, entityID)}},
		Context:    map[string]any{},
	}
}

// Аренда.

// RentalCreated builds the rental row by the new template (протокол D3,
// решение #1317 §7): the header reads the term («11 месяцев») or, for an
// open-ended rental, the start date («1 ноября»); the tenant and the period
// with years moved into the details. The header value segment carries the
// rental link — the header no longer depends on the tenant's name.
func RentalCreated(entityID uuid.UUID, tenantName string, from, to time.Time) Entry {
	return Entry{
		Kind:       KindRental,
		Action:     ActionRentalCreated,
		BaseAction: BaseAdded,
		Segments:   rentalHeader("Добавлена аренда: ", entityID, from, to),
		Details:    rentalDetails(tenantName, from, to),
		Context:    rentalContext(tenantName, from, to),
	}
}

// RentalUpdated builds the rental-terms row by the unified change template
// (решение 12.10): «Аренда изменена: [новый срок / дата начала]» with the
// tenant and period diffs in the details — only the changed fields get a
// line (the «продление» is a UI scenario of the planned-end edit, ADR
// 0061 §4 → changed).
func RentalUpdated(entityID uuid.UUID, oldTenant, newTenant string, oldFrom, oldTo, newFrom, newTo time.Time) Entry {
	return Entry{
		Kind:       KindRental,
		Action:     ActionRentalUpdated,
		BaseAction: BaseChanged,
		Segments:   rentalHeader("Аренда изменена: ", entityID, newFrom, newTo),
		Details:    rentalChangeDetails(oldTenant, newTenant, oldFrom, oldTo, newFrom, newTo),
		Context:    rentalContext(newTenant, newFrom, newTo),
	}
}

// RentalCompleted builds the completion row.
func RentalCompleted(entityID uuid.UUID, tenantName string) Entry {
	// An unnamed tenant reads without the colon, and the row drops the
	// link — nothing to attach it to.
	segments := Segments{
		{Text: "Аренда завершена: "},
		{Text: tenantName, Link: entityLink(KindRental, entityID)},
	}
	if tenantName == "" {
		segments = Segments{{Text: "Аренда завершена"}}
	}
	return Entry{
		Kind:       KindRental,
		Action:     ActionRentalCompleted,
		BaseAction: BaseCompleted,
		Segments:   segments,
		Context:    map[string]any{ctxKeyTenant: tenantName},
	}
}

// RentalDeleted builds the deletion row — no link, the entity is gone.
func RentalDeleted(entityID uuid.UUID, tenantName string) Entry {
	// An unnamed tenant reads without the dangling colon.
	segments := Segments{{Text: "Аренда удалена: "}, {Text: tenantName}}
	if tenantName == "" {
		segments = Segments{{Text: "Аренда удалена"}}
	}
	return Entry{
		Kind:       KindRental,
		Action:     ActionRentalDeleted,
		BaseAction: BaseDeleted,
		Segments:   segments,
		Context:    map[string]any{ctxKeyTenant: tenantName},
	}
}

// rentalHeader builds the rental rows' header: the action prefix plus the
// term («11 месяцев») or, for an open-ended rental, the start date
// («1 ноября», without the year); the header value is the single linked
// segment (the rental page) and never depends on the tenant's name.
func rentalHeader(prefix string, entityID uuid.UUID, from, to time.Time) Segments {
	var header string
	switch {
	case !to.IsZero():
		header = formatTerm(from, to)
	case !from.IsZero():
		header = formatDayMonthGenitive(from)
	}
	segments := Segments{{Text: prefix}}
	if header != "" {
		segments = append(segments, Segment{Text: header, Link: entityLink(KindRental, entityID)})
	} else {
		segments[0].Text = strings.TrimSuffix(prefix, ": ")
	}
	return segments
}

// rentalDetails builds the rental-created row's gray lines: the tenant line
// (only with a name) and the period line with the years. Годы здесь есть —
// в отличие от заголовка (D3).
func rentalDetails(tenantName string, from, to time.Time) DetailLines {
	var details DetailLines
	if tenantName != "" {
		details = append(details, detailChange("Арендатор", nil, tenantName, "Арендатор удалён"))
	}
	if !from.IsZero() {
		details = append(details, "Период: "+formatPeriodLine(from, to))
	}
	return details
}

// rentalChangeDetails builds the rental-updated row's gray lines: the diff
// per changed field only — the tenant (when the card moved) and the period
// (when either bound moved), the D3 wording inside the quotes.
func rentalChangeDetails(oldTenant, newTenant string, oldFrom, oldTo, newFrom, newTo time.Time) DetailLines {
	var details DetailLines
	if oldTenant != newTenant {
		details = append(details, detailChange("Арендатор", oldTenant, newTenant, "Арендатор удалён"))
	}
	if !oldFrom.Equal(newFrom) || !oldTo.Equal(newTo) {
		details = append(details,
			"Период: «"+formatPeriodLine(oldFrom, oldTo)+"» → «"+formatPeriodLine(newFrom, newTo)+"»")
	}
	return details
}

func rentalContext(tenantName string, from, to time.Time) map[string]any {
	return map[string]any{
		ctxKeyTenant:  tenantName,
		"period_from": formatContextDate(from),
		"period_to":   formatContextDate(to),
	}
}

func formatContextDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// Платежи.

// PaymentCreated builds the rule-created row.
func PaymentCreated(entityID uuid.UUID, title string) Entry {
	return paymentRow(ActionPaymentCreated, BaseAdded, "Добавлен платёж: ", entityID, title)
}

// PaymentUpdated builds the rule-updated row by the unified change template
// (решение 12.10): «Платёж изменён: [новое название]» — the new title's
// snapshot survives future renames, the diffs (название, сумма, расписание,
// дата окончания, категория — ADR 0065's field dictionary) live in the
// details, the values display-ready from the calling module (решение #1317
// §2, противоречие №4: старое название дотягивается до точки вызова).
func PaymentUpdated(entityID uuid.UUID, title string, changes []AttributeChange) Entry {
	return Entry{
		Kind:       KindPayment,
		Action:     ActionPaymentUpdated,
		BaseAction: BaseChanged,
		Segments:   changeHeader("Платёж изменён: ", title, entityLink(KindPayment, entityID)),
		Details:    changeDetails(changes),
		Context:    map[string]any{ctxKeyTitle: title, ctxKeyChanges: changes},
	}
}

// PaymentDeleted builds the rule-deletion row — no link.
func PaymentDeleted(entityID uuid.UUID, title string) Entry {
	return Entry{
		Kind:       KindPayment,
		Action:     ActionPaymentDeleted,
		BaseAction: BaseDeleted,
		Segments:   Segments{{Text: "Платёж удалён: "}, {Text: title}},
		Context:    map[string]any{ctxKeyTitle: title},
	}
}

// PaymentPaused builds the pause row.
func PaymentPaused(entityID uuid.UUID, title string) Entry {
	return paymentRow(ActionPaymentPaused, BaseChanged, "Платёж поставлен на паузу: ", entityID, title)
}

// PaymentResumed builds the resume row.
func PaymentResumed(entityID uuid.UUID, title string) Entry {
	return paymentRow(ActionPaymentResumed, BaseChanged, "Платёж возобновлён: ", entityID, title)
}

// PaymentPausedByArchive builds the archive bridge's system pause row
// (тикет #1342): the manual action ids stay (the feed's icons and filters
// are not retrained), the copy names the archive as the reason and the
// context carries the origin mark.
func PaymentPausedByArchive(entityID uuid.UUID, title string) Entry {
	e := paymentRow(ActionPaymentPaused, BaseChanged,
		"Платёж поставлен на паузу (объект ушёл в архив): ", entityID, title)
	e.Context[ctxKeyPauseOrigin] = ctxPauseOriginAutoArchive
	return e
}

// PaymentResumedByArchive builds the archive bridge's system resume row
// (тикет #1342): only the bridge's own auto_archive pauses close on the
// property's return, the row reads the same way.
func PaymentResumedByArchive(entityID uuid.UUID, title string) Entry {
	e := paymentRow(ActionPaymentResumed, BaseChanged,
		"Платёж возобновлён (объект вернулся из архива): ", entityID, title)
	e.Context[ctxKeyPauseOrigin] = ctxPauseOriginAutoArchive
	return e
}

// PaymentCompleted builds the completion row (тикет #1340): the terminal
// state flip, «Завершение» in the base action's own word.
func PaymentCompleted(entityID uuid.UUID, title string) Entry {
	return paymentRow(ActionPaymentCompleted, BaseCompleted, "Платёж завершён: ", entityID, title)
}

func paymentRow(action Action, base BaseAction, prefix string, entityID uuid.UUID, title string) Entry {
	return Entry{
		Kind:       KindPayment,
		Action:     action,
		BaseAction: base,
		Segments: Segments{
			{Text: prefix},
			{Text: title, Link: entityLink(KindPayment, entityID)},
		},
		Context: map[string]any{ctxKeyTitle: title},
	}
}

// Операции.

// OperationCreated builds the operation row: the title (the due date lives
// in the context only).
func OperationCreated(entityID uuid.UUID, title string, due time.Time) Entry {
	return operationRow(ActionOperationCreated, BaseAdded, "Добавлена операция: ", entityID, title, due)
}

// OperationPaid builds the manual payment mark (the canon copy is «Операция
// оплачена», owner decision 2026-09-22).
func OperationPaid(entityID uuid.UUID, title string, due time.Time) Entry {
	return operationRow(ActionOperationPaid, BaseCompleted, "Операция оплачена: ", entityID, title, due)
}

// OperationDeleted builds the «Отменённая операция» tombstone — no link, the
// operation is gone (ADR 0061 §4: the deletion maps to «Удаление»).
func OperationDeleted(entityID uuid.UUID, title string, due time.Time) Entry {
	return Entry{
		Kind:       KindOperation,
		Action:     ActionOperationDeleted,
		BaseAction: BaseDeleted,
		Segments:   titledSegments("Отменённая операция: ", title, nil),
		Context:    map[string]any{ctxKeyTitle: title, ctxKeyDueDate: formatContextDate(due)},
	}
}

func operationRow(action Action, base BaseAction, prefix string, entityID uuid.UUID, title string, due time.Time) Entry {
	return Entry{
		Kind:       KindOperation,
		Action:     action,
		BaseAction: base,
		Segments:   titledSegments(prefix, title, entityLink(KindOperation, entityID)),
		Context:    map[string]any{ctxKeyTitle: title, ctxKeyDueDate: formatContextDate(due)},
	}
}

// titledSegments builds the «prefix + title» run shared by the operation
// and task rows; a nil link is the deleted operation's no-link tombstone.
// Срок в строку не пишется (аудит #876: голые подписи дат убраны из
// ленты) — живёт в context для деталей.
func titledSegments(prefix, title string, link *Link) Segments {
	return Segments{{Text: prefix}, {Text: title, Link: link}}
}

// Контакты.

// ContactCreated builds the contact row: the ФИО snapshot.
func ContactCreated(entityID uuid.UUID, fullName string) Entry {
	return Entry{
		Kind:       KindContact,
		Action:     ActionContactCreated,
		BaseAction: BaseAdded,
		Segments: Segments{
			{Text: "Добавлен контакт: "},
			{Text: fullName, Link: entityLink(KindContact, entityID)},
		},
		Context: map[string]any{ctxKeyName: fullName},
	}
}

// ContactUpdated builds the contact row by the unified change template
// (решение 12.10): «Контакт изменён: [новое ФИО]» with the per-field diffs
// in the details — ФИО, роль, телефон, почта, мессенджер, заметка, полными
// реальными значениями (аменда §5: «пояснения существуют, чтобы показать,
// что изменилось»).
func ContactUpdated(entityID uuid.UUID, fullName string, changes []AttributeChange) Entry {
	return Entry{
		Kind:       KindContact,
		Action:     ActionContactUpdated,
		BaseAction: BaseChanged,
		Segments:   changeHeader("Контакт изменён: ", fullName, entityLink(KindContact, entityID)),
		Details:    changeDetails(changes),
		Context:    map[string]any{ctxKeyName: fullName, ctxKeyChanges: changes},
	}
}

// ContactDeleted builds the contact deletion row — no link.
func ContactDeleted(entityID uuid.UUID, fullName string) Entry {
	return Entry{
		Kind:       KindContact,
		Action:     ActionContactDeleted,
		BaseAction: BaseDeleted,
		Segments:   Segments{{Text: "Контакт удалён: "}, {Text: fullName}},
		Context:    map[string]any{ctxKeyName: fullName},
	}
}

// ContactPhotoAdded builds the card photo row (ADR 0065): the ФИО snapshot.
func ContactPhotoAdded(entityID uuid.UUID, fullName string) Entry {
	return Entry{
		Kind:       KindContact,
		Action:     ActionContactPhotoAdded,
		BaseAction: BaseAdded,
		Segments: Segments{
			{Text: "Добавлено фото контакта: "},
			{Text: fullName, Link: entityLink(KindContact, entityID)},
		},
		Context: map[string]any{ctxKeyName: fullName},
	}
}

// ContactPhotoDeleted builds the card photo deletion row.
func ContactPhotoDeleted(entityID uuid.UUID, fullName string) Entry {
	return Entry{
		Kind:       KindContact,
		Action:     ActionContactPhotoDeleted,
		BaseAction: BaseDeleted,
		Segments: Segments{
			{Text: "Удалено фото контакта: "},
			{Text: fullName, Link: entityLink(KindContact, entityID)},
		},
		Context: map[string]any{ctxKeyName: fullName},
	}
}

// ContactMovedFrom builds the source leg of the cross-property move row
// (тикет #856): this object's feed lost the card to another one. The ФИО is
// the action-time snapshot, the link points at the card's own page.
func ContactMovedFrom(entityID uuid.UUID, fullName string) Entry {
	return contactRebindRow(ActionContactMoved, "Контакт перенесён на другой объект: ", entityID, fullName)
}

// ContactMovedTo builds the destination leg of the cross-property move row:
// this object's feed gained the card from another one.
func ContactMovedTo(entityID uuid.UUID, fullName string) Entry {
	return contactRebindRow(ActionContactMoved, "Контакт перенесён с другого объекта: ", entityID, fullName)
}

// ContactBound builds the bind row (тикет #856): a previously unbound card
// landed on the object — the single end the rebind touches.
func ContactBound(entityID uuid.UUID, fullName string) Entry {
	return contactRebindRow(ActionContactBound, "Контакт привязан к объекту: ", entityID, fullName)
}

// ContactUnbound builds the unbind row: the card left the object for the
// owner's book — the source is the only end to anchor (ADR 0061 §3 keeps
// «an unbound card writes no rows»: the card is bound at action time).
func ContactUnbound(entityID uuid.UUID, fullName string) Entry {
	return contactRebindRow(ActionContactUnbound, "Контакт отвязан от объекта: ", entityID, fullName)
}

func contactRebindRow(action Action, prefix string, entityID uuid.UUID, fullName string) Entry {
	return Entry{
		Kind:       KindContact,
		Action:     action,
		BaseAction: BaseChanged,
		Segments: Segments{
			{Text: prefix},
			{Text: fullName, Link: entityLink(KindContact, entityID)},
		},
		Context: map[string]any{ctxKeyName: fullName},
	}
}

// Задачи.

// TaskRuleCreated builds the rule row (the product word for a rule is
// «задача» — the repeating template behind the occurrences).
func TaskRuleCreated(entityID uuid.UUID, title string) Entry {
	return taskRuleRow(ActionTaskRuleCreated, BaseAdded, "Добавлена задача: ", entityID, title)
}

// TaskRuleUpdated builds the rule-updated row by the unified change template
// (решение 12.10): «Задача изменена: [новое название]» with the per-field
// diffs in the details — название, дата, время, повтор, комментарий (макет
// 3258-77019: «Дата: «19.02.2026» → «20.03.2027»»).
func TaskRuleUpdated(entityID uuid.UUID, title string, changes []AttributeChange) Entry {
	return Entry{
		Kind:       KindTask,
		Action:     ActionTaskRuleUpdated,
		BaseAction: BaseChanged,
		Segments:   changeHeader("Задача изменена: ", title, entityLink(KindTask, entityID)),
		Details:    changeDetails(changes),
		Context:    map[string]any{ctxKeyTitle: title, ctxKeyChanges: changes},
	}
}

// TaskRuleDeleted builds the rule-deletion row — no link (rule deletion is
// hard, but the row's snapshot keeps the title readable, ADR 0061 §3).
func TaskRuleDeleted(entityID uuid.UUID, title string) Entry {
	return Entry{
		Kind:       KindTask,
		Action:     ActionTaskRuleDeleted,
		BaseAction: BaseDeleted,
		Segments:   Segments{{Text: "Задача удалена: "}, {Text: title}},
		Context:    map[string]any{ctxKeyTitle: title},
	}
}

func taskRuleRow(action Action, base BaseAction, prefix string, entityID uuid.UUID, title string) Entry {
	return Entry{
		Kind:       KindTask,
		Action:     action,
		BaseAction: base,
		Segments: Segments{
			{Text: prefix},
			{Text: title, Link: entityLink(KindTask, entityID)},
		},
		Context: map[string]any{ctxKeyTitle: title},
	}
}

// TaskCompleted builds the completion row: the title (the occurrence's
// due date lives in the context only).
func TaskCompleted(entityID uuid.UUID, title string, due time.Time) Entry {
	return Entry{
		Kind:       KindTask,
		Action:     ActionTaskCompleted,
		BaseAction: BaseCompleted,
		Segments:   titledSegments("Задача выполнена: ", title, entityLink(KindTask, entityID)),
		Context:    map[string]any{ctxKeyTitle: title, ctxKeyDueDate: formatContextDate(due)},
	}
}

// TaskUncompleted builds the revert row (→ «Изменение», ADR 0061 §4).
func TaskUncompleted(entityID uuid.UUID, title string) Entry {
	return Entry{
		Kind:       KindTask,
		Action:     ActionTaskUncompleted,
		BaseAction: BaseChanged,
		Segments: Segments{
			{Text: "Задача возвращена в работу: "},
			{Text: title, Link: entityLink(KindTask, entityID)},
		},
		Context: map[string]any{ctxKeyTitle: title},
	}
}

// TaskCompletedCleared builds the bulk-cleanup row — one per property, the
// removed count in the text (ADR 0061 §3).
func TaskCompletedCleared(count int) Entry {
	return Entry{
		Kind:       KindTask,
		Action:     ActionTaskCompletedCleared,
		BaseAction: BaseDeleted,
		Segments:   Segments{{Text: "Удалены выполненные задачи: "}, {Text: strconv.Itoa(count)}},
		Context:    map[string]any{"count": count},
	}
}

// Участники.

// MemberInvited builds the pending-invitation row — the invitee is known by
// email only, so the snapshot is the email.
func MemberInvited(email string) Entry {
	return Entry{
		Kind:       KindMember,
		Action:     ActionMemberInvited,
		BaseAction: BaseAdded,
		Segments:   Segments{{Text: "Отправлено приглашение: "}, {Text: email}},
		Context:    map[string]any{ctxKeyEmail: email},
	}
}

// MemberAdded builds the membership row for a registered participant; the
// label is the display-name snapshot with a fallback to the anonymous label
// «Пользователь» — never a phone, never the email (the access canon:
// userLabel; карта #1105, аменд #1123).
func MemberAdded(userID uuid.UUID, label string) Entry {
	return Entry{
		Kind:       KindMember,
		Action:     ActionMemberAdded,
		BaseAction: BaseAdded,
		Segments: Segments{
			{Text: "Добавлен участник: "},
			{Text: label, Link: entityLink(KindMember, userID)},
		},
		Context: map[string]any{ctxKeyLabel: label},
	}
}

// MemberRoleChanged builds the role-change row by the unified change
// template (решение 12.10): «Участник изменён: [имя]» + «Роль: «Управление»
// → «Просмотр»» in the details (было «Роль участника изменена» с диффом в
// заголовке).
func MemberRoleChanged(userID uuid.UUID, name string, oldRole, newRole ActorRole) Entry {
	return Entry{
		Kind:       KindMember,
		Action:     ActionMemberRoleChanged,
		BaseAction: BaseChanged,
		Segments:   changeHeader("Участник изменён: ", name, entityLink(KindMember, userID)),
		Details:    DetailLines{"Роль: «" + RoleLabel(oldRole) + "» → «" + RoleLabel(newRole) + "»"},
		Context:    map[string]any{ctxKeyName: name, "old_role": string(oldRole), "new_role": string(newRole)},
	}
}

// MemberRemoved builds the single revoke row — no link, the membership is
// gone.
func MemberRemoved(label string) Entry {
	return Entry{
		Kind:       KindMember,
		Action:     ActionMemberRemoved,
		BaseAction: BaseDeleted,
		Segments:   Segments{{Text: "Участник удалён: "}, {Text: label}},
		Context:    map[string]any{ctxKeyLabel: label},
	}
}

// MemberParticipantRemoved builds one property leg of the bulk participant
// removal (#694).
func MemberParticipantRemoved(label string) Entry {
	return Entry{
		Kind:       KindMember,
		Action:     ActionMemberParticipantRemoved,
		BaseAction: BaseDeleted,
		Segments:   Segments{{Text: "Участник удалён: "}, {Text: label}},
		Context:    map[string]any{ctxKeyLabel: label},
	}
}

// MemberInvitationCancelled builds the cancelled-invitation row.
func MemberInvitationCancelled(email string) Entry {
	return Entry{
		Kind:       KindMember,
		Action:     ActionMemberInvitationCancelled,
		BaseAction: BaseDeleted,
		Segments:   Segments{{Text: "Приглашение отменено: "}, {Text: email}},
		Context:    map[string]any{ctxKeyEmail: email},
	}
}

// MemberLeft builds the self-exit row.
func MemberLeft(label string) Entry {
	return Entry{
		Kind:       KindMember,
		Action:     ActionMemberLeft,
		BaseAction: BaseDeleted,
		Segments:   Segments{{Text: "Участник вышел: "}, {Text: label}},
		Context:    map[string]any{ctxKeyLabel: label},
	}
}
