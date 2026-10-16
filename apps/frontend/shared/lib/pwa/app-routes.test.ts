import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { APP_ROUTE_PREFIXES, isAppRoute } from './app-routes';
import {
    STANDALONE_DB_KEY,
    STANDALONE_DB_NAME,
    STANDALONE_DB_STORE,
} from './standalone-store';

describe('isAppRoute', () => {
  it('returns true for exact app routes', () => {
    expect(isAppRoute('/properties')).toBe(true);
    expect(isAppRoute('/profile')).toBe(true);
    expect(isAppRoute('/login')).toBe(true);
  });

  it('returns true for nested app routes', () => {
    expect(isAppRoute('/properties/123')).toBe(true);
    expect(isAppRoute('/profile/tariff/payments/abc')).toBe(true);
  });

  it('returns true for the global sections of the unified chrome (#556)', () => {
    // Топ-поверхность после сноса кабинета: лента задач/операций/контактов,
    // заглушки «Платежи»/«Участники» — офлайн-навигация на них должна
    // отдавать брендированный offline.html, а не системную ошибку.
    expect(isAppRoute('/tasks')).toBe(true);
    expect(isAppRoute('/tasks/new')).toBe(true);
    expect(isAppRoute('/operations')).toBe(true);
    expect(isAppRoute('/operations/expenses')).toBe(true);
    expect(isAppRoute('/contacts')).toBe(true);
    expect(isAppRoute('/contacts/search')).toBe(true);
    expect(isAppRoute('/payments')).toBe(true);
    expect(isAppRoute('/participants')).toBe(true);
  });

  it('ignores query string and hash', () => {
    expect(isAppRoute('/properties?tab=overview')).toBe(true);
    expect(isAppRoute('/profile#settings')).toBe(true);
    expect(isAppRoute('/?returnTo=/properties')).toBe(false);
  });

  it('returns false for the landing root and marketing routes (invariant)', () => {
    // The most important invariant: SW must never intercept the landing.
    expect(isAppRoute('/')).toBe(false);
    expect(isAppRoute('/pricing')).toBe(false);
    expect(isAppRoute('/about')).toBe(false);
    expect(isAppRoute('/faq')).toBe(false);
  });

  it('returns false for prefixes that merely share a stem', () => {
    // /properties must not match /properties-extra or /property
    expect(isAppRoute('/properties-extra')).toBe(false);
    expect(isAppRoute('/property')).toBe(false);
    expect(isAppRoute('/profile-extra')).toBe(false);
    expect(isAppRoute('/loginpage')).toBe(false);
  });

  it('returns false for API, static assets, and other origins', () => {
    expect(isAppRoute('/api/properties')).toBe(false);
    expect(isAppRoute('/_next/static/chunk.js')).toBe(false);
    expect(isAppRoute('/icons/icon-192.png')).toBe(false);
  });

  it('returns false for removed rental routes (dead screens, not app)', () => {
    // Домен аренд удалён (спека #434): старые пути больше не перехватываются
    // сервис-воркером и уходят в сеть как обычные 404.
    expect(isAppRoute('/leases')).toBe(false);
    expect(isAppRoute('/tenants')).toBe(false);
    expect(isAppRoute('/finance')).toBe(false);
    expect(isAppRoute('/calendar')).toBe(false);
  });

  it('returns false for the removed support route (modal #766, not a page)', () => {
    // Страница /support снесена (карта #761, тикет #766): «Поддержка» —
    // модалка «Связаться с нами», старые ссылки уходят в сеть как 404.
    expect(isAppRoute('/support')).toBe(false);
    expect(isAppRoute('/support/faq')).toBe(false);
  });

  it('returns false for the removed dashboard route (тикет #865)', () => {
    // Легаси /dashboard снесён (карта #862, тикет #865): дом кабинета —
    // /properties, start_url манифеста переведён на него; старые ссылки
    // уходят в сеть как обычный 404.
    expect(isAppRoute('/dashboard')).toBe(false);
    expect(isAppRoute('/dashboard/tab')).toBe(false);
  });
});

/** Читает строки в одинарных кавычках из блока исходника между startMarker
 * и закрывающей `]`. Так тесты сверяют инлайн-копии списков в файлах,
 * которые нельзя импортировать (public/sw.js) или которые не экспортируют
 * список константой (proxy.ts, robots.txt). */
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

