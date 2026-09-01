import { describe, expect, it } from 'vitest';
import type { PropertyAttributes, PropertyType } from '@/entities/property';
import { findField, type AttrField } from './catalog';
import type { AttrKey } from '../model/attr-keys';
import { toWireAttribute, toWireAttributes, sanitizeAttributeInput } from './wire';

/** Поле каталога для примера: отсутствие ключа у типа — ошибка теста. */
function field(type: PropertyType, key: AttrKey): AttrField {
  const found = findField(type, key);
  if (found === undefined) {
    throw new Error(`в каталоге нет поля ${key} у типа ${type}`);
  }
  return found;
}

describe('toWireAttribute', () => {
  it('числовое поле: запятая — десятичный разделитель, значение уходит числом', () => {
    const area = field('apartment', 'area_total');
    expect(toWireAttribute(area, '47,5')).toBe(47.5);
    expect(toWireAttribute(area, '47.5')).toBe(47.5);
    expect(toWireAttribute(area, 12)).toBe(12);
  });

  it('целочисленное поле: дробная часть запрещена — значение отбрасывается', () => {
    const floor = field('apartment', 'floor');
    expect(toWireAttribute(floor, '5')).toBe(5);
    expect(toWireAttribute(floor, '-2')).toBe(-2);
    expect(toWireAttribute(floor, '5,5')).toBeUndefined();
  });

  it('нечислимый ввод числового поля отбрасывается', () => {
    const ceiling = field('apartment', 'ceiling_height');
    expect(toWireAttribute(ceiling, 'высокие')).toBeUndefined();
  });

  it('enum и строковые поля уходят строками, пустая строка отбрасывается', () => {
    const bathroom = field('apartment', 'bathroom');
    const spot = field('parking', 'spot_number');
    expect(toWireAttribute(bathroom, 'combined')).toBe('combined');
    expect(toWireAttribute(spot, '12-А')).toBe('12-А');
    expect(toWireAttribute(spot, '   ')).toBeUndefined();
  });
});

describe('toWireAttributes', () => {
  it('оставляет только ключи каталога текущего типа и заполненные значения', () => {
    const attrs: PropertyAttributes = {
      bathroom: 'separate',
      area_total: '47,5',
      // чужой ключ от прежнего типа — не попадает в payload типа apartment
      land_area: '6',
      // пустая строка — незаполненное значение
      balcony: '',
    };
    expect(toWireAttributes('apartment', attrs)).toStrictEqual({
      bathroom: 'separate',
      area_total: 47.5,
    });
  });

  it('пустой ввод даёт пустой объект — «нет характеристик»', () => {
    expect(toWireAttributes('land', {})).toStrictEqual({});
    expect(toWireAttributes('land', { land_area: '' })).toStrictEqual({});
  });
});

describe('sanitizeAttributeInput', () => {
  it('дробное число: буквы и лишние разделители не вводятся', () => {
    const area = field('apartment', 'area_total');
    expect(sanitizeAttributeInput(area, '4 7,5а')).toBe('47,5');
    // Второй разделитель выбрасывается, цифры после него остаются.
    expect(sanitizeAttributeInput(area, '4,5.6')).toBe('4,56');
    expect(sanitizeAttributeInput(area, '.5')).toBe(',5');
    expect(sanitizeAttributeInput(area, '')).toBe('');
  });

  it('целое число: только цифры; минус — если каталог допускает отрицательные', () => {
    const year = field('apartment', 'year_built');
    const floor = field('apartment', 'floor');
    // Буквы (в т.ч. похожая на ноль «о») не вводятся.
    expect(sanitizeAttributeInput(year, '2о19')).toBe('219');
    expect(sanitizeAttributeInput(year, '-2019')).toBe('2019');
    expect(sanitizeAttributeInput(floor, '-')).toBe('-');
    expect(sanitizeAttributeInput(floor, '-3')).toBe('-3');
    expect(sanitizeAttributeInput(floor, '3-4')).toBe('34');
    expect(sanitizeAttributeInput(floor, '-3,5')).toBe('-35');
  });

  it('строковое поле не фильтруется', () => {
    const spot = field('parking', 'spot_number');
    expect(sanitizeAttributeInput(spot, '12-А / место')).toBe('12-А / место');
  });
});
