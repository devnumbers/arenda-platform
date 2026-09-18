import { describe, expect, it } from 'vitest';
import { formatDateTimeHeading, formatDayMonth, formatDayMonthWithYear, formatOverdueDays, formatRangeBound, formatTime } from './date-format';

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

  it('формат границы диапазона: текущий год — «1 ноября», другой — «01.01.2025»', () => {
    expect(formatRangeBound('2026-11-01', '2026-09-19')).toBe('1 ноября');
    expect(formatRangeBound('2026-09-19', '2026-09-19')).toBe('19 сентября');
    expect(formatRangeBound('2025-01-01', '2026-09-19')).toBe('01.01.2025');
  });

  it('склонение срока просрочки: 1 день, 3 дня, 5 дней, 21 день, 11–14 дней', () => {
    expect(formatOverdueDays(1)).toBe('1 день');
    expect(formatOverdueDays(3)).toBe('3 дня');
    expect(formatOverdueDays(5)).toBe('5 дней');
    expect(formatOverdueDays(21)).toBe('21 день');
    expect(formatOverdueDays(11)).toBe('11 дней');
    expect(formatOverdueDays(14)).toBe('14 дней');
  });

  it('заголовок-дата детали платежа: «10 августа 2026, 10:56» (#624)', () => {
    expect(formatDateTimeHeading('2026-08-10T10:56:00')).toBe('10 августа 2026, 10:56');
    expect(formatDateTimeHeading('2026-01-01T07:05:00')).toBe('1 января 2026, 07:05');
  });

  it('время момента: «14:40», ведущий ноль часа, невалидное — «—» (#744)', () => {
    expect(formatTime('2026-09-17T14:40:00Z')).toMatch(/^\d{2}:\d{2}$/);
    expect(formatTime('2026-09-17T07:05:00')).toBe('07:05');
    expect(formatTime('не дата')).toBe('—');
  });
});
