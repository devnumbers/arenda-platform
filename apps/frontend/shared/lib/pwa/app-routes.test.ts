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

describe('app route list sync with service worker', () => {
  // Guards against drift between the TS source of truth and the inline copy
  // kept in public/sw.js (which cannot import TS at runtime).
  it('public/sw.js contains the same prefix list', () => {
    const swPath = resolve(process.cwd(), 'public/sw.js');
    const swSource = readFileSync(swPath, 'utf8');

    for (const prefix of APP_ROUTE_PREFIXES) {
      expect(swSource, `public/sw.js missing prefix ${prefix}`).toContain(`'${prefix}'`);
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
