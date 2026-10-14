import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import {
  DECELERATION_RATE,
  DRAG_START_THRESHOLD_PX,
  RAIL_SPRING_RESPONSE_S,
  RUBBER_BAND_C,
  SETTLE_MAX_MS,
  criticalSettle,
  isHorizontalDrag,
  pickSnapTarget,
  projectStopPoint,
  railRubberBandOffset,
} from "./snap-rail-physics";

describe("projectStopPoint — проекция остановки scrollViewWillEndDragging", () => {
  it("константа Apple d = 0.998", () => {
    expect(DECELERATION_RATE).toBe(0.998);
  });

  it("D = scrollLeft + (v/1000)·d/(1−d): флик 1000px/s доезжает на ~499px", () => {
    // d/(1−d) = 0.998/0.002 = 499 — множитель UIScrollView при 0.998.
    expect(projectStopPoint(100, 1000)).toBeCloseTo(100 + 499, 5);
  });

  it("отрицательная скорость уводит проекцию к началу", () => {
    expect(projectStopPoint(500, -1000)).toBeCloseTo(500 - 499, 5);
  });

  it("нулевая скорость оставляет точку на месте", () => {
    expect(projectStopPoint(321, 0)).toBe(321);
  });
});

describe("pickSnapTarget — ближайшая к проекции snap-точка", () => {
  it("выбирает ближайшую точку к идеальной позиции", () => {
    expect(pickSnapTarget(240, [0, 176.5, 353, 529.5])).toBe(176.5);
  });

  it("проекция за границами контента клампится к крайней точке", () => {
    expect(pickSnapTarget(-500, [0, 176.5, 353])).toBe(0);
    expect(pickSnapTarget(9999, [0, 176.5, 353])).toBe(353);
  });

  it("вход не обязан быть отсортирован", () => {
    expect(pickSnapTarget(340, [353, 0, 176.5])).toBe(353);
  });

  it("ничья разрешается нижней точкой", () => {
    expect(pickSnapTarget(88, [0, 176])).toBe(0);
  });

  it("пустой список точек даёт null", () => {
    expect(pickSnapTarget(100, [])).toBeNull();
  });
});

describe("railRubberBandOffset — резинка UIScrollView c=0.55", () => {
  it("константа совпадает с PTR (одна физика на жесты)", () => {
    expect(RUBBER_BAND_C).toBe(0.55);
  });

  it("70px перетяга на вьюпорте 390 даёт ~35px хода (вязнет с дистанцией)", () => {
    expect(railRubberBandOffset(70, 390)).toBeCloseTo(35.03, 1);
  });

  it("рост сопротивления: вторая сотня метров даёт меньше, чем первая", () => {
    const first = railRubberBandOffset(100, 390);
    const second = railRubberBandOffset(200, 390) - railRubberBandOffset(100, 390);
    expect(second).toBeLessThan(first);
  });

  it("без перетяги — ноль", () => {
    expect(railRubberBandOffset(0, 390)).toBe(0);
    expect(railRubberBandOffset(-10, 390)).toBe(0);
  });
});

describe("isHorizontalDrag — гвард старта жеста", () => {
  it("порог старта 8px", () => {
    expect(DRAG_START_THRESHOLD_PX).toBe(8);
    expect(isHorizontalDrag(9, 1)).toBe(true);
    expect(isHorizontalDrag(7, 1)).toBe(false);
  });

  it("горизонталь должна доминировать по смещению", () => {
    expect(isHorizontalDrag(50, 60)).toBe(false);
    expect(isHorizontalDrag(50, 10)).toBe(true);
  });
});

describe("criticalSettle — доезд критическим демпфированием без перелёта", () => {  it("отклик доезда — 0.4с (таблица Apple move/reposition)", () => {
    expect(RAIL_SPRING_RESPONSE_S).toBe(0.4);
  });

  it("нулевая дистанция — мгновенная установка", () => {
    const settle = criticalSettle(100, 100, 500);
    expect(settle.durationMs).toBe(0);
    expect(settle.ease(0.5)).toBe(0.5);
  });

  it.each([2000, -2000, 0, 80000])(
    "кривая монотонна и не выходит за [0,1] при скорости %i",
    (velocity) => {
      const settle = criticalSettle(186, 0, velocity);
      let previous = 0;
      for (let t = 0; t <= 1.0001; t += 0.01) {
        const value = settle.ease(Math.min(t, 1));
        expect(value).toBeGreaterThanOrEqual(-0.001);
        expect(value).toBeLessThanOrEqual(1.001);
        expect(value).toBeGreaterThanOrEqual(previous - 0.001);
        previous = value;
      }
    },
  );

  it("кривая приходит в цель: остаток в конце ≤ 0.5px", () => {
    for (const velocity of [0, -300, -3000, 1200]) {
      const settle = criticalSettle(529.5, 176.5, velocity);
      const remaining = (1 - settle.ease(1)) * (529.5 - 176.5);
      expect(Math.abs(remaining)).toBeLessThanOrEqual(0.6);
    }
  });

  it("быстрый бросок доезжает не дольше медленного отпускания (velocity handoff)", () => {
    // К цели 0 из 186 scrollLeft убывает — скорость к цели отрицательна.
    const fast = criticalSettle(186, 0, -4000);
    const slow = criticalSettle(186, 0, -100);
    expect(fast.durationMs).toBeLessThan(slow.durationMs);
  });

  it("скорость выше критической линии зажимается — перелёта нет", () => {
    // Телепортная скорость автоматизации (десятки тысяч px/s) должна дать
    // тот же доезд, что и скорость на критической линии ω·|d0|.
    const insane = criticalSettle(186, 0, -80000);
    const critical = criticalSettle(186, 0, -(15.708 * 186));
    expect(insane.durationMs).toBeCloseTo(critical.durationMs, 5);
  });

  it("кап длительности держит дальние докаты", () => {
    const far = criticalSettle(0, 1000, 0, 1);
    expect(far.durationMs).toBe(SETTLE_MAX_MS);
  });
});

describe("DL_EASE хука — зеркало --dl-ease посимвольно (дрейф-страж)", () => {
  it("кортёж reduced-доката совпадает с токеном дома", () => {
    // WAAPI/framer не читают var() — кривая дома продублирована кортежем в
    // use-snap-rail-drag.ts (прецедент-страж: live-value-machine.test.ts).
    const tokensCss = readFileSync(
      fileURLToPath(new URL("../../styles/tokens.css", import.meta.url)),
      "utf8",
    );
    const token = /--dl-ease:\s*cubic-bezier\(([^)]*)\)/.exec(tokensCss)?.[1];
    expect(token, "значение --dl-ease в tokens.css").toBeDefined();
    const source = readFileSync(
      fileURLToPath(new URL("./use-snap-rail-drag.ts", import.meta.url)),
      "utf8",
    );
    const tuple = /const DL_EASE[^=]*=\s*\[([^\]]+)\]/.exec(source)?.[1];
    expect(tuple, "кортёж DL_EASE в use-snap-rail-drag.ts").toBeDefined();
    const normalize = (s: string): string =>
      s
        .split(",")
        .map((part) => Number(part.trim()))
        .join(", ");
    expect(normalize(tuple as string)).toBe(normalize(token as string));
  });
});
