import { describe, expect, it } from 'vitest';

import type { PropertyAttributes } from '@/entities/property';

import { propertyApartmentSummaryRows } from './apartment-summary';

describe('propertyApartmentSummaryRows', () => {
  it('полный набор квартиры (макет 1185:40820): комнаты, площадь, этаж, ремонт', () => {
    const attrs: PropertyAttributes = {
      rooms: 2,
      area_total: 35.5,
      floor: 12,
      floors_total: 16,
      renovation: 'euro',
    };
    expect(propertyApartmentSummaryRows('apartment', attrs)).toEqual([
      { label: 'Комнат', value: '2 комнаты' },
      { label: 'Общая площадь', value: '35,5 м²' },
      { label: 'Этаж', value: '12 из 16' },
      { label: 'Ремонт', value: 'Евро' },
    ]);
  });

  it('этаж без этажности — один', () => {
    const attrs: PropertyAttributes = { floor: 3 };
    expect(propertyApartmentSummaryRows('apartment', attrs)).toEqual([
      { label: 'Этаж', value: '3' },
    ]);
  });

  it('частичный набор — только заполненные характеристики', () => {
    const attrs: PropertyAttributes = { rooms: 1, renovation: 'cosmetic' };
    expect(propertyApartmentSummaryRows('apartment', attrs)).toEqual([
      { label: 'Комнат', value: '1 комната' },
      { label: 'Ремонт', value: 'Косметический' },
    ]);
  });

  it('пустые характеристики — строк нет', () => {
    expect(propertyApartmentSummaryRows('apartment', {})).toEqual([]);
  });

  it('согласование комнат: 1 комната, 2 комнаты, 5 комнат', () => {
    const roomsValue = (rooms: number): string =>
      propertyApartmentSummaryRows('apartment', { rooms }).find(
        (row) => row.label === 'Комнат',
      )?.value ?? '';
    expect(roomsValue(1)).toBe('1 комната');
    expect(roomsValue(2)).toBe('2 комнаты');
    expect(roomsValue(5)).toBe('5 комнат');
  });
});
