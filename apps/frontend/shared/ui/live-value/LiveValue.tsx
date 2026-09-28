'use client';

import {
  useCallback,
  useLayoutEffect,
  useRef,
  useState,
  type JSX,
  type ReactNode,
} from 'react';
import { useReducedMotion } from '@/shared/lib/hooks/useReducedMotion';
import { cn } from '@/shared/lib/cn';
import {
  CANON_TIMINGS,
  REDUCED_TIMINGS,
  STALE_MAX_MS,
  initialLiveValueState,
  reduceLiveValue,
  type LiveValueEvent,
  type LiveValueMode,
  type LiveValueState,
  type LiveValueTimings,
} from './live-value-machine';
import styles from './LiveValue.module.css';

export type LiveValueProps = {
  /** Сменяемый дискриминатор значения (роль, статус, метка): сравнение
   * prev/next запускает канон. Дети — визуал ТЕКУЩЕГО значения. */
  readonly valueKey: string | number;
  /** Значение в полёте перечитывания (react-query isFetching): фаза dim
   * канона C. В режиме B приглушения нет. */
  readonly refreshing?: boolean;
  /** Своё изменение (локальная мутация, оптимистичный UI) — без
   * анимаций: вспышка и dim — сигнал «кто-то другой поменял». */
  readonly own?: boolean;
  /** Канон подачи: crossfade — базовый C (поля, пилюли); flash — B
   * (плотные списки и таблицы, без кроссфейда). */
  readonly mode?: LiveValueMode;
  readonly className?: string;
  readonly children: ReactNode;
};

/** Снапшоты детей для фаз: dim держит старое значение, свап кладёт
 * старое/новое друг на друга. В state, не в рефах — рендер читает
 * только состояние (правило react-hooks/refs). */
type LiveValueSnapshots = {
  readonly stale: ReactNode | null;
  readonly old: ReactNode | null;
  readonly fresh: ReactNode | null;
};

const NO_SNAPSHOTS: LiveValueSnapshots = { stale: null, old: null, fresh: null };

/**
 * Оживающее значение — канон C подачи realtime-обновлений (решение
 * владельца 25.09 по прототипу #879, тикет #880): чужая правка по кадру
 * приглушает старое значение (dim 350мс), пауза держится на фактическое
 * перечитывание (≥300мс), новое проявляется blur-кроссфейдом (220/380мс)
 * с подсветкой-вспышкой (1200мс). Эталон — демо
 * `.scratch/ui-demos/realtime-updates` ветки frontend-foundation.
 *
 * Источник правки компоненту неизвестен: он анимирует любую смену
 * `valueKey`, приехавшую через react-query (SSE-кадры лишь инвалидируют
 * семейства — ADR 0062 §2, wiring #717), поэтому потребители оживают без
 * правок при появлении новых инвалидаций. Смена без перечитывания
 * (влитие в кэш) идёт сразу в кроссфейд.
 *
 * Layout не двигаем: во время анимации новое значение живёт абсолютом
 * над старым, вспышка — на коробке с компенсирующими полями; место
 * значения резервируется старым (правило E демо: анимируем только
 * реально изменившееся поле, экран целиком не мигает). Reduced-motion
 * укорачивает фазы до ~150мс-порядка (§8) — CSS через медиа-токены,
 * JS-ожидания через REDUCED_TIMINGS. Машина состояний и тайминги — в
 * `live-value-machine.ts` (чистые, тестированные); здесь только клей:
 * переходы диспетчатся из layout-эффектов до краски (классы свапа — в
 * первом кадре), рендер читает только state.
 */
