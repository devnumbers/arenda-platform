'use client';

import {
  useEffect,
  useId,
  useRef,
  useState,
  type JSX,
  type KeyboardEvent,
  type MouseEvent,
} from 'react';
import { cn } from '@/shared/lib/cn';
import {
  WHEEL_LOOP_ANCHOR,
  isAnchorRow,
  loopRecenterRows,
  loopRowCount,
  loopSyncRow,
  modRows,
} from './wheel-loop';

/** Колесо-«крутилка» дизайн-слоя (редизайн 2026-09-04 по решению
 * владельца, Figma 1539-82659): вертикальный список с нативным скроллом
 * и снапом к ряду, инерция тач-скролла, ленты-градиенты сверху и снизу и
 * серая полоса выбора за центральной строкой. Геометрия: окно 240px
 * (класс h-[240px]) = 5 рядов по 48px (WHEEL_ROW_HEIGHT) — по 2 ряда над
 * и под выбранным (правка владельца: видно на 2 ряда, не 3), поля по
 * 96px (класс py-[96px]) — ряд садится в полосу пиксель в пиксель.
 * Полоса выбора 48px без закруглений (правка владельца — прямые углы).
 * Кегль всех рядов единый — Mobile/Text/XL (20/24 Regular), различие
 * только цветом: выбранный #171A1C (content), остальные #6F787C
 * (content-secondary) — каскад кеглей старого макета 848:8720 отменён.
 * Значение коммитится,
 * когда прокрутка осела; клик по ряду и клавиатура коммитят сразу.
 *
 * Клавиатура — собственная, а не общий модуль listbox-keyboard: то колесо
 * со скролл-снапом и aria-activedescendant, без roving focus, без Enter/
 * Space/Escape — семантика стрелок иная; в режиме loop стрелки заворачивают
 * по кругу (решение владельца #810), без loop — упираются в края.
 *
 * Режим loop (#810: часы, минуты, месяцы — «как у Apple»): логический цикл
 * items отрисован 31 раз (wheel-loop.ts), у края лента молча перепрыгивает
 * на полспана — прыжок невидим (содержимое периодично) и не прерывает
 * инерцию: запас до зоны прыжка недостижим одним жестом. Скринридерам
 * показан только средний (якорный) цикл: role=option и стабильные id там,
 * повторы aria-hidden; aria-activedescendant всегда указывает на якорную
 * копию выбранного значения. При min/max цикл составляют только разрешённые
 * значения (MonthYearPicker режет месяц-колесо) — закольцовка не открывает
 * запрещённые. Годы остаются линейными: ось времени не замыкается. */

/** Высота ряда (px); окно колеса — 5 рядов. */
const WHEEL_ROW_HEIGHT = 48;
/** Пауза тишины скролла, после которой ряд под полосой считается выбранным. */
const SETTLE_TIMEOUT_MS = 150;

/** Ряд ленты под полосой для текущего scrollTop (с клампом в ленту). */
function rowFromScrollTop(list: HTMLUListElement, maxRow: number): number {
  return Math.max(0, Math.min(Math.round(list.scrollTop / WHEEL_ROW_HEIGHT), maxRow));
}

export type WheelPickerItem = {
  readonly value: string;
  readonly label: string;
};

export type WheelPickerProps = {
  readonly items: ReadonlyArray<WheelPickerItem>;
  readonly value: string;
  readonly onValueChange: (value: string) => void;
  /** Имя колеса для скринридеров («Месяц», «Год»). */
  readonly label: string;
  /** Замкнуть ленту в кольцо: крутится без конца в обе стороны
   * (часы, минуты, месяцы). Без loop лента конечная. */
  readonly loop?: boolean;
  /** Прокрутка дошла до края списка (за 2 ряда) — родитель может удлинить
   * items (бесконечная лента годов пикера месяц/год). Только без loop. */
  readonly onNearEnd?: () => void;
  /** То же у верхнего края: родитель может удлинить items назад
   * (лента годов пикера периода операций уходит в прошлое без предела).
   * Только без loop. */
  readonly onNearStart?: () => void;
  /** Полоса выбора за центральным рядом; false — когда полосу рисует общий
   * контейнер (WheelPickerSheet: одна полоса на все колонки, скругление
   * только по внешним краям — решение владельца 2026-09-04). */
  readonly strip?: boolean;
  readonly className?: string;
};

