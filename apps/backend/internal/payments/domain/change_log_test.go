package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ruleFixture builds a fully valid rule; the diff cases mutate single fields.
func ruleFixture(t *testing.T) Payment {
	t.Helper()
	monthly, err := NewMonthlyRecurrence([]int{1}, false)
	require.NoError(t, err)
	return Payment{
		ID:            uuidMustV7(),
		OwnerID:       uuidMustV7(),
		PropertyID:    uuidMustV7(),
		Type:          TypeExpense,
		Title:         "Юридические услуги",
		AmountKopecks: 250000,
		Recurrence:    monthly,
		Since:         d("2026-10-01"),
		AutoPay:       true,
	}
}

func TestDiffPaymentChanges_Title(t *testing.T) {
	t.Parallel()
	before := ruleFixture(t)
	after := before
	after.Title = "Юрист — октябрь"

	changes := DiffPaymentChanges(before, after)
	require.Len(t, changes, 1, "one field edited — one change")

	change := changes[0]
	assert.Equal(t, ChangeFieldTitle, change.Field)
	var oldTitle, newTitle string
	require.NoError(t, json.Unmarshal(change.Old, &oldTitle))
	require.NoError(t, json.Unmarshal(change.New, &newTitle))
	assert.Equal(t, "Юридические услуги", oldTitle)
	assert.Equal(t, "Юрист — октябрь", newTitle)

	assert.True(t, HasTitleChange(changes), "title edits carry «Название изменено»")
}

func TestHasTitleChange(t *testing.T) {
	t.Parallel()
	t.Run("no title row — not manual", func(t *testing.T) {
		t.Parallel()
		assert.False(t, HasTitleChange(nil))
		assert.False(t, HasTitleChange([]FieldChange{}))
	})
}

func TestDiffPaymentChanges_FullDictionary(t *testing.T) {
	t.Parallel()
	before := ruleFixture(t)
	userCategoryID := uuidMustV7()
	before.Category = CategoryRef{UserCategoryID: &userCategoryID, UserCategoryName: new("Няня")}

	after := before
	after.Type = TypeIncome
	after.Title = "Няня — октябрь"
	after.AmountKopecks = 300000
	weekly, err := NewWeeklyRecurrence([]time.Weekday{time.Monday, time.Thursday})
	require.NoError(t, err)
	after.Recurrence = weekly
	after.Category = CategoryRef{Slug: new("rent")}
	end := d("2026-12-31")
	after.EndDate = &end
	after.AutoPay = false
	after.ReminderOffsetDays = new(3)

	changes := DiffPaymentChanges(before, after)

	// Словарный порядок: чипы экрана читаются в этом порядке.
	fields := make([]ChangeField, len(changes))
	for i, change := range changes {
		fields[i] = change.Field
	}
	assert.Equal(t, []ChangeField{
		ChangeFieldType,
		ChangeFieldTitle,
		ChangeFieldAmount,
		ChangeFieldRecurrence,
		ChangeFieldCategory,
		ChangeFieldEndDate,
		ChangeFieldAutoPay,
		ChangeFieldReminderOffset,
	}, fields, "every dictionary field moved — the row lists all eight in order")

	var oldType, newType string
	require.NoError(t, json.Unmarshal(changes[0].Old, &oldType))
	require.NoError(t, json.Unmarshal(changes[0].New, &newType))
	assert.Equal(t, "expense", oldType)
	assert.Equal(t, "income", newType)

	var oldAmount, newAmount int64
	require.NoError(t, json.Unmarshal(changes[2].Old, &oldAmount))
	require.NoError(t, json.Unmarshal(changes[2].New, &newAmount))
	assert.Equal(t, int64(250000), oldAmount, "kopecks travel as numbers")
	assert.Equal(t, int64(300000), newAmount)

	// Регулярность — канонический JSON-объект, как в payments.recurrence.
	var newRecurrence struct {
		Kind     string `json:"kind"`
		Weekdays []int  `json:"weekdays"`
	}
	require.NoError(t, json.Unmarshal(changes[3].New, &newRecurrence))
	assert.Equal(t, string(RecurrenceWeekly), newRecurrence.Kind)
	assert.Equal(t, []int{1, 4}, newRecurrence.Weekdays)

	// Категория — ссылка + снапшот лейбла на момент правки (лейбл живёт и
	// после удаления пользовательской категории).
	var oldCategory, newCategory map[string]any
	require.NoError(t, json.Unmarshal(changes[4].Old, &oldCategory))
	require.NoError(t, json.Unmarshal(changes[4].New, &newCategory))
	assert.Equal(t, map[string]any{"userCategoryId": userCategoryID.String(), "label": "Няня"}, oldCategory)
	assert.Equal(t, map[string]any{"slug": "rent", "label": "Арендная плата"}, newCategory)

	// Даты — «2006-01-02» либо null.
	var newEnd *string
	require.NoError(t, json.Unmarshal(changes[5].New, &newEnd))
	require.NotNil(t, newEnd)
	assert.Equal(t, "2026-12-31", *newEnd)

	var oldAutoPay, newAutoPay bool
	require.NoError(t, json.Unmarshal(changes[6].Old, &oldAutoPay))
	require.NoError(t, json.Unmarshal(changes[6].New, &newAutoPay))
	assert.True(t, oldAutoPay)
	assert.False(t, newAutoPay)

	var newReminder *int
	require.NoError(t, json.Unmarshal(changes[7].New, &newReminder))
	require.NotNil(t, newReminder)
	assert.Equal(t, 3, *newReminder)
}

