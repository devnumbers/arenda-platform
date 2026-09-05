import { contactFullName, type Contact } from '@/entities/contact';

/**
 * Клиентская модель плоской книги (глобальная страница контактов, макеты
 * 1726:65083/65136/85937). Порядок строк задаёт сервер (sort/order GET
 * /contacts, русская коллация совпадает с клиентской группировкой) — здесь
 * только группировка следом за входным порядком и модель строки.
 */

export type ContactBookGroup = {
  readonly label: string;
  readonly contacts: Contact[];
};

/** Заголовок группы контактов без объекта (макет 1726:85937). */
export const UNBOUND_GROUP_LABEL = 'Общие контакты';

/** Разбор ?sort=&order= строки книги (конвенция страницы «Объекты»):
 * неизвестные и отсутствующие значения — дефолт (имя по возрастанию). */
export function parseContactBookSortParams(
  sort: string | string[] | undefined,
  order: string | string[] | undefined,
): { sort: 'name' | 'property'; order: 'asc' | 'desc' } {
  const sortValue = typeof sort === 'string' ? sort : '';
  const orderValue = typeof order === 'string' ? order : '';
  return {
    sort: sortValue === 'property' ? 'property' : 'name',
    order: orderValue === 'desc' ? 'desc' : 'asc',
  };
}

/**
 * Группы книги при сортировке по имени (макет 1726:65083): буква — первый
 * символ ФИО в верхнем регистре; порядок групп и строк следует входному
 * (уже серверно отсортированному) списку.
 */
export function groupBookByLetter(
  contacts: ReadonlyArray<Contact>,
): ReadonlyArray<ContactBookGroup> {
  const groups: ContactBookGroup[] = [];
  const byLetter = new Map<string, Contact[]>();
  for (const contact of contacts) {
    const letter = contactFullName(contact).charAt(0).toUpperCase();
    let bucket = byLetter.get(letter);
    if (bucket === undefined) {
      bucket = [];
      byLetter.set(letter, bucket);
      groups.push({ label: letter, contacts: bucket });
    }
    bucket.push(contact);
  }
  return groups;
}

/**
 * Группы книги при сортировке по объекту (макет 1726:85937): «Общие
 * контакты» (без объекта — сервер держит их первыми в обоих направлениях),
 * дальше — по одному имени объекта; порядок групп и строк внутри следует
 * входному (серверному) порядку.
 */
export function groupBookByProperty(
  contacts: ReadonlyArray<Contact>,
): ReadonlyArray<ContactBookGroup> {
  const groups: ContactBookGroup[] = [];
  const byLabel = new Map<string, Contact[]>();
  for (const contact of contacts) {
    const label = contact.propertyName ?? UNBOUND_GROUP_LABEL;
    let bucket = byLabel.get(label);
    if (bucket === undefined) {
      bucket = [];
      byLabel.set(label, bucket);
      groups.push({ label, contacts: bucket });
    }
    bucket.push(contact);
  }
  return groups;
}

/**
 * Подзаголовок строки книги (макеты 1726:65083/85937): роль, у
 * привязанного — роль с объектом в скобках («Электрик (Услуги)»); без роли —
 * только имя объекта; не задано ничего — подзаголовка нет.
 */
export function contactBookRowSubtitle(contact: Contact): string | undefined {
  const role = contact.role.trim();
  const property = contact.propertyName ?? '';
  if (role !== '' && property !== '') {
    return `${role} (${property})`;
  }
  return role !== '' ? role : property !== '' ? property : undefined;
}
