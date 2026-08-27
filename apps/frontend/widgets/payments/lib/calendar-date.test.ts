import { describe, expect, it } from 'vitest';
import { isoToUtcDate, utcDateToIso } from './calendar-date';

describe('calendar-date — конвертация между IsoDate и Date', () => {
  it.each([
    ['2026-08-27', '2026-08-27'],
    ['2024-02-29', '2024-02-29'],
    ['2027-01-01', '2027-01-01'],
  ] as const)('%s туда-обратно без сдвига суток', (input, expected) => {
    expect(utcDateToIso(isoToUtcDate(input))).toBe(expected);
  });

  it('UTC-дата не смещается под локальной зоной машины', () => {
    const date = isoToUtcDate('2026-08-27');
    expect(date.getUTCMonth()).toBe(7);
    expect(date.getUTCDate()).toBe(27);
  });
});
