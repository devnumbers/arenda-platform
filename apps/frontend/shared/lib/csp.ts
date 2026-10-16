/**
 * CSP фронта: общий строгий хвост обеих полис — шага 1 (next.config.ts,
 * блокирующая) и шага 2 (proxy.ts, Report-Only-разведка тикета #406,
 * решение #331) — плюс разбор отчётов нарушений. Полисы шага 2 зеркалит
 * будущую блокирующую (рецепт
 * https://nextjs.org/docs/app/guides/content-security-policy): nonce
 * только в script-src/style-src, host-source в script-src игнорируется
 * браузером из-за strict-dynamic — ровно то, что разведка измеряет.
 * Новый внешний origin (CDN, шрифты, аналитика) — правка CSP_BASE_DIRECTIVES,
 * она меняет обе полисы разом.
 */

/** Общие директивы обеих полис; источник истины для новых origin'ов. */
export const CSP_BASE_DIRECTIVES = [
  "default-src 'self'",
  "img-src 'self' data:",
  "font-src 'self'",
  "connect-src 'self'",
  "object-src 'none'",
  "base-uri 'self'",
  "form-action 'self'",
  "frame-ancestors 'none'",
  'upgrade-insecure-requests',
] as const;

/** Nonce per request: base64 of a random UUID, the Next.js guide recipe. */
export function generateCspNonce(): string {
  return Buffer.from(crypto.randomUUID()).toString('base64');
}

export function buildCspReportOnlyPolicy(nonce: string, { isDev }: { isDev: boolean }): string {
  return [
    `script-src 'self' 'nonce-${nonce}' 'strict-dynamic'${isDev ? " 'unsafe-eval'" : ''}`,
    `style-src 'self' 'nonce-${nonce}'`,
    ...CSP_BASE_DIRECTIVES,
    'report-uri /api/csp-report',
  ].join('; ');
}

/** One flattened violation, ready to be logged as a single stdout JSON line (Vector -> Uptrace). */
export interface CspViolationRecord {
  msg: 'csp_report';
  documentUri: string | undefined;
  directive: string | undefined;
  blockedUri: string | undefined;
  sample: string | undefined;
  disposition: string | undefined;
  sourceFile: string | undefined;
  line: number | undefined;
  column: number | undefined;
  statusCode: number | undefined;
}

const SAMPLE_MAX_LENGTH = 120;

/**
 * Верхняя граница тела отчёта: реальные csp-report-POST'ы — единицы KB.
 * Большие тела не парсим вовсе — неаутентифицированный эндпоинт не должен
 * стать вектором раздувания stdout-логов.
 */
export const REPORT_BODY_MAX_BYTES = 16384;

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function pickString(record: Record<string, unknown>, key: string): string | undefined {
  const value = record[key];
  return typeof value === 'string' ? value : undefined;
}

function pickNumber(record: Record<string, unknown>, key: string): number | undefined {
  const value = record[key];
  return typeof value === 'number' ? value : undefined;
}

function toViolationRecord(report: Record<string, unknown>): CspViolationRecord {
  return {
    msg: 'csp_report',
    documentUri: pickString(report, 'document-uri'),
    directive: pickString(report, 'effective-directive') ?? pickString(report, 'violated-directive'),
    blockedUri: pickString(report, 'blocked-uri'),
    sample: pickString(report, 'script-sample')?.slice(0, SAMPLE_MAX_LENGTH),
    disposition: pickString(report, 'disposition'),
    sourceFile: pickString(report, 'source-file'),
    line: pickNumber(report, 'line-number'),
    column: pickNumber(report, 'column-number'),
    statusCode: pickNumber(report, 'status-code'),
  };
}

/**
 * Both wire formats browsers post to the report endpoint: the classic
 * `{"csp-report": {...}}` (report-uri) and the Reporting-API array of
 * typed items. Malformed bodies yield no records, never a throw — the
 * endpoint must stay green so the report stream keeps flowing.
 */
export function summarizeCspReports(body: unknown): CspViolationRecord[] {
  if (Array.isArray(body)) {
    return body
      .filter(
        (item): item is { type: string; body: Record<string, unknown> } =>
          isRecord(item) && item.type === 'csp-violation' && isRecord(item.body),
      )
      .map((item) => toViolationRecord(item.body));
  }
  if (isRecord(body) && isRecord(body['csp-report'])) {
    return [toViolationRecord(body['csp-report'])];
  }
  return [];
}
