import type { Contact } from '../model/types';

/** Отображаемое имя карточки: непустые части в порядке Имя Фамилия Отчество —
 * зеркало FullName домена contacts (ADR 0054), та же форма в админке. */
export function contactFullName(contact: Contact): string {
  return [contact.firstName, contact.lastName, contact.patronymic]
    .filter((part) => part.length > 0)
    .join(' ');
}
