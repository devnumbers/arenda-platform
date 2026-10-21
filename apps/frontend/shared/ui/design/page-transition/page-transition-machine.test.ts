import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  COVER_DELAY_MS,
  COVER_DURATION_MS,
  REVEAL_DURATION_MS,
  createPageTransitionMachine,
} from './page-transition-machine';

/** Фазовая машина шторки переходов (порт PageTransitionOverlay alterix-new,
 * канон таймингов DESIGN.md §8): idle → covering (белый лист) → навигация →
 * revealing (лист уходит по факту смены URL) → idle. Поведение — публичный
 * интерфейс машины: navigate/notifyNavigated/reset и подписка на фазу. */

function createMachine() {
  const machine = createPageTransitionMachine();
  const phases: string[] = [];
  machine.subscribe((phase) => phases.push(phase));
  return { machine, phases };
}

describe('Фазовая машина шторки переходов', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('стартует в idle и не сообщает фазу до первого события', () => {
    const { machine, phases } = createMachine();

    expect(machine.getPhase()).toBe('idle');
    expect(phases).toEqual([]);
  });

  it('navigate покрывает экран и выполняет навигацию после паузы покрытия', () => {
    const { machine, phases } = createMachine();
    const navigate = vi.fn();

    machine.navigate(navigate);
    expect(machine.getPhase()).toBe('covering');
    expect(navigate).not.toHaveBeenCalled();

    vi.advanceTimersByTime(COVER_DELAY_MS);
    expect(navigate).toHaveBeenCalledTimes(1);
    // после запуска навигации лист ещё держит экран до смены URL
    expect(machine.getPhase()).toBe('covering');
    expect(phases).toEqual(['covering']);
  });

  it('по смене URL после навигации лист раскрывается и возвращается в idle', () => {
    const { machine, phases } = createMachine();

    machine.navigate(vi.fn());
    vi.advanceTimersByTime(COVER_DELAY_MS);
    machine.notifyNavigated();

    expect(machine.getPhase()).toBe('revealing');
    expect(phases).toEqual(['covering', 'revealing']);

    vi.advanceTimersByTime(REVEAL_DURATION_MS);
    expect(machine.getPhase()).toBe('idle');
    expect(phases).toEqual(['covering', 'revealing', 'idle']);
  });

  it('смена URL до запуска навигации отменяет её и раскрывает лист', () => {
    const { machine } = createMachine();
    const navigate = vi.fn();

    machine.navigate(navigate);
    machine.notifyNavigated();

    // гонка с Link: выдержка покрытия ещё не истекла — лист добирает
    // непрозрачность и только потом уходит, отложенный пуш отменён
    vi.advanceTimersByTime(COVER_DURATION_MS);
    expect(machine.getPhase()).toBe('revealing');
    vi.advanceTimersByTime(REVEAL_DURATION_MS);
    expect(navigate).not.toHaveBeenCalled();
    expect(machine.getPhase()).toBe('idle');
  });

  it('раскрытие ждёт минимальную выдержку покрытия (гонка с Link)', () => {
    const { machine, phases } = createMachine();
    const navigate = vi.fn();

    machine.navigate(navigate);
    vi.advanceTimersByTime(50);
    machine.notifyNavigated();

    // Link сменил URL сам, рано: лист ещё не добрал непрозрачность —
    // фаза revealing включается только на выдержке покрытия (250ms)
    expect(machine.getPhase()).toBe('covering');
    expect(phases).toEqual(['covering']);

    vi.advanceTimersByTime(COVER_DURATION_MS - 50);
    expect(machine.getPhase()).toBe('revealing');

    vi.advanceTimersByTime(REVEAL_DURATION_MS);
    expect(machine.getPhase()).toBe('idle');
    expect(navigate).not.toHaveBeenCalled();
  });

  it('навигация в фазе covering заменяет отложенную (последняя побеждает)', () => {
    const { machine } = createMachine();
    const first = vi.fn();
    const second = vi.fn();

    machine.navigate(first);
    machine.navigate(second);
    vi.advanceTimersByTime(COVER_DELAY_MS);

    expect(first).not.toHaveBeenCalled();
    expect(second).toHaveBeenCalledTimes(1);
  });

  it('navigate в фазе revealing снова покрывает экран (новый переход)', () => {
    const { machine, phases } = createMachine();
    const navigate = vi.fn();

    machine.navigate(vi.fn());
    machine.notifyNavigated();
    // выдержка покрытия истекает — фаза доходит до revealing, новый
    // переход стартует второй цикл покрытия
    vi.advanceTimersByTime(COVER_DURATION_MS);
    machine.navigate(navigate);

    expect(machine.getPhase()).toBe('covering');
    expect(phases).toEqual(['covering', 'revealing', 'covering']);

    vi.advanceTimersByTime(COVER_DELAY_MS);
    expect(navigate).toHaveBeenCalledTimes(1);
  });

  it('без смены URL навигация всё равно раскрывается по фолбэку', () => {
    const { machine, phases } = createMachine();

    machine.navigate(vi.fn());
    vi.advanceTimersByTime(COVER_DELAY_MS);
    vi.advanceTimersByTime(3000);

    expect(machine.getPhase()).toBe('idle');
    expect(phases).toEqual(['covering', 'revealing', 'idle']);
  });

  it('notifyNavigated в idle — пустая операция', () => {
    const { machine, phases } = createMachine();

    machine.notifyNavigated();

    expect(machine.getPhase()).toBe('idle');
    expect(phases).toEqual([]);
  });

  it('reset гасит таймеры и возвращает лист в idle (bfcache, pageshow)', () => {
    const { machine, phases } = createMachine();
    const navigate = vi.fn();

    machine.navigate(navigate);
    machine.reset();

    expect(machine.getPhase()).toBe('idle');
    vi.advanceTimersByTime(COVER_DELAY_MS + REVEAL_DURATION_MS + 3000);
    expect(navigate).not.toHaveBeenCalled();
    expect(machine.getPhase()).toBe('idle');
    expect(phases).toEqual(['covering', 'idle']);
  });

  it('отписка снимает слушателя фазы', () => {
    const machine = createPageTransitionMachine();
    const phases: string[] = [];
    const unsubscribe = machine.subscribe((phase) => phases.push(phase));

    unsubscribe();
    machine.navigate(vi.fn());

    expect(phases).toEqual([]);
  });
});
