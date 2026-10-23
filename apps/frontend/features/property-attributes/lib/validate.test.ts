import { describe, expect, it } from 'vitest';
import type { PropertyType, PropertyAttributes } from '@/entities/property';
import type { AttrKey } from '@/features/property-attributes/model/attr-keys';
import { findField } from '@/features/property-attributes/lib/catalog';
import {
  filterByType,
  validateAttributes,
  validateField,
} from '@/features/property-attributes/lib/validate';

// Helper: narrow a PropertyType literal without runtime cost.
function asType<T extends PropertyType>(t: T): T {
  return t;
}

// ---------------------------------------------------------------------------
// validateField — per kind
// ---------------------------------------------------------------------------

describe('validateField — unknown field / type', () => {
  it('returns undefined when the field is not found for the type', () => {
    // 'land_area' is not part of the apartment catalog.
    expect(validateField(asType('apartment'), 'land_area', 5)).toBeUndefined();
  });

  it('returns undefined for undefined value', () => {
    expect(validateField(asType('apartment'), 'rooms', undefined)).toBeUndefined();
  });

  it('returns undefined for empty string value', () => {
    expect(validateField(asType('apartment'), 'rooms', '')).toBeUndefined();
  });
});

describe('validateField — enum (bathroom on apartment)', () => {
  const type = asType('apartment');
  const key: AttrKey = 'bathroom';

  it.each(['combined', 'separate', 'multiple'])(
    'returns undefined for a valid option %s',
    (option) => {
      expect(validateField(type, key, option)).toBeUndefined();
    },
  );

  it('returns "значение некорректно" for a value not in options', () => {
    expect(validateField(type, key, 'ensuite')).toBe('значение некорректно');
  });

  it('returns "значение некорректно" for a numeric value (enum expects string)', () => {
    // enum only accepts string values that match options; a number never matches.
    expect(validateField(type, key, 2)).toBe('значение некорректно');
  });

  it('returns undefined for empty string', () => {
    expect(validateField(type, key, '')).toBeUndefined();
  });
});

describe('validateField — number (area_total on apartment)', () => {
  const type = asType('apartment');
  const key: AttrKey = 'area_total';
  const def = findField(type, key);
  if (def?.kind !== 'number') throw new Error('expected number field');

  it('returns undefined for a value at/above min', () => {
    expect(validateField(type, key, def.min)).toBeUndefined();
    expect(validateField(type, key, 50)).toBeUndefined();
  });

  it('returns "не менее X unit" for a value below min', () => {
    expect(validateField(type, key, def.min - 0.1)).toBe(
      `не менее ${def.min} ${def.unit}`,
    );
  });

  it('has no upper bound: arbitrary values are valid', () => {
    expect(validateField(type, key, 100001)).toBeUndefined();
    expect(validateField(type, key, 5_000_000)).toBeUndefined();
  });

  it('returns "не менее X unit" for NaN string', () => {
    expect(validateField(type, key, 'abc')).toBe(
      `не менее ${def.min} ${def.unit}`,
    );
  });

  it('respects decimals: 1 decimal allowed for area_total', () => {
    expect(validateField(type, key, 50.1)).toBeUndefined();
    expect(validateField(type, key, '50.1')).toBeUndefined();
  });

  it('returns "не более N знаков после запятой" when too many decimals', () => {
    // area_total has decimals=1
    expect(validateField(type, key, 50.12)).toBe(
      `не более ${def.decimals} знаков после запятой`,
    );
    expect(validateField(type, key, '50.123')).toBe(
      `не более ${def.decimals} знаков после запятой`,
    );
  });
});

describe('validateField — number decimals boundary (ceiling_height, decimals=2)', () => {
  const type = asType('apartment');
  const key: AttrKey = 'ceiling_height';
  const def = findField(type, key);
  if (def?.kind !== 'number') throw new Error('expected number field');

  it('allows exactly decimals digits', () => {
    expect(validateField(type, key, 2.75)).toBeUndefined();
  });

  it('rejects decimals+1 digits', () => {
    expect(validateField(type, key, 2.751)).toBe(
      `не более ${def.decimals} знаков после запятой`,
    );
  });
});

