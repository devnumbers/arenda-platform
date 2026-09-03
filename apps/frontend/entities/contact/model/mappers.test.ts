import { describe, expect, it } from 'vitest';
import type { components } from '@/shared/api/dto';
import { mapContact } from './mappers';

type ContactDto = components['schemas']['ContactResponse'];

const contactDto: ContactDto = {
  id: '0198b6a7-1000-7000-8000-0000000000c1',
  propertyId: '0198b6a7-1000-7000-8000-00000000prop',
  firstName: 'Анна',
  lastName: 'Петрова',
  patronymic: 'Сергеевна',
  role: 'сантехник',
  phone: '+79001234567',
  email: 'anna@example.ru',
  messengerName: 'Telegram',
  messengerUsername: '@anna_fix',
  note: 'Код домофона 1234',
  createdAt: '2026-09-01T09:00:00Z',
  updatedAt: '2026-09-02T10:30:00Z',
};

describe('mapContact — DTO → entity', () => {
  it('скалярные поля переносятся один в один (контракт camelCase)', () => {
    const contact = mapContact(contactDto);

    expect(contact.id).toBe(contactDto.id);
    expect(contact.firstName).toBe('Анна');
    expect(contact.lastName).toBe('Петрова');
    expect(contact.patronymic).toBe('Сергеевна');
    expect(contact.role).toBe('сантехник');
    expect(contact.phone).toBe('+79001234567');
    expect(contact.email).toBe('anna@example.ru');
    expect(contact.messengerName).toBe('Telegram');
    expect(contact.messengerUsername).toBe('@anna_fix');
    expect(contact.note).toBe('Код домофона 1234');
    expect(contact.createdAt).toBe('2026-09-01T09:00:00Z');
    expect(contact.updatedAt).toBe('2026-09-02T10:30:00Z');
  });

  it('привязка к объекту переносится как есть', () => {
    expect(mapContact(contactDto).propertyId).toBe('0198b6a7-1000-7000-8000-00000000prop');
  });

  it('контакт «без объекта» (wire null) — propertyId undefined', () => {
    const unbound = mapContact({ ...contactDto, propertyId: null });

    expect(unbound.propertyId).toBeUndefined();
  });
});
