import { describe, expect, it } from 'vitest';
import { initialYearsWindow } from './years-window';

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
