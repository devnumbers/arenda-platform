import { describe, expect, it } from 'vitest';
import { daysOverdue } from './overdue-days';

describe('daysOverdue', () => {
  it('просрочка на целые дни', () => {
    expect(daysOverdue('2026-08-22', '2026-08-27')).toBe(5);
    expect(daysOverdue('2026-08-26', '2026-08-27')).toBe(1);
  });

  it('дата сегодняшнего дня — ещё не просрочка (0)', () => {
    expect(daysOverdue('2026-08-27', '2026-08-27')).toBe(0);
  });

  it('будущая дата не уходит в минус', () => {
    expect(daysOverdue('2026-08-28', '2026-08-27')).toBe(0);
  });

  it('через границу месяца', () => {
    expect(daysOverdue('2026-07-30', '2026-08-02')).toBe(3);
  });
});
