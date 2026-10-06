import { describe, expect, it } from 'vitest';
import { visualKeyboardInset } from './useVisualKeyboardInset';

/**
 * Контракт расчёта клавиатурного инсета (#1151, research #1148, раздел B):
 * насколько низ visual viewport (видимая область) выше низа layout viewport,
 * к которому приколочена fixed-панель StickyBottomBar. iOS Safari
 * клавиатурой уменьшает только visual viewport — инсет возникает именно
 * там; на Android с interactive-widget=resizes-content layout уже ужат
 * и инсет остаётся 0.
 */
describe('visualKeyboardInset — зазор над клавиатурой из visual viewport', () => {
  it('клавиатура 300px: layout 800, низ visual 500 → инсет 300', () => {
    expect(visualKeyboardInset(800, 500)).toBe(300);
  });

  it('клавиатура закрыта: низ visual совпадает с layout → 0', () => {
    expect(visualKeyboardInset(800, 800)).toBe(0);
  });

  it('дробные CSS-пиксели округляются до целых (translateY без субпикселей)', () => {
    expect(visualKeyboardInset(800.4, 512.2)).toBe(288);
  });

  it('visual ниже layout (пинч-зум, оверскролл) — клампится в 0: сдвиг только вверх', () => {
    expect(visualKeyboardInset(800, 900)).toBe(0);
  });
});
