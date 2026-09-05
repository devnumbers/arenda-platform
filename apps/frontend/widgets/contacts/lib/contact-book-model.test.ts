import { describe, expect, it } from 'vitest';

import type { Contact } from '@/entities/contact';
import {
  UNBOUND_GROUP_LABEL,
  contactBookRowSubtitle,
  groupBookByLetter,
  groupBookByProperty,
  parseContactBookSortParams,
} from './contact-book-model';

const contact = (overrides: Partial<Contact>): Contact => ({
  id: 'c1',
  propertyId: undefined,
  firstName: 'Анна',
  lastName: '',
  patronymic: '',
  role: '',
  phone: '',
  email: '',
  messengerName: '',
  messengerUsername: '',
  note: '',
  createdAt: '2026-09-04T10:00:00Z',
  updatedAt: '2026-09-04T10:00:00Z',
  ...overrides,
});

describe('groupBookByLetter — группы книги при сортировке по имени', () => {
  it('группирует по первой букве ФИО, порядок следует входному списку', () => {
    const groups = groupBookByLetter([
      contact({ id: '1', firstName: 'Анна' }),
      contact({ id: '2', firstName: 'Артём' }),
      contact({ id: '3', firstName: 'Борис' }),
    ]);

    expect(groups.map((group) => group.label)).toEqual(['А', 'Б']);
    expect(groups[0]?.contacts.map((c) => c.id)).toEqual(['1', '2']);
    expect(groups[1]?.contacts.map((c) => c.id)).toEqual(['3']);
  });
});

describe('groupBookByProperty — группы книги при сортировке по объекту', () => {
  it('безобъектные попадают в группу «Общие контакты», остальные — по имени объекта', () => {
    const groups = groupBookByProperty([
      contact({ id: '1', propertyId: undefined }),
      contact({ id: '2', propertyId: 'p1', propertyName: 'Моя квартира' }),
      contact({ id: '3', propertyId: 'p1', propertyName: 'Моя квартира' }),
      contact({ id: '4', propertyId: 'p2', propertyName: 'Студия' }),
    ]);

    expect(groups.map((group) => group.label)).toEqual([
      UNBOUND_GROUP_LABEL,
      'Моя квартира',
      'Студия',
    ]);
    expect(groups[0]?.contacts.map((c) => c.id)).toEqual(['1']);
    expect(groups[1]?.contacts.map((c) => c.id)).toEqual(['2', '3']);
  });

  it('порядок групп следует входному (серверному) порядку — убывание не переставляет группы', () => {
    const groups = groupBookByProperty([
      contact({ id: '1', propertyId: 'p2', propertyName: 'Студия' }),
      contact({ id: '2', propertyId: 'p1', propertyName: 'Моя квартира' }),
      contact({ id: '3' }),
    ]);

    expect(groups.map((group) => group.label)).toEqual([
      'Студия',
      'Моя квартира',
      UNBOUND_GROUP_LABEL,
    ]);
  });
});

describe('contactBookRowSubtitle — подзаголовок строки книги', () => {
  it('роль и объект — «Роль (Объект)»', () => {
    expect(
      contactBookRowSubtitle(
        contact({ role: 'Электрик', propertyId: 'p1', propertyName: 'Услуги' }),
      ),
    ).toBe('Электрик (Услуги)');
  });

  it('без объекта — только роль', () => {
    expect(contactBookRowSubtitle(contact({ role: 'Арендатор' }))).toBe('Арендатор');
  });

  it('без роли, но с объектом — только объект', () => {
    expect(
      contactBookRowSubtitle(contact({ propertyId: 'p1', propertyName: 'Услуги' })),
    ).toBe('Услуги');
  });

  it('не задано ничего — подзаголовка нет', () => {
    expect(contactBookRowSubtitle(contact({}))).toBeUndefined();
  });
});

describe('parseContactBookSortParams — разбор ?sort=&order= книги', () => {
  it('известные значения проходят', () => {
    expect(parseContactBookSortParams('property', 'desc')).toEqual({
      sort: 'property',
      order: 'desc',
    });
    expect(parseContactBookSortParams('name', 'asc')).toEqual({ sort: 'name', order: 'asc' });
  });

  it('отсутствующие и неизвестные — дефолт (имя по возрастанию)', () => {
    expect(parseContactBookSortParams(undefined, undefined)).toEqual({
      sort: 'name',
      order: 'asc',
    });
    expect(parseContactBookSortParams('bogus', 'sideways')).toEqual({
      sort: 'name',
      order: 'asc',
    });
  });

  it('массив значений трактуется как отсутствие', () => {
    expect(parseContactBookSortParams(['property'], ['desc'])).toEqual({
      sort: 'name',
      order: 'asc',
    });
  });
});
