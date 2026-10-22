import { describe, expect, it } from 'vitest';
import { propertyCategoryOptions, propertyCreateStepReady } from './property-create-draft';

describe('propertyCreateStepReady', () => {
  it('шаг 1 готов, когда выбрана категория', () => {
    expect(propertyCreateStepReady(1, {})).toBe(false);
    expect(propertyCreateStepReady(1, { type: 'apartment' })).toBe(true);
  });

  it('шаг 2 готов, когда адрес непустой не по видимости', () => {
    expect(propertyCreateStepReady(2, { type: 'apartment' })).toBe(false);
    expect(propertyCreateStepReady(2, { type: 'apartment', address: '   ' })).toBe(false);
    expect(propertyCreateStepReady(2, { type: 'apartment', address: 'Ленина, 1' })).toBe(true);
  });

  it('шаг 3 готов всегда — название необязательно (#1001), пустое регенерирует бэк', () => {
    expect(propertyCreateStepReady(3, { type: 'apartment', address: 'Ленина, 1' })).toBe(true);
    expect(
      propertyCreateStepReady(3, { type: 'apartment', address: 'Ленина, 1', name: ' ' }),
    ).toBe(true);
    expect(
      propertyCreateStepReady(3, { type: 'apartment', address: 'Ленина, 1', name: 'Моя квартира' }),
    ).toBe(true);
  });
});

describe('propertyCategoryOptions', () => {
  it('одиннадцать категорий шага 1 — типы жилья стали главными (аменда владельца 22.10, Figma 3625-89189)', () => {
    expect(propertyCategoryOptions).toHaveLength(11);
    expect(propertyCategoryOptions.map((option) => option.value)).toStrictEqual([
      'apartment',
      'studio',
      'apartments',
      'room',
      'house',
      'commercial',
      'office',
      'warehouse',
      'garage',
      'parking',
      'land',
    ]);
  });

  it('порядок и подписи макета (Figma 3625-89189): студия и апартаменты сразу после квартиры', () => {
    expect(propertyCategoryOptions.map((option) => option.label)).toStrictEqual([
      'Квартира',
      'Студия',
      'Апартаменты',
      'Комната',
      'Дом',
      'Коммерческое помещение',
      'Офис',
      'Склад',
      'Гараж',
      'Машиноместо',
      'Земельный участок',
    ]);
  });
});
