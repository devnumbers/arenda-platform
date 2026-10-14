/** Тактильный тик колеса на смену ряда под полосой (решение владельца
 * 2026-10-14; канон Apple §13 — каузальный отклик детента, как
 * UISelectionFeedbackGenerator у UIPickerView). Веб-вибрация есть только
 * на Android Chrome: iOS Safari не реализует navigator.vibrate — тик
 * там честный no-op, desktop без вибромотора молчит. Форма тика:
 * 8мс — короткий «щелчок» детента; пауза между тиками ≥40мс — на
 * быстром флике ряды пролетают чаще, чем вибромотор успевает отыграть
 * щелчок, без паузы они сливаются в сплошной гул. */

/** Минимальная пауза между тиками (мс). */
export const WHEEL_TICK_GAP_MS = 40;

/** Длительность одного тика (мс). */
export const WHEEL_TICK_MS = 8;

/** Пора ли тикать: первый тик всегда, дальше — не чаще GAP. */
export function shouldTickRow(now: number, lastTickAt: number | null): boolean {
  return lastTickAt === null || now - lastTickAt >= WHEEL_TICK_GAP_MS;
}

/** Тик по вибромотору; отсутствие API — тишина (no-op). */
export function tickRow(): void {
  if (typeof navigator !== 'undefined' && typeof navigator.vibrate === 'function') {
    navigator.vibrate(WHEEL_TICK_MS);
  }
}
