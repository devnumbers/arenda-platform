import { describe, expect, it } from 'vitest';
import { addDays, cmp, dateInMonth, lastDayOfMonth } from './dates';

describe('cmp', () => {
  it.each([
    ['2026-01-31', '2026-02-01', -1],
    ['2026-02-28', '2026-02-28', 0],
    ['2026-03-01', '2026-02-28', 1],
    ['2026-12-31', '2027-01-01', -1],
  ] as const)('cmp(%s, %s) = %i', (a, b, expected) => {
    expect(cmp(a, b)).toBe(expected);
  });
});

describe('addDays', () => {
  it.each([
    ['2026-01-30', 1, '2026-01-31'],
    ['2026-01-31', 1, '2026-02-01'],
    ['2026-02-28', 1, '2026-03-01'],
    ['2024-02-28', 2, '2024-03-01'],
    ['2026-03-01', -1, '2026-02-28'],
    ['2026-12-31', 1, '2027-01-01'],
  ] as const)('addDays(%s, %i) = %s (границы месяцев без сдвига зон)', (
    iso,
    days,
    expected,
  ) => {
    expect(addDays(iso, days)).toBe(expected);
  });
});

describe('lastDayOfMonth', () => {
  it('февраль не високосного года — 28', () => {
    expect(lastDayOfMonth(2026, 1)).toBe(28);
  });

  it('февраль високосного года — 29', () => {
    expect(lastDayOfMonth(2024, 1)).toBe(29);
    expect(lastDayOfMonth(2100, 1)).toBe(28);
  });

  it('апрель — 30, декабрь — 31', () => {
    expect(lastDayOfMonth(2026, 3)).toBe(30);
    expect(lastDayOfMonth(2026, 11)).toBe(31);
  });
});

describe('dateInMonth — прижатие дня к последнему дню месяца', () => {
  it('31-е прижимается к 28/29/30 коротких месяцев (решение №1/№10)', () => {
    expect(dateInMonth(2026, 0, 31)).toBe('2026-01-31');
    expect(dateInMonth(2026, 1, 31)).toBe('2026-02-28');
    expect(dateInMonth(2024, 1, 31)).toBe('2024-02-29');
    expect(dateInMonth(2026, 3, 31)).toBe('2026-04-30');
  });

  it('29 февраля прижимается к последнему дню февраля невисокосного года', () => {
    expect(dateInMonth(2026, 1, 29)).toBe('2026-02-28');
    expect(dateInMonth(2028, 1, 29)).toBe('2028-02-29');
  });

  it('monthIndex0 вне 0..11 нормализуется Date.UTC', () => {
    expect(dateInMonth(2026, 12, 15)).toBe('2027-01-15');
    expect(dateInMonth(2026, -1, 15)).toBe('2025-12-15');
  });
});
