import { describe, expect, it } from 'vitest';

import { iconsFitInRow, stackColumnWidth, stackFallbackWidth } from './icon-row-fit';

describe('iconsFitInRow', () => {
  it('полуколонка эталонного 375: три круга, четвёртый не влезает', () => {
    // 131.5px: 44 + 2×32 = 108 влезает, 140 — нет (решение владельца
    // 22.10: невлезающие в один ряд не показываются).
    expect(iconsFitInRow(131.5, 4)).toBe(3);
  });

  it('полная колонка эталонного 375: все семь под потолком', () => {
    // 279px вместило бы 8, потолок одной группы — 7.
    expect(iconsFitInRow(279, 7)).toBe(7);
  });

  it('широкая полуколонка (окно 520): потолок 4, хотя влезло бы больше', () => {
    expect(iconsFitInRow(204, 4)).toBe(4);
  });

  it('узкая legacy-колонка (320): только два', () => {
    expect(iconsFitInRow(104, 4)).toBe(2);
  });

  it('граница точного влезания: 108 → 3, 107 → 2', () => {
    expect(iconsFitInRow(108, 4)).toBe(3);
    expect(iconsFitInRow(107, 4)).toBe(2);
  });

  it('меньше круга — ноль, без отрицательных', () => {
    expect(iconsFitInRow(43, 4)).toBe(0);
    expect(iconsFitInRow(0, 4)).toBe(0);
  });
});

describe('stackColumnWidth', () => {
  it('одна группа — вся ширина контента', () => {
    expect(stackColumnWidth(279, true)).toBe(279);
  });

  it('две группы — пополам с учётом зазора 16', () => {
    expect(stackColumnWidth(279, false)).toBe(131.5);
  });
});

describe('stackFallbackWidth — первый кадр до замера', () => {
  // Дефолт — колонка эталонного телефона 375: на актуальных телефонах
  // первый кадр уже правильный, без мигания переноса.
  it('первый кадр: обе группы — 3, одна — 7', () => {
    expect(iconsFitInRow(stackFallbackWidth(false), 4)).toBe(3);
    expect(iconsFitInRow(stackFallbackWidth(true), 7)).toBe(7);
  });
});
