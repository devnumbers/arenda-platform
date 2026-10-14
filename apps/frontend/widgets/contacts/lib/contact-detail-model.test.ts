import { describe, expect, it } from 'vitest';

import type { Contact } from '@/entities/contact';
import { contactInfoView, contactValueRows } from './contact-detail-model';

const contact: Contact = {
  id: 'c1',
  propertyId: 'p1',
  firstName: 'Александр',
  lastName: 'Иванов',
  patronymic: 'Сергеевич',
  role: 'Арендатор',
  phone: '+79931231212',
  email: 'email@yandex.ru',
  messengerName: 'Телеграм',
  messengerUsername: 'username',
  note: 'Код домофона 1234',
  createdAt: '2026-09-03T10:00:00Z',
  updatedAt: '2026-09-03T10:00:00Z',
};

describe('contactValueRows', () => {
  it('строки телефона, почты и мессенджера — в порядке макета', () => {
    expect(contactValueRows(contact)).toStrictEqual([
      { key: 'phone', value: '+7 (993) 123-12-12', label: 'Телефон' },
      { key: 'email', value: 'email@yandex.ru', label: 'Электронная почта' },
      { key: 'messenger', value: 'username', label: 'Телеграм' },
    ]);
  });

  it('пустые значения не дают строк', () => {
    expect(
      contactValueRows({ ...contact, phone: '', email: '', messengerUsername: '' }),
    ).toStrictEqual([]);
  });

  it('имя пользователя без названия мессенджера — подпись «Мессенджер»', () => {
    expect(
      contactValueRows({ ...contact, email: '', messengerName: '' }),
    ).toStrictEqual([
      { key: 'phone', value: '+7 (993) 123-12-12', label: 'Телефон' },
      { key: 'messenger', value: 'username', label: 'Мессенджер' },
    ]);
  });
});

describe('contactInfoView — карточка «Информация» (#1334, макет 3304:76141)', () => {
  it('заполненные поля — строки без подписи пустоты', () => {
    const view = contactInfoView(contact);

    expect(view.emptyCaption).toBeNull();
    expect(view.rows.map((row) => row.key)).toStrictEqual(['phone', 'email', 'messenger']);
  });

  it('пусты все три поля связи — подпись «Нет информации», строк нет', () => {
    const view = contactInfoView({
      ...contact,
      phone: '',
      email: '',
      messengerUsername: '',
    });

    expect(view.emptyCaption).toBe('Нет информации');
    expect(view.rows).toStrictEqual([]);
  });

  it('частичная заполненность — только непустые строки, подписи нет (решение при чартинге)', () => {
    expect(contactInfoView({ ...contact, email: '', messengerUsername: '' }).rows).toStrictEqual([
      { key: 'phone', value: '+7 (993) 123-12-12', label: 'Телефон' },
    ]);
    expect(
      contactInfoView({ ...contact, phone: '', messengerUsername: '' }).emptyCaption,
    ).toBeNull();
    expect(contactInfoView({ ...contact, phone: '', email: '' }).rows).toStrictEqual([
      { key: 'messenger', value: 'username', label: 'Телеграм' },
    ]);
  });
});
