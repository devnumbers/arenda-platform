package photo

import (
	"github.com/google/uuid"
)

// UserPhotoPath is the profile photo's same-origin streaming path (ADR 0065;
// решение владельца #1286 — «показ участникам»): /api/v1/users/{id}/photo,
// читается самим пользователем и связанными с ним по общему читаемому
// объекту. Пустая строка — фото нет (nil-ключ): поле контракта nullable, а
// снапшоты payload'ов держат пустую строку. Стрим отдаёт фото, которое у
// профиля есть сейчас: удалённое или закрытое отзывом доступа фото отвечает
// 404, и клиент откатывается на заглушку.
func UserPhotoPath(userID uuid.UUID, photoKey *string) string {
	if photoKey == nil {
		return ""
	}
	return "/api/v1/users/" + userID.String() + "/photo"
}
