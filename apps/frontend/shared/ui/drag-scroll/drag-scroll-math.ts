/**
 * Pure math of the mouse-drag proxy over native horizontal snap-scroll
 * rails (тикет #1349; research/apple-style-drag-hub-sections.md).
 *
 * The gesture follows UIScrollView's deceleration model: the release
 * velocity is not played out frame by frame — the natural stop point is
 * *projected* up front and the rail rides to it (`projectStopPoint` +
 * `pickSnapTarget` here, the web counterpart of
 * `scrollViewWillEndDragging(withVelocity:targetContentOffset:)`; the ride
 * itself is `exponentialRide` — the same exp-decay motion the
 * framer-motion inertia generator produces). Constants are motion-dom's
 * own inertia defaults — the same code path every framer-motion
 * `dragMomentum` release takes.
 *
 * Touch is never involved: the hook gates the whole proxy on
 * `pointerType: mouse | pen`, so touch rails keep native momentum, bounce
 * and `scroll-snap-type: mandatory` untouched.
 */

/** Порог старта drag-жеста, px: смещение мыши до него — ещё клик
 * (карточка открывается), после — прокрутка (Embla `dragThreshold`,
 * mouse-only). */
export const DRAG_START_THRESHOLD_PX = 8;

/** Power проекции — дефолт inertia-генератора motion-dom:
 * идеальная точка остановки = scrollLeft + power·velocity(px/s). */
export const STOP_PROJECTION_POWER = 0.8;

/** Время-константа экспоненциального доезда к snap-цели, мс. Стартовая
 * калибровка — верх исследовательского диапазона (~325–500мс, §5 research):
 * glide «как у Apple»; уточняется на живой приёмке. */
export const RIDE_TIME_CONSTANT_MS = 500;

/** Остаток дистанции в конце доезда, px: меньше — и восстанавливать snap
 * безопасно, браузер не перещёлкнет рывком. */
export const RIDE_REST_PX = 0.5;
/** Границы длительности доезда: коротким движениям — минимальный такт,
 * дальний флик не бежит вечность. Кап подобран так, чтобы для лент хаба
 * (контент ≤ ~700px) инвариант остатка RIDE_REST_PX держался и на капе
 * (700·e^(−3700/500) < 0.5). */
export const RIDE_MIN_MS = 350;
export const RIDE_MAX_MS = 3700;

/**
 * Доезд от `fromPx` к `toPx` экспоненциальным затуханием — та же модель,
 * что у inertia-генератора motion (движение = target − амплитуда·e^(−t/τ)),
 * но с целью, посчитанной в этом модуле, и явными кейфреймами: доезд
 * исполняется обычным tween `animate(from, to, { durationMs, ease })`.
 *
 * `ease(t)` — нормированная экспонента; при некапнутой длительности остаток
 * дистанции в конце равен RIDE_REST_PX (за границы длительности — не больше
 * пары px, невидимо для перещёлкивания). Направление не важно. Нулевая
 * дистанция даёт durationMs 0 — хук исполняет доезд мгновенной установкой
 * (семантика reduced-motion и микропоправок).
 */
export function exponentialRide(
  fromPx: number,
  toPx: number,
  timeConstantMs: number = RIDE_TIME_CONSTANT_MS,
): { durationMs: number; ease: (t: number) => number } {
  const distance = Math.abs(toPx - fromPx);
  if (distance <= RIDE_REST_PX) {
    return { durationMs: 0, ease: (t) => t };
  }
  const uncapped = timeConstantMs * Math.log(distance / RIDE_REST_PX);
  const durationMs = Math.min(Math.max(uncapped, RIDE_MIN_MS), RIDE_MAX_MS);
  const rate = durationMs / timeConstantMs;
  return { durationMs, ease: (t) => 1 - Math.exp(-rate * t) };
}

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

/**
 * Идеальная точка остановки экспоненциальной модели: где естественно
 * остановилась бы лента при отпускании со скоростью `velocityPxPerS`
 * (то самое `ideal = origin + power·velocity` inertia-генератора).
 * Отрицательная скорость уводит точку к началу.
 */
export function projectStopPoint(
  scrollLeftPx: number,
  velocityPxPerS: number,
  power: number = STOP_PROJECTION_POWER,
): number {
  return scrollLeftPx + power * velocityPxPerS;
}

/**
 * Snap-цель доезда: ближайшая к идеальной точке snap-позиция. Проекция за
 * границами контента клампится к крайней точке (за краем тяги нет —
 * см. research §2.2/§5: резинку на нативном скроллере не имитируем).
 * Вход не обязан быть отсортирован; ничья разрешается нижней точкой.
 */
export function pickSnapTarget(
  idealPx: number,
  snapPointsPx: ReadonlyArray<number>,
): number | null {
  if (snapPointsPx.length === 0) return null;
  const sorted = [...snapPointsPx].sort((a, b) => a - b);
  const clamped = Math.min(
    Math.max(idealPx, sorted[0] as number),
    sorted[sorted.length - 1] as number,
  );
  let nearest = sorted[0] as number;
  for (const point of sorted) {
    if (Math.abs(point - clamped) < Math.abs(nearest - clamped)) {
      nearest = point;
    }
  }
  return nearest;
}
