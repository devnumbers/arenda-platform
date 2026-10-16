import type { Contact } from '../model/types';

/**
 * Сортировка «по недавним» (макет 1855:64129, экран выбора арендатора):
 * свежие карточки сверху — created_at DESC. Основной порядок теперь
 * серверный — GET /contacts?sort=created (#847): свежая карточка наверху
 * при книге любой длины. Эта функция — деградация для тёплого кэша:
 * срез, загруженный старым запросом (порции по имени, #600), или ответ
 * бэкенда без оси created она сортирует по уже загруженным строкам — за
 * пределами загруженного среза она ничего не видит. Равные даты сохраняют
 * входной порядок (стабильный sort). Вход не мутирует.
 */
export function contactSortByRecent(contacts: ReadonlyArray<Contact>): Contact[] {
  return [...contacts].sort(
    (a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime(),
  );
}
