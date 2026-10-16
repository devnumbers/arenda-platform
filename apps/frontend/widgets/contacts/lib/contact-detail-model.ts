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

/** Что показывает карточка «Информация» (#1334): строки заполненных полей
 * связи, а когда пусты все три — подпись «Нет информации» вместо строк
 * (макет 3304:76141, решение карты #1332). Карточка при любом раскладе
 * остаётся ссылкой на правку. */
export function contactInfoView(contact: Contact): {
  readonly rows: ContactValueRow[];
  readonly emptyCaption: string | null;
} {
  const rows = contactValueRows(contact);
  return { rows, emptyCaption: rows.length > 0 ? null : 'Нет информации' };
}
