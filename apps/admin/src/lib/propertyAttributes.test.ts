import { describe, expect, it } from 'vitest';
import {
  catalog,
  enumLabels,
  fieldLabels,
  fieldsForType,
  findField,
  formatAttributeValue,
  formatAttributesForCardGrouped,
  groupLabels,
  isPropertyType,
} from './propertyAttributes';
import type { AttrKey, PropertyAttributes, PropertyType } from './propertyAttributes';

function asType<T extends PropertyType>(t: T): T {
  return t;
}

// ---------------------------------------------------------------------------
// Catalog — fields per type
// ---------------------------------------------------------------------------

describe('catalog — every PropertyType has a non-empty field list', () => {
  const allTypes: PropertyType[] = [
    'apartment',
    'room',
    'apartments',
    'house',
    'commercial',
    'office',
    'warehouse',
    'garage',
    'parking',
    'land',
  ];

  it.each(allTypes)('catalog[%s] is a non-empty array', (type) => {
    expect(Array.isArray(catalog[type])).toBe(true);
    expect(catalog[type].length).toBeGreaterThan(0);
  });

  it('apartments shares the same field list as apartment (alias)', () => {
    expect(catalog.apartments).toBe(catalog.apartment);
  });

  it('commercial shares the same field list as office (alias)', () => {
    expect(catalog.commercial).toBe(catalog.office);
  });

  it('fieldsForType returns the catalog entry', () => {
    expect(fieldsForType(asType('apartment'))).toBe(catalog.apartment);
  });
});

