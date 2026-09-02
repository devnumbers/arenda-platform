import { describe, expect, it } from 'vitest';
import type { Contact } from '../model/types';
import { contactFullName } from './full-name';

const contact: Contact = {
  id: '0198b6a7-1000-7000-8000-0000000000c1',
  propertyId: undefined,
  firstName: 'Анна',
  lastName: 'Петрова',
  patronymic: 'Сергеевна',
  role: 'сантехник',
  phone: '+79001234567',
  email: '',
  messengerName: '',
  messengerUsername: '',
  note: '',
  createdAt: '2026-09-01T09:00:00Z',
  updatedAt: '2026-09-01T09:00:00Z',
};

describe('contactFullName — отображаемое имя карточки', () => {
  it('полный набор — Имя Фамилия Отчество (зеркало FullName домена, ADR 0051)', () => {
    expect(contactFullName(contact)).toBe('Анна Петрова Сергеевна');
  });

  it('только обязательное имя', () => {
    expect(contactFullName({ ...contact, lastName: '', patronymic: '' })).toBe('Анна');
  });

  it('имя и фамилия без отчества — без двойных пробелов', () => {
    expect(contactFullName({ ...contact, patronymic: '' })).toBe('Анна Петрова');
  });
});
