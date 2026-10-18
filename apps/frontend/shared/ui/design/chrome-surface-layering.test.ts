import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/**
 * Контракт слоёв хрома и колёс (план 047, догонка feel-check владельца —
 * карта #1437; разворот тотального сноса 046 после live-check 17.10):
 * (1) поверхность-страница не прячет меню ПК — лист правее сайдбара и
 * подъём хрома до z-60 (решение 25.09/#865) возвращены; (2) открытая
 * модалка опускает хром под скрим — точечное правило
 * body:has(.modal-overlay[data-state='open']) (лечение исходного бага
 * 16.10: сайдбар пробивал скрим колесного шита над поверхностью);
 * (3) ряд колеса не подсвечивается при нажатии (фикс 046 живёт). Стражи
 * дрейфа читают исходники по прецеденту press-feedback.
 */

const globalsCss = readFileSync(
  fileURLToPath(new URL('../../../app/globals.css', import.meta.url)),
  'utf8',
);

const wheelPickerSource = readFileSync(
  fileURLToPath(new URL('./wheel-picker.tsx', import.meta.url)),
  'utf8',
);

describe('Слои хрома: поверхность и модалка (план 047)', () => {
  it('поверхность поднимает хром: меню ПК видно над листом', () => {
    expect(
      globalsCss,
      'правило body:has(.fullscreen-surface) .desktop-chrome (подъём 25.09/#865)',
    ).toContain('body:has(.fullscreen-surface) .desktop-chrome');
  });

  it('лист поверхности начинается правее сайдбара', () => {
    expect(
      globalsCss,
      'сдвиг ::before поверхности за сайдбар (left: var(--sidebar-total))',
    ).toContain('left: var(--sidebar-total)');
  });

  it('открытая модалка опускает хром под скрим', () => {
    expect(
      globalsCss,
      'правило body:has(.modal-overlay[data-state=open]) .desktop-chrome (спуск плана 047)',
    ).toContain("body:has(.modal-overlay[data-state='open']) .desktop-chrome");
  });

  it('ряд колеса без подсветки нажатия (фикс 046 живёт)', () => {
    expect(wheelPickerSource, 'active:bg-surface-muted-active в WheelRow').not.toContain(
      'active:bg-surface-muted-active',
    );
  });
});
