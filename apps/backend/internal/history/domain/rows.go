package domain

import (
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
// A builder returns an Entry with the kind/action/base/segments/context
// already set; the calling module adds PropertyID, ActorID and ActorRole and
// records it in its transaction. A deleted entity leaves a row without a
// link — delete-builders omit the Link.

const (
	roleLabelFullAccess = "Полный доступ"
	roleLabelViewer     = "Просмотр"

	// The arrow between the old and the new value in the change rows.
	arrow = " → "

	// The context jsonb keys (ADR 0061 §5: the structured extras).
	ctxKeyName     = "name"
	ctxKeyOldName  = "old_name"
	ctxKeyNewName  = "new_name"
	ctxKeyOldValue = "old_value"
	ctxKeyNewValue = "new_value"
	ctxKeyTitle    = "title"
	ctxKeyTenant   = "tenant"
	ctxKeyLabel    = "label"
	ctxKeyDueDate  = "due_date"
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

// AttributeChange is one changed attribute of an object: the catalog key
// plus the old and new values. The RU labels live in the generated property
// catalog (not yet the compiled source), so the row text stays generic —
// the values travel in the context for the future read model.
type AttributeChange struct {
	Key string
	Old any
	New any
}

// PropertyAttributesChanged builds the attributes row: the values live in
// the context only, the text stays a sentence.
func PropertyAttributesChanged(entityID uuid.UUID, changes []AttributeChange) Entry {
	return Entry{
		Kind:       KindProperty,
		Action:     ActionPropertyAttributesChanged,
		BaseAction: BaseChanged,
		Segments:   Segments{{Text: "Изменены характеристики объекта", Link: entityLink(KindProperty, entityID)}},
		Context:    map[string]any{"attributes": changes},
	}
}

func formatDate(t time.Time) string {
	return t.Format("02.01.2006")
}

func formatPeriod(from, to time.Time) string {
	if to.IsZero() {
		return formatDate(from)
	}
	return formatDate(from) + " – " + formatDate(to)
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

// PropertyRenamed builds the rename row with the old → new name snapshot.
func PropertyRenamed(entityID uuid.UUID, oldName, newName string) Entry {
	return Entry{
		Kind:       KindProperty,
		Action:     ActionPropertyRenamed,
		BaseAction: BaseChanged,
		Segments: Segments{
			{Text: "Название объекта изменено: "},
			{Text: oldName},
			{Text: arrow},
			{Text: newName, Link: entityLink(KindProperty, entityID)},
		},
		Context: map[string]any{ctxKeyOldName: oldName, ctxKeyNewName: newName},
	}
}

// PropertyAddressChanged builds the address row; the address is optional, so
// an addition and a removal get their own wording.
func PropertyAddressChanged(entityID uuid.UUID, oldAddress, newAddress string) Entry {
	e := Entry{
		Kind:       KindProperty,
		Action:     ActionPropertyAddressChanged,
		BaseAction: BaseChanged,
		Context:    map[string]any{ctxKeyOldValue: oldAddress, ctxKeyNewValue: newAddress},
	}
	switch {
	case oldAddress == "":
		e.Segments = Segments{
			{Text: "Добавлен адрес объекта: "},
			{Text: newAddress, Link: entityLink(KindProperty, entityID)},
		}
	case newAddress == "":
		e.Segments = Segments{{Text: "Удалён адрес объекта", Link: entityLink(KindProperty, entityID)}}
	default:
		e.Segments = Segments{
			{Text: "Адрес объекта изменён: "},
			{Text: oldAddress},
			{Text: arrow},
			{Text: newAddress, Link: entityLink(KindProperty, entityID)},
		}
	}
	return e
}

// PropertyDescriptionChanged builds the description row: the values live in
// the context only, the text stays a sentence.
func PropertyDescriptionChanged(entityID uuid.UUID, oldDescription, newDescription string) Entry {
	return Entry{
		Kind:       KindProperty,
		Action:     ActionPropertyDescriptionChanged,
		BaseAction: BaseChanged,
		Segments:   Segments{{Text: "Изменено описание объекта", Link: entityLink(KindProperty, entityID)}},
		Context:    map[string]any{ctxKeyOldValue: oldDescription, ctxKeyNewValue: newDescription},
	}
}

// PropertyUpdated builds the several-field-groups row: «Данные объекта
// изменены: название, адрес».
func PropertyUpdated(entityID uuid.UUID, fieldLabels []string) Entry {
	segments := Segments{{Text: "Данные объекта изменены: ", Link: entityLink(KindProperty, entityID)}}
	for i, label := range fieldLabels {
		if i > 0 {
			segments = append(segments, Segment{Text: ", "})
		}
		segments = append(segments, Segment{Text: label})
	}
	return Entry{
		Kind:       KindProperty,
		Action:     ActionPropertyUpdated,
		BaseAction: BaseChanged,
		Segments:   segments,
		Context:    map[string]any{"fields": fieldLabels},
	}
}

// PropertyPhotoAdded builds the photo-added row.
func PropertyPhotoAdded(entityID uuid.UUID) Entry {
	return Entry{
		Kind:       KindProperty,
		Action:     ActionPropertyPhotoAdded,
		BaseAction: BaseAdded,
		Segments:   Segments{{Text: "Добавлено фото объекта", Link: entityLink(KindProperty, entityID)}},
		Context:    map[string]any{},
	}
}

// PropertyPhotoDeleted builds the photo-deleted row.
func PropertyPhotoDeleted(entityID uuid.UUID) Entry {
	return Entry{
		Kind:       KindProperty,
		Action:     ActionPropertyPhotoDeleted,
		BaseAction: BaseDeleted,
		Segments:   Segments{{Text: "Удалено фото объекта", Link: entityLink(KindProperty, entityID)}},
		Context:    map[string]any{},
	}
}

// PropertyPinned builds the pin row (a re-pin of an already pinned object is
// not an action and never reaches the journal).
func PropertyPinned(entityID uuid.UUID) Entry {
	return Entry{
		Kind:       KindProperty,
		Action:     ActionPropertyPinned,
		BaseAction: BaseChanged,
		Segments:   Segments{{Text: "Объект закреплён", Link: entityLink(KindProperty, entityID)}},
		Context:    map[string]any{},
	}
}

// PropertyUnpinned builds the unpin row.
func PropertyUnpinned(entityID uuid.UUID) Entry {
	return Entry{
		Kind:       KindProperty,
		Action:     ActionPropertyUnpinned,
		BaseAction: BaseChanged,
		Segments:   Segments{{Text: "Объект откреплён", Link: entityLink(KindProperty, entityID)}},
		Context:    map[string]any{},
	}
}

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

// Аренда.

// RentalCreated builds the rental row: the tenant name plus the period.
func RentalCreated(entityID uuid.UUID, tenantName string, from, to time.Time) Entry {
	return rentalRow(ActionRentalCreated, BaseAdded, "Добавлена аренда: ", entityID, tenantName, from, to)
}

// RentalUpdated builds the rental-terms row (the «продление» is a UI scenario
// of the planned-end edit, ADR 0061 §4 → changed).
func RentalUpdated(entityID uuid.UUID, tenantName string, from, to time.Time) Entry {
	return rentalRow(ActionRentalUpdated, BaseChanged, "Условия аренды изменены: ", entityID, tenantName, from, to)
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

func rentalRow(action Action, base BaseAction, prefix string, entityID uuid.UUID, tenantName string, from, to time.Time) Entry {
	// An unnamed tenant (a rental without a contact) reads without the
	// colon, and the row drops the link — nothing to attach it to.
	segments := make(Segments, 0, 2)
	if tenantName == "" {
		segments = append(segments, Segment{Text: strings.TrimSuffix(prefix, ": ")})
	} else {
		segments = append(segments,
			Segment{Text: prefix},
			Segment{Text: tenantName, Link: entityLink(KindRental, entityID)},
		)
	}
	// The period segment is appended unconditionally: formatPeriod is never
	// empty (formatDate is a plain t.Format), the indefinite form included
	// (to = zero → «(дата)»).
	segments = append(segments, Segment{Text: " (" + formatPeriod(from, to) + ")"})
	return Entry{
		Kind:       KindRental,
		Action:     action,
		BaseAction: base,
		Segments:   segments,
		Context:    map[string]any{"tenant": tenantName, "period_from": formatContextDate(from), "period_to": formatContextDate(to)},
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

// PaymentUpdated builds the rule-updated row; the title snapshot is the new
// one — the row survives future renames.
func PaymentUpdated(entityID uuid.UUID, title string) Entry {
	return paymentRow(ActionPaymentUpdated, BaseChanged, "Платёж изменён: ", entityID, title)
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

// OperationCreated builds the operation row: the title plus the due date.
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
		Segments:   titledWithDeadlineSegments("Отменённая операция: ", title, due, nil),
		Context:    map[string]any{ctxKeyTitle: title, ctxKeyDueDate: formatContextDate(due)},
	}
}

func operationRow(action Action, base BaseAction, prefix string, entityID uuid.UUID, title string, due time.Time) Entry {
	return Entry{
		Kind:       KindOperation,
		Action:     action,
		BaseAction: base,
		Segments:   titledWithDeadlineSegments(prefix, title, due, entityLink(KindOperation, entityID)),
		Context:    map[string]any{ctxKeyTitle: title, ctxKeyDueDate: formatContextDate(due)},
	}
}

// titledWithDeadlineSegments builds the «prefix + title (срок дата)» run —
// the shape shared by the operation and task rows; a nil link is the deleted
// operation's no-link tombstone, a zero due drops the deadline segment.
func titledWithDeadlineSegments(prefix, title string, due time.Time, link *Link) Segments {
	segments := Segments{{Text: prefix}, {Text: title, Link: link}}
	if !due.IsZero() {
		segments = append(segments, Segment{Text: " (срок " + formatDate(due) + ")"})
	}
	return segments
}

// Контакты.

// ContactCreated builds the contact row: the ФИО snapshot, never the phone
// (ADR 0061 §5).
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

// ContactUpdated builds the contact row with the old → new ФИО.
func ContactUpdated(entityID uuid.UUID, oldFullName, newFullName string) Entry {
	return Entry{
		Kind:       KindContact,
		Action:     ActionContactUpdated,
		BaseAction: BaseChanged,
		Segments: Segments{
			{Text: "Контакт изменён: "},
			{Text: oldFullName},
			{Text: arrow},
			{Text: newFullName, Link: entityLink(KindContact, entityID)},
		},
		Context: map[string]any{ctxKeyOldName: oldFullName, ctxKeyNewName: newFullName},
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

// TaskRuleUpdated builds the rule-updated row.
func TaskRuleUpdated(entityID uuid.UUID, title string) Entry {
	return taskRuleRow(ActionTaskRuleUpdated, BaseChanged, "Задача изменена: ", entityID, title)
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

// TaskCompleted builds the completion row: the title plus the occurrence's
// due date.
func TaskCompleted(entityID uuid.UUID, title string, due time.Time) Entry {
	return Entry{
		Kind:       KindTask,
		Action:     ActionTaskCompleted,
		BaseAction: BaseCompleted,
		Segments:   titledWithDeadlineSegments("Задача выполнена: ", title, due, entityLink(KindTask, entityID)),
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
// email only, so the snapshot is the email (ADR 0061 §5).
func MemberInvited(email string) Entry {
	return Entry{
		Kind:       KindMember,
		Action:     ActionMemberInvited,
		BaseAction: BaseAdded,
		Segments:   Segments{{Text: "Отправлено приглашение: "}, {Text: email}},
		Context:    map[string]any{"email": email},
	}
}

// MemberAdded builds the membership row for a registered participant; the
// label is the display-name snapshot with a fallback to the masked phone —
// never the email (the access canon: userLabel).
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

// MemberRoleChanged builds the role-change row: «Роль участника изменена:
// Иван Иванов: Полный доступ → Просмотр».
func MemberRoleChanged(userID uuid.UUID, name string, oldRole, newRole ActorRole) Entry {
	return Entry{
		Kind:       KindMember,
		Action:     ActionMemberRoleChanged,
		BaseAction: BaseChanged,
		Segments: Segments{
			{Text: "Роль участника изменена: "},
			{Text: name, Link: entityLink(KindMember, userID)},
			{Text: ": " + RoleLabel(oldRole) + arrow + RoleLabel(newRole)},
		},
		Context: map[string]any{"name": name, "old_role": string(oldRole), "new_role": string(newRole)},
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
		Context:    map[string]any{"email": email},
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
