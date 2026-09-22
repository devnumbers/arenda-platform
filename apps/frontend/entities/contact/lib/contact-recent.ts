import type { Contact } from '../model/types';

/**
 * Сортировка «по недавним» (макет 1855:64129, экран выбора арендатора):
 * свежие карточки сверху — created_at DESC. Список клиентский: книга
 * объекта идёт с сервера порциями по имени (#600), сортировка применяется
 * к уже загруженному срезу; равные даты сохраняют входной порядок
 * (стабильный sort). Вход не мутирует.
 */
export function contactSortByRecent(contacts: ReadonlyArray<Contact>): Contact[] {
  return [...contacts].sort(
    (a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime(),
  );
}
