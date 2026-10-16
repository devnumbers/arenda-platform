import { describe, expect, it } from 'vitest';

import { historySegmentHref } from './segment-links';

describe('historySegmentHref', () => {
  it('ведёт каждый вид словаря на его существующую страницу (ADR 0061 §7: детали — существующие страницы сущностей)', () => {
    const propertyId = 'b1eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';
    const id = 'c1eebc99-9c0b-4ef8-bb6d-6bb9bd380a22';

    expect(historySegmentHref({ kind: 'property', id }, propertyId)).toBe(`/properties/${id}`);
    // Аренда по id: незавершённую страница аренды сама уводит на текущий экран.
    expect(historySegmentHref({ kind: 'rental', id }, propertyId)).toBe(
      `/properties/${propertyId}/rentals/${id}`,
    );
    expect(historySegmentHref({ kind: 'payment', id }, propertyId)).toBe(
      `/properties/${propertyId}/payments/${id}`,
    );
    expect(historySegmentHref({ kind: 'operation', id }, propertyId)).toBe(
      `/properties/${propertyId}/operations/${id}`,
    );
    expect(historySegmentHref({ kind: 'contact', id }, propertyId)).toBe(
      `/properties/${propertyId}/contacts/${id}`,
    );
    // Задача — правило: страница у вхождения одна — правка правила (канон списков задач).
    expect(historySegmentHref({ kind: 'task', id }, propertyId)).toBe(
      `/properties/${propertyId}/tasks/${id}/edit`,
    );
    // Участник — страница участника хаба (uuid юзера из ссылки).
    expect(historySegmentHref({ kind: 'member', id }, propertyId)).toBe(`/participants/${id}`);
  });

  it('неизвестный вид даёт null — фрагмент остаётся текстом', () => {
    expect(
      historySegmentHref({ kind: 'future' as 'property', id: 'x' }, 'p'),
    ).toBeNull();
  });
});
