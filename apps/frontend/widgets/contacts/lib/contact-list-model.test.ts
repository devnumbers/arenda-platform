import { describe, expect, it } from 'vitest';
import type { Contact } from '@/entities/contact';
import {
  contactRowModel,
  contactSortByName,
  groupContactsByLetter,
  type ContactSortOrder,
} from './contact-list-model';

const contact: Contact = {
  id: '0198b6a7-1000-7000-8000-0000000000c1',
  propertyId: '0198b6a7-1000-7000-8000-00000000prop',
  firstName: 'Анна',
  lastName: 'Петрова',
  patronymic: '',
  role: 'сантехник',
  phone: '+79001234567',
  email: '',
  messengerName: '',
  messengerUsername: '',
  note: '',
  createdAt: '2026-09-01T09:00:00Z',
  updatedAt: '2026-09-01T09:00:00Z',
};

const named = (firstName: string, lastName = '', role = ''): Contact => ({
  ...contact,
  id: `id-${firstName}-${lastName}`,
  firstName,
  lastName,
  role,
});

describe('contactRowModel — модель строки списка контактов (#508, макет 1527:74139)', () => {
  it('заголовок — имя, подзаголовок — роль; телефона в строке нет', () => {
    expect(contactRowModel(contact)).toStrictEqual({
      title: 'Анна Петрова',
      subtitle: 'сантехник',
    });
  });

  it('без роли — строка без подзаголовка', () => {
    expect(contactRowModel(named('ООО «Чистый дом»'))).toStrictEqual({
      title: 'ООО «Чистый дом»',
      subtitle: undefined,
    });
  });
});

describe('contactSortByName — сортировка по имени (кнопка «Имя», макет 1539:85395)', () => {
  const list = [named('Григорий'), named('борис'), named('Анна'), named('Анна', 'Архипова')];

  it('asc — от А до Я, регистр не влияет', () => {
    const sorted = contactSortByName(list, 'asc').map((item) => item.firstName);
    expect(sorted).toStrictEqual(['Анна', 'Анна', 'борис', 'Григорий']);
  });

  it('desc — от Я до А', () => {
    const sorted = contactSortByName(list, 'desc').map((item) => item.firstName);
    expect(sorted).toStrictEqual(['Григорий', 'борис', 'Анна', 'Анна']);
  });
});

describe('groupContactsByLetter — алфавитные группы (макет 1527:74139)', () => {
  it('группирует по первой букве имени в верхнем регистре, порядок групп — по входному списку', () => {
    const groups = groupContactsByLetter([
      named('Анна'),
      named('артем'),
      named('Борис'),
      named('Анна', 'Архипова'),
    ]);

    expect(groups.map((group) => group.letter)).toStrictEqual(['А', 'Б']);
    expect(groups[0]?.contacts.map((item) => item.firstName)).toStrictEqual(['Анна', 'артем', 'Анна']);
    expect(groups[1]?.contacts.map((item) => item.firstName)).toStrictEqual(['Борис']);
  });

  it('пустой список — без групп', () => {
    expect(groupContactsByLetter([])).toStrictEqual([]);
  });
});

describe('ContactSortOrder', () => {
  it('два направления — asc и desc', () => {
    const orders: ContactSortOrder[] = ['asc', 'desc'];
    expect(orders).toHaveLength(2);
  });
});
