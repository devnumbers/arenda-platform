export type TransitionPhase = 'idle' | 'vt-pending' | 'covering' | 'revealing';

export type TransitionListener = (phase: TransitionPhase) => void;

/** Ручка view transition: скипнуть анимацию пары old/new (шторка накрыла
 * экран — кроссфейд под ней не виден, или переход заменён свежим). */
export interface ViewTransitionHandle {
  readonly skip: () => void;
}

/** DOM-раннер document.startViewTransition; инжектится оверлеем — машина
 * не знает про DOM (тесты подсовывают фейк). undefined — браузер без VT
 * (Safari < 18, Firefox < 144): машина живёт одним путём шторки. */
export type StartViewTransition = (update: () => Promise<void>) => ViewTransitionHandle;

/** Тайминги шторки — собственные токены --pt-cover-duration/--pt-reveal-duration
 * в tokens.css (решение владельца 21.10, «как у Apple», агрессивный
 * вариант ~350ms на цикл; порт alterix-new был 260/280/520). Пауза
 * покрытия (130ms) — окно до запуска навигации: у программных переходов
 * лист успевает добрать непрозрачность до full-cover, у кликов по
 * ссылкам она же играет роль страховки для «голых» якорей без Link
 * (сам Link пушит немедленно, и ранняя смена URL отменяет отложенный
 * пуш — см. гонку в notifyNavigated). Значения обязаны совпадать с
 * токенами tokens.css — страж дрейфа сверяет их в
 * page-transition-overlay.test.ts. */
export const COVER_DURATION_MS = 120;
export const COVER_DELAY_MS = 130;
export const REVEAL_DURATION_MS = 200;
export const NAVIGATION_FALLBACK_MS = 1800;

/** Предохранитель VT-пути (решение владельца 21.10): страница не
 * закоммитилась за 600ms — экран накрывается шторкой, навигация доезжает
 * под ней, кроссфейд скипается. Живой старый экран без отклика дольше
 * этого нативному ощущению противоречит. */
export const TRANSITION_WATCHDOG_MS = 600;

/** Кроссфейд VT-пути — чисто CSS-значение (анимацию ведёт браузер по
 * токену --pt-crossfade-duration); константа живёт для стража дрейфа
 * машина ↔ tokens.css. 200ms — уровень системных фейдов iOS (решение
 * владельца 21.10, Q2); reduced-motion укорачивает токен до 100ms. */
export const CROSSFADE_DURATION_MS = 200;

type Timer = ReturnType<typeof setTimeout>;

/**
 * Фазовая машина переходов между страницами. Единая точка и для кликов
 * по ссылкам, и для программной навигации (pushPage/replacePage/goBack).
 * Два движка за одним интерфейсом:
 *
 * — VT-путь (раннер инжектирован): navigate стартует
 * document.startViewTransition сразу — старый экран снимается в снимок и
 * остаётся живым (фаза vt-pending шторку не зажигает, страница
 * интерактивна, сигнал «тап принят» даёт нажатие кнопки); навигация
 * выполняется внутри update-колбэка (после снятия снимка), коммит URL
 * (notifyNavigated) закрывает update-промис — браузер анимирует пару
 * old/new сам, кроссфейдом --pt-crossfade-duration, машина сразу в idle.
 *
 * — фолбэк-путь шторки (без VT): navigate откладывает переход до конца
 * покрытия экрана, reveal по факту смены URL, фолбэк поднимает лист,
 * если навигация не доехала.
 *
 * Общие предохранители: watchdog 600ms (молчащая сеть накрывается
 * шторкой) и NAVIGATION_FALLBACK_MS (навигация не доехала вовсе —
 * промисы закрыты, переход скипнут, шторка уходит). Машина не знает про
 * React и DOM — оверлей подписывается на фазу и инжектит раннер.
 */
export interface PageTransitionMachine {
  readonly getPhase: () => TransitionPhase;
  readonly subscribe: (listener: TransitionListener) => () => void;
  /** Начать переход. В фазе covering новая навигация заменяет отложенную
   * (последняя побеждает), в revealing/vt-pending — запускает новый
   * переход, незакрытый view transition скипается. */
  readonly navigate: (perform: () => void) => void;
  /** URL сменился: закрыть update-промисы (кроссфейд VT или раскрытие
   * листа); переход, ещё не запущенный машиной, отменяется (смена URL
   * пришла не от неё — например, кнопка «Назад»). */
  readonly notifyNavigated: () => void;
  /** Полный сброс в idle с гашением таймеров (pageshow/bfcache). */
  readonly reset: () => void;
}

