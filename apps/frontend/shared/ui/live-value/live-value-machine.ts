/**
 * Машина последовательности realtime-оживания значения (канон C, решение
 * владельца 25.09 по прототипу #879; тикет #880): кадр чужой правки
 * приглушает значение (dim 350мс), пауза держится на фактическое
 * перечитывание (≥300мс — dim+hold = 650мс, чтобы последовательность
 * читалась глазом, а не мигала), новое проявляется blur-кроссфейдом
 * (220/380мс) с подсветкой-вспышкой (1200мс). Своё изменение — без
 * анимаций. Режим B (flash, плотные списки/таблицы) — вспышка без dim и
 * кроссфейда. Чистые тайминги и переходы — здесь, чтобы тестировать в
 * node-окружении без DOM (компонентных тестов стек не практикует);
 * CSS-фазы живут на токенах --live-* (tokens.css), JS ждёт те же интервалы
 * через CANON/REDUCED_TIMINGS (reduced-motion укорачивает всё до
 * ~150мс-порядка — §8).
 */

/** Канон подачи: crossfade — базовый канон C для полей и пилюль; flash —
 * канон B для плотных списков и таблиц. */
export type LiveValueMode = 'crossfade' | 'flash';

export type LiveValuePhase = 'idle' | 'stale' | 'swap';

export type LiveValueState = {
  readonly phase: LiveValuePhase;
  /** Момент входа в фазу (performance.now()-подобные мс). */
  readonly startedAt: number;
  /** stale: новое значение уже приехало, свап ждёт конец перечитывания. */
  readonly sawChange: boolean;
  /** swap: ещё одна смена в очереди — сеттл переиграет свап. */
  readonly pendingChange: boolean;
  /** Номер входа в свап: растёт на каждом (пере)старте — ключ
   * перемонтирования оверлея, чтобы keyframes переигрывались. */
  readonly generation: number;
  /** Абсолютный дедлайн таймера фазы — единственная правда о таймерах:
   * компонент ставит ровно один таймер на state.dueAt, машина —
   * единственный авторитет таймингов. null — таймер не нужен. */
  readonly dueAt: number | null;
};

export type LiveValueEvent =
  | { type: 'refreshStart'; at: number }
  | { type: 'refreshEnd'; at: number }
  | { type: 'change'; at: number; own: boolean }
  /** Сгорел таймер, назначенный в dueAt (свап по hold или сеттл). */
  | { type: 'due'; at: number };

/** Результат перехода: состояние уже несёт дедлайн таймера (dueAt). */
export type LiveValueReduction = {
  readonly state: LiveValueState;
};

/** Канон: 350 dim + 300 hold = 650 до свапа; чистка оверлея после
 * затухания вспышки (1200) с малым хвостом; проявление нового значения —
 * blur-кроссфейд (токен --live-blur-in). */
export const CANON_TIMINGS: LiveValueTimings = { staleTotalMs: 650, settleMs: 1250, blurInMs: 380 };

/** Страховочный дедлайн фазы dim (грабля живой приёмки #880): событие
 * конца перечитывания не может потеряться навсегда — при_hold-таймер без
 * смены значения поднимает dim сам, со сменой играет свап. Канонные
 * перечитывания (доли секунды — секунды) дедлайна не касаются. */
export const STALE_MAX_MS = 4000;

/** reduced-motion: ~150мс-порядок на фазу (§8, решение владельца
 * 2026-09-08) — CSS-токены укорачиваются медиазапросом, JS ждёт то же. */
export const REDUCED_TIMINGS: LiveValueTimings = { staleTotalMs: 250, settleMs: 300, blurInMs: 100 };

export type LiveValueTimings = {
  /** Минимальная суммарная пауза dim+hold до свапа от начала dim. */
  readonly staleTotalMs: number;
  /** От старта свапа до снятия оверлея (вспышка 1200 + хвост). */
  readonly settleMs: number;
  /** Проявление нового значения blur-кроссфейдом (WAAPI-движения размера
   * и резкости в компоненте) — токен --live-blur-in tokens.css. */
  readonly blurInMs: number;
};

/** Дедлайн фазы одной формулой — и в переходах, и у компонента один
 * источник: stale держится на hold (со сменой) или на страховочном
 * STALE_MAX (без неё — потерянный refreshEnd не застревает в dim),
 * свап чистится после затухания вспышки. */