describe('app route list sync with service worker', () => {
  // Guards against drift between the TS source of truth and the inline copy
  // kept in public/sw.js (which cannot import TS at runtime).
  it('public/sw.js keeps an exact mirror of the prefix list', () => {
    const swPath = resolve(process.cwd(), 'public/sw.js');
    const swSource = readFileSync(swPath, 'utf8');

    // Двунаправленная сверка множеств: инлайн-копия не должна ни терять
    // префиксы (навигация уйдёт мимо offline-фолбэка), ни копить лишние
    // (SW начнёт перехватывать чужие навигации).
    const swPrefixes = extractQuotedList(swSource, 'const APP_ROUTE_PREFIXES = [');
    expect([...swPrefixes].sort()).toEqual([...APP_ROUTE_PREFIXES].sort());
  });
});

describe('app route list sync with landing robots.txt', () => {
  // robots.txt лендинга — владелец публичной поверхности хоста (карта #649):
  // Disallow-блок кабинета зеркалит APP_ROUTE_PREFIXES минус /login (цель
  // CTA лендинга). Лишняя строка прячет живой раздел от индексации,
  // пропущенная — открывает приватную ленту краулеру, поэтому сверяем
  // множества в обе стороны.
  it('Disallow block equals APP_ROUTE_PREFIXES minus /login', () => {
    const robotsPath = resolve(process.cwd(), '../landing/public/robots.txt');
    const robotsSource = readFileSync(robotsPath, 'utf8');

    const disallowPaths = Array.from(
      robotsSource.matchAll(/^Disallow:\s*(\S+)/gm),
      (match) => match[1] ?? '',
    );

    // Отдельный хвостовой блок robots.txt прячет бэкенд-пасстру (/api/,
    // /webhooks/) — он вне зеркала кабинных префиксов и объявлен здесь явно.
    const backendPassthrough = ['/api/', '/webhooks/'];
    const expected = [
      ...APP_ROUTE_PREFIXES.filter((prefix) => prefix !== '/login'),
      ...backendPassthrough,
    ];

    expect(disallowPaths.sort()).toEqual(expected.sort());
  });
});

describe('app route list sync with proxy matcher', () => {
  // proxy.ts — /me-гейт разделов кабинета (#887); его matcher держит те же
  // топ-пути. Сверяем покрытие в одну сторону (префиксы → matcher): matcher
  // вправе покрывать пути вне списка префиксов, строгого равенства не требуем.
  it('covers every prefix with an exact <prefix> or <prefix>/:path* pattern', () => {
    const proxyPath = resolve(process.cwd(), 'proxy.ts');
    const proxySource = readFileSync(proxyPath, 'utf8');

    const matcherEntries = extractQuotedList(proxySource, 'matcher: [');

    // /subscription сознательно вне /me-гейта прокси — унификация требует
    // behavior-решения, вне скоупа гейта.
    const outsideProxyGate: ReadonlySet<string> = new Set(['/subscription']);

    for (const prefix of APP_ROUTE_PREFIXES) {
      const covered = matcherEntries.some(
        (entry) => entry === prefix || entry === `${prefix}/:path*`,
      );
      if (outsideProxyGate.has(prefix)) {
        expect(covered, `${prefix} must stay outside the proxy /me-gate`).toBe(false);
        continue;
      }
      expect(covered, `proxy.ts matcher missing exact pattern for ${prefix}`).toBe(true);
    }
  });
});

describe('standalone-store marker sync with service worker', () => {
  // The SW keeps its own inline copy of the IndexedDB name/store/key to read
  // the standalone flag written by standalone-store.ts. This test catches drift
  // on all three markers, mirroring the prefix-list guard above.
  it('public/sw.js references the standalone DB name, store, and key', () => {
    const swPath = resolve(process.cwd(), 'public/sw.js');
    const swSource = readFileSync(swPath, 'utf8');

    const markers: ReadonlyArray<[string, string]> = [
      ['DB name', STANDALONE_DB_NAME],
      ['store', STANDALONE_DB_STORE],
      ['key', STANDALONE_DB_KEY],
    ];

    for (const [label, value] of markers) {
      expect(
        swSource,
        `public/sw.js missing standalone ${label} '${value}'`,
      ).toContain(`'${value}'`);
    }
  });
});
