// Сбор CSP-отчётов разведки шага 2 (тикет #406): report-uri полиса из
// proxy.ts постит сюда нарушения Report-Only-политики. Каждая находка —
// одна stdout-строка JSON, её подхватывает Vector и уносит в Uptrace
// (docs/deployment.md, Observability). Статический роут выигрывает у
// BFF-catch-all [...path], на бэкенд ничего не проксируется.
import { NextResponse } from 'next/server';
import { REPORT_BODY_MAX_BYTES, summarizeCspReports } from '@/shared/lib/csp';

export async function POST(request: Request): Promise<NextResponse> {
  const contentLength = Number(request.headers.get('content-length'));
  if (Number.isFinite(contentLength) && contentLength > REPORT_BODY_MAX_BYTES) {
    return new NextResponse(null, { status: 204 });
  }
  try {
    const body: unknown = await request.json();
    for (const record of summarizeCspReports(body)) {
      console.log(JSON.stringify(record));
    }
  } catch {
    // Битое тело отчёта не должно превращаться в 500: браузер ответ
    // игнорирует, а шумный эндпоинт лишь потерял бы поток отчётов.
  }
  return new NextResponse(null, { status: 204 });
}
