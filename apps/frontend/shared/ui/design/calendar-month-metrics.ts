/** Точная геометрия месяца бесконечного календаря (план 051; с плана 052 —
 * размеры виртуальных строк TanStack Virtual).
 *
 * Геометрия месяца известна заранее и считается точно: рядов
 * `ceil((пустые + дни)/7)`, высота = заголовок 24 (text-xl/leading-6) +
 * mt-4 16 + ряды×клетка + (ряды−1)×2 (gap-0.5); клетка квадратна ширине
 * колонки: (ширина − 32 px-4 − 12 межклеточных)/7. Ширина колонки
 * приходит ResizeObserver'ом; до замера — телефонный фолбэк 360 (план
 * 050): виртуальные строки монтируются только после замера, якорный
 * скролл открытия считает по той же формуле. */

import { daysInMonth, firstWeekdayOfMonth } from './month-grid';

/** Ряды месячной сетки: пустые клетки первой недели + дни, по 7 на ряд. */
export function calendarMonthGridRows(days: number, leadingBlanks: number): number {
  return Math.ceil((leadingBlanks + days) / 7);
}

/** Оценка высоты блока месяца (без margin) до первого рендера. */
export function calendarMonthIntrinsicPx(args: {
  readonly rows: number;
  /** Ширина колонки ленты; null — ещё не замерена. */
  readonly columnWidth: number | null;
}): number {
  if (args.columnWidth === null) {
    return 360;
  }
  const cell = (args.columnWidth - 32 - 12) / 7;
  return 24 + 16 + args.rows * cell + (args.rows - 1) * 2;
}

/** Высоты виртуальных строк ленты (план 052; отступ у всех — план 053):
 * контент месяца + межстрочный отступ 32 у КАЖДОЙ строки (виртуализатор
 * позиционирует строки абсолютно — margin-flow нет, отступ живёт в
 * размере строки как padding-bottom). Отступ у всех, включая последнюю:
 * лента растёт append'ом вниз (план 053), и бывшая последняя строка
 * обязана сохранить свой отступ, иначе новые строки поедут на 32px выше
 * суммы старых высот. Хвостовой отступ последней строки растворяется в
 * хвосте прокрутки (FEED_TAIL_PX). */
export function calendarMonthRowHeights(
  months: ReadonlyArray<{ readonly year: number; readonly month0: number }>,
  columnWidth: number | null,
): number[] {
  return months.map(({ year, month0 }) => {
    const rows = calendarMonthGridRows(daysInMonth(year, month0), firstWeekdayOfMonth(year, month0));
    const content = calendarMonthIntrinsicPx({ rows, columnWidth });
    return content + 32;
  });
}

/** Смещение верхней кромки строки index (высоты строк ленты): вход
 * якорного скролла открытия и прыжков чипа. При append'е вниз (план 053)
 * смещения существующих строк неизменны — рост истории только добавляет
 * высоты в хвост. */
export function rowOffset(heights: ReadonlyArray<number>, index: number): number {
  let offset = 0;
  for (let i = 0; i < index && i < heights.length; i += 1) {
    offset += heights[i] ?? 0;
  }
  return offset;
}
