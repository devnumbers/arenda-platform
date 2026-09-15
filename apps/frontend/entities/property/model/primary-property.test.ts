import { describe, expect, it } from 'vitest';
import type { Property } from '@/entities/property';
import { comparePrimaryProperty } from './primary-property';

function makeProperty(overrides: Partial<Property> & { id: string }): Property {
  return {
    name: 'Квартира',
    type: 'apartment',
    address: 'Москва, ул. Ленина, 1',
    attributes: {},
    status: 'active',
    members_count: 0,
    created_at: '2026-01-01T00:00:00Z',
    pinned_at: null,
    ...overrides,
  };
}

describe('comparePrimaryProperty (канон «Основной объект», #584)', () => {
  it('основной выше не-основного', () => {
    const primary = makeProperty({ id: 'p', pinned_at: '2026-09-01T00:00:00Z' });
    const plain = makeProperty({ id: 'x' });
    expect(comparePrimaryProperty(primary, plain)).toBeLessThan(0);
    expect(comparePrimaryProperty(plain, primary)).toBeGreaterThan(0);
  });

  it('среди основных — первый отмеченный (раньший pinned_at) выше', () => {
    const early = makeProperty({ id: 'early', pinned_at: '2026-09-01T00:00:00Z' });
    const late = makeProperty({ id: 'late', pinned_at: '2026-09-05T00:00:00Z' });
    expect(comparePrimaryProperty(early, late)).toBeLessThan(0);
    expect(comparePrimaryProperty(late, early)).toBeGreaterThan(0);
  });

  it('одинаковое время отметки — эквивалентны', () => {
    const at = '2026-09-01T00:00:00Z';
    const a = makeProperty({ id: 'a', pinned_at: at });
    const b = makeProperty({ id: 'b', pinned_at: at });
    expect(comparePrimaryProperty(a, b)).toBe(0);
  });

  it('оба не-основные — эквивалентны', () => {
    expect(comparePrimaryProperty(makeProperty({ id: 'a' }), makeProperty({ id: 'b' }))).toBe(0);
  });
});
