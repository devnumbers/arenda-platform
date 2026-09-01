import { describe, expect, it } from 'vitest';
import type { PropertyType } from '@/entities/property';
import {
  buildPropertyCreateCommand,
  type PropertyAttributesPort,
} from './property-create-submit';

/**
 * Порт каталога — стаб с его минимальным контрактом: ключи двух типов
 * (квартира и участок) и проводное преобразование «заполненная строка
 * остаётся, пустая отбрасывается». Реальные реализации каталога
 * покрываются тестами features/property-attributes.
 */

const STUB_KEYS: Partial<Record<PropertyType, readonly string[]>> = {
  apartment: ['rooms', 'area_total', 'area_living'],
  apartments: ['rooms', 'area_total', 'area_living'],
  house: ['area_total', 'floors_total'],
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
      // Строки с пробелом изображают нечислимый ввод — wire такой мусор
      // отбрасывает, как и реальный каталог.
      if (typeof value === 'string' && value.trim().length > 0 && !value.includes(' ')) {
        wire[key] = value.trim();
      }
    }
    return wire;
  },
  validateAttributes: (_type, attrs) => (attrs.land_area === '999999' ? { land_area: 'от 0.01 до 1000000 сотки' } : {}),
  fieldKeys: (type) => new Set(STUB_KEYS[type] ?? []),
};

describe('buildPropertyCreateCommand', () => {
  it('полный черновик даёт команду: обрезанные строки, атрибуты в проводном виде', () => {
    const command = buildPropertyCreateCommand(
      {
        type: 'apartments',
        address: '  г. Москва, Ленина, д. 1  ',
        name: '  Моя квартира ',
        description: '  Сдаётся на год  ',
        attributes: { rooms: '2', land_area: '6' },
      },
      stubCatalog,
    );
    expect(command).toStrictEqual({
      name: 'Моя квартира',
      type: 'apartments',
      address: 'г. Москва, Ленина, д. 1',
      description: 'Сдаётся на год',
      attributes: { rooms: '2' },
    });
  });

  it('без названия (обязательное поле) команда не строится', () => {
    expect(buildPropertyCreateCommand({ type: 'garage', address: 'Ленина, 1' }, stubCatalog)).toBeUndefined();
    expect(
      buildPropertyCreateCommand({ type: 'garage', address: 'Ленина, 1', name: '   ' }, stubCatalog),
    ).toBeUndefined();
  });

  it('без типа или адреса команда не строится', () => {
    expect(buildPropertyCreateCommand({ name: 'Гараж' }, stubCatalog)).toBeUndefined();
    expect(buildPropertyCreateCommand({ type: 'garage', name: 'Гараж' }, stubCatalog)).toBeUndefined();
  });

  it('некорректное значение характеристики блокирует команду', () => {
    expect(
      buildPropertyCreateCommand(
        {
          type: 'land',
          address: 'Ленина, 1',
          name: 'Участок',
          attributes: { land_area: '999999' },
        },
        stubCatalog,
      ),
    ).toBeUndefined();
  });

  it('«сырое» значение ключа каталога, не дожившее до payload, блокирует команду', () => {
    // Стаб превращает в wire только значения-строки без пробелов:
    // «6 соток» отбрасывается — команда не должна молча потерять поле.
    expect(
      buildPropertyCreateCommand(
        {
          type: 'land',
          address: 'Ленина, 1',
          name: 'Участок',
          attributes: { land_area: '6 соток' },
        },
        stubCatalog,
      ),
    ).toBeUndefined();
  });

  it('чужой ключ прежнего типа не мешает команде (он не входит в payload)', () => {
    const command = buildPropertyCreateCommand(
      {
        type: 'apartment',
        address: 'Ленина, 1',
        name: 'Квартира',
        attributes: { land_area: '6' },
      },
      stubCatalog,
    );
    expect(command).toBeDefined();
    // Единственный ключ черновика — чужой: в payload атрибутов не осталось.
    expect(command?.attributes).toBeUndefined();
  });

  it('пустое описание и пустые характеристики не попадают в команду', () => {
    const command = buildPropertyCreateCommand(
      {
        type: 'land',
        address: 'Ленина, 1',
        name: 'Участок',
        description: '   ',
        attributes: { land_area: '' },
      },
      stubCatalog,
    );
    expect(command).toStrictEqual({
      name: 'Участок',
      type: 'land',
      address: 'Ленина, 1',
    });
  });
});
