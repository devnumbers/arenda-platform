package application

// Диффы карточки для пояснений единой строки (решение владельца 12.10,
// карта #1324; аменда ADR 0061 §5): все сдвинувшиеся поля полными реальными
// значениями, PII включительно — телефон, почта, мессенджер; заметка полным
// текстом без резки.

import (
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/contacts/domain"
	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
)

func contactHistoryTestContact(mutate func(c *domain.Contact)) domain.Contact {
	c := domain.Contact{
		ID:        uuid.Must(uuid.NewV7()),
		FirstName: "Пётр",
		LastName:  "Иванов",
		Role:      "сантехник",
		Phone:     "+79161234567",
	}
	if mutate != nil {
		mutate(&c)
	}
	return c
}

func TestContactHistoryChanges_CarriesEveryMovedField(t *testing.T) {
	t.Parallel()
	before := contactHistoryTestContact(nil)
	after := contactHistoryTestContact(func(c *domain.Contact) {
		c.FirstName = "Пётр"
		c.LastName = "Сидоров"
		c.Role = "электрик"
		c.Phone = "+79211112233"
		c.Email = "petr.sidorov@mail.ru"
		c.MessengerName = messengerTelegram
		c.MessengerUsername = "@petrov"
		c.Note = "код домофона 1234, звонить после шести"
	})

	changes := contactHistoryChanges(before, after)
	// ФИО собрано из частей — одна строка диффа, не по части на поле.
	entry := historydomain.ContactUpdated(after.ID, after.FullName(), changes)
	want := historydomain.DetailLines{
		`ФИО: «` + createdContactFullName + `» → «` + contactFirstName + " " + rebindLastName + `»`,
		`Роль: «сантехник» → «электрик»`,
		`Телефон: «+79161234567» → «+79211112233»`,
		`Email: «petr.sidorov@mail.ru»`,
		`Мессенджер: «` + messengerTelegram + ` @petrov»`,
		`Заметка: «код домофона 1234, звонить после шести»`,
	}
	if len(entry.Details) != len(want) {
		t.Fatalf("details = %q, want %q", entry.Details, want)
	}
	for i := range want {
		if entry.Details[i] != want[i] {
			t.Errorf("details[%d] = %q, want %q", i, entry.Details[i], want[i])
		}
	}
}

func TestContactHistoryChanges_MessengerLabelComposes(t *testing.T) {
	t.Parallel()
	// Пара мессенджера читается целиком: смена одного поля пары даёт дифф
	// собранной подписи; пустая пара читается как удаление.
	before := contactHistoryTestContact(func(c *domain.Contact) {
		c.MessengerName = "WhatsApp"
		c.MessengerUsername = "+79161234567"
	})
	after := contactHistoryTestContact(func(c *domain.Contact) {
		c.MessengerName = messengerTelegram
		c.MessengerUsername = "@petrov"
	})

	entry := historydomain.ContactUpdated(after.ID, after.FullName(), contactHistoryChanges(before, after))
	want := historydomain.DetailLines{`Мессенджер: «WhatsApp +79161234567» → «` + messengerTelegram + ` @petrov»`}
	if len(entry.Details) != 1 || entry.Details[0] != want[0] {
		t.Errorf("details = %q, want %q", entry.Details, want)
	}
}

func TestContactHistoryChanges_EqualCardsDiffToNil(t *testing.T) {
	t.Parallel()
	if changes := contactHistoryChanges(contactHistoryTestContact(nil), contactHistoryTestContact(nil)); changes != nil {
		t.Errorf("changes = %v, want nil", changes)
	}
}
