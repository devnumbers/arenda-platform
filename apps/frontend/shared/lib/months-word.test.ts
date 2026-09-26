import { describe, expect, it } from 'vitest';
import { monthsWord } from './months-word';

describe('monthsWord', () => {
  it('склоняет именительный падеж: месяц / месяца / месяцев', () => {
    expect(monthsWord(1)).toBe('месяц');
    expect(monthsWord(2)).toBe('месяца');
    expect(monthsWord(5)).toBe('месяцев');
  });

  it('11, 12, 13, 14 — «месяцев»', () => {
    expect(monthsWord(11)).toBe('месяцев');
    expect(monthsWord(12)).toBe('месяцев');
    expect(monthsWord(13)).toBe('месяцев');
    expect(monthsWord(14)).toBe('месяцев');
  });

  it('21, 22, 101, 111 — через последнюю цифру', () => {
    expect(monthsWord(21)).toBe('месяц');
    expect(monthsWord(22)).toBe('месяца');
    expect(monthsWord(101)).toBe('месяц');
    expect(monthsWord(111)).toBe('месяцев');
  });
});
