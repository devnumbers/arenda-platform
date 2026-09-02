import { contactFullName } from '@/entities/contact';
import type { Contact } from '@/entities/contact';

/**
 * Модель строки списка контактов (#508): первая строка — ФИО + роль через
 * разделитель (роль — свободный текст, необязательная), вторая — телефон;
 * без телефона вторая строка не рисуется.
 */
export function contactRowModel(contact: Contact): {
  title: string;
  subtitle: string | undefined;
} {
  const name = contactFullName(contact);
  return {
    title: contact.role.length > 0 ? `${name} · ${contact.role}` : name,
    subtitle: contact.phone.length > 0 ? contact.phone : undefined,
  };
}
