import { describe, expect, it } from 'vitest';
import { formatCountdown, remainingSecondsUntil } from './countdown';

describe('remainingSecondsUntil', () => {
  it('без дедлайна остатка нет', () => {
    expect(remainingSecondsUntil(null, 1_000_000)).toBe(0);
  });

  it('пуск на 60 с показывает 59 — первая цифра макета «через 00:59»', () => {
    expect(remainingSecondsUntil(60_000, 0)).toBe(59);
  });

  it('остаток тикает по целым секундам', () => {
    expect(remainingSecondsUntil(60_000, 1_400)).toBe(58);
    expect(remainingSecondsUntil(60_000, 2_000)).toBe(57);
  });

  it('последняя секунда и ровно дедлайн — 0', () => {
    expect(remainingSecondsUntil(60_000, 59_200)).toBe(0);
    expect(remainingSecondsUntil(60_000, 60_000)).toBe(0);
  });

  it('просроченный дедлайн — 0, а не отрицательное', () => {
    expect(remainingSecondsUntil(60_000, 61_500)).toBe(0);
  });
});

describe('formatCountdown', () => {
  it('формат ММ:СС с ведущими нулями', () => {
    expect(formatCountdown(0)).toBe('00:00');
    expect(formatCountdown(7)).toBe('00:07');
    expect(formatCountdown(59)).toBe('00:59');
  });

  it('минуты считаются от 60 секунд', () => {
    expect(formatCountdown(60)).toBe('01:00');
    expect(formatCountdown(754)).toBe('12:34');
  });
});
