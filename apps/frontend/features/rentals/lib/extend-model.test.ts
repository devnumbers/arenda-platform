import { describe, expect, it } from 'vitest';
import { rentalExtendMinDate } from './extend-model';

describe('rentalExtendMinDate', () => {
  it('срочная с будущим окончанием — строго позже текущего', () => {
    expect(rentalExtendMinDate('2027-03-23', '2026-09-23', '2026-09-23')).toBe('2027-03-24');
  });

  it('окончание сегодня — первый доступный день завтра', () => {
    expect(rentalExtendMinDate('2026-09-23', '2026-03-10', '2026-09-23')).toBe('2026-09-24');
  });

  it('needs_attention (окончание в прошлом) — от сегодня', () => {
    expect(rentalExtendMinDate('2026-09-01', '2026-03-10', '2026-09-23')).toBe('2026-09-23');
  });

  it('бессрочная начавшаяся — от сегодня', () => {
    expect(rentalExtendMinDate(null, '2026-03-10', '2026-09-23')).toBe('2026-09-23');
  });

  it('upcoming бессрочная (начало в будущем) — от дня после начала, не от сегодня', () => {
    expect(rentalExtendMinDate(null, '2026-10-01', '2026-09-23')).toBe('2026-10-02');
  });

  it('upcoming срочная — граница продления строже дня после начала', () => {
    expect(rentalExtendMinDate('2026-10-05', '2026-10-01', '2026-09-23')).toBe('2026-10-06');
  });
});
