import { describe, expect, it } from 'vitest';
import { formatDayMonth, formatDayMonthWithYear, formatOverdueDays } from './date-format';

describe('formatDayMonth', () => {
  it('«11 августа» — день и склонённый месяц без года', () => {
    expect(formatDayMonth('2026-08-11')).toBe('11 августа');
  });

  it('первое число без ведущего нуля', () => {
    expect(formatDayMonth('2027-06-01')).toBe('1 июня');
  });

  it('декабрьская дата', () => {
    expect(formatDayMonth('2026-12-31')).toBe('31 декабря');
  });
});

describe('formatDayMonthWithYear — резолюция #452', () => {
  const today = '2026-08-27';

  it('дата текущего года — без года: «13 мая»', () => {
    expect(formatDayMonthWithYear('2026-05-13', today)).toBe('13 мая');
  });

  it('дата другого года — с годом: «13 мая, 2027»', () => {
    expect(formatDayMonthWithYear('2027-05-13', today)).toBe('13 мая, 2027');
  });

  it('прошлый год тоже помечается годом: «17 августа, 2025»', () => {
    expect(formatDayMonthWithYear('2025-08-17', today)).toBe('17 августа, 2025');
  });

  it('граница года: 31 декабря этого года и 1 января следующего', () => {
    expect(formatDayMonthWithYear('2026-12-31', today)).toBe('31 декабря');
    expect(formatDayMonthWithYear('2027-01-01', today)).toBe('1 января, 2027');
  });
});

describe('formatOverdueDays — «N дней» просрочки (резолюция #452)', () => {
  it.each([
    [1, '1 день'],
    [2, '2 дня'],
    [4, '4 дня'],
    [5, '5 дней'],
    [11, '11 дней'],
    [12, '12 дней'],
    [14, '14 дней'],
    [21, '21 день'],
    [22, '22 дня'],
    [25, '25 дней'],
    [101, '101 день'],
    [111, '111 дней'],
    [112, '112 дней'],
  ] as const)('%i → «%s»', (days, expected) => {
    expect(formatOverdueDays(days)).toBe(expected);
  });
});
