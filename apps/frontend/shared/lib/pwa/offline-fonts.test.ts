import { describe, expect, it } from 'vitest';
import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { OFFLINE_FONT_PATHS } from './offline-fonts';

// Шрифты офлайн-страницы: Onest самохостится из public/fonts, но офлайн-страница
// не может ни импортировать TS, ни обратиться к сети — @font-face продублирован
// в её разметке, а сами файлы прекэшит public/sw.js. Три файла обязаны меняться
// одним коммитом; тест ловит дрейф (паттерн guard-тестов public/sw.js, см.
// app-routes.test.ts и support.test.ts).
const offlineHtml = readFileSync(resolve(process.cwd(), 'public/offline.html'), 'utf8');
const swSource = readFileSync(resolve(process.cwd(), 'public/sw.js'), 'utf8');

/** Читает строки в одинарных кавычках из блока исходника между startMarker
 * и закрывающей `]`. Локальная копия хелпера app-routes.test.ts: сверяем
 * инлайн-копию списка в public/sw.js, который нельзя импортировать. */
function extractQuotedList(source: string, startMarker: string): ReadonlyArray<string> {
  const start = source.indexOf(startMarker);
  expect(start, `marker not found in source: ${startMarker}`).toBeGreaterThanOrEqual(0);
  const end = source.indexOf(']', start);
  expect(end, `closing ']' not found after: ${startMarker}`).toBeGreaterThan(start);
  return Array.from(
    source.slice(start, end).matchAll(/'([^']+)'/g),
    (match) => match[1] ?? '',
  );
}

describe('offline page fonts stay in sync', () => {
  it('every precached font exists in public/fonts', () => {
    for (const fontPath of OFFLINE_FONT_PATHS) {
      // fontPath с ведущим слэшем: конкатенация, а не resolve-сегмент —
      // абсолютный сегмент сбросил бы базу на корень файловой системы.
      expect(
        existsSync(resolve(process.cwd(), 'public' + fontPath)),
        `missing font file: public${fontPath}`,
      ).toBe(true);
    }
  });

  it('public/sw.js keeps an exact mirror of the font list', () => {
    const swFonts = extractQuotedList(swSource, 'const OFFLINE_FONT_PATHS = [');
    expect([...swFonts].sort()).toEqual([...OFFLINE_FONT_PATHS].sort());
  });

  it('public/offline.html declares @font-face for exactly the precached fonts', () => {
    // В обе стороны: пропущенный url() оставит текст системным шрифтом,
    // лишний — объявит шрифт, которого офлайн не достать (прекэша нет).
    const htmlFontUrls = Array.from(
      offlineHtml.matchAll(/url\('(\/fonts\/[^']+)'\)/g),
      (match) => match[1] ?? '',
    );
    expect(htmlFontUrls.sort()).toEqual([...OFFLINE_FONT_PATHS].sort());
  });

  it('offline.html @font-face bodies mirror globals.css', () => {
    // url() в списке — не всё: разошедшиеся font-weight/unicode-range
    // дадут неверную glyph-отрисовку (или тишину системного фолбэка) только
    // офлайн. Сверяем строки каждого прекэшенного блока с его источником
    // в app/globals.css.
    const globalsCss = readFileSync(resolve(process.cwd(), 'app/globals.css'), 'utf8');
    for (const fontPath of OFFLINE_FONT_PATHS) {
      const start = globalsCss.indexOf(`url('${fontPath}')`);
      expect(start, `globals.css has no @font-face for ${fontPath}`).toBeGreaterThanOrEqual(0);
      const block = globalsCss.slice(start, globalsCss.indexOf('}', start));
      for (const marker of ['font-weight', 'font-display', 'unicode-range']) {
        const line = block
          .split('\n')
          .find((candidate) => candidate.includes(marker))
          ?.trim();
        expect(line, `${marker} line for ${fontPath} in globals.css`).toBeDefined();
        expect(offlineHtml).toContain(line);
      }
    }
  });
});
