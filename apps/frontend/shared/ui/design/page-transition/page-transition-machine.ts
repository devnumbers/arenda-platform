export type TransitionPhase = 'idle' | 'covering' | 'revealing';

export type TransitionListener = (phase: TransitionPhase) => void;

/** Тайминги канона DESIGN.md §8: покрытие --dl-duration-fast (250ms),
 * раскрытие --dl-duration-move (350ms). Пауза покрытия (280ms) — окно до
 * запуска навигации: у программных переходов лист успевает добрать
 * непрозрачность до full-cover, у кликов по ссылкам она же играет роль
 * страховки для «голых» якорей без Link (сам Link пушит немедленно и
 * ранняя смена URL отменяет отложенный пуш — см. notifyNavigable-гонку
 * в notifyNavigated). Порт alterix-new (260/280/520): числа канона
 * неотличимы на глаз. Значения должны совпадать с CSS-переменными в
 * page-transition-overlay.module.css — страж дрейфа сверяет их в
 * page-transition-overlay.test.ts. */
export const COVER_DURATION_MS = 250;
export const COVER_DELAY_MS = 280;
export const REVEAL_DURATION_MS = 350;
export const NAVIGATION_FALLBACK_MS = 1800;

type Timer = ReturnType<typeof setTimeout>;

/**
 * Фазовая машина шторки переходов между страницами. Единая точка и для
 * кликов по ссылкам, и для программной навигации (pushPage/replacePage/
 * goBack): navigate откладывает переход до конца покрытия экрана, reveal
 * происходит по факту смены URL (notifyNavigated), фолбэк поднимает лист,
 * если навигация не доехала. Машина не знает про React и DOM — оверлей
 * подписывается на фазу, канал навигации приходит извне.
 */
export interface PageTransitionMachine {
  readonly getPhase: () => TransitionPhase;
  readonly subscribe: (listener: TransitionListener) => () => void;
  /** Начать переход: покрыть экран и выполнить переход после покрытия.
   * В фазе covering новая навигация заменяет отложенную (последняя
   * побеждает), в revealing — запускает новый цикл покрытия. */
  readonly navigate: (perform: () => void) => void;
  /** URL сменился: раскрыть лист; переход, ещё не запущенный машиной,
   * отменяется (смена URL пришла не от неё — например, кнопка «Назад»). */
  readonly notifyNavigated: () => void;
  /** Полный сброс в idle с гашением таймеров (pageshow/bfcache). */
  readonly reset: () => void;
}

export function createPageTransitionMachine(): PageTransitionMachine {
  let phase: TransitionPhase = 'idle';
  const listeners = new Set<TransitionListener>();

  let performTimer: Timer | null = null;
  let holdTimer: Timer | null = null;
  let revealTimer: Timer | null = null;
  let fallbackTimer: Timer | null = null;
  let pendingNavigate: (() => void) | null = null;
  let coverStartedAt = 0;

  const clearTimer = (timer: Timer | null): Timer | null => {
    if (timer !== null) clearTimeout(timer);
    return null;
  };

  const setPhase = (next: TransitionPhase): void => {
    if (phase === next) return;
    phase = next;
    for (const listener of listeners) listener(phase);
  };

  const startReveal = (): void => {
    performTimer = clearTimer(performTimer);
    holdTimer = clearTimer(holdTimer);
    fallbackTimer = clearTimer(fallbackTimer);
    pendingNavigate = null;
    setPhase('revealing');
    revealTimer = clearTimer(revealTimer);
    revealTimer = setTimeout(() => {
      revealTimer = null;
      setPhase('idle');
    }, REVEAL_DURATION_MS);
  };

  return {
    getPhase: () => phase,

    subscribe: (listener: TransitionListener): (() => void) => {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },

    navigate: (perform: () => void): void => {
      holdTimer = clearTimer(holdTimer);
      revealTimer = clearTimer(revealTimer);
      fallbackTimer = clearTimer(fallbackTimer);
      performTimer = clearTimer(performTimer);
      pendingNavigate = perform;
      coverStartedAt = Date.now();
      setPhase('covering');
      performTimer = setTimeout(() => {
        performTimer = null;
        const navigate = pendingNavigate;
        pendingNavigate = null;
        navigate?.();
        fallbackTimer = setTimeout(startReveal, NAVIGATION_FALLBACK_MS);
      }, COVER_DELAY_MS);
    },

    notifyNavigated: (): void => {
      if (phase !== 'covering') return;
      // Минимальная выдержка покрытия: URL мог смениться рано (гонка с
      // Link — тот пушит сам и не смотрит на preventDefault) — лист
      // добирает непрозрачность до конца покрытия и только потом уходит.
      const holdRemaining = Math.max(0, COVER_DURATION_MS - (Date.now() - coverStartedAt));
      performTimer = clearTimer(performTimer);
      pendingNavigate = null;
      if (holdRemaining === 0) {
        startReveal();
        return;
      }
      holdTimer = clearTimer(holdTimer);
      holdTimer = setTimeout(() => {
        holdTimer = null;
        startReveal();
      }, holdRemaining);
    },

    reset: (): void => {
      performTimer = clearTimer(performTimer);
      holdTimer = clearTimer(holdTimer);
      revealTimer = clearTimer(revealTimer);
      fallbackTimer = clearTimer(fallbackTimer);
      pendingNavigate = null;
      setPhase('idle');
    },
  };
}
