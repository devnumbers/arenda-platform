import { describe, expect, it } from 'vitest';
import { formatDayMonth, formatDayMonthWithYear, formatOverdueDays } from './date-format';

describe('date-format', () => {
  it('день и склонённый месяц без года: «11 августа»', () => {
    expect(formatDayMonth('2026-08-11')).toBe('11 августа');
    expect(formatDayMonth('2026-03-01')).toBe('1 марта');
  });

  it('год добавляется только вне текущего года: «13 мая» / «13 мая, 2027»', () => {
    const today = '2026-09-04';
    expect(formatDayMonthWithYear('2026-05-13', today)).toBe('13 мая');
    expect(formatDayMonthWithYear('2027-05-13', today)).toBe('13 мая, 2027');
  });

  it('склонение срока просрочки: 1 день, 3 дня, 5 дней, 21 день, 11–14 дней', () => {
    expect(formatOverdueDays(1)).toBe('1 день');
    expect(formatOverdueDays(3)).toBe('3 дня');
    expect(formatOverdueDays(5)).toBe('5 дней');
    expect(formatOverdueDays(21)).toBe('21 день');
    expect(formatOverdueDays(11)).toBe('11 дней');
    expect(formatOverdueDays(14)).toBe('14 дней');
  });
});
