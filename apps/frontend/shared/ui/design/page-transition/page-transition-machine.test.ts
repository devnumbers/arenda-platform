import { afterEach, beforeEach, describe, expect, it, vi, type Mock } from 'vitest';
import {
  COVER_DELAY_MS,
  COVER_DURATION_MS,
  NAVIGATION_FALLBACK_MS,
  REVEAL_DURATION_MS,
  TRANSITION_WATCHDOG_MS,
  createPageTransitionMachine,
} from './page-transition-machine';

/** Фазовая машина переходов между страницами (канон DESIGN.md §8): два
 * движка за одним интерфейсом navigate/notifyNavigated/reset. Основной —
 * кроссфейд View Transitions API: navigate стартует переход сразу, экран
 * остаётся живым (фаза vt-pending шторку не зажигает), коммит URL
 * закрывает update-промис и браузер анимирует пару old/new сам. Без VT
 * (Safari<18, Firefox<144) — прежняя шторка: covering → навигация →
 * revealing. Предохранитель 600ms переводит молчащую сеть под шторку. */

function createMachine() {
  const machine = createPageTransitionMachine();
  const phases: string[] = [];
  machine.subscribe((phase) => phases.push(phase));
  return { machine, phases };
}

type FakeVt = {
  update: (() => Promise<void>) | null;
  skip: Mock<() => void>;
};

/** Фейк document.startViewTransition: ручка со счётчиком skip, update
 * тест вызывает сам — так фиксируется момент «браузер снял старый экран
 * и выполняет обновление». */
function createVtMachine() {
  const fakes: FakeVt[] = [];
  const startViewTransition = vi.fn((update: () => Promise<void>) => {
    const fake: FakeVt = { update, skip: vi.fn((): void => {}) };
    fakes.push(fake);
    return { skip: fake.skip };
  });
  const machine = createPageTransitionMachine(startViewTransition);
  const phases: string[] = [];
  machine.subscribe((phase) => phases.push(phase));
  // Индексы в strict-режиме дают FakeVt | undefined — доступ через
  // акссессор с честным падением, если переход не стартовал.
  const fakeAt = (index: number): FakeVt => {
    const fake = fakes[index];
    if (fake === undefined) {
      throw new Error(`view transition #${index} не стартовал`);
    }
    return fake;
  };
  return { machine, phases, fakes, startViewTransition, fakeAt };
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

describe('VT-путь: кроссфейд View Transitions API', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('navigate стартует view transition сразу, экран живой в vt-pending', () => {
    const { machine, phases, startViewTransition } = createVtMachine();
    const perform = vi.fn();

    machine.navigate(perform);

    expect(machine.getPhase()).toBe('vt-pending');
    expect(phases).toEqual(['vt-pending']);
    expect(startViewTransition).toHaveBeenCalledTimes(1);
    // навигация выполняется не по тапу, а внутри update-колбэка — после
    // снятия снимка старого экрана
    expect(perform).not.toHaveBeenCalled();
  });

  it('update-колбэк выполняет навигацию, коммит закрывает промис в idle без скипа', async () => {
    const { machine, phases, fakeAt } = createVtMachine();
    const perform = vi.fn();

    machine.navigate(perform);
    const update = fakeAt(0).update;
    expect(update).not.toBeNull();
    const updatePromise = update?.();
    expect(perform).toHaveBeenCalledTimes(1);

    machine.notifyNavigated();
    await updatePromise;

    expect(machine.getPhase()).toBe('idle');
    expect(phases).toEqual(['vt-pending', 'idle']);
    // кроссфейд доезжает сам — скипать его машина не имеет права
    expect(fakeAt(0).skip).not.toHaveBeenCalled();
  });

  it('предохранитель: молчащая сеть накрывается шторкой, коммит скипает кроссфейд и раскрывает', async () => {
    const { machine, phases, fakeAt } = createVtMachine();
    const perform = vi.fn();

    machine.navigate(perform);
    const update = fakeAt(0).update;
    vi.advanceTimersByTime(TRANSITION_WATCHDOG_MS);
    expect(machine.getPhase()).toBe('covering');

    const updatePromise = update?.();
    machine.notifyNavigated();
    await updatePromise;
    // шторка непрозрачна — кроссфейд под ней не виден, скипнут; выдержка
    // покрытия свежая, раскрытие после неё
    expect(fakeAt(0).skip).toHaveBeenCalledTimes(1);
    expect(machine.getPhase()).toBe('covering');

    vi.advanceTimersByTime(COVER_DURATION_MS);
    expect(machine.getPhase()).toBe('revealing');
    vi.advanceTimersByTime(REVEAL_DURATION_MS);
    expect(machine.getPhase()).toBe('idle');
    expect(phases).toEqual(['vt-pending', 'covering', 'revealing', 'idle']);
  });

  it('новый переход в vt-pending скипает незакрытый и стартует свежий', async () => {
    const { machine, fakes, fakeAt } = createVtMachine();
    const first = vi.fn();
    const second = vi.fn();

    machine.navigate(first);
    machine.navigate(second);

    expect(fakes).toHaveLength(2);
    expect(fakeAt(0).skip).toHaveBeenCalledTimes(1);
    expect(machine.getPhase()).toBe('vt-pending');

    // свежий переход выполняет навигацию по коммиту — последняя цель
    // побеждает, старая навигация не выполняется
    const updatePromise = fakeAt(1).update?.();
    expect(second).toHaveBeenCalledTimes(1);
    expect(first).not.toHaveBeenCalled();

    machine.notifyNavigated();
    await updatePromise;
    expect(machine.getPhase()).toBe('idle');
    expect(fakeAt(1).skip).not.toHaveBeenCalled();
  });

  it('фолбэк: навигация не доехала — промис закрыт, переход скипнут, шторка уходит', async () => {
    const { machine, phases, fakeAt } = createVtMachine();
    const perform = vi.fn();

    machine.navigate(perform);
    const updatePromise = fakeAt(0).update?.();
    vi.advanceTimersByTime(TRANSITION_WATCHDOG_MS);
    vi.advanceTimersByTime(NAVIGATION_FALLBACK_MS - TRANSITION_WATCHDOG_MS);
    await updatePromise;

    expect(fakeAt(0).skip).toHaveBeenCalledTimes(1);
    vi.advanceTimersByTime(REVEAL_DURATION_MS);
    expect(machine.getPhase()).toBe('idle');
    expect(phases).toEqual(['vt-pending', 'covering', 'revealing', 'idle']);
  });

  it('reset скипает незакрытый переход и гасит предохранители (bfcache, pageshow)', () => {
    const { machine, phases, fakeAt } = createVtMachine();

    machine.navigate(vi.fn());
    machine.reset();

    expect(fakeAt(0).skip).toHaveBeenCalledTimes(1);
    expect(machine.getPhase()).toBe('idle');
    vi.advanceTimersByTime(TRANSITION_WATCHDOG_MS + NAVIGATION_FALLBACK_MS + REVEAL_DURATION_MS);
    expect(machine.getPhase()).toBe('idle');
    expect(phases).toEqual(['vt-pending', 'idle']);
  });

  it('notifyNavigated в idle и без VT-переходов — пустая операция', () => {
    const { machine, phases } = createVtMachine();

    machine.notifyNavigated();

    expect(machine.getPhase()).toBe('idle');
    expect(phases).toEqual([]);
  });
});
