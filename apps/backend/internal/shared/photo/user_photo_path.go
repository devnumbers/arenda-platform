package photo

import (
	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/storage"
)

// UserPhotoPath is the profile photo's same-origin streaming path (ADR 0065;
// решение владельца #1286 — «показ участникам»): /api/users/{id}/photo,
// читается самим пользователем и связанными с ним по общему читаемому
// объекту. Пустая строка — фото нет (nil-ключ): поле контракта nullable, а
// снапшоты payload'ов держат пустую строку. Путь несёт версию ключа (?v=):
// замена фото = новый UUID-ключ = новый URL, кэш браузера никогда не
// показывает прежние байты. Стрим отдаёт фото, которое у профиля есть
// сейчас: удалённое или закрытое отзывом доступа фото отвечает 404, и
// клиент откатывается на заглушку.
func UserPhotoPath(userID uuid.UUID, photoKey *string) string {
	if photoKey == nil {
		return ""
	}
	path := "/api/users/" + userID.String() + "/photo"
	return storage.VersionedPhotoURL(path, *photoKey)
}