/** Ряд ленты. dark — ряд под полосой; selected — коммиченное значение
 * (aria), в loop совпадают не всегда. Мемоизация рядов — за React
 * Compiler'ом: при скролле меняется только dark двух рядов. */
function WheelRow({
  rowId,
  label,
  dark,
  selected,
  index,
  option,
}: {
  readonly rowId: string;
  readonly label: string;
  readonly dark: boolean;
  readonly selected: boolean;
  readonly index: number;
  readonly option: boolean;
}): JSX.Element {
  return (
    <li
      id={rowId}
      {...(option
        ? { role: 'option', 'aria-selected': selected }
        : { 'aria-hidden': true })}
      data-index={index}
      style={{ height: WHEEL_ROW_HEIGHT }}
      className={cn(
        // Единый кегль всех рядов (Figma 1539-82659): выбранный
        // темнеет, остальные серые — без каскада размеров.
        'flex shrink-0 cursor-pointer snap-center items-center justify-center font-sans text-xl leading-6 text-content-secondary transition-colors',
        dark && 'text-content',
      )}
    >
      {label}
    </li>
  );
}

export function WheelPicker({
  items,
  value,
  onValueChange,
  label,
  loop = false,
  onNearEnd,
  onNearStart,
  strip = true,
  className,
}: WheelPickerProps): JSX.Element {
  const listRef = useRef<HTMLUListElement | null>(null);
  const settleTimerRef = useRef<number | null>(null);
  // Первая установка колеса — мгновенная (как у iOS: выбранная строка уже
  // в центре при открытии), дальнейшие синхронизации — плавные.
  const instantRef = useRef(true);
  // Длина цикла, под которую позиционирована лента: отличает ретаргет
  // внутри ленты (ближайший представитель) от смены самой ленты
  // (ре-анкор, loopSyncRow).
  const loopLengthRef = useRef(items.length);
  // Длина списка, о крае которого колесо уже сообщило (onNearEnd /
  // onNearStart): защита от повторных вызовов на каждом скролл-событии,
  // пока родитель удлиняет. Только без loop.
  const nearEndAtLengthRef = useRef<number | null>(null);
  const nearStartAtLengthRef = useRef<number | null>(null);
  const optionId = useId();
  const valueIndex = Math.max(items.findIndex((item) => item.value === value), 0);
  // Позиция скролла в физических рядах ленты: без loop ряд = индекс
  // items, в loop — строка удлинённой ленты (якорный цикл при старте).
  const [scrollRow, setScrollRow] = useState(() =>
    loop ? WHEEL_LOOP_ANCHOR * items.length + valueIndex : valueIndex,
  );
  const rowCount = loop ? loopRowCount(items.length) : items.length;

  useEffect(
    () => () => {
      if (settleTimerRef.current !== null) window.clearTimeout(settleTimerRef.current);
    },
    [],
  );

  // Колесо следует за значением: внешний выбор или клавиатура плавно
  // докручивают нужный ряд в центр; после собственного скролла человека
  // ничего не дёргается — позиция уже совпадает с целевой. Актуальный
  // индекс для типографики приходит из событий скролла этого прокрута.
  // В loop первая установка садится на якорную копию (середина ленты —
  // максимальный запас пробега в обе стороны), дальше — ближайший
  // представитель значения; смена длины цикла — ре-анкор на якорную
  // копию: прежняя позиция мерялась чужой лентой.
  useEffect(() => {
    const list = listRef.current;
    if (list === null) return;
    const maxRow = loop ? loopRowCount(items.length) - 1 : items.length - 1;
    let targetRow = valueIndex;
    if (loop) {
      if (instantRef.current) {
        targetRow = WHEEL_LOOP_ANCHOR * items.length + valueIndex;
      } else {
        targetRow = loopSyncRow(
          rowFromScrollTop(list, maxRow),
          valueIndex,
          items.length,
          loopLengthRef.current,
        );
      }
      // Лента переезжает под новую длину — фиксируем её до следующей
      // синхронизации, чтобы та знала, откуда ре-анкориться.
      loopLengthRef.current = items.length;
    }
    if (Math.round(list.scrollTop / WHEEL_ROW_HEIGHT) !== targetRow) {
      list.scrollTo({
        top: targetRow * WHEEL_ROW_HEIGHT,
        behavior: instantRef.current ? 'auto' : 'smooth',
      });
    }
    instantRef.current = false;
  }, [valueIndex, loop, items.length]);

  const commitIndex = (index: number): void => {
    const item = items[index];
    if (item !== undefined && item.value !== value) {
      onValueChange(item.value);
    }
  };

  const handleScroll = (): void => {
    const list = listRef.current;
    if (list === null) return;
    const row = rowFromScrollTop(list, rowCount - 1);
    setScrollRow(row);
    if (loop) {
      const jumpRows = loopRecenterRows(row, items.length);
      if (jumpRows !== 0) {
        // Речентр у края: мгновенный перенос на полспана к середине —
        // невидим (содержимое периодично) и не прерывает инерцию:
        // запас до зоны прыжка недостижим одним жестом. Подсветку
        // двигаем сразу, подтвердит рядом вызванный прыжком
        // scroll-событие (оно же переустановит settle на новом месте).
        list.scrollTop += jumpRows * WHEEL_ROW_HEIGHT;
        setScrollRow(row + jumpRows);
      }
    } else {
      // Повторный сигнал о том же крае — только после удлинения списка
      // родителем (иначе инерционный доскат у края растянул бы его на
      // сотни).
      if (
        onNearEnd !== undefined &&
        row >= items.length - 2 &&
        nearEndAtLengthRef.current !== items.length
      ) {
        nearEndAtLengthRef.current = items.length;
        onNearEnd();
      }
      if (
        onNearStart !== undefined &&
        row <= 1 &&
        nearStartAtLengthRef.current !== items.length
      ) {
        nearStartAtLengthRef.current = items.length;
        onNearStart();
      }
    }
    if (settleTimerRef.current !== null) window.clearTimeout(settleTimerRef.current);
    settleTimerRef.current = window.setTimeout(
      () => commitIndex(loop ? modRows(row, items.length) : row),
      SETTLE_TIMEOUT_MS,
    );
  };

  const handleKeyDown = (event: KeyboardEvent<HTMLUListElement>): void => {
    let next: number;
    if (event.key === 'ArrowUp') {
      next = loop ? modRows(valueIndex - 1, items.length) : Math.max(valueIndex - 1, 0);
    } else if (event.key === 'ArrowDown') {
      next = loop
        ? modRows(valueIndex + 1, items.length)
        : Math.min(valueIndex + 1, items.length - 1);
    } else if (event.key === 'Home') next = 0;
    else if (event.key === 'End') next = items.length - 1;
    else return;
    event.preventDefault();
    commitIndex(next);
  };

  // Клик по ряду — pointer-сокращение; клавиатурный эквивалент живёт на
  // самом списке (стрелки/Tab), поэтому клики делегированы контейнеру.
  const handleClick = (event: MouseEvent<HTMLUListElement>): void => {
    const option =
      event.target instanceof Element ? event.target.closest<HTMLElement>('li[data-index]') : null;
    const index = option?.dataset.index;
    if (index !== undefined) {
      commitIndex(Number(index));
    }
  };

  return (
    <div className={cn('relative', className)}>
      {strip && (
        <div
          aria-hidden
          className="pointer-events-none absolute inset-x-0 top-1/2 h-12 -translate-y-1/2 bg-surface-muted"
        />
      )}
      <ul
        ref={listRef}
        role="listbox"
        aria-label={label}
        aria-activedescendant={`${optionId}-${valueIndex}`}
        tabIndex={0}
        onScroll={handleScroll}
        onKeyDown={handleKeyDown}
        onClick={handleClick}
        className="relative flex h-[240px] list-none flex-col snap-y snap-mandatory overflow-y-scroll overscroll-y-contain py-[96px] outline-none [scrollbar-width:none] focus-visible:ring-2 focus-visible:ring-primary [&::-webkit-scrollbar]:hidden"
      >
        {Array.from({ length: rowCount }, (_, row) => {
          const logical = loop ? modRows(row, items.length) : row;
          const item = items[logical];
          if (item === undefined) return null;
          const option = !loop || isAnchorRow(row, items.length);
          return (
            <WheelRow
              key={loop ? row : item.value}
              rowId={`${optionId}-${option ? logical : `r${row}`}`}
              label={item.label}
              dark={row === scrollRow}
              selected={option && logical === valueIndex}
              index={logical}
              option={option}
            />
          );
        })}
      </ul>
      <div
        aria-hidden
        className="pointer-events-none absolute inset-x-0 top-0 h-12 bg-gradient-to-b from-surface to-transparent"
      />
      <div
        aria-hidden
        className="pointer-events-none absolute inset-x-0 bottom-0 h-12 bg-gradient-to-t from-surface to-transparent"
      />
    </div>
  );
}
