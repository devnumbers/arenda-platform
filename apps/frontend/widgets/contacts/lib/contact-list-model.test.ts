import { describe, expect, it } from 'vitest';
import type { Contact } from '@/entities/contact';
import { contactRowModel } from './contact-list-model';

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

describe('contactRowModel — модель строки списка контактов (#508, макет 1527:74139)', () => {
  it('заголовок — имя, подзаголовок — роль; телефона в строке нет', () => {
    expect(contactRowModel(contact)).toStrictEqual({
      title: 'Анна Петрова',
      subtitle: 'сантехник',
    });
  });

  it('без роли — строка без подзаголовка', () => {
    expect(contactRowModel({ ...contact, role: '' })).toStrictEqual({
      title: 'Анна Петрова',
      subtitle: undefined,
    });
  });
});
