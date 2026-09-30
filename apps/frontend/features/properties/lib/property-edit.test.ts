import { describe, expect, it } from 'vitest';
import type { Property, PropertyType } from '@/entities/property';
import {
  buildPropertyEditCommand,
  initialPropertyEditDraft,
  type PropertyAttributesPort,
  propertyEditDirty,
} from './property-edit';

/**
 * Порт каталога — стаб, как в property-create-submit.test.ts: ключи двух
 * типов, проводное преобразование «заполненная строка без пробелов
 * остаётся, остальное отбрасывается».
 */

const STUB_KEYS: Partial<Record<PropertyType, readonly string[]>> = {
  apartment: ['rooms', 'area_total', 'area_living'],
  apartments: ['rooms', 'area_total', 'area_living'],
  room: ['area_total'],
  land: ['land_area', 'land_type'],
};

const stubCatalog: PropertyAttributesPort = {
  toWireAttributes: (type, attrs) => {
    const keys = STUB_KEYS[type];
    const wire: Record<string, string | number> = {};
    if (keys === undefined) {
      return wire;
    }
    for (const key of keys) {
      const value = attrs[key];
      if (typeof value === 'string' && value.trim().length > 0 && !value.includes(' ')) {
        wire[key] = value.trim();
      }
      if (typeof value === 'number') {
        wire[key] = value;
      }
    }
    return wire;
  },
  validateAttributes: (_type, attrs) => (attrs.area_total === '999999' ? { area_total: 'ошибка' } : {}),
  fieldKeys: (type) => new Set(STUB_KEYS[type] ?? []),
};

const baseProperty: Property = {
  id: 'prop-1',
  name: 'Квартира на Ленина',
  type: 'apartment',
  address: 'Ленина, 34, Екатеринбург',
  attributes: { area_total: 45.5, rooms: '2' },
  status: 'active',
  members_count: 1,
  created_at: '2026-09-01T10:00:00Z',
  pinned_at: null,
};

describe('initialPropertyEditDraft', () => {
  it('черновик повторяет объект: скаляры атрибутов как есть, описание пустой строкой', () => {
    expect(initialPropertyEditDraft(baseProperty)).toStrictEqual({
      type: 'apartment',
      name: 'Квартира на Ленина',
      address: 'Ленина, 34, Екатеринбург',
      description: '',
      attributes: { area_total: 45.5, rooms: '2' },
    });
  });

  it('описание объекта попадает в черновик', () => {
    const draft = initialPropertyEditDraft({ ...baseProperty, description: 'Уютная' });
    expect(draft.description).toBe('Уютная');
  });
});

describe('buildPropertyEditCommand', () => {
  it('полный черновик даёт команду: обрезанные строки, описание пустого — undefined', () => {
    const draft = {
      type: 'apartment' as PropertyType,
      name: '  Квартира  ',
      address: '  Ленина, 34  ',
      description: '   ',
      attributes: { area_total: 45.5, rooms: '2', land_area: '6' },
    };
    expect(buildPropertyEditCommand(draft, stubCatalog)).toStrictEqual({
      name: 'Квартира',
      type: 'apartment',
      address: 'Ленина, 34',
      description: undefined,
      attributes: { area_total: 45.5, rooms: '2' },
    });
  });

  it('непустое описание попадает в команду обрезанным', () => {
    const draft = {
      type: 'apartment' as PropertyType,
      name: 'Квартира',
      address: 'Ленина, 34',
      description: '  Сдаётся  ',
      attributes: {},
    };
    expect(buildPropertyEditCommand(draft, stubCatalog)?.description).toBe('Сдаётся');
  });

  it('без типа или адреса команда не строится; пустое название строит — регенерирует бэк (#1001)', () => {
    expect(
      buildPropertyEditCommand({ name: 'Квартира', address: 'Ленина, 34', description: '', attributes: {} }, stubCatalog),
    ).toBeUndefined();
    expect(
      buildPropertyEditCommand({ type: 'apartment', name: 'Квартира', address: '  ', description: '', attributes: {} }, stubCatalog),
    ).toBeUndefined();
    // Очищенное название уходит пустой строкой — сервер генерирует заново.
    const cleared = buildPropertyEditCommand(
      { type: 'apartment', name: '   ', address: 'Ленина, 34', description: '', attributes: {} },
      stubCatalog,
    );
    expect(cleared).toStrictEqual({
      name: '',
      type: 'apartment',
      address: 'Ленина, 34',
      description: undefined,
      attributes: {},
    });
  });

  it('некорректная характеристика блокирует команду', () => {
    const draft = {
      type: 'apartment' as PropertyType,
      name: 'Квартира',
      address: 'Ленина, 34',
      description: '',
      attributes: { area_total: '999999' },
    };
    expect(buildPropertyEditCommand(draft, stubCatalog)).toBeUndefined();
  });

  it('«сырое» значение ключа каталога, не дожившее до payload, блокирует команду', () => {
    const draft = {
      type: 'apartment' as PropertyType,
      name: 'Квартира',
      address: 'Ленина, 34',
      description: '',
      attributes: { area_total: '45 метров' },
    };
    expect(buildPropertyEditCommand(draft, stubCatalog)).toBeUndefined();
  });
});

