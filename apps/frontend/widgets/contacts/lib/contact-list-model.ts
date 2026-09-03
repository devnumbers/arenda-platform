import { contactFullName } from '@/entities/contact';
import type { Contact } from '@/entities/contact';

/** Направление сортировки по имени (кнопка «Имя», макет 1539:85395). */
export type ContactSortOrder = 'asc' | 'desc';

/** Локальная сортировка по имени: русская коллация, регистр не важен. */
const nameCollator = new Intl.Collator('ru');

/**
 * Сортировка списка по ФИО: «Имя от А до Я» (asc) / «Имя от Я до А» (desc).
 * Список клиентский (сервер порядок книги не задаёт), бекенд не участвует.
 */
export function contactSortByName(contacts: ReadonlyArray<Contact>, order: ContactSortOrder): Contact[] {
  return [...contacts].sort((a, b) => nameCollator.compare(contactFullName(a), contactFullName(b)) * (order === 'asc' ? 1 : -1));
}

/**
 * Алфавитные группы списка (макет 1527:74139): буква — первый символ ФИО в
 * верхнем регистре; порядок групп и строк следует входному (уже
 * отсортированному) списку.
 */
export function groupContactsByLetter(
  contacts: ReadonlyArray<Contact>,
): ReadonlyArray<{ letter: string; contacts: Contact[] }> {
  const groups: { letter: string; contacts: Contact[] }[] = [];
  const byLetter = new Map<string, Contact[]>();
  for (const contact of contacts) {
    const letter = contactFullName(contact).charAt(0).toUpperCase();
    let bucket = byLetter.get(letter);
    if (bucket === undefined) {
      bucket = [];
      byLetter.set(letter, bucket);
      groups.push({ letter, contacts: bucket });
    }
    bucket.push(contact);
  }
  return groups;
}

/**
 * Модель строки списка (#508, макет 1527:74139): заголовок — имя, подзаголовок
 * — роль (без роли строки-подзаголовка нет). Телефон в строке не показывается.
 */
export function contactRowModel(contact: Contact): {
  title: string;
  subtitle: string | undefined;
} {
  return {
    title: contactFullName(contact),
    subtitle: contact.role.length > 0 ? contact.role : undefined,
  };
}
