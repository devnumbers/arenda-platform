import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  ACTION_ERROR_PATTERN,
  ACTION_SUCCESS_MS,
  outcomeHaptic,
  resetOutcomeHaptic,
  shouldFireOutcomeHaptic,
} from './action-haptics';

describe('shouldFireOutcomeHaptic', () => {
  it('первый исход всегда', () => {
    expect(shouldFireOutcomeHaptic(1000, null)).toBe(true);
  });

  it('чаще паузы — глушится', () => {
    expect(shouldFireOutcomeHaptic(1050, 1000)).toBe(false);
  });

  it('по паузе — тикает', () => {
    expect(shouldFireOutcomeHaptic(1100, 1000)).toBe(true);
  });
});

describe('outcomeHaptic', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    Reflect.deleteProperty(navigator, 'vibrate');
    resetOutcomeHaptic();
  });

  it('успех — одиночный тик ACTION_SUCCESS_MS', () => {
    const vibrate = vi.fn();
    Reflect.set(navigator, 'vibrate', vibrate);
    outcomeHaptic('success');
    expect(vibrate).toHaveBeenCalledWith(ACTION_SUCCESS_MS);
  });

  it('ошибка — двойной паттерн ACTION_ERROR_PATTERN', () => {
    const vibrate = vi.fn();
    Reflect.set(navigator, 'vibrate', vibrate);
    outcomeHaptic('error');
    expect(vibrate).toHaveBeenCalledWith([...ACTION_ERROR_PATTERN]);
  });

  it('без API — no-op без исключений', () => {
    Reflect.deleteProperty(navigator, 'vibrate');
    expect(() => outcomeHaptic('success')).not.toThrow();
    expect(() => outcomeHaptic('error')).not.toThrow();
  });

  it('троттлинг сквозной на оба исхода: success, через 50мс error — тишина', () => {
    const vibrate = vi.fn();
    Reflect.set(navigator, 'vibrate', vibrate);
    vi.spyOn(Date, 'now')
      .mockReturnValueOnce(1000)
      .mockReturnValueOnce(1050);
    outcomeHaptic('success');
    outcomeHaptic('error');
    expect(vibrate).toHaveBeenCalledTimes(1);
  });

  it('resetOutcomeHaptic открывает троттлинг', () => {
    const vibrate = vi.fn();
    Reflect.set(navigator, 'vibrate', vibrate);
    vi.spyOn(Date, 'now').mockReturnValue(1000);
    outcomeHaptic('success');
    resetOutcomeHaptic();
    outcomeHaptic('error');
    expect(vibrate).toHaveBeenCalledTimes(2);
  });
});