function dueAtOf(state: Omit<LiveValueState, 'dueAt'>, timings: LiveValueTimings): number | null {
  switch (state.phase) {
    case 'stale':
      return state.startedAt + (state.sawChange ? timings.staleTotalMs : STALE_MAX_MS);
    case 'swap':
      return state.startedAt + timings.settleMs;
    case 'idle':
      return null;
  }
}

export function initialLiveValueState(): LiveValueState {
  return { phase: 'idle', startedAt: 0, sawChange: false, pendingChange: false, generation: 0, dueAt: null };
}

export function reduceLiveValue(
  state: LiveValueState,
  event: LiveValueEvent,
  mode: LiveValueMode,
  timings: LiveValueTimings,
): LiveValueReduction {
  const settle = (next: Omit<LiveValueState, 'dueAt'>): LiveValueReduction => ({
    state: { ...next, dueAt: dueAtOf(next, timings) },
  });
  const idle = (): Omit<LiveValueState, 'dueAt'> => ({
    phase: 'idle',
    startedAt: event.at,
    sawChange: false,
    pendingChange: false,
    generation: state.generation,
  });
  const swap = (at: number, pending = false): LiveValueReduction =>
    settle({
      phase: 'swap',
      startedAt: at,
      sawChange: false,
      pendingChange: pending,
      generation: state.generation + 1,
    });

  switch (event.type) {
    case 'refreshStart':
      // Режим B приглушения не знает; повторный refreshStart в dim — шум.
      if (mode === 'flash' || state.phase !== 'idle') {
        return { state };
      }
      return settle({
        phase: 'stale',
        startedAt: event.at,
        sawChange: false,
        pendingChange: false,
        generation: state.generation,
      });

    case 'refreshEnd':
      if (state.phase !== 'stale') {
        return { state };
      }
      if (!state.sawChange) {
        // Перечитывание не принесло смены — просто снимаем приглушение
        // (transition на .stale вернёт плавно).
        return settle(idle());
      }
      return startSwapFromStale(state, event.at, timings, settle);

    case 'due':
      if (state.phase === 'stale') {
        if (state.sawChange) {
          // Смена уже в кэше: свап по hold, потерянный refreshEnd не нужен.
          return startSwapFromStale(state, event.at, timings, settle);
        }
        if (event.at - state.startedAt >= STALE_MAX_MS) {
          // Перечитывание затянулось за страховочный дедлайн — снимаем dim
          // (приезд значения позже сыграет прямой кроссфейд из idle).
          return settle(idle());
        }
        // Dim держится на фактическое перечитывание — дедлайн не сдвинулся.
        return { state };
      }
      if (state.phase === 'swap') {
        if (state.pendingChange) {
          // Очередная смена: переигрываем свап (новый generation — ключ
          // оверлея), новая чистка.
          return swap(event.at);
        }
        return settle(idle());
      }
      return { state };

    case 'change':
      if (event.own) {
        // Своё изменение (оптимистичный UI): мгновенно и тихо из любой фазы.
        return settle(idle());
      }
      if (state.phase === 'idle') {
        return swap(event.at);
      }
      if (state.phase === 'stale') {
        // Смена в кэше — hold-дедлайн становится актуальным даже при
        // потерянном refreshEnd.
        return settle({ ...state, sawChange: true });
      }
      // swap: смена в очередь — сеттл переиграет.
      return settle({ ...state, pendingChange: true });
  }
}

function startSwapFromStale(
  state: LiveValueState,
  at: number,
  timings: LiveValueTimings,
  settle: (next: Omit<LiveValueState, 'dueAt'>) => LiveValueReduction,
): LiveValueReduction {
  const elapsed = at - state.startedAt;
  if (elapsed >= timings.staleTotalMs) {
    // Hold уже вычтен фактической доставкой — свап немедленно.
    return settle({
      phase: 'swap',
      startedAt: at,
      sawChange: false,
      pendingChange: false,
      generation: state.generation + 1,
    });
  }
  // Доставлено слишком быстро — держим dim до полного hold (дедлайн уже
  // стоит в состоянии), due доиграет свап.
  return { state };
}
