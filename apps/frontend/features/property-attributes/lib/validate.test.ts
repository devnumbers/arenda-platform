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

describe('validateField — enum (rooms on apartment)', () => {
  const type = asType('apartment');
  const key: AttrKey = 'rooms';

  it.each(['studio', '1', '2', '3', '4', '5', '6', '7_plus'])(
    'returns undefined for a valid option %s',
    (option) => {
      expect(validateField(type, key, option)).toBeUndefined();
    },
  );

  it('returns "значение некорректно" for a value not in options', () => {
    expect(validateField(type, key, '8')).toBe('значение некорректно');
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
  const def = findField(type, key)!;
  if (def.kind !== 'number') throw new Error('expected number field');

  it('returns undefined for a value within [min, max]', () => {
    expect(validateField(type, key, def.min)).toBeUndefined();
    expect(validateField(type, key, def.max)).toBeUndefined();
    expect(validateField(type, key, 50)).toBeUndefined();
  });

  it('returns "от X до Y unit" for a value below min', () => {
    expect(validateField(type, key, def.min - 0.1)).toBe(
      `от ${def.min} до ${def.max} ${def.unit}`,
    );
  });

  it('returns "от X до Y unit" for a value above max', () => {
    expect(validateField(type, key, def.max + 1)).toBe(
      `от ${def.min} до ${def.max} ${def.unit}`,
    );
  });

  it('returns "от X до Y unit" for NaN string', () => {
    expect(validateField(type, key, 'abc')).toBe(
      `от ${def.min} до ${def.max} ${def.unit}`,
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
  const def = findField(type, key)!;
  if (def.kind !== 'number') throw new Error('expected number field');

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
  const def = findField(type, key)!;
  if (def.kind !== 'integer') throw new Error('expected integer field');

  it('returns undefined for an integer within [min, max]', () => {
    expect(validateField(type, key, def.min)).toBeUndefined();
    expect(validateField(type, key, def.max)).toBeUndefined();
    expect(validateField(type, key, 5)).toBeUndefined();
  });

  it('returns "от X до Y" for a non-integer (fractional)', () => {
    expect(validateField(type, key, 5.5)).toBe(`от ${def.min} до ${def.max}`);
  });

  it('returns "от X до Y" for a value below min', () => {
    expect(validateField(type, key, def.min - 1)).toBe(`от ${def.min} до ${def.max}`);
  });

  it('returns "от X до Y" for a value above max', () => {
    expect(validateField(type, key, def.max + 1)).toBe(`от ${def.min} до ${def.max}`);
  });

  it('returns "от X до Y" for NaN', () => {
    expect(validateField(type, key, 'abc')).toBe(`от ${def.min} до ${def.max}`);
  });
});

describe('validateField — string (spot_number on parking)', () => {
  const type = asType('parking');
  const key: AttrKey = 'spot_number';
  const def = findField(type, key)!;
  if (def.kind !== 'string') throw new Error('expected string field');

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
      rooms: 'invalid', // not in options -> error
    };
    const errors = validateAttributes(type, attrs);
    expect(errors.area_total).toBe('от 1 до 100000 м²');
    expect(errors.floor).toBe('от -3 до 200');
    expect(errors.rooms).toBe('значение некорректно');
  });

  it('ignores keys not in the catalog for the type', () => {
    const type = asType('apartment');
    const attrs: PropertyAttributes = {
      land_area: 5, // not an apartment field
      unknown_key: 'whatever',
      rooms: '2',
    };
    const errors = validateAttributes(type, attrs);
    expect(errors).toEqual({});
    expect(errors).not.toHaveProperty('land_area');
    expect(errors).not.toHaveProperty('unknown_key');
  });

  it('ignores non-string/number values', () => {
    const type = asType('apartment');
    const attrs = {
      rooms: '2',
      weird: { foo: 'bar' },
    } as unknown as PropertyAttributes;
    const errors = validateAttributes(type, attrs);
    expect(errors).toEqual({});
  });

  it('a fully valid apartment yields no errors', () => {
    const type = asType('apartment');
    const attrs: PropertyAttributes = {
      rooms: '2',
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
// Cross-field rules (CRITICAL — these are the migration contract)
// ---------------------------------------------------------------------------

describe('validateAttributes — cross-field floor > floors_total', () => {
  it.each([asType('apartment'), asType('apartments')])(
    'sets floor error "этаж не может превышать этажность" on %s when floor > floors_total',
    (type) => {
      const attrs: PropertyAttributes = { floor: 10, floors_total: 5 };
      const errors = validateAttributes(type, attrs);
      expect(errors.floor).toBe('этаж не может превышать этажность');
    },
  );

  it('does NOT set the cross-field error when floor == floors_total', () => {
    const attrs: PropertyAttributes = { floor: 5, floors_total: 5 };
    expect(validateAttributes(asType('apartment'), attrs).floor).toBeUndefined();
  });

  it('does NOT set the cross-field error when floor < floors_total', () => {
    const attrs: PropertyAttributes = { floor: 3, floors_total: 9 };
    expect(validateAttributes(asType('apartment'), attrs).floor).toBeUndefined();
  });

  it('does NOT apply cross-field when floors_total is individually invalid', () => {
    // floors_total below min(1) -> individually invalid -> cross-rule skipped.
    const attrs: PropertyAttributes = { floor: 10, floors_total: 0 };
    const errors = validateAttributes(asType('apartment'), attrs);
    expect(errors.floor).toBeUndefined();
    expect(errors.floors_total).toBe('от 1 до 200');
  });

  it('cross-rule is skipped when floor is individually invalid', () => {
    // floor fractional -> individually invalid -> cross-rule does not override its own error.
    const attrs: PropertyAttributes = { floor: 5.5, floors_total: 2 };
    const errors = validateAttributes(asType('apartment'), attrs);
    expect(errors.floor).toBe('от -3 до 200');
  });
});

describe('validateAttributes — cross-field area_living > area_total', () => {
  it.each([asType('apartment'), asType('apartments')])(
    'sets area_living error "не может превышать общую площадь" on %s',
    (type) => {
      const attrs: PropertyAttributes = { area_living: 60, area_total: 50 };
      const errors = validateAttributes(type, attrs);
      expect(errors.area_living).toBe('не может превышать общую площадь');
    },
  );

  it('does NOT set error when area_living == area_total', () => {
    const attrs: PropertyAttributes = { area_living: 50, area_total: 50 };
    expect(validateAttributes(asType('apartment'), attrs).area_living).toBeUndefined();
  });

  it('cross-rule is skipped when area_total is individually invalid', () => {
    const attrs: PropertyAttributes = { area_living: 60, area_total: 0 };
    const errors = validateAttributes(asType('apartment'), attrs);
    expect(errors.area_living).toBeUndefined();
    expect(errors.area_total).toBe('от 1 до 100000 м²');
  });
});

describe('validateAttributes — cross-field area_kitchen > area_total', () => {
  it.each([asType('apartment'), asType('apartments'), asType('room')])(
    'sets area_kitchen error "не может превышать общую площадь" on %s',
    (type) => {
      const attrs: PropertyAttributes = { area_kitchen: 60, area_total: 50 };
      const errors = validateAttributes(type, attrs);
      expect(errors.area_kitchen).toBe('не может превышать общую площадь');
    },
  );

  it('does NOT set error when area_kitchen == area_total', () => {
    const attrs: PropertyAttributes = { area_kitchen: 50, area_total: 50 };
    expect(validateAttributes(asType('apartment'), attrs).area_kitchen).toBeUndefined();
  });

  it('cross-rule is skipped when area_total is individually invalid', () => {
    const attrs: PropertyAttributes = { area_kitchen: 60, area_total: 0 };
    const errors = validateAttributes(asType('apartment'), attrs);
    expect(errors.area_kitchen).toBeUndefined();
    expect(errors.area_total).toBe('от 1 до 100000 м²');
  });
});

describe('validateAttributes — string numeric inputs are parsed for cross-rules', () => {
  it('parses string floor/floors_total for the cross-field comparison', () => {
    const attrs: PropertyAttributes = { floor: '10', floors_total: '5' };
    expect(validateAttributes(asType('apartment'), attrs).floor).toBe(
      'этаж не может превышать этажность',
    );
  });

  it('parses string areas for the cross-field comparison', () => {
    const attrs: PropertyAttributes = { area_living: '60', area_total: '50' };
    expect(validateAttributes(asType('apartment'), attrs).area_living).toBe(
      'не может превышать общую площадь',
    );
  });
});

// ---------------------------------------------------------------------------
// filterByType
// ---------------------------------------------------------------------------

describe('filterByType', () => {
  it('keeps only keys allowed for the type', () => {
    const attrs: PropertyAttributes = {
      rooms: '2',
      area_total: 50,
      land_area: 5, // not an apartment field
    };
    const filtered = filterByType(asType('apartment'), attrs);
    expect(filtered).toHaveProperty('rooms', '2');
    expect(filtered).toHaveProperty('area_total', 50);
    expect(filtered).not.toHaveProperty('land_area');
  });

  it('drops unknown keys', () => {
    const attrs: PropertyAttributes = { rooms: '2', whatever: 1 } as PropertyAttributes;
    const filtered = filterByType(asType('apartment'), attrs);
    expect(filtered).not.toHaveProperty('whatever');
  });

  it('keeps only string and number values, drops other types', () => {
    const attrs = {
      rooms: '2',
      area_total: 50,
      nested: { x: 1 },
      arr: [1, 2],
      bool: true,
    } as unknown as PropertyAttributes;
    const filtered = filterByType(asType('apartment'), attrs);
    expect(filtered).toHaveProperty('rooms', '2');
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