func TestDiffPaymentChanges_ClearingOptionals(t *testing.T) {
	t.Parallel()
	before := ruleFixture(t)
	end := d("2026-12-31")
	before.EndDate = &end
	before.ReminderOffsetDays = new(7)
	after := before
	after.EndDate = nil
	after.ReminderOffsetDays = nil

	changes := DiffPaymentChanges(before, after)
	require.Len(t, changes, 2)

	assert.Equal(t, ChangeFieldEndDate, changes[0].Field)
	assert.Equal(t, ChangeFieldReminderOffset, changes[1].Field)

	assert.JSONEq(t, `"2026-12-31"`, string(changes[0].Old))
	assert.JSONEq(t, `null`, string(changes[0].New), "cleared end date travels as null")

	assert.JSONEq(t, `7`, string(changes[1].Old))
	assert.JSONEq(t, `null`, string(changes[1].New), "cleared reminder travels as null")
}

func TestDiffPaymentChanges_MonthlyMultiDayRecurrence(t *testing.T) {
	t.Parallel()
	// Пример владельца из макета: чип «Регулярность изменена: каждый месяц,
	// 1, 10, 15 числа» — значение едет структурой, текст рендерит фронт
	// (recurrenceLabel, резолюция #452).
	before := ruleFixture(t)
	after := before
	monthly, err := NewMonthlyRecurrence([]int{15, 1, 10}, false)
	require.NoError(t, err)
	after.Recurrence = monthly

	changes := DiffPaymentChanges(before, after)
	require.Len(t, changes, 1)
	assert.Equal(t, ChangeFieldRecurrence, changes[0].Field)

	var oldRecurrence, newRecurrence struct {
		Kind        string `json:"kind"`
		DaysOfMonth []int  `json:"daysOfMonth"`
	}
	require.NoError(t, json.Unmarshal(changes[0].Old, &oldRecurrence))
	require.NoError(t, json.Unmarshal(changes[0].New, &newRecurrence))
	assert.Equal(t, string(RecurrenceMonthly), oldRecurrence.Kind)
	assert.Equal(t, []int{1}, oldRecurrence.DaysOfMonth)
	assert.Equal(t, string(RecurrenceMonthly), newRecurrence.Kind)
	assert.Equal(t, []int{1, 10, 15}, newRecurrence.DaysOfMonth,
		"the constructor's sorted canonical set is what travels")
}

func TestDiffPaymentChanges_NoChanges(t *testing.T) {
	t.Parallel()
	rule := ruleFixture(t)
	assert.Nil(t, DiffPaymentChanges(rule, rule), "equal rules diff to nil — no row is written")
}

func TestDiffPaymentChanges_TitleOnlyWhenChanged(t *testing.T) {
	t.Parallel()
	before := ruleFixture(t)
	after := before
	after.AmountKopecks = before.AmountKopecks + 1

	changes := DiffPaymentChanges(before, after)
	require.Len(t, changes, 1)
	assert.Equal(t, ChangeFieldAmount, changes[0].Field)
	assert.False(t, HasTitleChange(changes), "a title untouched by the edit carries no «Название изменено»")
}
