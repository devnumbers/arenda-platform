import { describe, expect, it } from 'vitest';
import {
  WHEEL_LOOP_ANCHOR,
  WHEEL_LOOP_CYCLES,
  isAnchorRow,
  loopNearestRow,
  loopRecenterRows,
  loopRowCount,
  loopSyncRow,
  modRows,
} from './wheel-loop';

describe('wheel-loop', () => {
  it('modRows — положительный остаток: логический индекс физического ряда', () => {
    expect(modRows(0, 24)).toBe(0);
    expect(modRows(5, 24)).toBe(5);
    expect(modRows(24, 24)).toBe(0);
    expect(modRows(385, 24)).toBe(1); // 16-й цикл, второй ряд
    expect(modRows(-1, 24)).toBe(23);
    expect(modRows(-25, 60)).toBe(35);
  });

  it('loopRowCount — цикл отрисован 31 раз: часы 744, минуты 1860, месяцы 372', () => {
    expect(WHEEL_LOOP_CYCLES).toBe(31);
    expect(WHEEL_LOOP_ANCHOR).toBe(15);
    expect(loopRowCount(24)).toBe(744);
    expect(loopRowCount(60)).toBe(1860);
    expect(loopRowCount(12)).toBe(372);
  });

  it('isAnchorRow — role=option только у среднего цикла', () => {
    const n = 24;
    expect(isAnchorRow(0, n)).toBe(false);
    expect(isAnchorRow(14 * n + 5, n)).toBe(false);
    expect(isAnchorRow(15 * n + 0, n)).toBe(true);
    expect(isAnchorRow(15 * n + 23, n)).toBe(true);
    expect(isAnchorRow(16 * n + 5, n)).toBe(false);
    expect(isAnchorRow(loopRowCount(n) - 1, n)).toBe(false);
  });

  it('loopRecenterRows — в середине скачок не нужен', () => {
    const n = 24;
    expect(loopRecenterRows(15 * n, n)).toBe(0);
    expect(loopRecenterRows(20 * n, n)).toBe(0);
  });

  it('loopRecenterRows — у верхнего края (меньше цикла запаса) прыжок на полспана вниз', () => {
    const n = 24;
    expect(loopRecenterRows(0, n)).toBe(WHEEL_LOOP_ANCHOR * n);
    expect(loopRecenterRows(n, n)).toBe(WHEEL_LOOP_ANCHOR * n);
    expect(loopRecenterRows(n + 1, n)).toBe(0);
  });

  it('loopRecenterRows — у нижнего края прыжок на полспана вверх', () => {
    const n = 24;
    const last = loopRowCount(n) - 1;
    expect(loopRecenterRows(last, n)).toBe(-WHEEL_LOOP_ANCHOR * n);
    expect(loopRecenterRows(last - n, n)).toBe(-WHEEL_LOOP_ANCHOR * n);
    expect(loopRecenterRows(last - n - 1, n)).toBe(0);
  });

  it('после прыжка позиция вне обеих зон — каскадных прыжков нет (все размеры цикла)', () => {
    for (const n of [1, 2, 9, 12, 24, 60]) {
      const rowCount = loopRowCount(n);
      const inTopZone = (row: number): boolean => row <= n;
      const inBottomZone = (row: number): boolean => row >= rowCount - 1 - n;
      for (let row = 0; row < rowCount; row += 1) {
        const jump = loopRecenterRows(row, n);
        if (jump === 0) continue;
        const target = row + jump;
        expect(target).toBeGreaterThanOrEqual(0);
        expect(target).toBeLessThanOrEqual(rowCount - 1);
        expect(inTopZone(target), `n=${n} row=${row} → ${target} в верхней зоне`).toBe(false);
        expect(inBottomZone(target), `n=${n} row=${row} → ${target} в нижней зоне`).toBe(false);
      }
    }
  });

  it('loopNearestRow — тот же логический индекс: ряд не меняется', () => {
    const n = 24;
    const row = 15 * n + 7;
    expect(loopNearestRow(row, 7, n)).toBe(row);
  });

  it('loopNearestRow — ближайший представитель логического индекса, а не якорная копия', () => {
    const n = 24;
    // ушли на 3 ряда вниз от якоря — логический 5 ближе копией в этом же цикле
    expect(loopNearestRow(15 * n + 8, 5, n)).toBe(15 * n + 5);
    // ушли на 3 ряда вверх — логический 1
    expect(loopNearestRow(15 * n + 2, 1, n)).toBe(15 * n + 1);
  });

  it('loopNearestRow — заворот через край цикла: не длиннее полцикла', () => {
    const n = 24;
    const row = 15 * n + 2;
    // логический 22 = текущий 2 минус 4 ряда (через край цикла), а не плюс 20
    expect(loopNearestRow(row, 22, n)).toBe(15 * n - 2);
    expect(loopNearestRow(row, 12, n)).toBe(row + 10); // ровно полцикла — вниз
  });

  it('loopNearestRow сохраняет логический индекс и работает от любой позиции', () => {
    for (const n of [1, 2, 9, 12, 24, 60]) {
      for (let logical = 0; logical < n; logical += 1) {
        for (const row of [0, 5, 15 * n, 20 * n + 3, loopRowCount(n) - 1]) {
          const target = loopNearestRow(row, logical, n);
          expect(modRows(target, n)).toBe(logical);
          expect(Math.abs(target - row)).toBeLessThanOrEqual(n / 2);
        }
      }
    }
  });

  it('loopSyncRow — смена длины цикла ре-анкорит на якорную копию (граничный год режет месяц-колесо)', () => {
    // 12 → 4: позиция 123 — это кламп физического ряда в новую ленту
    // (loopRowCount(4) − 1); ближайший представитель Sep = 124 вне ленты,
    // кламп оставил бы 123 = Dec — молчаливая подмена черновика месяца.
    expect(loopSyncRow(123, 0, 4, 12)).toBe(WHEEL_LOOP_ANCHOR * 4);
    // 4 → 12: подъём обратно тоже садит на якорную копию, а не на ближнюю.
    expect(loopSyncRow(50, 8, 12, 4)).toBe(WHEEL_LOOP_ANCHOR * 12 + 8);
  });

  // Полный перебор ~250 тыс. комбинаций идёт дольше дефолтных 5 с vitest
  // под нагрузкой параллельного сьюта — явный таймаут, масштаб не режем.
  it(
    'loopSyncRow — при смене длины цель всегда в новой ленте и сохраняет логический индекс',
    () => {
      // Только пары n ≠ m: при равных длинах окно вне ленты невозможно
      // (инвариант середины ленты держит речентр), кламп-класс не возникает.
      const lengths = [1, 2, 4, 9, 12, 24, 60];
      for (const n of lengths) {
        for (const m of lengths) {
          if (n === m) continue;
          for (let row = 0; row < loopRowCount(n); row += 1) {
            for (let logical = 0; logical < m; logical += 1) {
              const target = loopSyncRow(row, logical, m, n);
              expect(modRows(target, m), `n=${n}→m=${m} row=${row} logical=${logical}`).toBe(
                logical,
              );
              expect(target).toBeGreaterThanOrEqual(0);
              expect(target).toBeLessThanOrEqual(loopRowCount(m) - 1);
            }
          }
        }
      }
    },
    15000,
  );

  it('loopSyncRow без смены длины — это loopNearestRow', () => {
    // Ряды из середины ленты: ближайший представитель не выходит за края,
    // поэтому шов совпадает с loopNearestRow дословно.
    for (const n of [1, 2, 4, 9, 12, 24, 60]) {
      for (const logical of [0, Math.floor(n / 2), n - 1]) {
        for (const row of [
          WHEEL_LOOP_ANCHOR * n,
          WHEEL_LOOP_ANCHOR * n + n - 1,
          loopRowCount(n) - 1 - n,
        ]) {
          expect(loopSyncRow(row, logical, n, n)).toBe(loopNearestRow(row, logical, n));
        }
      }
    }
  });
});
