import { describe, expect, it } from 'vitest';
import { WHEEL_MAX_FLING_ROWS, WHEEL_NEAR_EDGE_ROWS } from './wheel-edge';
import {
  YEARS_BACKWARD_EXTENSION,
  YEARS_EXTENSION,
  YEARS_INITIAL_BACKWARD_SEED,
  initialYearsWindow,
} from './years-window';

describe('константы удлинения ленты годов', () => {
  it('вперёд достраивается по 25 — размер не влияет на ощущение, стену убирает зона триггера', () => {
    expect(YEARS_EXTENSION).toBe(25);
  });

  it('назад — шаг равен сиду (134 = зона + максимальный флик, план 049)', () => {
    // Шаг доращивания зеркалит стартовый посев (план 048): как и сид,
    // он обязан выносить позицию из зоны триггера с запасом под самый
    // жёсткий флик — после компенсации колесо вне зоны (settle не
    // достраивает по кругу), а следующий флик (≤54 ряда) стены не
    // достаёт. 400 было круглым запасом: каждый шаг монтирует 400
    // анимированных рядов, инвариант физики держится и на 134.
    expect(YEARS_BACKWARD_EXTENSION).toBe(
      WHEEL_NEAR_EDGE_ROWS + WHEEL_MAX_FLING_ROWS,
    );
  });
});

describe('стартовый посев назад — физика флика, не круглый запас (план 048)', () => {
  it('сид = зона триггера + максимальный флик (134, вывод из wheel-edge)', () => {
    expect(YEARS_INITIAL_BACKWARD_SEED).toBe(
      WHEEL_NEAR_EDGE_ROWS + WHEEL_MAX_FLING_ROWS,
    );
  });

  it('первый жёсткий флик назад (≤54 ряда) не достаёт до стены', () => {
    expect(YEARS_INITIAL_BACKWARD_SEED).toBeGreaterThan(WHEEL_MAX_FLING_ROWS);
  });

  it('зона триггера входит по пути первого флика — доращивание стартует', () => {
    expect(YEARS_INITIAL_BACKWARD_SEED - WHEEL_MAX_FLING_ROWS).toBeLessThanOrEqual(
      WHEEL_NEAR_EDGE_ROWS,
    );
  });

  it('шаг доращивания выносит из зоны с запасом под следующий флик (план 049)', () => {
    // Худший случай: доращивка сработала на ряде 0 — после компенсации
    // колесо на ряде = шагу; самый жёсткий флик назад (54 ряда) должен
    // остаться вне зоны (иначе следующий settle начнёт новую доращивку
    // слишком рано — не круг, но чаще, чем требует физика).
    expect(YEARS_BACKWARD_EXTENSION - WHEEL_MAX_FLING_ROWS).toBeGreaterThanOrEqual(
      WHEEL_NEAR_EDGE_ROWS,
    );
  });
});

describe('initialYearsWindow — стартовая лента годов шита без границ', () => {
  it('выбранный в текущем году: от текущего года на 25 вперёд (запас под жёсткий флик ~50 рядов)', () => {
    expect(initialYearsWindow(2026, 2026)).toEqual({ start: 2026, end: 2051 });
  });

  it('выбранный в будущем: лента начинается от текущего года и покрывает выбранного', () => {
    expect(initialYearsWindow(2030, 2026)).toEqual({ start: 2026, end: 2055 });
  });

  it('выбранный в прошлом (чужое value без границ): лента всё же содержит выбранного', () => {
    expect(initialYearsWindow(2020, 2026)).toEqual({ start: 2020, end: 2051 });
  });
});
