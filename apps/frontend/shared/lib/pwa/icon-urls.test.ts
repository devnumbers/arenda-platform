import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

/**
 * Guard-тест версионирования URL бренд-иконок.
 *
 * `/icons/*` раздаётся с `Cache-Control: immutable` на год (next.config.ts,
 * решение владельца 22.10.2026): смена арта под тем же URL для кэшей не
 * существует — браузер и WebAPK держат старые байты до истечения срока, и
 * переустановка PWA не помогает (HTTP-кэш Chrome удалением приложения не
 * чистится). Баг 26–27.10: maskable заменён в repо, а на устройстве после
 * переустановки оставался прежним. Правило: смена арта — бампни `?v=ГГГГММДД`
 * у всех потребителей URL: `app/manifest.ts` (манифестные иконки),
 * `shared/lib/pwa/push-payload.ts` и `public/sw.js` (push-иконка). Тест ловит
 * неверсионированный URL и расхождение версий между тремя файлами.
 */
const manifestPath = resolve(process.cwd(), 'app/manifest.ts');
const pushPayloadPath = resolve(process.cwd(), 'shared/lib/pwa/push-payload.ts');
const swPath = resolve(process.cwd(), 'public/sw.js');

const VERSIONED_ICON_URL = /\/icons\/icon-[a-z0-9-]+\.png\?v=(\d{8})/g;

function collectVersionedIconUrls(source: string): string[] {
  return source.match(VERSIONED_ICON_URL) ?? [];
}

function distinctVersions(urls: string[]): string[] {
  return [...new Set(urls.map((url) => url.split('?v=')[1] ?? ''))];
}

describe('brand icon URL versioning', () => {
  const manifestSource = readFileSync(manifestPath, 'utf8');
  const pushPayloadSource = readFileSync(pushPayloadPath, 'utf8');
  const swSource = readFileSync(swPath, 'utf8');

  it('every manifest icon src carries a ?v= version', () => {
    const srcs = [...manifestSource.matchAll(/src: '(\/icons\/[^']+)'/g)].map(
      (match) => match[1],
    );
    expect(srcs.length).toBeGreaterThanOrEqual(4);
    for (const src of srcs) {
      expect(src).toMatch(/^\/icons\/icon-[a-z0-9-]+\.png\?v=\d{8}$/);
    }
  });

  it('push icon path carries ?v= in push-payload.ts and its sw.js copy', () => {
    expect(pushPayloadSource).toContain(
      "const PUSH_ICON_PATH = '/icons/icon-192.png?v=",
    );
    expect(swSource).toContain("var PUSH_ICON_PATH = '/icons/icon-192.png?v=");
  });

  it('all icon URL versions across the three consumers match', () => {
    const allUrls = [
      ...collectVersionedIconUrls(manifestSource),
      ...collectVersionedIconUrls(pushPayloadSource),
      ...collectVersionedIconUrls(swSource),
    ];
    // 4 манифестных src + push-иконка в двух копиях.
    expect(allUrls.length).toBeGreaterThanOrEqual(6);
    expect(distinctVersions(allUrls)).toHaveLength(1);
  });
});
