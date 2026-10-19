/**
 * Pure math of the mouse/pen drag proxy over native horizontal snap-scroll
 * rails (тикет #1349; решение владельца 14.10 — возврат с Embla на своё
 * решение с физикой по эталону Apple; канон — DESIGN.md §8).
 *
 * Every constant here is the measured/de-facto canonical value of
 * UIScrollView (WWDC18 803 «Designing Fluid Interfaces», WWDC23 «Animate
 * with springs»):
 *
 * - The natural stop point is *projected* up front from the release
 *   velocity — the web counterpart of
 *   `scrollViewWillEndDragging(withVelocity:targetContentOffset:)`:
 *   `D = scrollLeft + (v/1000)·d/(1−d)` at `d = 0.998` (the standard
 *   UIScrollView deceleration rate; the web counterpart of scroll
 *   deceleration, apple-design §6).
 * - The settle to the snap target is a critically damped decay WITHOUT
 *   overshoot (damping 1.0, response 0.4s — Apple's move/reposition
 *   table), starting at the finger's release velocity clamped to the
 *   critical damping line (`criticalSettle`) — UIScrollView decelerates
 *   INTO the target and never crosses it; bounce lives only in the edge
 *   rubber band.
 * - The edge rubber band is the reverse-engineered UIScrollView curve
 *   `b = (1 − 1/(x·c/H + 1))·H` with `c = 0.55` — the same constant and
 *   function the PTR gesture runs (`pull-math.ts`), so every gesture in
 *   the app shares one physics.
 *
 * Touch never enters the proxy: the hook gates it on
 * `pointerType: mouse | pen`, so touch rails keep native momentum, native
 * rubber-band bounce and `scroll-snap-type: x mandatory` untouched.
 */

/** Порог старта drag-жеста, px: смещение мыши до него — ещё клик
 * (карточка открывается), после — прокрутка (mouse-only). */
export const DRAG_START_THRESHOLD_PX = 8;

/** Deceleration rate UIScrollView (норм). Множитель проекции d/(1−d) = 499. */
export const DECELERATION_RATE = 0.998;

/** Отклик доезда, с (таблица Apple «move / reposition»: damping 1.0,
 * response 0.4). Собственная частота критически демпфированного звена
 * ω = 2π/response — на полпереода до цели доезжает за ~response. */
export const RAIL_SPRING_RESPONSE_S = 0.4;
export const SETTLE_OMEGA = (2 * Math.PI) / RAIL_SPRING_RESPONSE_S;

/** Резинка у краёв — константа UIScrollView; сама функция живёт в
 * pull-math (одна физика на все жесты приложения), сюда реэкспортируется
 * под именем модуля. */
export { rubberBandOffset as railRubberBandOffset } from "@/shared/ui/pull-to-refresh/pull-math";
export const RUBBER_BAND_C = 0.55;

/**
 * Идеальная точка остановки при отпускании со скоростью `velocityPxPerS`:
 * `D = scrollLeft + (v/1000)·d/(1−d)` — то самое
 * `targetContentOffset` проекции UIScrollView. Отрицательная скорость
 * уводит точку к началу.
 */
export function projectStopPoint(
  scrollLeftPx: number,
  velocityPxPerS: number,
  decelerationRate: number = DECELERATION_RATE,
): number {
  return (
    scrollLeftPx +
    (velocityPxPerS / 1000) * (decelerationRate / (1 - decelerationRate))
  );
}

/** Визуальный вылет резинки за край считается делегированием в
 * pull-math (`railRubberBandOffset`) — отдельных локальных копий нет. */

/**
 * Гвард старта горизонтального drag-жеста: горизонталь должна доминировать
 * по смещению (|dx| > |dy|) и превысить порог — иначе жест это клик или
 * вертикальный скролл страницы.
 */
export function isHorizontalDrag(
  dxPx: number,
  dyPx: number,
  thresholdPx: number = DRAG_START_THRESHOLD_PX,
): boolean {
  return Math.abs(dxPx) > Math.abs(dyPx) && Math.abs(dxPx) > thresholdPx;
}

/** Остаток дистанции в конце доезда, px: меньше — и восстанавливать snap
 * безопасно, браузер не перещёлкнет рывком. */
export const SETTLE_REST_PX = 0.5;
/** Кап длительности доезда, мс — дальние броски не бегут вечность. */
export const SETTLE_MAX_MS = 1200;

/**
 * Доезд к цели критическим демпфированием БЕЗ перелёта — так оседает
 * UIScrollView: декелерация в цель, никогда не через неё (отскок живёт
 * только в резинке у краёв). Замкнутая форма:
 * `disp(t) = (d₀ + (v₀ + ω·d₀)·t)·e^(−ω·t)`, где начальная скорость
 * подписана и направлена к цели. Собственная частота — АДАПТИВНАЯ:
 * `ω = max(ω_base, |v₀|/|d₀|)` поднимает линию критического демпфирования
 * до скорости жеста, поэтому доезд всегда стартует СО скоростью отпускания
 * (velocity handoff без излома, apple-design §5). Прежний зажим скорости
 * вниз до ω_base·|d₀| обрывал короткие флики между соседними снапами
 * кирпичной стеной: палец отпускали на 2500px/s, а доезд стартовал с
 * 785px/s (решение владельца 18.10 — «слишком резкий»). Перелёта нет
 * и теперь: b = v₀ + ω·d₀ ≥ 0 при ω ≥ |v₀|/|d₀| — кривая декелерирует В
 * цель и никогда не через неё.
 *
 * Возвращает длительность (момент, когда |disp| ≤ SETTLE_REST_PX, шаг
 * поиска 8мс, кап SETTLE_MAX_MS) и нормированную кривую для
 * `animate(from, to, { duration, ease })` — движение исполняет motion
 * (ADR 0031), математика детерминирована и тестируема.
 */
export function criticalSettle(
  fromPx: number,
  toPx: number,
  velocityPxPerS: number,
  omega: number = SETTLE_OMEGA,
): { durationMs: number; ease: (t: number) => number } {
  const d0 = fromPx - toPx;
  if (Math.abs(d0) <= SETTLE_REST_PX) {
    return { durationMs: 0, ease: (t) => t };
  }
  const toward = -Math.sign(d0) * velocityPxPerS; // >0 — движение к цели
  const vToward = Math.max(toward, 0);
  const omegaEff = Math.max(omega, vToward / Math.abs(d0));
  const v0Real = -Math.sign(d0) * vToward;
  const b = v0Real + omegaEff * d0;
  const disp = (tSec: number): number =>
    (d0 + b * tSec) * Math.exp(-omegaEff * tSec);
  const stepSec = 0.008;
  let durationSec = 0;
  while (durationSec < SETTLE_MAX_MS / 1000) {
    durationSec += stepSec;
    if (Math.abs(disp(durationSec)) <= SETTLE_REST_PX) break;
  }
  const durationMs = Math.min(durationSec * 1000, SETTLE_MAX_MS);
  const total = durationMs / 1000;
  return {
    durationMs,
    ease: (t: number) => 1 - disp(t * total) / d0,
  };
}
