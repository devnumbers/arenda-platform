import { describe, expect, it } from 'vitest';
import {
  initialPropertyCreateStep,
  isApartmentCategory,
  propertyCategoryOptions,
  propertyCreateStepReady,
  propertyHousingTypeOptions,
  validatePropertyCreateDraft,
  type PropertyCreateDraft,
} from './property-create-draft';

describe('validatePropertyCreateDraft', () => {
  it('мусор вместо черновика даёт пустой дефолт', () => {
    expect(validatePropertyCreateDraft(null)).toStrictEqual({});
    expect(validatePropertyCreateDraft('червь')).toStrictEqual({});
    expect(validatePropertyCreateDraft(42)).toStrictEqual({});
    expect(validatePropertyCreateDraft([1, 2])).toStrictEqual({});
  });

  it('валидные поля сохраняются', () => {
    const draft: PropertyCreateDraft = {
      type: 'apartment',
      address: 'Ленина, 1',
      name: 'Моя квартира',
      description: 'Уютная',
    };
    expect(validatePropertyCreateDraft(draft)).toStrictEqual(draft);
  });

  it('пустые строки отбрасываются — мусор из хранилища не всплывает', () => {
    expect(
      validatePropertyCreateDraft({ type: 'room', address: '', name: '', description: '' }),
    ).toStrictEqual({ type: 'room' });
  });

  it('неизвестный тип роняет весь черновик', () => {
    expect(validatePropertyCreateDraft({ type: 'castle', address: 'Ленина, 1' })).toStrictEqual({});
  });

  it('все известные типы проходят, включая apartments и studio', () => {
    for (const { value } of propertyCategoryOptions) {
      expect(validatePropertyCreateDraft({ type: value })).toStrictEqual({ type: value });
    }
    expect(validatePropertyCreateDraft({ type: 'apartments' })).toStrictEqual({ type: 'apartments' });
    expect(validatePropertyCreateDraft({ type: 'studio' })).toStrictEqual({ type: 'studio' });
  });

  it('атрибуты проходят через coerce: скаляры остаются, остальное сбрасывается', () => {
    expect(
      validatePropertyCreateDraft({
        type: 'apartment',
        attributes: { rooms: '3', total_area: 56.5, junk: null, nested: { a: 1 } },
      }),
    ).toStrictEqual({ type: 'apartment', attributes: { rooms: '3', total_area: 56.5 } });
  });

  it('поле step старого черновика игнорируется — позицию шага выводит флоу', () => {
    expect(
      validatePropertyCreateDraft({ step: 4, type: 'apartment', address: 'Ленина, 1' }),
    ).toStrictEqual({ type: 'apartment', address: 'Ленина, 1' });
  });
});

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

describe('initialPropertyCreateStep', () => {
  it('восстанавливает первый незавершённый шаг', () => {
    expect(initialPropertyCreateStep({})).toBe(1);
    expect(initialPropertyCreateStep({ type: 'apartment' })).toBe(2);
    expect(initialPropertyCreateStep({ type: 'apartment', address: 'Ленина, 1' })).toBe(3);
    expect(
      initialPropertyCreateStep({ type: 'apartment', address: 'Ленина, 1', name: 'Моя квартира' }),
    ).toBe(3);
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
  it('три чипа «Тип жилья»: квартира → апартаменты → студия (#1003)', () => {
    expect(propertyHousingTypeOptions.map((option) => option.value)).toStrictEqual([
      'apartment',
      'apartments',
      'studio',
    ]);
    expect(propertyHousingTypeOptions.map((option) => option.label)).toStrictEqual([
      'Квартира',
      'Апартаменты',
      'Студия',
    ]);
  });

  it('студия внутри категории «Квартира»', () => {
    expect(isApartmentCategory('studio')).toBe(true);
    expect(isApartmentCategory('house')).toBe(false);
  });
});
