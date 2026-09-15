import { describe, expect, it } from 'vitest';
import type { Property } from '@/entities/property';
import { resolvePropertiesLandingHref } from './property-landing';

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

describe('resolvePropertiesLandingHref (лендинг таба «Объекты»)', () => {
  it('данные ещё не загружены — список', () => {
    expect(resolvePropertiesLandingHref(undefined)).toBe('/properties');
  });

  it('пустая книга — список', () => {
    expect(resolvePropertiesLandingHref([])).toBe('/properties');
  });

  it('есть основной — его страница', () => {
    const properties = [
      makeProperty({ id: 'b' }),
      makeProperty({ id: 'a', pinned_at: '2026-09-01T00:00:00Z' }),
    ];
    expect(resolvePropertiesLandingHref(properties)).toBe('/properties/a');
  });

  it('несколько основных — первый отмеченный', () => {
    const properties = [
      makeProperty({ id: 'late', pinned_at: '2026-09-05T00:00:00Z' }),
      makeProperty({ id: 'early', pinned_at: '2026-09-01T00:00:00Z' }),
    ];
    expect(resolvePropertiesLandingHref(properties)).toBe('/properties/early');
  });

  it('основного нет, активный один — его страница', () => {
    expect(resolvePropertiesLandingHref([makeProperty({ id: 'only' })])).toBe('/properties/only');
  });

  it('единственный активный среди архивных — его страница', () => {
    const properties = [
      makeProperty({ id: 'only' }),
      makeProperty({ id: 'old', status: 'archived' }),
    ];
    expect(resolvePropertiesLandingHref(properties)).toBe('/properties/only');
  });

  it('два активных без основного — список', () => {
    expect(resolvePropertiesLandingHref([makeProperty({ id: 'a' }), makeProperty({ id: 'b' })])).toBe('/properties');
  });

  it('одни архивные — список', () => {
    expect(resolvePropertiesLandingHref([makeProperty({ id: 'old', status: 'archived' })])).toBe('/properties');
  });
});
