import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { CABINET_ROUTE_PREFIXES, isCabinetRoute } from './cabinet-routes';
import {
    STANDALONE_DB_KEY,
    STANDALONE_DB_NAME,
    STANDALONE_DB_STORE,
} from './standalone-store';

describe('isCabinetRoute', () => {
  it('returns true for exact cabinet routes', () => {
    expect(isCabinetRoute('/dashboard')).toBe(true);
    expect(isCabinetRoute('/profile')).toBe(true);
    expect(isCabinetRoute('/login')).toBe(true);
  });

  it('returns true for nested cabinet routes', () => {
    expect(isCabinetRoute('/properties/123')).toBe(true);
    expect(isCabinetRoute('/finance/2026/01')).toBe(true);
    expect(isCabinetRoute('/leases/abc-456/edit')).toBe(true);
  });

  it('ignores query string and hash', () => {
    expect(isCabinetRoute('/dashboard?tab=overview')).toBe(true);
    expect(isCabinetRoute('/profile#settings')).toBe(true);
    expect(isCabinetRoute('/?returnTo=/dashboard')).toBe(false);
  });

  it('returns false for the landing root and marketing routes (invariant)', () => {
    // The most important invariant: SW must never intercept the landing.
    expect(isCabinetRoute('/')).toBe(false);
    expect(isCabinetRoute('/pricing')).toBe(false);
    expect(isCabinetRoute('/about')).toBe(false);
    expect(isCabinetRoute('/faq')).toBe(false);
  });

  it('returns false for prefixes that merely share a stem', () => {
    // /dashboard must not match /dashboards or /dashboard-x
    expect(isCabinetRoute('/dashboards')).toBe(false);
    expect(isCabinetRoute('/profile-extra')).toBe(false);
    expect(isCabinetRoute('/loginpage')).toBe(false);
  });

  it('returns false for API, static assets, and other origins', () => {
    expect(isCabinetRoute('/api/reminders')).toBe(false);
    expect(isCabinetRoute('/_next/static/chunk.js')).toBe(false);
    expect(isCabinetRoute('/icons/icon-192.png')).toBe(false);
  });
});

describe('cabinet route list sync with service worker', () => {
  // Guards against drift between the TS source of truth and the inline copy
  // kept in public/sw.js (which cannot import TS at runtime).
  it('public/sw.js contains the same prefix list', () => {
    const swPath = resolve(process.cwd(), 'public/sw.js');
    const swSource = readFileSync(swPath, 'utf8');

    for (const prefix of CABINET_ROUTE_PREFIXES) {
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
