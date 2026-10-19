import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/**
 * Контракт плавающего лейбла TextField (план 040, догонка feel-check
 * владельца 16.10 — карта #1437): приподнятый лейбл — настоящие 13px/15px
 * на top 10px (макеты Figma 948-46863/46874/46915/46926), не
 * смасштабированный 16px трансформа плана 015. Гигиена 015 сохраняется:
 * переход только морфящихся свойств (не transition-all) и
 * reduced-motion-сокращение.
 */

const source = readFileSync(
  fileURLToPath(new URL('./text-field.tsx', import.meta.url)),
  'utf8',
);

/** Блок классов лейбла titleIn — единственное место морфа. Срез по якорю
 * (регекс по `[^)]*` рвётся на скобках внутри
 * `peer-[:not(:placeholder-shown)]`). */
const anchor = source.indexOf("'pointer-events-none absolute left-0");
const labelBlock = anchor === -1 ? undefined : source.slice(anchor, anchor + 700);

describe('Плавающий лейбл TextField — вёрстка по Figma', () => {
  it('блок лейбла найден', () => {
    expect(labelBlock, 'блок классов лейбла в text-field.tsx').toBeDefined();
  });

  it('покой: лейбл — 16px/18px на top 19px', () => {
    expect(labelBlock).toContain("'top-[19px] text-base leading-[18px]'");
  });

  it('приподнятый: настоящие 13px/15px на top 10px, не scale', () => {
    expect(labelBlock).toContain('peer-focus:top-[10px]');
    expect(labelBlock).toContain('peer-focus:text-[13px]');
    expect(labelBlock).toContain('peer-focus:leading-[15px]');
    expect(labelBlock).toContain(
      'peer-[:not(:placeholder-shown)]:top-[10px]',
    );
    expect(labelBlock).toContain(
      'peer-[:not(:placeholder-shown)]:text-[13px]',
    );
    expect(labelBlock).toContain(
      'peer-[:not(:placeholder-shown)]:leading-[15px]',
    );
    expect(labelBlock, 'scale-трансформа нет').not.toContain('scale-');
    expect(labelBlock, 'translate-морфа нет').not.toContain('translate-y-');
    expect(labelBlock, 'origin морфа нет').not.toContain('origin-top-left');
  });

  it('гигиена переходов: только морфящиеся свойства + reduced-сокращение', () => {
    expect(labelBlock).toContain(
      'transition-[top,font-size,line-height]',
    );
    expect(labelBlock).toContain('duration-300');
    expect(labelBlock).toContain('motion-reduce:duration-150');
    expect(labelBlock, 'transition-all не вернулся').not.toContain(
      'transition-all',
    );
  });
});
