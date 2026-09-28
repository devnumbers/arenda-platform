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
};

export type LiveValueEvent =
  | { type: 'refreshStart'; at: number }
  | { type: 'refreshEnd'; at: number }
  | { type: 'change'; at: number; own: boolean }
  /** Сгорел таймер, назначенный в dueAt (свап по hold или сеттл). */
  | { type: 'due'; at: number };

/** Что назначить компоненту: таймер, который при сгорании шлёт `due`. */
export type LiveValueReduction = {
  readonly state: LiveValueState;
  readonly dueAt: number | null;
};

/** Канон: 350 dim + 300 hold = 650 до свапа; чистка оверлея после
 * затухания вспышки (1200) с малым хвостом. */
export const CANON_TIMINGS: LiveValueTimings = { staleTotalMs: 650, settleMs: 1250 };

/** Страховочный дедлайн фазы dim (грабля живой приёмки #880): событие
 * конца перечитывания не может потеряться навсегда — при_hold-таймер без
 * смены значения поднимает dim сам, со сменой играет свап. Канонные
 * перечитывания (доли секунды — секунды) дедлайна не касаются. */
export const STALE_MAX_MS = 4000;

/** reduced-motion: ~150мс-порядок на фазу (§8, решение владельца
 * 2026-09-08) — CSS-токены укорачиваются медиазапросом, JS ждёт то же. */
export const REDUCED_TIMINGS: LiveValueTimings = { staleTotalMs: 250, settleMs: 300 };

export type LiveValueTimings = {
  /** Минимальная суммарная пауза dim+hold до свапа от начала dim. */
  readonly staleTotalMs: number;
  /** От старта свапа до снятия оверлея (вспышка 1200 + хвост). */
  readonly settleMs: number;
};

export function initialLiveValueState(): LiveValueState {
  return { phase: 'idle', startedAt: 0, sawChange: false, pendingChange: false, generation: 0 };
}

export function reduceLiveValue(
  state: LiveValueState,
  event: LiveValueEvent,
  mode: LiveValueMode,
  timings: LiveValueTimings,
): LiveValueReduction {
  const idle: LiveValueState = {
    phase: 'idle',
    startedAt: event.at,
    sawChange: false,
    pendingChange: false,
    generation: state.generation,
  };
  const swap = (at: number, pending = false): LiveValueReduction => ({
    state: {
      phase: 'swap',
      startedAt: at,
      sawChange: false,
      pendingChange: pending,
      generation: state.generation + 1,
    },
    dueAt: at + timings.settleMs,
  });

  switch (event.type) {
    case 'refreshStart':
      // Режим B приглушения не знает; повторный refreshStart в dim — шум.
      if (mode === 'flash' || state.phase !== 'idle') {
        return { state, dueAt: null };
      }
      return {
        state: { phase: 'stale', startedAt: event.at, sawChange: false, pendingChange: false, generation: state.generation },
        dueAt: null,
      };

    case 'refreshEnd':
      if (state.phase !== 'stale') {
        return { state, dueAt: null };
      }
      if (!state.sawChange) {
        // Перечитывание не принесло смены — просто снимаем приглушение
        // (transition на .stale вернёт плавно).
        return { state: idle, dueAt: null };
      }
      return startSwapFromStale(state, event.at, timings);

    case 'due':
      if (state.phase === 'stale') {
        if (state.sawChange) {
          // Смена уже в кэше: свап по hold, потерянный refreshEnd не нужен.
          return startSwapFromStale(state, event.at, timings);
        }
        if (event.at - state.startedAt >= STALE_MAX_MS) {
          // Перечитывание затянулось за страховочный дедлайн — снимаем dim
          // (приезд значения позже сыграет прямой кроссфейд из idle).
          return { state: idle, dueAt: null };
        }
        // Dim держится на фактическое перечитывание.
        return { state, dueAt: null };
      }
      if (state.phase === 'swap') {
        if (state.pendingChange) {
          // Очередная смена: переигрываем свап (новый generation — ключ
          // оверлея), новая чистка.
          return swap(event.at);
        }
        return { state: idle, dueAt: null };
      }
      return { state, dueAt: null };

    case 'change':
      if (event.own) {
        // Своё изменение (оптимистичный UI): мгновенно и тихо из любой
        // фазы — таймеры компонент гасит по dueAt: null.
        return { state: idle, dueAt: null };
      }
      if (state.phase === 'idle') {
        return swap(event.at);
      }
      if (state.phase === 'stale') {
        return {
          state: { ...state, sawChange: true },
          dueAt: null,
        };
      }
      // swap: смена в очередь — сеттл переиграет.
      return { state: { ...state, pendingChange: true }, dueAt: null };
  }
}

function startSwapFromStale(
  state: LiveValueState,
  at: number,
  timings: LiveValueTimings,
): LiveValueReduction {
  const elapsed = at - state.startedAt;
  if (elapsed >= timings.staleTotalMs) {
    // Hold уже вычтен фактической доставкой — свап немедленно.
    return {
      state: {
        phase: 'swap',
        startedAt: at,
        sawChange: false,
        pendingChange: false,
        generation: state.generation + 1,
      },
      dueAt: at + timings.settleMs,
    };
  }
  // Доставлено слишком быстро — держим dim до полного hold, компоненту
  // назначен таймер «due» на остаток.
  return { state, dueAt: state.startedAt + timings.staleTotalMs };
}
