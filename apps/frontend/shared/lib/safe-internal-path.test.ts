import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { safeInternalPath } from './safe-internal-path';

describe('safeInternalPath', () => {
    it('allows internal absolute paths', () => {
        expect(safeInternalPath('/properties')).toBe('/properties');
        expect(safeInternalPath('/leases/123?tab=payments')).toBe('/leases/123?tab=payments');
        expect(safeInternalPath('/')).toBe('/');
    });

    it('rejects null, undefined, and empty values', () => {
        expect(safeInternalPath(null)).toBeNull();
        expect(safeInternalPath(undefined)).toBeNull();
        expect(safeInternalPath('')).toBeNull();
    });

    it('rejects relative and external URLs', () => {
        expect(safeInternalPath('dashboard')).toBeNull();
        expect(safeInternalPath('https://evil.example/path')).toBeNull();
        expect(safeInternalPath('mailto:someone@example.com')).toBeNull();
    });

    it('rejects protocol-relative URLs', () => {
        expect(safeInternalPath('//evil.example')).toBeNull();
        expect(safeInternalPath('//evil.example/path')).toBeNull();
    });

    it('rejects backslash tricks that WHATWG URL normalizes into an external redirect', () => {
        // `new URL('/\\evil.com', origin)` resolves to `https://evil.com/` because
        // the WHATWG parser treats `\` as `/` for special schemes. The guard must
        // reject the raw string before it ever reaches `new URL` in proxy.ts.
        expect(safeInternalPath('/\\evil.com')).toBeNull();
        expect(safeInternalPath('\\evil.com')).toBeNull();
    });

    it('rejects /login targets to avoid redirect loops', () => {
        expect(safeInternalPath('/login')).toBeNull();
        expect(safeInternalPath('/login?from=/properties')).toBeNull();
        expect(safeInternalPath('/login/sub')).toBeNull();
    });

    it('WHATWG URL really does normalize /\\ into a cross-origin redirect', () => {
        // Pins the platform fact the guard above defends against: if this ever
        // stops holding, the /\\ rejection is still correct but no longer load-bearing.
        expect(new URL('/\\evil.com', 'https://app.example').href).toBe('https://evil.com/');
    });
});

describe('service worker inline copy stays in sync', () => {
    // public/sw.js keeps an inline copy of this check in resolveClickTarget
    // (it cannot import TS at runtime). Its behaviour must match safeInternalPath
    // on the security-relevant cases: leading /, no //, no /\.
    const swPath = resolve(process.cwd(), 'public/sw.js');
    const swSource = readFileSync(swPath, 'utf8');

    it('public/sw.js resolveClickTarget rejects both // and /\\', () => {
        expect(swSource, 'SW must reject protocol-relative URLs').toContain("url[1] === '/'");
        expect(swSource, 'SW must reject backslash-normalized URLs').toContain("url[1] === '\\\\'");
    });
});
