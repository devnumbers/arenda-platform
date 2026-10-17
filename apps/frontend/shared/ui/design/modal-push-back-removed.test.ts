import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/**
 * Контракт модалок: фон под скримом статичен (план 043, догонка
 * feel-check владельца 16.10 — карта #1437). Push-back оболочки
 * (.screen-shell: scale 0.96 + сдвиг 8px, решение 14.10) отклонён
 * владельцем — «страница отдаляется, вырезать»; страж дрейфа читает
 * globals.css по прецеденту press-feedback (правило без слоёв —
 * единственное место механики). План 047 легитимно вернул селектор
 * body:has(.modal-overlay) для точечного z-спуска хрома — гардит именно
 * механика отодвигания (трансформ .screen-shell), не селектор.
 */

const globalsCss = readFileSync(
  fileURLToPath(new URL('../../../app/globals.css', import.meta.url)),
  'utf8',
);

describe('Push-back фона модалок вырезан', () => {
  it('хук body:has(.modal-overlay не тянет трансформ оболочки', () => {
    expect(
      globalsCss,
      'push-back: правило .screen-shell на крючке modal-overlay',
    ).not.toContain(".modal-overlay[data-state='open']) .screen-shell");
  });

  it('механики отодвигания нет: translateY(8px) + scale(0.96)', () => {
    expect(globalsCss, 'трансформация push-back').not.toContain(
      'translateY(8px) scale(0.96)',
    );
  });

  it('вход карточки модалки сохранён (решение владельца: «карточку оставить»)', () => {
    expect(
      /\.modal-card\[data-state='open'\]\s*\{[^}]*\}/.exec(globalsCss)?.[0],
      'блок .modal-card[data-state=open]',
    ).toBeDefined();
  });
});
