import { describe, expect, it } from 'vitest';

import {
  buildContactCreateCommand,
  buildContactUpdateCommand,
  contactFormErrors,
  contactFormReady,
} from './contact-form';

const filled = {
  firstName: 'Александр',
  lastName: 'Петров',
  patronymic: 'Сергеевич',
  role: 'Арендатор',
  phone: '+7 (912) 345-67-89',
  email: 'a.petrov@mail.ru',
  messengerName: 'Телеграм',
  messengerUsername: 'apetrov',
  note: 'Код домофона 1234',
  propertyId: 'p1' as string | null,
};

describe('buildContactCreateCommand', () => {
  it('переносит поля формы в команду: трим по краям, телефон в +7XXXXXXXXXX', () => {
    expect(
      buildContactCreateCommand({ ...filled, firstName: '  Александр ', phone: '89123456789' }),
    ).toStrictEqual({
      propertyId: 'p1',
      firstName: 'Александр',
      lastName: 'Петров',
      patronymic: 'Сергеевич',
      role: 'Арендатор',
      phone: '+79123456789',
      email: 'a.petrov@mail.ru',
      messengerName: 'Телеграм',
      messengerUsername: 'apetrov',
      note: 'Код домофона 1234',
    });
  });

  it('складывает пустые и пробельные необязательные поля в пустые строки', () => {
    expect(
      buildContactCreateCommand({
        ...filled,
        lastName: '   ',
        patronymic: '',
        role: '',
        phone: '',
        email: '',
        messengerName: '',
        messengerUsername: '',
        note: '',
        propertyId: null,
      }),
    ).toStrictEqual({
      propertyId: null,
      firstName: 'Александр',
      lastName: '',
      patronymic: '',
      role: '',
      phone: '',
      email: '',
      messengerName: '',
      messengerUsername: '',
      note: '',
    });
  });
});

describe('contactFormErrors', () => {
  it('заполненная форма без ошибок', () => {
    expect(contactFormErrors(filled)).toStrictEqual({});
  });

  it('пустое имя — единственная обязательная ошибка', () => {
    expect(contactFormErrors({ ...filled, firstName: '   ' })).toStrictEqual({
      firstName: 'Укажите имя',
    });
  });

  it('частично набранный телефон — ошибка формата', () => {
    expect(contactFormErrors({ ...filled, phone: '+7 (912) 345' })).toStrictEqual({
      phone: 'Некорректный номер телефона',
    });
  });

  it('кривая почта — ошибка формата, пустая почта — не ошибка', () => {
    expect(contactFormErrors({ ...filled, email: 'не-почта' })).toStrictEqual({
      email: 'Некорректный адрес почты',
    });
    expect(contactFormErrors({ ...filled, email: '' })).toStrictEqual({});
  });

  it('несколько ошибок собираются разом', () => {
    expect(
      contactFormErrors({ ...filled, firstName: '', phone: '123', email: '@@' }),
    ).toStrictEqual({
      firstName: 'Укажите имя',
      phone: 'Некорректный номер телефона',
      email: 'Некорректный адрес почты',
    });
  });
});

describe('contactFormReady', () => {
  it('готова, когда имя непусто — даже с пустыми остальными полями', () => {
    expect(
      contactFormReady({ ...filled, lastName: '', phone: '', email: '', propertyId: null }),
    ).toBe(true);
  });

  it('не готова, пока имя из одних пробелов', () => {
    expect(contactFormReady({ ...filled, firstName: '  ' })).toBe(false);
  });
});

describe('buildContactUpdateCommand', () => {
  it('нормализует так же, как создание: трим, телефон в +7XXXXXXXXXX', () => {
    expect(
      buildContactUpdateCommand({
        ...filled,
        firstName: '  Александр ',
        phone: '89123456789',
        propertyId: null,
      }),
    ).toStrictEqual({
      propertyId: null,
      firstName: 'Александр',
      lastName: 'Петров',
      patronymic: 'Сергеевич',
      role: 'Арендатор',
      phone: '+79123456789',
      email: 'a.petrov@mail.ru',
      messengerName: 'Телеграм',
      messengerUsername: 'apetrov',
      note: 'Код домофона 1234',
    });
  });

  it('сброс привязки передаётся явным propertyId: null («без объекта»)', () => {
    expect(buildContactUpdateCommand({ ...filled, propertyId: null }).propertyId).toBeNull();
    expect(buildContactUpdateCommand({ ...filled }).propertyId).toBe('p1');
  });
});
