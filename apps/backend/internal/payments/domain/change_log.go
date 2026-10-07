package domain

// The payment change log's domain model (ADR 0065): the append-only story of
// one rule's user edits — one entry per manual action, its payload the
// ordered per-field old→new diff. The chip screen renders values on the
// frontend; nothing here builds sentences (research #1184, map #1183).

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ChangeAction is what the entry records: a terms edit («изменения» rows with
// the field diff) or the pause/resume state flips — rows with an empty
// changes array of their own action (решение владельца 07.10, map #1183).
// Creation writes no entry (nothing changed yet); deletion writes none — the
// FK CASCADE takes the history with the rule, and the «Платёж удалён»
// tombstone stays in the action journal (ADR 0061).
type ChangeAction string

// The entry actions of the change log.
const (
	ChangeUpdated ChangeAction = "updated"
	ChangePaused  ChangeAction = "paused"
	ChangeResumed ChangeAction = "resumed"
)

// ChangeField names one field of the rule's diff — the same dictionary the
// mutation audit carries (application.updatedFields), stable wire keys.
type ChangeField string

// The field dictionary, in the diff's stable output order (the chip screen
// renders in this order).
const (
	ChangeFieldType           ChangeField = "type"
	ChangeFieldTitle          ChangeField = "title"
	ChangeFieldAmount         ChangeField = "amount_kopecks"
	ChangeFieldRecurrence     ChangeField = "recurrence"
	ChangeFieldCategory       ChangeField = "category_slug"
	ChangeFieldEndDate        ChangeField = "end_date"
	ChangeFieldAutoPay        ChangeField = "auto_pay"
	ChangeFieldReminderOffset ChangeField = "reminder_offset_days"
)

// FieldChange is one field's old→new pair. Both values are typed JSON, never
// strings-with-formatting: kopecks travel as numbers, the recurrence as its
// canonical JSON object, dates as "2006-01-02" or null — the frontend owns
// the human text (recurrenceLabel, formatMoneyKopecks).
type FieldChange struct {
	Field ChangeField
	Old   json.RawMessage
	New   json.RawMessage
}

// ChangeEntry is one stored row of the log: who edited the rule, what kind of
// action it was and which fields moved. Pause/resume rows carry an empty
// Changes — the state flip is the content.
type ChangeEntry struct {
	ID        uuid.UUID
	PaymentID uuid.UUID
	ActorID   uuid.UUID
	Action    ChangeAction
	Changes   []FieldChange
	CreatedAt time.Time
}

// DiffPaymentChanges returns the before→after diff of the rule in the field
// dictionary's order, one FieldChange per moved field. Equal rules diff to
// nil. It is the single content source of the log's «updated» entries: both
// write seams — PaymentService's PATCH and the rental pipeline's terms sync —
// diff the loaded rule against the merged one through it (ADR 0065).
func DiffPaymentChanges(before, after Payment) []FieldChange {
	var changes []FieldChange
	if before.Type != after.Type {
		changes = append(changes, FieldChange{
			Field: ChangeFieldType,
			Old:   changeString(string(before.Type)),
			New:   changeString(string(after.Type)),
		})
	}
	if before.Title != after.Title {
		changes = append(changes, FieldChange{
			Field: ChangeFieldTitle,
			Old:   changeString(before.Title),
			New:   changeString(after.Title),
		})
	}
	if before.AmountKopecks != after.AmountKopecks {
		changes = append(changes, FieldChange{
			Field: ChangeFieldAmount,
			Old:   changeInt(before.AmountKopecks),
			New:   changeInt(after.AmountKopecks),
		})
	}
	if beforeRec, afterRec, moved := changeRecurrenceMoved(before.Recurrence, after.Recurrence); moved {
		changes = append(changes, FieldChange{
			Field: ChangeFieldRecurrence,
			Old:   beforeRec,
			New:   afterRec,
		})
	}
	if !changeCategoryEqual(before.Category, after.Category) {
		changes = append(changes, FieldChange{
			Field: ChangeFieldCategory,
			Old:   changeCategory(before.Category),
			New:   changeCategory(after.Category),
		})
	}
	if !changeDatesEqual(before.EndDate, after.EndDate) {
		changes = append(changes, FieldChange{
			Field: ChangeFieldEndDate,
			Old:   changeDate(before.EndDate),
			New:   changeDate(after.EndDate),
		})
	}
	if before.AutoPay != after.AutoPay {
		changes = append(changes, FieldChange{
			Field: ChangeFieldAutoPay,
			Old:   changeBool(before.AutoPay),
			New:   changeBool(after.AutoPay),
		})
	}
	if !changeReminderEqual(before.ReminderOffsetDays, after.ReminderOffsetDays) {
		changes = append(changes, FieldChange{
			Field: ChangeFieldReminderOffset,
			Old:   changeReminderOffset(before.ReminderOffsetDays),
			New:   changeReminderOffset(after.ReminderOffsetDays),
		})
	}
	return changes
}

// HasTitleChange reports whether the diff carries a title edit. By the
// diff-patch semantics a title in the patch is always a user edit (the
// frontend sends title only when the user touched it; the rental seam never
// edits titles), so this is the «Название изменено» chip's source and the
// cue that flips payments.title_is_manual (ADR 0065).
func HasTitleChange(changes []FieldChange) bool {
	for _, change := range changes {
		if change.Field == ChangeFieldTitle {
			return true
		}
	}
	return false
}

// changeJSON encodes one change value. The typed argument at every call site
// is provably safe (string/int/bool/plain structs), but the encoders keep the
// checked error and panic loudly: a marshal failure means a rule state the
// validators should never have let through (the shared/cursor.Encode canon).
func changeJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic("change log: marshal change value: " + err.Error())
	}
	return b
}

