import { describe, expect, it } from 'vitest';

import type { Contact } from '../model/types';
import { contactSortByRecent } from './contact-recent';

function contact(id: string, createdAt: string): Contact {
  return {
    id,
    propertyId: undefined,
    firstName: id,
    lastName: '',
    patronymic: '',
    role: '',
    phone: '',
    email: '',
    messengerName: '',
    messengerUsername: '',
    note: '',
    createdAt,
    updatedAt: createdAt,
  };
}

describe('contactSortByRecent', () => {
  it('ставит свежие контакты первыми (created_at DESC)', () => {
    const list = [
      contact('старый', '2026-01-05T10:00:00Z'),
      contact('новый', '2026-09-01T10:00:00Z'),
      contact('средний', '2026-05-20T10:00:00Z'),
    ];

    expect(contactSortByRecent(list).map((item) => item.id)).toStrictEqual([
      'новый',
      'средний',
      'старый',
    ]);
  });

  it('не меняет входной список и сохраняет порядок равных дат (стабильность)', () => {
    const a = contact('а', '2026-03-01T10:00:00Z');
    const b = contact('б', '2026-03-01T10:00:00Z');
    const list = [b, a, contact('старый', '2025-01-01T00:00:00Z')];

    const sorted = contactSortByRecent(list);

    expect(sorted).not.toBe(list);
    expect(sorted.map((item) => item.id)).toStrictEqual(['б', 'а', 'старый']);
    expect(list.map((item) => item.id)).toStrictEqual(['б', 'а', 'старый']);
  });
});
