import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createLongPress } from './useLongPress';

/**
 * Contract of the pure long-press gesture factory (ticket #814, owner
 * decisions 23.09). The `useLongPress` hook is a thin React wrapper — one
 * factory instance per mount, unmount cleanup via `dispose` — so every
 * behavioral guarantee of the gesture is pinned here, on the factory.
 * Pure logic only — the gesture is exercised without React and without
 * DOM: pointer events are plain literals, timers are `vi.useFakeTimers`.
 */

/** Event literal for the factory: the structural slice it may read. */
type TestPointerEvent = {
  pointerType: string;
  clientX: number;
  clientY: number;
};

const touchAt = (x = 0, y = 0): TestPointerEvent => ({
  pointerType: 'touch',
  clientX: x,
  clientY: y,
});

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe('createLongPress hold timer (touch only)', () => {
  it.each([{ pointerType: 'mouse' }, { pointerType: 'pen' }] as const)(
    'pointerType=$pointerType never starts the hold timer',
    ({ pointerType }) => {
      const onLongPress = vi.fn();
      const lp = createLongPress({ onLongPress });

      lp.handlers.onPointerDown({ pointerType, clientX: 3, clientY: 4 });
      lp.handlers.onPointerMove({ pointerType, clientX: 30, clientY: 40 });
      vi.advanceTimersByTime(10_000);

      expect(onLongPress).not.toHaveBeenCalled();
      expect(vi.getTimerCount()).toBe(0);
      expect(lp.consumeClickAfterLongPress()).toBe(false);
      expect(lp.isTouchPointer()).toBe(false);
    },
  );

  it('pointerType=touch arms isTouchPointer', () => {
    const lp = createLongPress({ onLongPress: () => undefined });
    expect(lp.isTouchPointer()).toBe(false);

    lp.handlers.onPointerDown(touchAt());

    expect(lp.isTouchPointer()).toBe(true);
  });

  it('fires onLongPress exactly once, exactly at delayMs', () => {
    const onLongPress = vi.fn();
    const lp = createLongPress({ onLongPress, delayMs: 300 });

    lp.handlers.onPointerDown(touchAt(5, 7));
    vi.advanceTimersByTime(299);
    expect(onLongPress).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    expect(onLongPress).toHaveBeenCalledOnce();
    vi.advanceTimersByTime(5_000);
    expect(onLongPress).toHaveBeenCalledOnce();
  });

  it('default delayMs is 500', () => {
    const onLongPress = vi.fn();
    const lp = createLongPress({ onLongPress });

    lp.handlers.onPointerDown(touchAt());
    vi.advanceTimersByTime(499);
    expect(onLongPress).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    expect(onLongPress).toHaveBeenCalledOnce();
  });
});

describe('createLongPress cancellation', () => {
  it('pointerUp before delayMs cancels the hold', () => {
    const onLongPress = vi.fn();
    const lp = createLongPress({ onLongPress });

    lp.handlers.onPointerDown(touchAt());
    vi.advanceTimersByTime(200);
    lp.handlers.onPointerUp();
    vi.advanceTimersByTime(10_000);

    expect(onLongPress).not.toHaveBeenCalled();
    expect(vi.getTimerCount()).toBe(0);
    expect(lp.consumeClickAfterLongPress()).toBe(false);
  });

  it.each([
    { dx: 10, dy: 0, fires: true }, // hypot = 10 — ровно граница, не скролл
    { dx: 0, dy: 10, fires: true },
    { dx: 6, dy: 8, fires: true }, // hypot = 10 — граница по диагонали
    { dx: 11, dy: 0, fires: false },
    { dx: 6, dy: 9, fires: false }, // hypot ≈ 10.8
    { dx: 8, dy: 8, fires: false }, // hypot ≈ 11.3
  ])(
    'pointerMove by ($dx, $dy) with tolerancePx=10 — still a hold: $fires',
    ({ dx, dy, fires }) => {
      const onLongPress = vi.fn();
      const lp = createLongPress({ onLongPress, tolerancePx: 10 });

      lp.handlers.onPointerDown(touchAt(100, 100));
      lp.handlers.onPointerMove({
        pointerType: 'touch',
        clientX: 100 + dx,
        clientY: 100 + dy,
      });
      vi.advanceTimersByTime(500);

      expect(onLongPress).toHaveBeenCalledTimes(fires ? 1 : 0);
    },
  );

  it('pointercancel cancels the hold and never arms suppression', () => {
    const onLongPress = vi.fn();
    const lp = createLongPress({ onLongPress });

    lp.handlers.onPointerDown(touchAt());
    vi.advanceTimersByTime(200);
    lp.handlers.onPointerCancel();
    vi.advanceTimersByTime(10_000);

    expect(onLongPress).not.toHaveBeenCalled();
    expect(vi.getTimerCount()).toBe(0);
    expect(lp.consumeClickAfterLongPress()).toBe(false);
  });

  it('move/up/cancel before any pointerdown are no-ops', () => {
    const onLongPress = vi.fn();
    const lp = createLongPress({ onLongPress });

    lp.handlers.onPointerMove(touchAt(50, 50));
    lp.handlers.onPointerUp();
    lp.handlers.onPointerCancel();
    vi.advanceTimersByTime(10_000);

    expect(onLongPress).not.toHaveBeenCalled();
    expect(vi.getTimerCount()).toBe(0);
  });
});

