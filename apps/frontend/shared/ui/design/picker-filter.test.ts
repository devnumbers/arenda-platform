import { describe, expect, it } from 'vitest';
import { filterPickerOptions, pickerOptionByValue } from './picker-filter';

describe('filterPickerOptions', () => {
  const options = [
    { value: 'a', label: 'Моя квартира', hint: 'Новаторов, 8' },
    { value: 'b', label: 'Дача', hint: 'Приозёрная, 2' },
    { value: 'c', label: 'Квартира на набережной' },
  ] as const;

  it('пустой запрос возвращает все опции в исходном порядке', () => {
    expect(filterPickerOptions(options, '')).toStrictEqual([...options]);
    expect(filterPickerOptions(options, '   ')).toStrictEqual([...options]);
  });

  it('ищет по названию без учёта регистра', () => {
    expect(filterPickerOptions(options, 'квартира').map((option) => option.value)).toStrictEqual([
      'a',
      'c',
    ]);
    expect(filterPickerOptions(options, 'ДАЧА').map((option) => option.value)).toStrictEqual(['b']);
  });

  it('ищет по подсказке, когда названия не совпадают', () => {
    expect(filterPickerOptions(options, 'новаторов').map((option) => option.value)).toStrictEqual(['a']);
    expect(filterPickerOptions(options, 'приозёрная').map((option) => option.value)).toStrictEqual(['b']);
  });

  it('тримит запрос и не находит лишнего', () => {
    expect(filterPickerOptions(options, '  дача  ').map((option) => option.value)).toStrictEqual(['b']);
    expect(filterPickerOptions(options, 'дач а')).toStrictEqual([]);
  });

  it('возвращает новый массив, исходный не меняется', () => {
    const snapshot = [...options];
    const result = filterPickerOptions(options, 'квартира');
    expect(result).not.toBe(options);
    expect(options).toStrictEqual(snapshot);
  });
});

describe('pickerOptionByValue', () => {
  const options = [
    { value: 'a', label: 'Моя квартира' },
    { value: 'b', label: 'Дача' },
  ] as const;

  it('находит опцию по значению', () => {
    expect(pickerOptionByValue(options, 'b')).toStrictEqual({ value: 'b', label: 'Дача' });
  });

  it('пустое значение и промах дают undefined', () => {
    expect(pickerOptionByValue(options, null)).toBeUndefined();
    expect(pickerOptionByValue(options, undefined)).toBeUndefined();
    expect(pickerOptionByValue(options, 'c')).toBeUndefined();
  });
});
