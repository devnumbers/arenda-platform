/** Тактильный исход действия (решение владельца 2026-10-14, тикет
 * #1420 — скоуп B: вибрируют исходы успеха/ошибки вместе с видимым
 * сигналом; коммит без сигнала не хаптится; пресс не хаптится). Канон
 * Apple §13: каузальность (тот же момент, что тост/попап) и utility
 * (только успех/ошибка — info/warning/loading молчат). Веб-вибрация
 * есть только на Android Chrome: iOS Safari не реализует
 * navigator.vibrate — честный no-op (тот же договор, что у тика колеса
 * wheel-haptics.ts). Паттерны: успех — один короткий 16мс «готово»
 * (длиннее тика детента, читается как исход, не как прокрутку); ошибка
 * — двойной [32, 60, 32], читается как отказ и не спутывается с
 * одиночками. Пауза ≥100мс между исходами: синхронные соседи одного
 * действия (тост + попап) сливаются в один тик, а не в «брр». */

/** Длительность тика успеха (мс). */
export const ACTION_SUCCESS_MS = 16;

/** Паттерн тика ошибки: вибро-пауза-вибро (мс). */
export const ACTION_ERROR_PATTERN = [32, 60, 32] as const;

/** Минимальная пауза между исходами (мс). */
export const ACTION_HAPTIC_GAP_MS = 100;

let lastFireAt: number | null = null;

/** Пора ли вибрировать: первый исход всегда, дальше — не чаще GAP. */
export function shouldFireOutcomeHaptic(
  now: number,
  lastAt: number | null,
): boolean {
  return lastAt === null || now - lastAt >= ACTION_HAPTIC_GAP_MS;
}

/** Вибрация исхода; отсутствие API — тишина (no-op, включая SSR). */
export function outcomeHaptic(outcome: 'success' | 'error'): void {
  if (typeof navigator !== 'undefined' && typeof navigator.vibrate === 'function') {
    const now = Date.now();
    if (shouldFireOutcomeHaptic(now, lastFireAt)) {
      lastFireAt = now;
      navigator.vibrate(outcome === 'success' ? ACTION_SUCCESS_MS : [...ACTION_ERROR_PATTERN]);
    }
  }
}

/** Сброс троттлинга — шов для тестов. */
export function resetOutcomeHaptic(): void {
  lastFireAt = null;
}
