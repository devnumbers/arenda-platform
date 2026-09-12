import { describe, expect, it } from 'vitest';
import { skeletonBlockClass, skeletonRowWidths } from './skeleton-parts';

describe('skeleton-parts', () => {
  describe('skeletonBlockClass', () => {
    it('тон: на белой поверхности — bg-surface-muted, внутри серой карточки — приглушённый', () => {
      expect(skeletonBlockClass('base')).toBe('bg-surface-muted');
      expect(skeletonBlockClass('muted')).toBe('bg-surface-muted-hover');
    });
  });

  describe('skeletonRowWidths', () => {
    it('ноль и отрицательное число — пустой список', () => {
      expect(skeletonRowWidths(0)).toEqual([]);
      expect(skeletonRowWidths(-3)).toEqual([]);
    });

    it('длина результата равна count', () => {
      expect(skeletonRowWidths(1)).toHaveLength(1);
      expect(skeletonRowWidths(5)).toHaveLength(5);
    });

    it('каждая строка несёт ширины заголовка и подзаголовка', () => {
      for (const widths of skeletonRowWidths(3)) {
        expect(widths.title).toMatch(/^w-\d+\/\d+$/);
        expect(widths.subtitle).toMatch(/^w-\d+\/\d+$/);
      }
    });

    it('цикл детерминирован: строки за пределами цикла повторяют его с начала', () => {
      const widths = skeletonRowWidths(7);
      expect(widths[3]).toEqual(widths[0]);
      expect(widths[4]).toEqual(widths[1]);
      expect(widths[6]).toEqual(widths[0]);
    });

    it('соседние строки цикла различаются — список не выглядит механическим повтором', () => {
      const widths = skeletonRowWidths(3);
      expect(widths[0]).not.toEqual(widths[1]);
      expect(widths[1]).not.toEqual(widths[2]);
    });

    it('один и тот же count даёт одинаковый DOM-результат (гидратация не расходится)', () => {
      expect(skeletonRowWidths(4)).toEqual(skeletonRowWidths(4));
    });
  });
});