describe('createLongPress skipOn', () => {
  it('skipOn=true: gesture never starts and suppression is not armed', () => {
    const onLongPress = vi.fn();
    const seen: TestPointerEvent[] = [];
    const lp = createLongPress({
      onLongPress,
      skipOn: (event) => {
        seen.push(event);
        return event.clientX === 999;
      },
    });

    lp.handlers.onPointerDown({ pointerType: 'touch', clientX: 999, clientY: 0 });
    vi.advanceTimersByTime(10_000);

    expect(seen).toEqual([{ pointerType: 'touch', clientX: 999, clientY: 0 }]);
    expect(onLongPress).not.toHaveBeenCalled();
    expect(vi.getTimerCount()).toBe(0);
    expect(lp.consumeClickAfterLongPress()).toBe(false);
  });

  it('skipOn=false starts the gesture as usual', () => {
    const onLongPress = vi.fn();
    const lp = createLongPress({
      onLongPress,
      skipOn: (event) => event.clientX === 999,
    });

    lp.handlers.onPointerDown(touchAt(1, 1));
    vi.advanceTimersByTime(500);

    expect(onLongPress).toHaveBeenCalledOnce();
  });
});

describe('createLongPress consumeClickAfterLongPress', () => {
  it('false before firing; after firing exactly one true, then false', () => {
    const onLongPress = vi.fn();
    const lp = createLongPress({ onLongPress });

    lp.handlers.onPointerDown(touchAt());
    expect(lp.consumeClickAfterLongPress()).toBe(false);
    vi.advanceTimersByTime(500);
    expect(onLongPress).toHaveBeenCalledOnce();
    expect(lp.consumeClickAfterLongPress()).toBe(true);
    expect(lp.consumeClickAfterLongPress()).toBe(false);
  });
});

