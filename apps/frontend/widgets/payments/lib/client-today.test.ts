import { describe, expect, it } from 'vitest';
import { clientTodayIso } from './client-today';

describe('clientTodayIso', () => {
  it('форматирует локальную дату как YYYY-MM-DD без зон', () => {
    expect(clientTodayIso(new Date(2026, 7, 27))).toBe('2026-08-27');
    expect(clientTodayIso(new Date(2027, 0, 3))).toBe('2027-01-03');
  });

  it('не съезжает через UTC: полночь локального дня — тот же день', () => {
    expect(clientTodayIso(new Date(2026, 7, 27, 0, 5))).toBe('2026-08-27');
    expect(clientTodayIso(new Date(2026, 7, 27, 23, 59))).toBe('2026-08-27');
  });
});
