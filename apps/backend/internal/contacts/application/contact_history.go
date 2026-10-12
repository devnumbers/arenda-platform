package application

// Диффы карточки для единой строки истории (решение владельца 12.10, карта
// #1324): «Контакт изменён: [новое ФИО]» + по пояснению на каждое
// изменившееся поле полными реальными значениями — PII включительно
// (аменда ADR 0061 §5: «не бойся показывать персональные данные»): ФИО,
// роль, телефон, почта, мессенджер, заметка (полным текстом без резки).

import (
	"github.com/nambers/arenda-planform/apps/backend/internal/contacts/domain"
	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
)

// contactHistoryChanges diffs the loaded card against the stored one for
// the action journal's unified change row. Поля — словарь предиката
// contactChanged: карточка сдвинулась → пояснения называют каждое поле.
func contactHistoryChanges(before, after domain.Contact) []historydomain.AttributeChange {
	var changes []historydomain.AttributeChange
	if before.FullName() != after.FullName() {
		changes = append(changes, historydomain.AttributeChange{
			Key: "full_name", Label: "ФИО",
			Old: before.FullName(), New: after.FullName(),
		})
	}
	if before.Role != after.Role {
		changes = append(changes, historydomain.AttributeChange{
			Key: "role", Label: "Роль",
			Old: before.Role, New: after.Role,
		})
	}
	if before.Phone != after.Phone {
		changes = append(changes, historydomain.AttributeChange{
			Key: "phone", Label: "Телефон",
			Old: before.Phone, New: after.Phone,
		})
	}
	if before.Email != after.Email {
		changes = append(changes, historydomain.AttributeChange{
			Key: "email", Label: "Email",
			Old: before.Email, New: after.Email,
		})
	}
	if before.MessengerName != after.MessengerName || before.MessengerUsername != after.MessengerUsername {
		changes = append(changes, historydomain.AttributeChange{
			Key: "messenger", Label: "Мессенджер",
			Old: before.MessengerLabel(), New: after.MessengerLabel(),
		})
	}
	if before.Note != after.Note {
		changes = append(changes, historydomain.AttributeChange{
			Key: "note", Label: "Заметка",
			Old: before.Note, New: after.Note,
		})
	}
	return changes
}
