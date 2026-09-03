import { describe, expect, it } from 'vitest';
import {
  daysOverdue,
  formatCompletedLabel,
  formatOverdueAgo,
  formatSectionDate,
} from './date-format';

const TODAY = '2026-09-10';

describe('формат дат задач', () => {
  it('датирует секцию чужого года годом', () => {
    expect(formatSectionDate('2026-05-13', TODAY)).toBe('13 мая');
    expect(formatSectionDate('2027-05-13', TODAY)).toBe('13 мая, 2027');
  });

  it('подписывает выполнение с датой факта', () => {
    expect(formatCompletedLabel('2026-08-12', TODAY)).toBe('Выполнена 12 августа');
    expect(formatCompletedLabel('2027-08-12', TODAY)).toBe('Выполнена 12 августа, 2027');
  });

  it('считает дни просрочки по календарным суткам и склоняет', () => {
    expect(daysOverdue('2026-09-03', TODAY)).toBe(7);
    expect(formatOverdueAgo('2026-09-03', TODAY)).toBe('7 дней назад');
    expect(formatOverdueAgo('2026-09-09', TODAY)).toBe('1 день назад');
    expect(formatOverdueAgo('2026-09-05', TODAY)).toBe('5 дней назад');
    expect(formatOverdueAgo('2026-08-20', TODAY)).toBe('21 день назад');
  });
});
