import { describe, expect, it } from 'vitest';
import {
  isApartmentCategory,
  propertyCategoryOf,
  propertyCategoryOptions,
  propertyCreateStepReady,
  propertyHousingTypeOptions,
} from './property-create-draft';

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
  it('девять категорий шага 1 — без apartments и studio (они живут в «Тип жилья», #1003)', () => {
    expect(propertyCategoryOptions).toHaveLength(9);
    expect(propertyCategoryOptions.some((option) => option.value === 'apartments')).toBe(false);
    expect(propertyCategoryOptions.some((option) => option.value === 'studio')).toBe(false);
  });

  it('порядок и подписи макета (Figma 1213-52111), лейблы домена', () => {
    expect(propertyCategoryOptions.map((option) => option.label)).toStrictEqual([
      'Квартира',
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

describe('propertyHousingTypeOptions', () => {
  it('три чипа «Тип жилья» в порядке кадра: квартира → студия → апартаменты (#1003, #1081)', () => {
    expect(propertyHousingTypeOptions.map((option) => option.value)).toStrictEqual([
      'apartment',
      'studio',
      'apartments',
    ]);
    expect(propertyHousingTypeOptions.map((option) => option.label)).toStrictEqual([
      'Квартира',
      'Студия',
      'Апартаменты',
    ]);
  });

  it('студия внутри категории «Квартира»', () => {
    expect(isApartmentCategory('studio')).toBe(true);
    expect(isApartmentCategory('house')).toBe(false);
  });
});

describe('propertyCategoryOf', () => {
  it('типы жилья показывает категория «Квартира» — уточнение живёт в «Типе жилья» (#1081)', () => {
    expect(propertyCategoryOf('apartment')).toBe('apartment');
    expect(propertyCategoryOf('apartments')).toBe('apartment');
    expect(propertyCategoryOf('studio')).toBe('apartment');
  });

  it('остальные категории неизменны', () => {
    expect(propertyCategoryOf('garage')).toBe('garage');
    expect(propertyCategoryOf('land')).toBe('land');
    expect(propertyCategoryOf('room')).toBe('room');
  });
});
