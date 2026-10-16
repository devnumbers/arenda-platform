import { describe, expect, it } from 'vitest';
import { contactSortByName, groupContactsByLetter } from './contact-alphabet';
import type { Contact } from '../model/types';

function contact(id: string, firstName: string, lastName = ''): Contact {
  return {
    id,
    propertyId: 'property-1',
    firstName,
    lastName,
    patronymic: '',
    role: '',
    phone: '',
    email: '',
    messengerName: '',
    messengerUsername: '',
    note: '',
    createdAt: '2026-09-05T00:00:00Z',
    updatedAt: '2026-09-05T00:00:00Z',
  };
}

const BOOK = [contact('1', 'Александр'), contact('2', 'Борис', 'Кузнецов'), contact('3', 'Артем')];

describe('contactSortByName', () => {
  it('сортирует по полному имени по возрастанию и убыванию, не мутируя вход', () => {
    const asc = contactSortByName(BOOK, 'asc');
    expect(asc.map((item) => item.id)).toStrictEqual(['1', '3', '2']);
    const desc = contactSortByName(BOOK, 'desc');
    expect(desc.map((item) => item.id)).toStrictEqual(['2', '3', '1']);
    expect(BOOK.map((item) => item.id)).toStrictEqual(['1', '2', '3']);
  });

  it('регистр не влияет', () => {
    const list = [contact('1', 'Григорий'), contact('2', 'борис'), contact('3', 'Анна')];
    expect(contactSortByName(list, 'asc').map((item) => item.firstName)).toStrictEqual([
      'Анна',
      'борис',
      'Григорий',
    ]);
  });
});

describe('groupContactsByLetter', () => {
  it('группирует по первой букве ФИО в верхнем регистре, следуя порядку входа', () => {
    const sorted = contactSortByName(BOOK, 'asc');
    expect(groupContactsByLetter(sorted)).toStrictEqual([
      { letter: 'А', contacts: [sorted[0], sorted[1]] },
      { letter: 'Б', contacts: [sorted[2]] },
    ]);
  });

  it('строчная первая буква поднимается в верхний регистр', () => {
    const groups = groupContactsByLetter([contact('1', 'артем')]);
    expect(groups.map((group) => group.letter)).toStrictEqual(['А']);
  });

  it('пустой список — без групп', () => {
    expect(groupContactsByLetter([])).toStrictEqual([]);
  });
});
