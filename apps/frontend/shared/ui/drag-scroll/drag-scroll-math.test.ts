import { describe, expect, it } from "vitest";
import {
  DRAG_START_THRESHOLD_PX,
  RIDE_MAX_MS,
  RIDE_TIME_CONSTANT_MS,
  STOP_PROJECTION_POWER,
  exponentialRide,
  isHorizontalDrag,
  pickSnapTarget,
  projectStopPoint,
} from "./drag-scroll-math";

// Математика mouse-drag прокси горизонтальных snap-лент (тикет #1349;
// research/apple-style-drag-hub-sections.md): гвард старта жеста,
// проекция точки остановки (веб-аналог targetContentOffset UIScrollView)
// и выбор snap-цели. Все функции чистые — канон node-env без DOM
// (прецедент shared/ui/pull-to-refresh/pull-math.ts).

describe("isHorizontalDrag — гвард старта drag-жеста", () => {
  it("порог: смещение до порога включительно не стартует жест (клик жив)", () => {
    expect(DRAG_START_THRESHOLD_PX).toBe(8);
    expect(isHorizontalDrag(0, 0)).toBe(false);
    expect(isHorizontalDrag(4, 0)).toBe(false);
    expect(isHorizontalDrag(DRAG_START_THRESHOLD_PX, 0)).toBe(false);
    expect(isHorizontalDrag(-DRAG_START_THRESHOLD_PX, 0)).toBe(false);
  });

  it("горизонталь доминирует: |dx| > |dy| и |dx| > порога", () => {
    expect(isHorizontalDrag(9, 0)).toBe(true);
    expect(isHorizontalDrag(-9, 0)).toBe(true);
    expect(isHorizontalDrag(20, 10)).toBe(true);
    expect(isHorizontalDrag(-50, -30)).toBe(true);
  });

  it("вертикаль доминирует или диагональ равна — жест не наш", () => {
    expect(isHorizontalDrag(20, 30)).toBe(false);
    expect(isHorizontalDrag(10, 10)).toBe(false);
    expect(isHorizontalDrag(0, 40)).toBe(false);
  });

  it("порог параметризуется (калибровка без правки кода)", () => {
    expect(isHorizontalDrag(9, 0, 10)).toBe(false);
    expect(isHorizontalDrag(11, 0, 10)).toBe(true);
  });
});

describe("projectStopPoint — идеальная точка остановки", () => {
  it("power — дефолт inertia-генератора motion-dom", () => {
    expect(STOP_PROJECTION_POWER).toBe(0.8);
  });

  it("нулевая скорость не уводит точку", () => {
    expect(projectStopPoint(120, 0)).toBe(120);
  });

  it("проекция scrollLeft + power·velocity (px/s)", () => {
    expect(projectStopPoint(100, 1000)).toBe(900);
    expect(projectStopPoint(100, 500)).toBe(500);
  });

  it("отрицательная скорость (движение к началу) уводит точку назад", () => {
    expect(projectStopPoint(200, -1000)).toBe(-600);
  });

  it("power параметризуется (калибровка без правки кода)", () => {
    expect(projectStopPoint(100, 1000, 0.5)).toBe(600);
  });
});

describe("pickSnapTarget — snap-цель, ближайшая к проекции", () => {
  const POINTS = [0, 176.5, 353, 529.5];

  it("пустой список целей не даёт ответа", () => {
    expect(pickSnapTarget(100, [])).toBeNull();
  });

  it("единственная точка — всегда цель", () => {
    expect(pickSnapTarget(100, [42])).toBe(42);
  });

  it("точное попадание в snap-точку", () => {
    expect(pickSnapTarget(353, POINTS)).toBe(353);
  });

  it("ближайшая к проекции между двумя точками", () => {
    expect(pickSnapTarget(80, POINTS)).toBe(0);
    expect(pickSnapTarget(200, POINTS)).toBe(176.5);
    expect(pickSnapTarget(300, POINTS)).toBe(353);
  });

  it("ровно посередине — детерминированно нижняя точка", () => {
    expect(pickSnapTarget((0 + 176.5) / 2, POINTS)).toBe(0);
  });

  it("проекция за границами контента клампится к крайней точке", () => {
    expect(pickSnapTarget(10_000, POINTS)).toBe(529.5);
    expect(pickSnapTarget(-500, POINTS)).toBe(0);
  });

  it("вход не обязан быть отсортированным", () => {
    expect(pickSnapTarget(200, [353, 0, 176.5, 529.5])).toBe(176.5);
  });
});

describe("калибровочные константы", () => {
  it("время-константа доезда в стартовой середине исследовательского диапазона", () => {
    // research/apple-style-drag-hub-sections.md §5: timeConstant ~325–500мс;
    // верх диапазона — glide «как у Apple», калибруется на живой приёмке.
    expect(RIDE_TIME_CONSTANT_MS).toBe(500);
  });
});

describe("exponentialRide — доезд к цели экспоненциальным затуханием", () => {
  it("нулевая дистанция — мгновенный доезд без анимации", () => {
    expect(exponentialRide(120, 120).durationMs).toBe(0);
    expect(exponentialRide(120, 120.4).durationMs).toBe(0);
  });

  it("ease(0) = 0, ease монотонна и к концу доезжает цель", () => {
    const { ease } = exponentialRide(45, 186);
    expect(ease(0)).toBe(0);
    expect(ease(0.3)).toBeLessThan(ease(0.6));
    expect(ease(0.6)).toBeLessThan(ease(0.9));
    // Остаток в конце — 0.5px по дизайну (инвариант проверен ниже), для
    // дистанции 141 это прогресс >0.99.
    expect(ease(1)).toBeGreaterThan(0.99);
  });

  it("форма — экспоненциальное затухание: остаток дистанции уменьшается в e раз за timeConstant", () => {
    const { ease } = exponentialRide(45, 186);
    const remaining = (t: number): number => 1 - ease(t);
    // За долю t* = τ/duration остаток составляет 1/e исходного.
    const { durationMs } = exponentialRide(45, 186);
    expect(remaining(500 / durationMs)).toBeCloseTo(1 / Math.E, 5);
  });

  it("к концу длительности остаток не превышает 0.5px (иначе браузер перещёлкнет рывком)", () => {
    for (const distance of [26.5, 141, 353, 698]) {
      const { durationMs, ease } = exponentialRide(0, distance);
      const restPx = distance * (1 - ease(1));
      expect(restPx).toBeLessThanOrEqual(0.51);
      expect(durationMs).toBeGreaterThan(0);
    }
  });

  it("длительность растёт с дистанцией и не зависит от направления", () => {
    const short = exponentialRide(0, 30).durationMs;
    const mid = exponentialRide(0, 141).durationMs;
    const long = exponentialRide(0, 700).durationMs;
    expect(short).toBeLessThan(mid);
    expect(mid).toBeLessThan(long);
    expect(exponentialRide(0, 141).durationMs).toBe(
      exponentialRide(141, 0).durationMs,
    );
  });

  it("длительность ограничена сверху (дальний флик не бежит вечность)", () => {
    const { durationMs } = exponentialRide(0, 100_000);
    expect(durationMs).toBe(RIDE_MAX_MS);
    expect(durationMs).toBeGreaterThan(exponentialRide(0, 700).durationMs);
  });
});
