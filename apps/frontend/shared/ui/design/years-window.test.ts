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

  it('назад — по 400: больше зоны триггера, иначе порочный цикл достроек', () => {
    // Достройка назад пишется в scrollTop только в паузе (посреди инерции
    // компенсация убила бы её). Если шаг меньше зоны триггера, после
    // компенсации колесо остаётся в зоне — следующий settle достраивает
    // ещё, и так по кругу без всякого скролла. Шаг 400 выносит позицию
    // за пределы зоны и за пределы максимального флика (~54 ряда).
    expect(YEARS_BACKWARD_EXTENSION).toBeGreaterThan(WHEEL_NEAR_EDGE_ROWS);
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

  it('шаг доращивания больше сида: после компенсации колесо вынесено из зоны', () => {
    expect(YEARS_BACKWARD_EXTENSION).toBeGreaterThan(YEARS_INITIAL_BACKWARD_SEED);
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
