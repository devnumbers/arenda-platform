/** Чистая математика бесконечного колеса WheelPicker (кольцо живёт у часов
 * и минут; месяцы крутятся конечной лентой — решение владельца 2026-10-13).
 * Лента значений — цикл из n логических рядов, отрисованный
 * WHEEL_LOOP_CYCLES раз подряд; позиция скролла меряется физическими
 * рядами. Когда до физического края остаётся один цикл запаса, колесо
 * молча перепрыгивает на полспана (WHEEL_LOOP_ANCHOR циклов) — содержимое
 * периодично, прыжок невидим. Инерцию прыжок не прерывает, потому что
 * запас до зоны прыжка (полспана минус цикл = 3 цикла; у часов 3456px)
 * недостижим одним жестом: дальность жёсткого флика ≈ 2600px
 * (v₀ ≈ 8000px/s × τ ≈ 325мс экспоненциальной декелерации). Девять циклов
 * вместо прежних 31 — прореживание DOM вдвое-втрое (минуты 1860 → 540
 * рядов) ради лёгкости прокрутки; запас после прыжка по-прежнему
 * превышает любой жест. DOM ограничен всегда: прыжок возвращает позицию
 * к середине ленты, а не наращивает её. */

/** Сколько раз логический цикл отрисован в DOM (нечётное — есть середина). */
export const WHEEL_LOOP_CYCLES = 9;

/** Индекс среднего (якорного) цикла: единственного с role=option. */
export const WHEEL_LOOP_ANCHOR = (WHEEL_LOOP_CYCLES - 1) / 2;

/** Положительный остаток: логический индекс физического ряда ленты. */
export function modRows(row: number, cycleLength: number): number {
  return ((row % cycleLength) + cycleLength) % cycleLength;
}

/** Всего физических рядов закольцованного колеса. */
export function loopRowCount(cycleLength: number): number {
  return cycleLength * WHEEL_LOOP_CYCLES;
}

/** Якорный ли ряд: средний цикл несёт role=option и стабильные id
 * (aria-activedescendant указывает сюда всегда). */
export function isAnchorRow(row: number, cycleLength: number): boolean {
  const start = WHEEL_LOOP_ANCHOR * cycleLength;
  return row >= start && row < start + cycleLength;
}

/** Скачок речентра в рядах: у края (меньше одного цикла запаса) —
 * полспана к середине, в середине — 0 (прыжок не нужен). */
export function loopRecenterRows(row: number, cycleLength: number): number {
  const halfSpan = WHEEL_LOOP_ANCHOR * cycleLength;
  if (row <= cycleLength) return halfSpan;
  if (row >= loopRowCount(cycleLength) - 1 - cycleLength) return -halfSpan;
  return 0;
}

/** Физический ряд, ближайший к текущей позиции, с данным логическим
 * индексом — короткая докрутка при внешней смене значения/клавиатуре;
 * переезд не длиннее полцикла. */
export function loopNearestRow(
  row: number,
  logical: number,
  cycleLength: number,
): number {
  let delta = logical - modRows(row, cycleLength);
  if (delta > cycleLength / 2) delta -= cycleLength;
  else if (delta < -cycleLength / 2) delta += cycleLength;
  return row + delta;
}

/** Ряд ленты для плавной синхронизации при ретаргете колеса. Смена длины
 * цикла (граничный год режет месяц-колесо) ре-анкорит на якорную копию:
 * физическая позиция мерялась прежней лентой и могла остаться за её
 * пределами — кламп посадил бы на чужой логический ряд. Клампы не нужны:
 * якорь в середине ленты, |переезд| ≤ полцикла по контракту
 * loopNearestRow, цель всегда в ленте. */
export function loopSyncRow(
  currentRow: number,
  valueIndex: number,
  cycleLength: number,
  prevCycleLength: number,
): number {
  if (prevCycleLength !== cycleLength) {
    return WHEEL_LOOP_ANCHOR * cycleLength + valueIndex;
  }
  return loopNearestRow(currentRow, valueIndex, cycleLength);
}
