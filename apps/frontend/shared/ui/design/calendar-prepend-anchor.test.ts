import { describe, expect, it } from 'vitest';

import { planPrependRestore } from './calendar-prepend-anchor';

describe('calendar: ручной якорь приписки (план 051)', () => {
  it('приписка над якорем — дельта offsetTop того же элемента', () => {
    expect(planPrependRestore({ anchorTop: 2200, capturedTop: 0 })).toBe(2200);
    expect(planPrependRestore({ anchorTop: 2159, capturedTop: 0 })).toBe(2159);
  });

  it('элемент пропал со сцены — писать нечего', () => {
    expect(planPrependRestore({ anchorTop: null, capturedTop: 0 })).toBeNull();
  });

  it('роста не было (захват дожил до чужого коммита) — null', () => {
    expect(planPrependRestore({ anchorTop: 0, capturedTop: 0 })).toBeNull();
  });

  it('отрицательная дельта — не пишем (не приписка)', () => {
    expect(planPrependRestore({ anchorTop: 100, capturedTop: 300 })).toBeNull();
  });
});
