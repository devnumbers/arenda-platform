import { describe, expect, it } from 'vitest';
import { buildCspReportOnlyPolicy, generateCspNonce, summarizeCspReports } from './csp';

const NONCE = 'dGVzdC1ub25jZS0xMjM0NTY3OA==';

function policy(isDev = false): string {
  return buildCspReportOnlyPolicy(NONCE, { isDev });
}

describe('generateCspNonce', () => {
  it('is unique per call and base64-encoded (Next.js CSP guide recipe)', () => {
    const nonce = generateCspNonce();
    expect(nonce).toMatch(/^[A-Za-z0-9+/]+={0,2}$/);
    expect(generateCspNonce()).not.toBe(nonce);
  });
});

describe('buildCspReportOnlyPolicy', () => {
  it('puts the nonce and strict-dynamic into script-src, without unsafe-inline', () => {
    const scriptSrc = policy().split(';').map((d) => d.trim()).find((d) => d.startsWith('script-src'));
    expect(scriptSrc).toBe(`script-src 'self' 'nonce-${NONCE}' 'strict-dynamic'`);
  });

  it('puts the nonce into style-src, without unsafe-inline', () => {
    const styleSrc = policy().split(';').map((d) => d.trim()).find((d) => d.startsWith('style-src'));
    expect(styleSrc).toBe(`style-src 'self' 'nonce-${NONCE}'`);
  });

  it('sends violations to the report endpoint', () => {
    expect(policy()).toContain('report-uri /api/csp-report');
  });

  it('keeps the strict step-1 directives for the rest', () => {
    expect(policy()).toContain("default-src 'self'");
    expect(policy()).toContain("img-src 'self' data:");
    expect(policy()).toContain("font-src 'self'");
    expect(policy()).toContain("connect-src 'self'");
    expect(policy()).toContain("object-src 'none'");
    expect(policy()).toContain("base-uri 'self'");
    expect(policy()).toContain("form-action 'self'");
    expect(policy()).toContain("frame-ancestors 'none'");
    expect(policy()).toContain('upgrade-insecure-requests');
  });

  it('adds unsafe-eval to script-src only in dev (React Refresh)', () => {
    const devPolicy = policy(true).split(';').map((d) => d.trim());
    expect(devPolicy.find((d) => d.startsWith('script-src'))).toBe(
      `script-src 'self' 'nonce-${NONCE}' 'strict-dynamic' 'unsafe-eval'`,
    );
    expect(devPolicy.find((d) => d.startsWith('style-src'))).toBe(`style-src 'self' 'nonce-${NONCE}'`);
  });

  it('inlines the nonce it was given, so SSR scripts match the header', () => {
    const other = buildCspReportOnlyPolicy('b3RoZXItbm9uY2U=', { isDev: false });
    expect(other).toContain("'nonce-b3RoZXItbm9uY2U='");
    expect(other).not.toContain(`'nonce-${NONCE}'`);
  });
});

describe('summarizeCspReports', () => {
  it('maps a report-uri body to one queryable record', () => {
    const records = summarizeCspReports({
      'csp-report': {
        'document-uri': 'https://dev.rentlee.ru/properties',
        'effective-directive': 'style-src-attr',
        'violated-directive': 'style-src',
        'blocked-uri': 'inline',
        'script-sample': 'color:red',
        disposition: 'report',
        'source-file': 'https://dev.rentlee.ru/_next/static/chunk.js',
        'line-number': 5,
        'column-number': 10,
        'status-code': 200,
      },
    });
    expect(records).toStrictEqual([
      {
        msg: 'csp_report',
        documentUri: 'https://dev.rentlee.ru/properties',
        directive: 'style-src-attr',
        blockedUri: 'inline',
        sample: 'color:red',
        disposition: 'report',
        sourceFile: 'https://dev.rentlee.ru/_next/static/chunk.js',
        line: 5,
        column: 10,
        statusCode: 200,
      },
    ]);
  });

  it('prefers the effective directive over the violated one', () => {
    const records = summarizeCspReports({
      'csp-report': { 'violated-directive': 'script-src', 'effective-directive': 'script-src-elem' },
    });
    expect(records[0]?.directive).toBe('script-src-elem');
  });

  it('falls back to the violated directive when effective is absent (older browsers)', () => {
    const records = summarizeCspReports({
      'csp-report': { 'violated-directive': 'style-src' },
    });
    expect(records[0]?.directive).toBe('style-src');
  });

  it('truncates long samples to keep log lines bounded', () => {
    const records = summarizeCspReports({
      'csp-report': { 'script-sample': 'x'.repeat(300) },
    });
    expect(records[0]?.sample).toHaveLength(120);
  });

  it('accepts Reporting-API array bodies and skips non-CSP items', () => {
    const records = summarizeCspReports([
      { type: 'csp-violation', body: { 'document-uri': 'https://dev.rentlee.ru/login', 'effective-directive': 'script-src-elem', 'blocked-uri': 'inline' } },
      { type: 'deprecation', body: {} },
    ]);
    expect(records).toStrictEqual([
      {
        msg: 'csp_report',
        documentUri: 'https://dev.rentlee.ru/login',
        directive: 'script-src-elem',
        blockedUri: 'inline',
        sample: undefined,
        disposition: undefined,
        sourceFile: undefined,
        line: undefined,
        column: undefined,
        statusCode: undefined,
      },
    ]);
  });

  it('returns nothing for malformed bodies instead of throwing', () => {
    expect(summarizeCspReports(null)).toEqual([]);
    expect(summarizeCspReports('nonsense')).toEqual([]);
    expect(summarizeCspReports({})).toEqual([]);
    expect(summarizeCspReports({ 'csp-report': 'not-an-object' })).toEqual([]);
  });
});
