import { useCallback, useEffect, useRef } from 'react';
import type { PointerEvent as ReactPointerEvent } from 'react';

/**
 * Long-press как touch-жест (зажатие — только тач, на десктопе клик;
 * тикет #814, решения владельца 23.09): pointerdown с pointerType='touch'
 * заводит таймер, сдвиг пальца дальше допуска (скролл) или подъём —
 * отменяют. Сработавшее зажатие съедает click-хвост: click в пределах
 * окна после подъёма игнорируется — жест и так применил onLongPress,
 * клик бы отменил его (тап-после-зажатия). Окно считается от подъёма
 * (при длинном холде click приходит много позже срабатывания) и
 * закрывается таймером, новым pointerdown или съеденным click — флаг
 * подавления одноразовый и не переживает чужие активации (клавиатурные
 * в том числе). Курсорные указатели (мышь/перо) таймер не заводят вовсе.
 *
 * Логика жеста живёт в чистой фабрике createLongPress: события — обычные
 * аргументы, никаких React-рефов и DOM (юнит-слой тестируется в node);
 * useLongPress — тонкая обёртка: один экземпляр фабрики на монтирование,
 * dispose — cleanup размонтирования.
 */
/** Задержка зажатия: iOS-стандарт ~0.5s (уточняется на приёмке). */
const DEFAULT_DELAY_MS = 500;
/** Допуск сдвига пальца: дальше — это скролл, зажатие отменяется. */
const DEFAULT_TOLERANCE_PX = 10;
/** Окно click-хвоста: сколько миллисекунд после подъёма указателя
 * ближайший click считается хвостом зажатия и съедается. Ручка приёмки
 * рядом с delayMs/tolerancePx (#814: «тайминги подсветки выбора — на
 * приёмке»). */
const CLICK_TAIL_WINDOW_MS = 300;

/** Срез pointer-события, достаточный жесту: фабрика не знает ни
 * React-событий, ни DOM — хук подставляет React-событие, тесты —
 * литералы. */
export type LongPressEvent = {
  readonly pointerType: string;
  readonly clientX: number;
  readonly clientY: number;
};

export type CreateLongPressOptions<E extends LongPressEvent> = {
  readonly onLongPress: () => void;
  /** Ручки приёмки (#814: «тайминги подсветки выбора — на приёмке»):
   * задержка срабатывания и допуск сдвига уточняются владельцем на
   * живой приёмке. Оба значения фиксируются при создании жеста, а
   * создание ленивое — первым pointer-событием, не монтированием:
   * конфигурация создания у единственного потребителя статична, в
   * отличие от onLongPress/skipOn, читаемых свежими через optionsRef. */
  readonly delayMs?: number;
  readonly tolerancePx?: number;
  /** Места, откуда жест зажатия не начинается (например, ручка dnd со
   * своим pointer-жестом): предикат по событию pointerdown. */
  readonly skipOn?: (event: E) => boolean;
};

export type CreatedLongPress<E extends LongPressEvent> = {
  readonly handlers: {
    readonly onPointerDown: (event: E) => void;
    readonly onPointerMove: (event: E) => void;
    readonly onPointerUp: () => void;
    readonly onPointerCancel: () => void;
  };
  /** Был ли последний незакрытый skipOn'ом pointerdown touch-указателем:
   * click приходит после pointerup, когда pointerType из события уже
   * недоступен. Пропущенный skipOn pointerdown (ручка dnd со своим
   * жестом) флаг не обновляет — skip-выход onPointerDown срабатывает
   * раньше записи флага. */
  readonly isTouchPointer: () => boolean;
  /** Одноразовое «клик пришёл после сработавшего зажатия»: true — клик
   * надо игнорировать. */
  readonly consumeClickAfterLongPress: () => boolean;
  /** Полный демонтаж жеста: оба таймера (зажатие и окно click-хвоста)
   * не должны пережить размонтирование. */
  readonly dispose: () => void;
};