export function LiveValue({
  valueKey,
  refreshing = false,
  own = false,
  mode = 'crossfade',
  className,
  children,
}: LiveValueProps): JSX.Element {
  const reducedMotion = useReducedMotion();
  const timings: LiveValueTimings = reducedMotion ? REDUCED_TIMINGS : CANON_TIMINGS;

  const [state, setState] = useState<LiveValueState>(initialLiveValueState);
  const [snapshots, setSnapshots] = useState<LiveValueSnapshots>(NO_SNAPSHOTS);

  // Книжение машины — в рефах: эффекты переходов читают прошлый коммит
  // как «старую сторону», рендер в рефы не заглядывает.
  const stateRef = useRef(state);
  const timingsRef = useRef(timings);
  const modeRef = useRef(mode);
  const ownRef = useRef(own);
  const childrenPropRef = useRef(children);
  const lastCommittedChildrenRef = useRef<ReactNode>(children);
  const staleChildrenRef = useRef<ReactNode>(children);
  const oldChildrenRef = useRef<ReactNode>(children);
  const newChildrenRef = useRef<ReactNode>(children);
  const prevKeyRef = useRef(valueKey);
  const prevRefreshingRef = useRef(refreshing);

  // Стабильный переход: читает только реф-книжение, событий — от эффектов.
  const transition = useCallback((event: LiveValueEvent): void => {
    const prevPhase = stateRef.current.phase;
    const prevGeneration = stateRef.current.generation;
    const result = reduceLiveValue(stateRef.current, event, modeRef.current, timingsRef.current);
    if (result.state === stateRef.current) {
      return;
    }
    const next = result.state;
    if (next.phase === 'stale' && prevPhase === 'idle') {
      // Вход в dim: снапшот прошлого коммита — ещё старое значение
      // (данные прилетят позже), оно остаётся на экране приглушённым.
      staleChildrenRef.current = lastCommittedChildrenRef.current;
    }
    if (next.generation !== prevGeneration) {
      // Вход в свап: старая сторона — то, что было на экране.
      oldChildrenRef.current =
        prevPhase === 'stale'
          ? staleChildrenRef.current
          : prevPhase === 'swap'
            ? // Переигранный свап: старая сторона — то, что только что
              // проявилось (новая сторона прошлого свапа).
              newChildrenRef.current
            : lastCommittedChildrenRef.current;
      newChildrenRef.current = childrenPropRef.current;
    }
    stateRef.current = next;
    const nextSnapshots: LiveValueSnapshots = {
      stale: staleChildrenRef.current,
      old: oldChildrenRef.current,
      fresh: newChildrenRef.current,
    };
    setState(next);
    setSnapshots(nextSnapshots);
  }, []);

  // Зеркало пропов для эффектов: layout-эффекты не читают пропы прошлых
  // замыканий, пишем актуальные значения до переходных эффектов (порядок
  // эффектов — определение выше-раньше).
  useLayoutEffect(() => {
    timingsRef.current = timings;
    modeRef.current = mode;
    ownRef.current = own;
  }, [timings, mode, own]);

  // Смена значения и перечитывание — одним эффектом до краски (классы
  // фаз в первом кадре): оба входа сверяются с прошлым коммитом в одном
  // месте, порядок событий машины не зависит от порядка двух эффектов.
  useLayoutEffect(() => {
    const keyChanged = prevKeyRef.current !== valueKey;
    const refreshFlipped = prevRefreshingRef.current !== refreshing;
    if (!keyChanged && !refreshFlipped) {
      return;
    }
    prevKeyRef.current = valueKey;
    prevRefreshingRef.current = refreshing;
    if (keyChanged) {
      transition({ type: 'change', at: performance.now(), own: ownRef.current });
    }
    if (refreshFlipped) {
      transition(
        refreshing
          ? { type: 'refreshStart', at: performance.now() }
          : { type: 'refreshEnd', at: performance.now() },
      );
    }
  }, [valueKey, refreshing, transition]);

  // Таймеры выводятся из состояния (самовосстановление вместо ручного
  // реестра). В stale таймер стоит ВСЕГДА — потеря события конца
  // перечитывания (грабля живой приёмки) не может застревать dim: со
  // сменой в кэше свап играет по hold, без — dim снимается на
  // страховочном дедлайне STALE_MAX. Отсчёт от абсолютного startedAt —
  // перезапуск эффекта не сдвигает момент срабатывания.
  useLayoutEffect(() => {
    const delay =
      state.phase === 'swap'
        ? Math.max(0, state.startedAt + timings.settleMs - performance.now())
        : state.phase === 'stale'
          ? Math.max(
              0,
              (state.startedAt + (state.sawChange ? timings.staleTotalMs : STALE_MAX_MS)) -
                performance.now(),
            )
          : null;
    if (delay === null) {
      return;
    }
    const timer = setTimeout(() => transition({ type: 'due', at: performance.now() }), delay);
    return () => clearTimeout(timer);
  }, [state, timings, transition]);

  // Снапшот детей — последним эффектом каждого коммита: переходы выше
  // читают прошлый коммит как «старую сторону».
  useLayoutEffect(() => {
    childrenPropRef.current = children;
    lastCommittedChildrenRef.current = children;
  }, [children]);

  if (state.phase === 'stale') {
    return (
      <span className={cn(styles.root, styles.stale, className)}>
        {snapshots.stale ?? children}
      </span>
    );
  }
  if (state.phase === 'swap') {
    return (
      <span className={cn(styles.root, className)}>
        <span key={state.generation} className={styles.flashBox}>
          {mode === 'crossfade' ? (
            <span className={styles.swap}>
              <span className={styles.oldValue} aria-hidden="true">
                {snapshots.old}
              </span>
              <span className={styles.newValue}>{snapshots.fresh}</span>
            </span>
          ) : (
            children
          )}
        </span>
      </span>
    );
  }
  return <span className={cn(styles.root, className)}>{children}</span>;
}