describe('findField', () => {
  it('finds an existing field by key', () => {
    const def = findField(asType('apartment'), 'area_total');
    expect(def).toBeDefined();
    expect(def?.kind).toBe('number');
  });

  it('returns undefined for a key not present in the type', () => {
    expect(findField(asType('apartment'), 'land_area')).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// isPropertyType
// ---------------------------------------------------------------------------

describe('isPropertyType', () => {
  it.each(['apartment', 'room', 'apartments', 'house', 'commercial', 'office', 'warehouse', 'garage', 'parking', 'land'])(
    'returns true for a known type %s',
    (value) => {
      expect(isPropertyType(value)).toBe(true);
    },
  );

  it('returns false for an unknown string', () => {
    expect(isPropertyType('unknown')).toBe(false);
  });

  it('returns false for non-string values', () => {
    expect(isPropertyType(123)).toBe(false);
    expect(isPropertyType(null)).toBe(false);
    expect(isPropertyType(undefined)).toBe(false);
    expect(isPropertyType({})).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// formatAttributeValue — per kind
// ---------------------------------------------------------------------------

describe('formatAttributeValue — enum', () => {
  it('returns the Russian label for a known enum value', () => {
    expect(formatAttributeValue(asType('apartment'), 'bathroom', 'combined', {})).toBe(
      'Совмещенный',
    );
  });

  it('falls back to the raw value when the enum label is missing', () => {
    expect(formatAttributeValue(asType('apartment'), 'bathroom', 'unknown_opt', {})).toBe(
      'unknown_opt',
    );
  });
});

describe('formatAttributeValue — number', () => {
  it('formats a number with the field decimals and unit (comma decimal separator)', () => {
    // area_total decimals=1, unit м²
    expect(formatAttributeValue(asType('apartment'), 'area_total', 55.5, {})).toBe('55,5 м²');
  });

  it('pads decimals to the configured precision', () => {
    // area_total decimals=1 -> "55,0"
    expect(formatAttributeValue(asType('apartment'), 'area_total', 55, {})).toBe('55,0 м²');
  });

  it('formats a numeric string the same way', () => {
    expect(formatAttributeValue(asType('apartment'), 'area_total', '55.5', {})).toBe('55,5 м²');
  });

  it('ceiling_height uses decimals=2 and unit "м"', () => {
    expect(formatAttributeValue(asType('apartment'), 'ceiling_height', 2.75, {})).toBe('2,75 м');
  });

  it('falls back to String(value) + unit when the value is not finite', () => {
    // Current behavior: when Number(value) is not finite, `formatted` becomes String(value),
    // but the unit is still appended. This is a known quirk — locked here as the contract.
    expect(formatAttributeValue(asType('apartment'), 'area_total', 'abc', {})).toBe('abc м²');
  });

  it('land_area uses unit "сотки"', () => {
    expect(formatAttributeValue(asType('house'), 'land_area', 10, {})).toBe('10,00 сотки');
  });
});

describe('formatAttributeValue — integer', () => {
  it('formats a plain integer as its string form', () => {
    expect(formatAttributeValue(asType('apartment'), 'year_built', 2010, {})).toBe('2010');
  });

  it('floor is rendered as "X из Y" when floors_total is present', () => {
    const attrs: PropertyAttributes = { floors_total: 9 };
    expect(formatAttributeValue(asType('apartment'), 'floor', 3, attrs)).toBe('3 из 9');
  });

  it('floor is rendered as plain number when floors_total is absent', () => {
    expect(formatAttributeValue(asType('apartment'), 'floor', 3, {})).toBe('3');
  });

  it('floor uses floors_total even when it is a numeric string', () => {
    const attrs: PropertyAttributes = { floors_total: '9' };
    expect(formatAttributeValue(asType('apartment'), 'floor', 3, attrs)).toBe('3 из 9');
  });
});

describe('formatAttributeValue — string', () => {
  it('returns the raw string value', () => {
    expect(formatAttributeValue(asType('parking'), 'spot_number', 'A12', {})).toBe('A12');
  });
});

describe('formatAttributeValue — unknown field', () => {
  it('returns String(value) when the field is not in the catalog for the type', () => {
    expect(formatAttributeValue(asType('apartment'), 'land_area', 5, {})).toBe('5');
  });
});

// ---------------------------------------------------------------------------
// formatAttributesForCardGrouped
// ---------------------------------------------------------------------------

describe('formatAttributesForCardGrouped', () => {
  it('returns groups with labels for a filled apartment', () => {
    const attrs: PropertyAttributes = {
      balcony: 'loggia',
      area_total: 55.5,
      floor: 3,
      floors_total: 9,
    };
    const groups = formatAttributesForCardGrouped(asType('apartment'), attrs);
    expect(groups.length).toBeGreaterThan(0);

    const firstGroup = groups[0];
    expect(firstGroup?.label).toBe(groupLabels.about_object);

    const labels = firstGroup?.items.map((i) => i.label) ?? [];
    expect(labels).toContain(fieldLabels.balcony);
    expect(labels).toContain(fieldLabels.area_total);

    const balconyItem = firstGroup?.items.find((i) => i.label === fieldLabels.balcony);
    expect(balconyItem?.value).toBe('Лоджия');
  });

  it('hides floors_total item when both floor and floors_total are present', () => {
    const attrs: PropertyAttributes = { floor: 3, floors_total: 9 };
    const groups = formatAttributesForCardGrouped(asType('apartment'), attrs);
    const allLabels = groups.flatMap((g) => g.items.map((i) => i.label));
    expect(allLabels).toContain(fieldLabels.floor);
    expect(allLabels).not.toContain(fieldLabels.floors_total);
  });

  it('shows floors_total item when only floors_total is present', () => {
    const attrs: PropertyAttributes = { floors_total: 9 };
    const groups = formatAttributesForCardGrouped(asType('apartment'), attrs);
    const allLabels = groups.flatMap((g) => g.items.map((i) => i.label));
    expect(allLabels).toContain(fieldLabels.floors_total);
  });

  it('skips empty/undefined values', () => {
    const attrs: PropertyAttributes = { balcony: '', area_total: undefined } as unknown as PropertyAttributes;
    const groups = formatAttributesForCardGrouped(asType('apartment'), attrs);
    expect(groups).toEqual([]);
  });

  it('uses null group label for types whose fields have group=null (office)', () => {
    const attrs: PropertyAttributes = { area_total: 100 };
    const groups = formatAttributesForCardGrouped(asType('office'), attrs);
    expect(groups[0]?.group).toBeNull();
    expect(groups[0]?.label).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// Labels completeness (light sanity contract)
// ---------------------------------------------------------------------------

describe('labels — every AttrKey has a field label', () => {
  const keys = Object.keys(fieldLabels) as AttrKey[];
  it('fieldLabels covers all known AttrKeys', () => {
    expect(keys.length).toBeGreaterThanOrEqual(21);
    for (const key of keys) {
      expect(typeof fieldLabels[key]).toBe('string');
      expect(fieldLabels[key].length).toBeGreaterThan(0);
    }
  });

  it('enumLabels has a Russian mapping for bathroom.combined', () => {
    expect(enumLabels.bathroom?.combined).toBe('Совмещенный');
  });
});
