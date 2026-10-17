/** Точная оценка высоты офскрин-блока месяца бесконечного календаря
 * (план 051).
 *
 * Фон: `contain-intrinsic-size` у `[content-visibility:auto]`-блока —
 * размер до первого рендера, и с `[overflow-anchor:none]` (ручной якорь
 * приписки, план 051) никакой движок больше не компенсирует расхождение
 * оценки с реальностью при первом рендере блока у края: статические 480
 * (десктоп) / 360 (телефон) против реальных 244–492px дёргали контент на
 * десятки пикселей при входе каждого месяца. Геометрия месяца известна
 * заранее — оценка считается точно: рядов `ceil((пустые + дни)/7)`,
 * высота = заголовок 24 (text-xl/leading-6) + mt-4 16 + ряды×клетка +
 * (ряды−1)×2 (gap-0.5); клетка квадратна ширине колонки: (ширина − 32
 * px-4 − 12 межклеточных)/7. Верхний отступ не-первого блока (mt-8) в
 * оценку не входит — margin живёт вне content-visibility и laid out
 * всегда. Ширина колонки приходит ResizeObserver'ом; до замера —
 * телефонный фолбэк 360 (план 050): первый кадр открытия якорится
 * offsetTop реального блока, расхождение живёт один рендер. */

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
