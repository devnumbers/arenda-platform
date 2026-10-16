import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/**
 * Контракт пресс-отклика дизайн-слоя (план 039, догонка feel-check
 * владельца 16.10 — карта #1437): канон пресса — только цветовой шаг.
 * Сжатие scale(0.97) из плана 007 отклонено владельцем и в CSS жить не
 * должно (страж дрейфа: класс без слоёв в globals.css — единственное
 * место механики, тест читает файл по прецеденту snap-rail-physics).
 */

const globalsCss = readFileSync(
  fileURLToPath(new URL('../../../app/globals.css', import.meta.url)),
  'utf8',
);

const pressableBlock = /\.pressable\s*\{[^}]*\}/.exec(globalsCss)?.[0];
const pressableActiveBlock = /\.pressable:active\s*\{[^}]*\}/.exec(globalsCss)?.[0];

describe('Канон пресс-отклика .pressable', () => {
  it('блок .pressable существует', () => {
    expect(pressableBlock, 'блок .pressable в globals.css').toBeDefined();
  });

  it('нажатие не сжимает: transform в каноне нет', () => {
    expect(pressableBlock, 'transition в .pressable').not.toContain('transform');
    expect(
      pressableActiveBlock,
      'правило .pressable:active (сжатие убрано целиком)',
    ).toBeUndefined();
  });

  it('цветовой шаг остаётся: переход несёт поверхности цвета на --dl-duration-fast', () => {
    for (const property of [
      'opacity',
      'background-color',
      'border-color',
      'color',
      'box-shadow',
    ]) {
      expect(
        pressableBlock,
        `переход ${property} на --dl-duration-fast`,
      ).toContain(`${property} var(--dl-duration-fast)`);
    }
  });
});
