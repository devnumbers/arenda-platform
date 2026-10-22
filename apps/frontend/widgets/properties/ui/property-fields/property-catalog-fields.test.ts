import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/**
 * Контракт полей каталога характеристик: единица измерения — инлайновый
 * суффикс, следующий за значением (решение владельца 2026-10-22, макет
 * 1218:54304); прежний префикс-единица слева снят.
 */

const source = readFileSync(
  fileURLToPath(new URL('./property-catalog-fields.tsx', import.meta.url)),
  'utf8',
);

describe('Поля каталога — единица измерения', () => {
  it('единица number-полей передаётся инлайновым суффиксом', () => {
    expect(source).toContain(
      'inlineSuffix={field.kind === \'number\' ? field.unit : undefined}',
    );
  });

  it('префикс-единица слева снята', () => {
    expect(source).not.toContain('prefix=');
  });
});
