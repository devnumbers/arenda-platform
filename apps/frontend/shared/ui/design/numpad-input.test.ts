import { describe, expect, it } from 'vitest';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import { NUMPAD_KEYS, inputNumpadKey, numpadKopecks } from './numpad-input';

describe('numpad-input', () => {
  it('раскладка 3×4: 1..9, запятая, 0, ластик', () => {
    expect(NUMPAD_KEYS).toEqual([
      '1',
      '2',
      '3',
      '4',
      '5',
      '6',
      '7',
      '8',
      '9',
      ',',
      '0',
      'erase',
    ]);
  });

  it('набор цифр: пустое значение и ведущий ноль заменяются', () => {
    expect(inputNumpadKey('', '1')).toBe('1');
    expect(inputNumpadKey('0', '5')).toBe('5');
    expect(inputNumpadKey('12', '3')).toBe('123');
  });

  it('не больше 2 копеечных разрядов и 7 разрядов рублей', () => {
    expect(inputNumpadKey('1,23', '4')).toBe('1,23');
    expect(inputNumpadKey('1234567', '8')).toBe('1234567');
    expect(inputNumpadKey('123456,59', '9')).toBe('123456,59');
  });

  it('запятая: одна, после нуля при пустом вводе', () => {
    expect(inputNumpadKey('', ',')).toBe('0,');
    expect(inputNumpadKey('1', ',')).toBe('1,');
    expect(inputNumpadKey('1,5', ',')).toBe('1,5');
  });

  it('ластик стирает по символу до пустоты', () => {
    expect(inputNumpadKey('1,5', 'erase')).toBe('1,');
    expect(inputNumpadKey('1,', 'erase')).toBe('1');
    expect(inputNumpadKey('1', 'erase')).toBe('');
    expect(inputNumpadKey('', 'erase')).toBe('');
  });

  it('копейки: пустота и нули — 0, иначе целые копейки', () => {
    expect(numpadKopecks('')).toBe(0);
    expect(numpadKopecks('0')).toBe(0);
    expect(numpadKopecks('0,')).toBe(0);
    expect(numpadKopecks('1')).toBe(100);
    expect(numpadKopecks('1,5')).toBe(150);
    expect(numpadKopecks('1234,56')).toBe(123456);
  });

  it('экранная сумма — formatMoneyKopecks поверх копеек: группировка и копейки без нулей', () => {
    const display = (value: string): string => formatMoneyKopecks(numpadKopecks(value));
    expect(display('')).toBe('0 ₽');
    expect(display('0')).toBe('0 ₽');
    expect(display('0,')).toBe('0 ₽');
    expect(display('2500')).toBe(`2\u00A0500 ₽`);
    expect(display('1234,5')).toBe(`1\u00A0234,5 ₽`);
    expect(display('9999999,99')).toBe(`9\u00A0999\u00A0999,99 ₽`);
  });
});
