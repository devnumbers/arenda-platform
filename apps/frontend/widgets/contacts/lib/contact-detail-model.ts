import { formatPhoneDisplay } from '@/shared/lib/phone';
import type { Contact } from '@/entities/contact';

/** Строка значения на карточке контакта (#510, макет 1285:55112): значение
 * с подписью и кнопкой копирования. Телефон показывается в маске,
 * мессенджер — имя пользователя с названием мессенджера в подписи. */
export type ContactValueRow = {
  readonly key: 'phone' | 'email' | 'messenger';
  readonly value: string;
  readonly label: string;
};

/** Заполненные контактные значения карточки — телефон, почта, мессенджер —
 * в порядке макета; пустые значения строк не дают. */
export function contactValueRows(contact: Contact): ContactValueRow[] {
  const rows: ContactValueRow[] = [];
  if (contact.phone !== '') {
    rows.push({ key: 'phone', value: formatPhoneDisplay(contact.phone), label: 'Телефон' });
  }
  if (contact.email !== '') {
    rows.push({ key: 'email', value: contact.email, label: 'Электронная почта' });
  }
  if (contact.messengerUsername !== '') {
    rows.push({
      key: 'messenger',
      value: contact.messengerUsername,
      label: contact.messengerName !== '' ? contact.messengerName : 'Мессенджер',
    });
  }
  return rows;
}