export function createPageTransitionMachine(
  startViewTransition?: StartViewTransition,
): PageTransitionMachine {
  let phase: TransitionPhase = 'idle';
  const listeners = new Set<TransitionListener>();

  let performTimer: Timer | null = null;
  let holdTimer: Timer | null = null;
  let revealTimer: Timer | null = null;
  let fallbackTimer: Timer | null = null;
  let watchdogTimer: Timer | null = null;
  let pendingNavigate: (() => void) | null = null;
  let coverStartedAt = 0;
  let vtHandle: ViewTransitionHandle | null = null;
  const vtResolvers = new Set<() => void>();
  /** Коммит случился после старта VT-перехода. Update-колбэк браузер
   * вызывает через кадр после startViewTransition — коммит может
   * приехать раньше него (Back в полёте, заменяющий клик): колбэк тогда
   * обязан закрыться сразу и не дёргать навигацию. По доке View
   * Transitions страница держит старый снимок, пока промис колбэка не
   * закрыт, — незакрытый промис навсегда морозит экран. */
  let vtCommitSeen = false;

  const clearTimer = (timer: Timer | null): Timer | null => {
    if (timer !== null) clearTimeout(timer);
    return null;
  };

  const setPhase = (next: TransitionPhase): void => {
    if (phase === next) return;
    phase = next;
    for (const listener of listeners) listener(phase);
  };

  // Коммит закрывает update-промисы всех живых переходов — включая
  // заменённые (их колбэки могли исполнить навигацию до скипа).
  const resolveVtResolvers = (): void => {
    if (vtResolvers.size === 0) return;
    const resolvers = [...vtResolvers];
    vtResolvers.clear();
    for (const resolve of resolvers) resolve();
  };

  const skipVt = (): void => {
    watchdogTimer = clearTimer(watchdogTimer);
    vtHandle?.skip();
    vtHandle = null;
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

  // Навигация не доехала вовсе: промисы закрыты, переход скипнут,
  // шторка (если была накрыта) уходит; в vt-pending revealing невидим.
  const onStuck = (): void => {
    fallbackTimer = null;
    resolveVtResolvers();
    skipVt();
    pendingNavigate = null;
    startReveal();
  };

  // Update-колбэк view transition: выполняет навигацию после снятия
  // снимка старого экрана и ждёт коммита (notifyNavigated). Заменённые
  // переходы добавляют резолверы в общий набор — коммит закрывает все.
  // Промис обязан закрыться всегда: реджект (бросок perform) и вечное
  // ожидание одинаково ломают экран — браузер держит старый снимок до
  // закрытия колбэка. perform в try/catch: упавшая навигация раскрывается
  // фолбэк-таймерами, ошибка анимации не становится ошибкой страницы.
  const runVtUpdate = (): Promise<void> =>
    new Promise<void>((resolve) => {
      if (vtCommitSeen) {
        // URL уже сменился мимо колбэка (Back, заменяющий клик):
        // повторная навигация драла бы URL туда-сюда.
        resolve();
        return;
      }
      const navigate = pendingNavigate;
      pendingNavigate = null;
      try {
        navigate?.();
      } catch {
        // фолбэк-таймер (NAVIGATION_FALLBACK_MS) закроет переход
      }
      vtResolvers.add(resolve);
    });

  const navigateVt = (runner: StartViewTransition, perform: () => void): void => {
    skipVt();
    fallbackTimer = clearTimer(fallbackTimer);
    vtCommitSeen = false;
    pendingNavigate = perform;
    setPhase('vt-pending');
    watchdogTimer = setTimeout(() => {
      watchdogTimer = null;
      coverStartedAt = Date.now();
      setPhase('covering');
    }, TRANSITION_WATCHDOG_MS);
    fallbackTimer = setTimeout(onStuck, NAVIGATION_FALLBACK_MS);
    vtHandle = runner(runVtUpdate);
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
      if (startViewTransition !== undefined) {
        navigateVt(startViewTransition, perform);
        return;
      }
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
      vtCommitSeen = true;
      resolveVtResolvers();
      if (phase === 'vt-pending') {
        // Обычный кроссфейд: браузер анимирует пару old/new сам по CSS
        // (--pt-crossfade-duration), машине фаза больше не нужна — ручку
        // забываем без скипа.
        watchdogTimer = clearTimer(watchdogTimer);
        fallbackTimer = clearTimer(fallbackTimer);
        pendingNavigate = null;
        vtHandle = null;
        setPhase('idle');
        return;
      }
      if (phase !== 'covering') return;
      // Шторка непрозрачна — кроссфейд под ней не виден, скипаем.
      vtHandle?.skip();
      vtHandle = null;
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
      watchdogTimer = clearTimer(watchdogTimer);
      // После сброса (bfcache, pageshow) живой колбэк закрывается сразу:
      // выполнять его отложенную навигацию за пределами страницы нельзя.
      vtCommitSeen = true;
      resolveVtResolvers();
      vtHandle?.skip();
      vtHandle = null;
      pendingNavigate = null;
      setPhase('idle');
    },
  };
}
