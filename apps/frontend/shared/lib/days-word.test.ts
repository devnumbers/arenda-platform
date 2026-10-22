import { describe, expect, it } from 'vitest';

import { daysWord } from './days-word';

describe('daysWord', () => {
  it('именительное склонение дня: 1 день, 2 дня, 5 дней', () => {
    expect(daysWord(1)).toBe('день');
    expect(daysWord(2)).toBe('дня');
    expect(daysWord(5)).toBe('дней');
  });

  it('числа на 11 — множественное (11, 111 дней)', () => {
    expect(daysWord(11)).toBe('дней');
    expect(daysWord(111)).toBe('дней');
  });

  it('числа на 21 — единственное (21, 31 день)', () => {
    expect(daysWord(21)).toBe('день');
    expect(daysWord(31)).toBe('день');
  });
});
