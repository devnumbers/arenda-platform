import { afterEach, describe, expect, it, vi } from 'vitest';
import { WHEEL_TICK_MS, shouldTickRow, tickRow } from './wheel-haptics';

describe('shouldTickRow', () => {
  it('первый тик всегда', () => {
    expect(shouldTickRow(1000, null)).toBe(true);
  });

  it('чаще паузы — глушится', () => {
    expect(shouldTickRow(1020, 1000)).toBe(false);
  });

  it('по паузе — тикает', () => {
    expect(shouldTickRow(1040, 1000)).toBe(true);
  });
});

describe('tickRow', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    Reflect.deleteProperty(navigator, 'vibrate');
  });

  it('зовёт navigator.vibrate длительностью тика', () => {
    const vibrate = vi.fn();
    Reflect.set(navigator, 'vibrate', vibrate);
    tickRow();
    expect(vibrate).toHaveBeenCalledWith(WHEEL_TICK_MS);
  });

  it('без API — no-op без исключений', () => {
    Reflect.deleteProperty(navigator, 'vibrate');
    expect(() => tickRow()).not.toThrow();
  });
});
