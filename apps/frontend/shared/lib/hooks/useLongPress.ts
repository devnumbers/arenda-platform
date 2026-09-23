import { useEffect, useRef } from 'react';
import type { PointerEvent as ReactPointerEvent } from 'react';

/**
 * Long-press как touch-жест (зажатие — только тач, на десктопе клик;
 * тикет #814, решения владельца 23.09): pointerdown с pointerType='touch'
 * заводит таймер, сдвиг пальца дальше допуска (скролл) или подъём —
 * отменяют. Сработавшее зажатие помечает ближайший за ним click на съедание
 * — жест и так применил onLongPress, клик бы отменил его (тап-после-зажатия).
 * Курсорные указатели (мышь/перо) таймер не заводят вовсе.
 */
/** Задержка зажатия: iOS-стандарт ~0.5s (уточняется на приёмке). */
const DEFAULT_DELAY_MS = 500;
/** Допуск сдвига пальца: дальше — это скролл, зажатие отменяется. */
const DEFAULT_TOLERANCE_PX = 10;

export type LongPressOptions = {
  readonly onLongPress: () => void;
  /** Ручки приёмки (#814: «тайминги подсветки выбора — на приёмке»):
   * задержка срабатывания и допуск сдвига уточняются владельцем на
   * живой приёмке. */
  readonly delayMs?: number;
  readonly tolerancePx?: number;
  /** Места, откуда жест зажатия не начинается (например, ручка dnd со
   * своим pointer-жестом): предикат по событию pointerdown. */
  readonly skipOn?: (event: ReactPointerEvent<HTMLElement>) => boolean;
};

export type LongPress = {
  readonly handlers: {
    readonly onPointerDown: (event: ReactPointerEvent<HTMLElement>) => void;
    readonly onPointerMove: (event: ReactPointerEvent<HTMLElement>) => void;
    readonly onPointerUp: () => void;
    readonly onPointerCancel: () => void;
  };
  /** Был ли последний pointerdown touch-указателем: click приходит после
   * pointerup, когда pointerType из события уже недоступен. */
  readonly isTouchPointer: () => boolean;
  /** Одноразовое «клик пришёл после сработавшего зажатия»: true — клик
   * надо игнорировать. */
  readonly consumeClickAfterLongPress: () => boolean;
};

export function useLongPress({
  onLongPress,
  delayMs = DEFAULT_DELAY_MS,
  tolerancePx = DEFAULT_TOLERANCE_PX,
  skipOn,
}: LongPressOptions): LongPress {
  const timerRef = useRef<number | null>(null);
  const originRef = useRef<{ x: number; y: number } | null>(null);
  const touchRef = useRef(false);
  const suppressClickRef = useRef(false);

  const cancelTimer = (): void => {
    if (timerRef.current !== null) {
      window.clearTimeout(timerRef.current);
      timerRef.current = null;
    }
    originRef.current = null;
  };

  // Таймер не должен пережить размонтирование строки.
  useEffect(() => cancelTimer, []);

  return {
    handlers: {
      onPointerDown: (event) => {
        if (skipOn?.(event) === true) {
          return;
        }
        cancelTimer();
        suppressClickRef.current = false;
        touchRef.current = event.pointerType === 'touch';
        if (event.pointerType !== 'touch') {
          return;
        }
        originRef.current = { x: event.clientX, y: event.clientY };
        timerRef.current = window.setTimeout(() => {
          timerRef.current = null;
          suppressClickRef.current = true;
          onLongPress();
        }, delayMs);
      },
      onPointerMove: (event) => {
        const origin = originRef.current;
        if (origin === null) {
          return;
        }
        const dx = event.clientX - origin.x;
        const dy = event.clientY - origin.y;
        if (Math.hypot(dx, dy) > tolerancePx) {
          cancelTimer();
        }
      },
      onPointerUp: cancelTimer,
      onPointerCancel: cancelTimer,
    },
    isTouchPointer: () => touchRef.current,
    consumeClickAfterLongPress: () => {
      const suppressed = suppressClickRef.current;
      suppressClickRef.current = false;
      return suppressed;
    },
  };
}
