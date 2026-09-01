import { describe, expect, it } from 'vitest';
import { addressSuggestionRow } from './address-suggestion';

describe('addressSuggestionRow', () => {
  it('город в начале значения отрезается в заголовок строки — как в макете', () => {
    expect(addressSuggestionRow('г. Москва, ул. Ленина, д. 31', 'Москва')).toStrictEqual({
      title: 'ул. Ленина, д. 31',
      subtitle: 'Москва',
    });
  });

  it('префикс «г» без точки тоже отрезается (формат DaData)', () => {
    expect(addressSuggestionRow('г Москва, ул Ленина, 31', 'Москва')).toStrictEqual({
      title: 'ул Ленина, 31',
      subtitle: 'Москва',
    });
  });

  it('город не в начале значения остаётся в заголовке и показывается подписью', () => {
    expect(addressSuggestionRow('Ленина, 31', 'Москва')).toStrictEqual({
      title: 'Ленина, 31',
      subtitle: 'Москва',
    });
  });

  it('значение из одного города не дублирует его подписью', () => {
    expect(addressSuggestionRow('Москва', 'Москва')).toStrictEqual({
      title: 'Москва',
      subtitle: undefined,
    });
  });

  it('без города строка показывается целиком', () => {
    expect(addressSuggestionRow('Ленина, 31', undefined)).toStrictEqual({
      title: 'Ленина, 31',
      subtitle: undefined,
    });
  });

  it('город сверяется без учёта регистра', () => {
    expect(addressSuggestionRow('москва, Тверская, 10', 'МОСКВА')).toStrictEqual({
      title: 'Тверская, 10',
      subtitle: 'МОСКВА',
    });
  });

  it('город-подстрока в середине значения не отрезается', () => {
    // «Москва» встречается внутри названия улицы — отрезать начало нельзя.
    expect(addressSuggestionRow('ул. Московская, 5', 'Москва')).toStrictEqual({
      title: 'ул. Московская, 5',
      subtitle: 'Москва',
    });
  });
});