describe('createLongPress click-tail window', () => {
  it('consume within the window after the lift → true exactly once', () => {
    const onLongPress = vi.fn();
    const lp = createLongPress({ onLongPress });

    lp.handlers.onPointerDown(touchAt());
    vi.advanceTimersByTime(500);
    expect(onLongPress).toHaveBeenCalledOnce();
    lp.handlers.onPointerUp();
    vi.advanceTimersByTime(200);

    expect(lp.consumeClickAfterLongPress()).toBe(true);
    expect(lp.consumeClickAfterLongPress()).toBe(false);
  });

  it('consume after the window has expired → false', () => {
    const onLongPress = vi.fn();
    const lp = createLongPress({ onLongPress });

    lp.handlers.onPointerDown(touchAt());
    vi.advanceTimersByTime(500);
    lp.handlers.onPointerUp();
    vi.advanceTimersByTime(300);

    expect(lp.consumeClickAfterLongPress()).toBe(false);
  });

  it('window counts from the lift, not from the firing: a long hold keeps the tail edible', () => {
    const onLongPress = vi.fn();
    const lp = createLongPress({ onLongPress });

    // Холд длился 1.5s после срабатывания; окно открывается подъёмом.
    lp.handlers.onPointerDown(touchAt());
    vi.advanceTimersByTime(500);
    expect(onLongPress).toHaveBeenCalledOnce();
    vi.advanceTimersByTime(1500);
    lp.handlers.onPointerUp();
    vi.advanceTimersByTime(299);
    expect(lp.consumeClickAfterLongPress()).toBe(true);

    vi.advanceTimersByTime(1);
    expect(lp.consumeClickAfterLongPress()).toBe(false);
  });

  it('a click far beyond the window after a long hold is not eaten', () => {
    const onLongPress = vi.fn();
    const lp = createLongPress({ onLongPress });

    lp.handlers.onPointerDown(touchAt());
    vi.advanceTimersByTime(2000);
    lp.handlers.onPointerUp();
    vi.advanceTimersByTime(300);

    expect(lp.consumeClickAfterLongPress()).toBe(false);
  });

  it('pointercancel also opens the window', () => {
    const onLongPress = vi.fn();
    const lp = createLongPress({ onLongPress });

    lp.handlers.onPointerDown(touchAt());
    vi.advanceTimersByTime(500);
    lp.handlers.onPointerCancel();
    vi.advanceTimersByTime(299);
    expect(lp.consumeClickAfterLongPress()).toBe(true);

    vi.advanceTimersByTime(2);
    expect(lp.consumeClickAfterLongPress()).toBe(false);
  });

  it('a plain tap without firing never arms the window', () => {
    const onLongPress = vi.fn();
    const lp = createLongPress({ onLongPress });

    lp.handlers.onPointerDown(touchAt());
    vi.advanceTimersByTime(100);
    lp.handlers.onPointerUp();

    expect(vi.getTimerCount()).toBe(0);
    expect(lp.consumeClickAfterLongPress()).toBe(false);
  });

  it('a new pointerdown drops the suppression and the pending window', () => {
    const onLongPress = vi.fn();
    const lp = createLongPress({ onLongPress });

    lp.handlers.onPointerDown(touchAt());
    vi.advanceTimersByTime(500);
    lp.handlers.onPointerUp();
    lp.handlers.onPointerDown(touchAt(2, 2));

    expect(vi.getTimerCount()).toBe(1); // только таймер нового холда
    vi.advanceTimersByTime(300);
    expect(lp.consumeClickAfterLongPress()).toBe(false);
    vi.advanceTimersByTime(200); // сработал холд нового жеста
    expect(onLongPress).toHaveBeenCalledTimes(2);
    expect(lp.consumeClickAfterLongPress()).toBe(true);
  });

  it('consume stops the pending window timer', () => {
    const onLongPress = vi.fn();
    const lp = createLongPress({ onLongPress });

    lp.handlers.onPointerDown(touchAt());
    vi.advanceTimersByTime(500);
    lp.handlers.onPointerUp();
    expect(lp.consumeClickAfterLongPress()).toBe(true);

    expect(vi.getTimerCount()).toBe(0);
    vi.advanceTimersByTime(10_000);
    expect(lp.consumeClickAfterLongPress()).toBe(false);
  });

  it('dispose clears both the hold timer and the pending window timer', () => {
    const onLongPress = vi.fn();
    const lp = createLongPress({ onLongPress });

    lp.handlers.onPointerDown(touchAt());
    expect(vi.getTimerCount()).toBe(1);
    lp.dispose();
    expect(vi.getTimerCount()).toBe(0);
    vi.advanceTimersByTime(10_000);
    expect(onLongPress).not.toHaveBeenCalled();

    lp.handlers.onPointerDown(touchAt());
    vi.advanceTimersByTime(500);
    lp.handlers.onPointerUp();
    expect(vi.getTimerCount()).toBe(1);
    lp.dispose();
    expect(vi.getTimerCount()).toBe(0);
  });
});
