import { describe, expect, it } from 'vitest';
import type { Property } from '@/entities/property';
import {
  comparePrimaryProperty,
  effectiveSelectedPropertyId,
  propertiesByPrimaryOrder,
} from './primary-property';

function makeProperty(overrides: Partial<Property> & { id: string }): Property {
  return {
    name: 'Квартира',
    type: 'apartment',
    address: 'Москва, ул. Ленина, 1',
    attributes: {},
    status: 'active',
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

describe('propertiesByPrimaryOrder — порядок строк шага «Выбрать объект»', () => {
  it('основные выше, среди основных — раньше отмеченный; прочие в исходном порядке', () => {
    const plain1 = makeProperty({ id: 'plain1' });
    const late = makeProperty({ id: 'late', pinned_at: '2026-09-05T00:00:00Z' });
    const plain2 = makeProperty({ id: 'plain2' });
    const early = makeProperty({ id: 'early', pinned_at: '2026-09-01T00:00:00Z' });
    expect(propertiesByPrimaryOrder([plain1, late, plain2, early]).map((p) => p.id)).toStrictEqual([
      'early',
      'late',
      'plain1',
      'plain2',
    ]);
  });

  it('пустой список остаётся пустым, исходный не мутирует', () => {
    const source = [makeProperty({ id: 'b' }), makeProperty({ id: 'a' })];
    expect(propertiesByPrimaryOrder([])).toStrictEqual([]);
    expect(propertiesByPrimaryOrder(source).map((p) => p.id)).toStrictEqual(['b', 'a']);
    expect(source.map((p) => p.id)).toStrictEqual(['b', 'a']);
  });
});

describe('effectiveSelectedPropertyId — предвыбор шага «Выбрать объект»', () => {
  it('до явного выбора дефолт — первый по канону «Основного объекта»', () => {
    const properties = [
      makeProperty({ id: 'plain' }),
      makeProperty({ id: 'primary', pinned_at: '2026-09-01T00:00:00Z' }),
    ];
    expect(effectiveSelectedPropertyId(undefined, properties)).toBe('primary');
  });

  it('явный выбор пользователя сильнее дефолта', () => {
    const properties = [
      makeProperty({ id: 'primary', pinned_at: '2026-09-01T00:00:00Z' }),
      makeProperty({ id: 'chosen' }),
    ];
    expect(effectiveSelectedPropertyId('chosen', properties)).toBe('chosen');
  });

  it('пустой список редактируемых — выбора нет', () => {
    expect(effectiveSelectedPropertyId(undefined, [])).toBeUndefined();
  });
});
