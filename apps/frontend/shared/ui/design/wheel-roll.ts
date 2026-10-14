/** Математика программного ролла колеса (#аудит-анимаций 2026-10-14):
 * клавиатура и внешняя смена значения докручиваются собственной
 * анимацией, а не нативным behavior:'smooth' — длительность и кривая
 * должны быть нашими (--dl-ease), старт — от презентационного значения
 * (канон Apple §3), а сам ролл — прерываемым жестом в любой момент.
 * Длительность растёт с дистанцией, но зажата: один ряд — быстрый
 * щелчок 120мс (бюджет кнопки из AUDIT.md), дальние прыжки (Home/End)
 * не дольше 450мс. Кривая — токен --dl-ease
 * (cubic-bezier(0.32, 0.72, 0, 1)), тот же, что у всех transition слоя. */

/** Нижний порог длительности ролла (мс): короткая докрутка не должна
 * ощущаться задержкой. */
export const WHEEL_ROLL_MIN_MS = 120;

/** Верхний порог длительности ролла (мс): дальний прыжок не катит долго. */
export const WHEEL_ROLL_MAX_MS = 450;

/** Множитель дистанции: 240px (5 рядов) ≈ 144мс. */
const WHEEL_ROLL_MS_PER_PX = 0.6;

/** Длительность ролла для дистанции в пикселях (зажата в [MIN, MAX]). */
export function rollDurationMs(distancePx: number): number {
  const raw = Math.abs(distancePx) * WHEEL_ROLL_MS_PER_PX;
  return Math.max(WHEEL_ROLL_MIN_MS, Math.min(WHEEL_ROLL_MAX_MS, raw));
}

/** Значение кубической кривой Безье (x1,y1,x2,y2) при прогрессе x:
 * решаем параметр t бинарным поиском по X-компоненте, возвращаем
 * Y-компоненту. Достаточно 24 итераций: точность ~1e-7 при диапазоне
 * t ∈ [0,1]; контрольные точки X в [0,1] — поиск сходится всегда. */
export function rollEase(
  x: number,
  x1 = 0.32,
  y1 = 0.72,
  x2 = 0,
  y2 = 1,
): number {
  if (x <= 0) return 0;
  if (x >= 1) return 1;
  const at = (t: number): [number, number] => {
    const omt = 1 - t;
    const xB = 3 * omt * omt * t * x1 + 3 * omt * t * t * x2 + t * t * t;
    const yB = 3 * omt * omt * t * y1 + 3 * omt * t * t * y2 + t * t * t;
    return [xB, yB];
  };
  let lo = 0;
  let hi = 1;
  for (let i = 0; i < 24; i += 1) {
    const mid = (lo + hi) / 2;
    const [xB] = at(mid);
    if (xB < x) lo = mid;
    else hi = mid;
  }
  const [, yB] = at((lo + hi) / 2);
  return yB;
}
