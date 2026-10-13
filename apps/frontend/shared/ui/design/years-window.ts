/** Стартовое окно ленты годов шита «Месяц и год» без границ: от текущего
 * года вперёд без конца (решение владельца 2026-10-13 — «годы от текущего
 * и к будущему»). Выбранный год всегда внутри ленты: старт не позже
 * выбранного (чужое value из прошлого не выкидывается — колесо обязано
 * найти своё значение), конец — на три года дальше большего из выбранного
 * и текущего; у нижнего края onNearEnd колеса удлиняет ленту вперёд. */
export function initialYearsWindow(
  selectedYear: number,
  currentYear: number,
): { readonly start: number; readonly end: number } {
  return {
    start: Math.min(currentYear, selectedYear),
    end: Math.max(currentYear, selectedYear) + 3,
  };
}