describe('propertyEditDirty', () => {
  it('черновик, повторяющий объект, не грязный', () => {
    const draft = initialPropertyEditDraft(baseProperty);
    expect(propertyEditDirty(draft, baseProperty, stubCatalog)).toBe(false);
  });

  it('смена названия делает грязным (обрезка не считается изменением)', () => {
    const draft = { ...initialPropertyEditDraft(baseProperty), name: '  Квартира на Ленина  ' };
    expect(propertyEditDirty(draft, baseProperty, stubCatalog)).toBe(false);
    expect(propertyEditDirty({ ...draft, name: 'Другая' }, baseProperty, stubCatalog)).toBe(true);
  });

  it('смена адреса и типа делает грязным', () => {
    const draft = initialPropertyEditDraft(baseProperty);
    expect(propertyEditDirty({ ...draft, address: 'Ленина, 35' }, baseProperty, stubCatalog)).toBe(true);
    expect(propertyEditDirty({ ...draft, type: 'apartments' }, baseProperty, stubCatalog)).toBe(true);
  });

  it('пустое описание против отсутствующего — не изменение; непустое — изменение', () => {
    const draft = initialPropertyEditDraft(baseProperty);
    expect(propertyEditDirty({ ...draft, description: '   ' }, baseProperty, stubCatalog)).toBe(false);
    expect(propertyEditDirty({ ...draft, description: 'Текст' }, baseProperty, stubCatalog)).toBe(true);
    const withDescription = { ...baseProperty, description: 'Текст' };
    const draftWithDescription = initialPropertyEditDraft(withDescription);
    expect(propertyEditDirty({ ...draftWithDescription, description: 'Другой текст' }, withDescription, stubCatalog)).toBe(true);
    expect(propertyEditDirty({ ...draftWithDescription, description: '' }, withDescription, stubCatalog)).toBe(true);
  });

  it('изменённое значение характеристики делает грязным', () => {
    const draft = initialPropertyEditDraft(baseProperty);
    expect(propertyEditDirty({ ...draft, attributes: { area_total: 50, rooms: '2' } }, baseProperty, stubCatalog)).toBe(true);
    expect(propertyEditDirty({ ...draft, attributes: { area_total: 45.5, rooms: '3' } }, baseProperty, stubCatalog)).toBe(true);
  });

  it('смена типа на каталог с другим набором полей делает грязным даже без правки значений', () => {
    const draft = { ...initialPropertyEditDraft(baseProperty), type: 'room' as PropertyType };
    // Проводные атрибуты комнаты ({area_total}) не равны атрибутам квартиры
    // в объекте ({area_total, rooms}) — правка несёт смену набора полей.
    expect(propertyEditDirty(draft, baseProperty, stubCatalog)).toBe(true);
  });

  it('невалидный черновик не грязный (кнопка сохранения и так погашена)', () => {
    const draft = { ...initialPropertyEditDraft(baseProperty), attributes: { area_total: '999999' } };
    expect(propertyEditDirty(draft, baseProperty, stubCatalog)).toBe(false);
  });
});