// changeString encodes a string change value.
func changeString(s string) json.RawMessage { return changeJSON(s) }

// changeInt encodes an integer change value (kopecks, reminder offsets).
func changeInt(i int64) json.RawMessage { return changeJSON(i) }

// changeBool encodes a boolean change value.
func changeBool(b bool) json.RawMessage { return changeJSON(b) }

// changeDate encodes an optional calendar date as "2006-01-02" or null.
func changeDate(date *time.Time) json.RawMessage {
	if date == nil {
		return json.RawMessage("null")
	}
	return changeJSON(date.Format(changeDateFormat))
}

// changeReminderOffset encodes the optional reminder offset (1/3/7 or null).
func changeReminderOffset(days *int) json.RawMessage {
	if days == nil {
		return json.RawMessage("null")
	}
	return changeInt(int64(*days))
}

// changeRecurrenceMoved compares two recurrences in their canonical JSON
// form — the constructors normalize the sets (sorted, unique), so equal
// schedules marshal to equal bytes. Returns the pair's encodings.
func changeRecurrenceMoved(before, after Recurrence) (beforeRaw, afterRaw json.RawMessage, moved bool) {
	beforeRaw, afterRaw = changeRecurrence(before), changeRecurrence(after)
	return beforeRaw, afterRaw, !bytes.Equal(beforeRaw, afterRaw)
}

// changeCategoryEqual compares category references: the reference moves, the
// resolved label cannot (it is re-resolved from the same reference within one
// write's transaction).
func changeCategoryEqual(before, after CategoryRef) bool {
	if (before.Slug == nil) != (after.Slug == nil) {
		return false
	}
	if before.Slug != nil && *before.Slug != *after.Slug {
		return false
	}
	if (before.UserCategoryID == nil) != (after.UserCategoryID == nil) {
		return false
	}
	if before.UserCategoryID != nil && *before.UserCategoryID != *after.UserCategoryID {
		return false
	}
	return true
}

// changeCategoryValue is the change value of the category field: the
// reference plus the label snapshot at edit time — the chip stays readable
// after the referenced user category is renamed or deleted (the snapshot
// canon of the operations' category_label, ADR 0049 §1).
type changeCategoryValue struct {
	Slug           *string `json:"slug,omitempty"`
	UserCategoryID *string `json:"userCategoryId,omitempty"`
	Label          string  `json:"label"`
}

// changeCategory encodes a category reference as its change value.
func changeCategory(ref CategoryRef) json.RawMessage {
	value := changeCategoryValue{Label: ref.SnapshotLabel()}
	switch {
	case ref.Slug != nil:
		value.Slug = ref.Slug
	case ref.UserCategoryID != nil:
		id := ref.UserCategoryID.String()
		value.UserCategoryID = &id
	}
	return changeJSON(value)
}

// changeRecurrence encodes the recurrence's canonical JSON form — the same
// shape the payments.recurrence column stores. The custom MarshalJSON can
// genuinely fail (an unknown kind would mean a rule that never passed
// validation); the error surfaces through changeJSON's panic.
func changeRecurrence(r Recurrence) json.RawMessage {
	return changeJSON(&r)
}

// changeDateFormat is the wire form of an optional calendar date value.
const changeDateFormat = "2006-01-02"

// changeReminderEqual compares two optional reminder offsets.
func changeReminderEqual(before, after *int) bool {
	switch {
	case before == nil && after == nil:
		return true
	case before == nil || after == nil:
		return false
	default:
		return *before == *after
	}
}

// changeDatesEqual compares two optional calendar dates.
func changeDatesEqual(a, b *time.Time) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	default:
		return a.Equal(*b)
	}
}