export function createLongPress<E extends LongPressEvent>({
  onLongPress,
  delayMs = DEFAULT_DELAY_MS,
  tolerancePx = DEFAULT_TOLERANCE_PX,
  skipOn,
}: CreateLongPressOptions<E>): CreatedLongPress<E> {
  let holdTimer: ReturnType<typeof setTimeout> | null = null;
  let clickTailTimer: ReturnType<typeof setTimeout> | null = null;
  let origin: { x: number; y: number } | null = null;
  let touchPointer = false;
  let suppressClick = false;

  const cancelHoldTimer = (): void => {
    if (holdTimer !== null) {
      clearTimeout(holdTimer);
      holdTimer = null;
    }
    origin = null;
  };

  const clearClickTailTimer = (): void => {
    if (clickTailTimer !== null) {
      clearTimeout(clickTailTimer);
      clickTailTimer = null;
    }
  };

  // Окно click-хвоста открывается подъёмом/отменой и закрывается таймером:
  // недоставленный click не оставляет подавление висеть до чужого
  // pointerdown. Окно считается от подъёма, не от срабатывания зажатия —
  // при длинном холде click приходит много позже срабатывания.
  const openClickTailWindow = (): void => {
    if (!suppressClick) {
      return;
    }
    clearClickTailTimer();
    clickTailTimer = setTimeout(() => {
      clickTailTimer = null;
      suppressClick = false;
    }, CLICK_TAIL_WINDOW_MS);
  };

  return {
    handlers: {
      onPointerDown: (event) => {
        if (skipOn?.(event) === true) {
          return;
        }
        cancelHoldTimer();
        // Новый жест снимает подавление: флаг не должен съесть чужую
        // активацию, если click-хвост так и не пришёл.
        suppressClick = false;
        clearClickTailTimer();
        touchPointer = event.pointerType === 'touch';
        if (event.pointerType !== 'touch') {
          return;
        }
        origin = { x: event.clientX, y: event.clientY };
        holdTimer = setTimeout(() => {
          holdTimer = null;
          suppressClick = true;
          onLongPress();
        }, delayMs);
      },
      onPointerMove: (event) => {
        if (origin === null) {
          return;
        }
        const dx = event.clientX - origin.x;
        const dy = event.clientY - origin.y;
        if (Math.hypot(dx, dy) > tolerancePx) {
          cancelHoldTimer();
        }
      },
      onPointerUp: () => {
        openClickTailWindow();
        cancelHoldTimer();
      },
      onPointerCancel: () => {
        openClickTailWindow();
        cancelHoldTimer();
      },
    },
    isTouchPointer: () => touchPointer,
    consumeClickAfterLongPress: () => {
      const suppressed = suppressClick;
      suppressClick = false;
      clearClickTailTimer();
      return suppressed;
    },
    dispose: () => {
      cancelHoldTimer();
      clearClickTailTimer();
    },
  };
}

/** React-обёртка над фабричными типами: событие —
 * ReactPointerEvent<HTMLElement>; докстринги полей — у фабричных типов
 * выше, здесь не дублируются. */
export type LongPressOptions = CreateLongPressOptions<ReactPointerEvent<HTMLElement>>;

/** То же для результата хука: CreatedLongPress без dispose — тот
 * внутренний cleanup размонтирования и наружу не отдаётся. */
export type LongPress = Omit<CreatedLongPress<ReactPointerEvent<HTMLElement>>, 'dispose'>;

export function useLongPress({
  onLongPress,
  delayMs,
  tolerancePx,
  skipOn,
}: LongPressOptions): LongPress {
  const gestureRef = useRef<CreatedLongPress<ReactPointerEvent<HTMLElement>> | null>(null);
  // Свежие опции каждого рендера — в ref: сработать зажатие должно на
  // актуальный onLongPress. Ref пишется в effect, читается только
  // трамплинами в момент события — во время рендера его не трогаем.
  const optionsRef = useRef<LongPressOptions>({ onLongPress, delayMs, tolerancePx, skipOn });
  const fireOnLongPress = useCallback((): void => {
    optionsRef.current.onLongPress();
  }, []);
  const skipGesture = useCallback(
    (event: ReactPointerEvent<HTMLElement>): boolean =>
      optionsRef.current.skipOn?.(event) === true,
    [],
  );
  useEffect(() => {
    optionsRef.current = { onLongPress, delayMs, tolerancePx, skipOn };
  });

  // Экземпляр — один на монтирование: состояние жеста не пересоздаётся
  // рендером. Создаётся лениво при первом событии — фабрика получает
  // рефы только из событийного потока, не из рендера.
  const ensureGesture = (): CreatedLongPress<ReactPointerEvent<HTMLElement>> => {
    let gesture = gestureRef.current;
    if (gesture === null) {
      gesture = createLongPress<ReactPointerEvent<HTMLElement>>({
        onLongPress: fireOnLongPress,
        delayMs,
        tolerancePx,
        skipOn: skipGesture,
      });
      gestureRef.current = gesture;
    }
    return gesture;
  };

  // Таймеры жеста не должны пережить размонтирование строки.
  useEffect(
    () => () => {
      gestureRef.current?.dispose();
    },
    [],
  );

  return {
    handlers: {
      onPointerDown: (event) => ensureGesture().handlers.onPointerDown(event),
      onPointerMove: (event) => ensureGesture().handlers.onPointerMove(event),
      onPointerUp: () => ensureGesture().handlers.onPointerUp(),
      onPointerCancel: () => ensureGesture().handlers.onPointerCancel(),
    },
    isTouchPointer: () => ensureGesture().isTouchPointer(),
    consumeClickAfterLongPress: () => ensureGesture().consumeClickAfterLongPress(),
  };
}