describe('validateField — integer (floor on apartment)', () => {
  const type = asType('apartment');
  const key: AttrKey = 'floor';
  const def = findField(type, key);
  if (def?.kind !== 'integer') throw new Error('expected integer field');

  it('returns undefined for an integer at/above min', () => {
    expect(validateField(type, key, def.min)).toBeUndefined();
    expect(validateField(type, key, 5)).toBeUndefined();
  });

  it('returns "не менее X" for a non-integer (fractional)', () => {
    expect(validateField(type, key, 5.5)).toBe(`не менее ${def.min}`);
  });

  it('returns "не менее X" for a value below min', () => {
    expect(validateField(type, key, def.min - 1)).toBe(`не менее ${def.min}`);
  });

  it('has no upper bound: arbitrary integers are valid', () => {
    expect(validateField(type, key, 201)).toBeUndefined();
    expect(validateField(type, key, 1000)).toBeUndefined();
  });

  it('returns "не менее X" for NaN', () => {
    expect(validateField(type, key, 'abc')).toBe(`не менее ${def.min}`);
  });
});

describe('validateField — string (spot_number on parking)', () => {
  const type = asType('parking');
  const key: AttrKey = 'spot_number';
  const def = findField(type, key);
  if (def?.kind !== 'string') throw new Error('expected string field');

  it('returns undefined for a string within maxLen', () => {
    expect(validateField(type, key, 'A12')).toBeUndefined();
    expect(validateField(type, key, '')).toBeUndefined();
  });

  it('returns undefined when length equals maxLen exactly', () => {
    const exact = 'x'.repeat(def.maxLen);
    expect(validateField(type, key, exact)).toBeUndefined();
  });

  it('returns "не более N символов" when length exceeds maxLen', () => {
    const tooLong = 'x'.repeat(def.maxLen + 1);
    expect(validateField(type, key, tooLong)).toBe(`не более ${def.maxLen} символов`);
  });

  it('counts characters (not UTF-16 units) via Array.from', () => {
    // A multi-byte char counts as a single character via Array.from.
    const single = '😀';
    expect(validateField(type, key, single)).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// validateAttributes
// ---------------------------------------------------------------------------

describe('validateAttributes — error aggregation', () => {
  it('collects multiple errors keyed by attribute', () => {
    const type = asType('apartment');
    const attrs: PropertyAttributes = {
      area_total: 0, // below min -> error
      floor: 5.5, // not integer -> error
      bathroom: 'invalid', // not in options -> error
    };
    const errors = validateAttributes(type, attrs);
    expect(errors.area_total).toBe('не менее 1 м²');
    expect(errors.floor).toBe('не менее -3');
    expect(errors.bathroom).toBe('значение некорректно');
  });

  it('ignores keys not in the catalog for the type', () => {
    const type = asType('apartment');
    const attrs: PropertyAttributes = {
      land_area: 5, // not an apartment field
      unknown_key: 'whatever',
      bathroom: 'separate',
    };
    const errors = validateAttributes(type, attrs);
    expect(errors).toEqual({});
    expect(errors).not.toHaveProperty('land_area');
    expect(errors).not.toHaveProperty('unknown_key');
  });

  it('ignores non-string/number values', () => {
    const type = asType('apartment');
    const attrs = {
      bathroom: 'separate',
      weird: { foo: 'bar' },
    } as unknown as PropertyAttributes;
    const errors = validateAttributes(type, attrs);
    expect(errors).toEqual({});
  });

  it('a fully valid apartment yields no errors', () => {
    const type = asType('apartment');
    const attrs: PropertyAttributes = {
      area_total: 55.5,
      area_living: 35,
      area_kitchen: 10,
      floor: 3,
      floors_total: 9,
      bathroom: 'separate',
      balcony: 'loggia',
      renovation: 'euro',
      year_built: 2010,
      ceiling_height: 2.7,
      parking_type: 'underground',
    };
    expect(validateAttributes(type, attrs)).toEqual({});
  });
});

// ---------------------------------------------------------------------------
// Cross-field rules: сняты (решение владельца 23.10.2026) — согласованность
// полей больше не валидируется, любые значения любых полей.
// ---------------------------------------------------------------------------

describe('validateAttributes — no cross-field rules', () => {
  it.each([asType('apartment'), asType('apartments'), asType('studio'), asType('room')])(
    'floor above floors_total is valid on %s',
    (type) => {
      const attrs: PropertyAttributes = { floor: 10, floors_total: 5 };
      expect(validateAttributes(type, attrs)).toEqual({});
    },
  );

  it.each([asType('apartment'), asType('apartments'), asType('studio')])(
    'area_living above area_total is valid on %s',
    (type) => {
      const attrs: PropertyAttributes = { area_living: 60, area_total: 50 };
      expect(validateAttributes(type, attrs)).toEqual({});
    },
  );

  it.each([asType('apartment'), asType('apartments'), asType('studio'), asType('room')])(
    'area_kitchen above area_total is valid on %s',
    (type) => {
      const attrs: PropertyAttributes = { area_kitchen: 60, area_total: 50 };
      expect(validateAttributes(type, attrs)).toEqual({});
    },
  );

  it('individual errors still surface on both fields', () => {
    // floors_total ниже минимума — своя ошибка остаётся; кросс-правила не
    // добавляют ничего.
    const attrs: PropertyAttributes = { floor: 10, floors_total: 0 };
    const errors = validateAttributes(asType('apartment'), attrs);
    expect(errors.floor).toBeUndefined();
    expect(errors.floors_total).toBe('не менее 1');
  });

  it('string numeric inputs are kept as-is (no cross-rule parsing)', () => {
    const attrs: PropertyAttributes = { floor: '10', floors_total: '5' };
    expect(validateAttributes(asType('apartment'), attrs)).toEqual({});
  });
});

// ---------------------------------------------------------------------------
// filterByType
// ---------------------------------------------------------------------------

describe('filterByType', () => {
  it('keeps only keys allowed for the type', () => {
    const attrs: PropertyAttributes = {
      bathroom: 'separate',
      area_total: 50,
      land_area: 5, // not an apartment field
    };
    const filtered = filterByType(asType('apartment'), attrs);
    expect(filtered).toHaveProperty('bathroom', 'separate');
    expect(filtered).toHaveProperty('area_total', 50);
    expect(filtered).not.toHaveProperty('land_area');
  });

  it('drops unknown keys', () => {
    const attrs: PropertyAttributes = { bathroom: 'separate', whatever: 1 };
    const filtered = filterByType(asType('apartment'), attrs);
    expect(filtered).not.toHaveProperty('whatever');
  });

  it('keeps only string and number values, drops other types', () => {
    const attrs = {
      bathroom: 'separate',
      area_total: 50,
      nested: { x: 1 },
      arr: [1, 2],
      bool: true,
    } as unknown as PropertyAttributes;
    const filtered = filterByType(asType('apartment'), attrs);
    expect(filtered).toHaveProperty('bathroom', 'separate');
    expect(filtered).toHaveProperty('area_total', 50);
    expect(filtered).not.toHaveProperty('nested');
    expect(filtered).not.toHaveProperty('arr');
    expect(filtered).not.toHaveProperty('bool');
  });

  it('returns an empty object when no keys match', () => {
    const attrs: PropertyAttributes = { land_area: 5, land_type: 'izhs' };
    expect(filterByType(asType('apartment'), attrs)).toEqual({});
  });
});
