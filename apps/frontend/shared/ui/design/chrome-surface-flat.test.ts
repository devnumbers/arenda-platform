import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/**
 * Контракт хрома и колёс (план 046, догонка feel-check владельца 16.10 —
 * карта #1437): (1) полноэкранная поверхность накрывает вьюпорт целиком —
 * подъём хрома над ней (z-60, решение 25.09/#865) и сдвиг листа правее
 * сайдбара отменены владельцем; хром (z-30) живёт под поверхностью и под
 * модалками. (2) Ряд колеса не подсвечивается при нажатии — отклик ряда
 * только полосой выбора. Стражи дрейфа читают исходники по прецеденту
 * press-feedback.
 */

const globalsCss = readFileSync(
  fileURLToPath(new URL('../../../app/globals.css', import.meta.url)),
  'utf8',
);

const wheelPickerSource = readFileSync(
  fileURLToPath(new URL('./wheel-picker.tsx', import.meta.url)),
  'utf8',
);

describe('Хром ПК под поверхностью и модалкой (план 046)', () => {
  it('подъём хрома над поверхностью не возвращается', () => {
    expect(globalsCss, 'правило body:has(.fullscreen-surface) .desktop-chrome').not.toContain(
      'body:has(.fullscreen-surface) .desktop-chrome',
    );
  });

  it('лист поверхности красится от левого края: сдвига праве сайдбара нет', () => {
    expect(
      globalsCss,
      'сдвиг ::before поверхности за сайдбар (left: var(--sidebar-total))',
    ).not.toContain('left: var(--sidebar-total)');
  });

  it('ряд колеса без подсветки нажатия', () => {
    expect(wheelPickerSource, 'active:bg-surface-muted-active в WheelRow').not.toContain(
      'active:bg-surface-muted-active',
    );
  });
});
