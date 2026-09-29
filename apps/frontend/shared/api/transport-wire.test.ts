import { describe, expect, it } from 'vitest';
import { ApiError } from './errors';
import { normalizeRequestHeaders, parseTransportResponse } from './transport-wire';

describe('normalizeRequestHeaders', () => {
  it('ставит Accept по умолчанию, когда вызывающий не задал его', () => {
    const headers = normalizeRequestHeaders({});
    expect(headers.get('Accept')).toBe('application/json');
  });

  it('не перекрывает явный Accept вызывающего', () => {
    const headers = normalizeRequestHeaders({ headers: { Accept: 'text/plain' } });
    expect(headers.get('Accept')).toBe('text/plain');
  });

  it('ставит Content-Type для строкового body, когда он не задан', () => {
    const headers = normalizeRequestHeaders({ body: JSON.stringify({ a: 1 }) });
    expect(headers.get('Content-Type')).toBe('application/json');
  });

  it('не перекрывает явный Content-Type вызывающего', () => {
    const headers = normalizeRequestHeaders({
      headers: { 'Content-Type': 'application/merge-patch+json' },
      body: JSON.stringify({ a: 1 }),
    });
    expect(headers.get('Content-Type')).toBe('application/merge-patch+json');
  });

  it('не ставит Content-Type, когда body не строка', () => {
    const body = new FormData();
    const headers = normalizeRequestHeaders({ body });
    expect(headers.has('Content-Type')).toBe(false);
  });
});

describe('parseTransportResponse', () => {
  it('возвращает распарсенный JSON успешного ответа', async () => {
    const response = new Response(JSON.stringify({ status: 'active' }), {
      headers: { 'Content-Type': 'application/json' },
    });
    await expect(parseTransportResponse(response)).resolves.toEqual({ status: 'active' });
  });

  it('возвращает undefined на 204 без тела', async () => {
    const response = new Response(null, { status: 204 });
    await expect(parseTransportResponse(response)).resolves.toBeUndefined();
  });

  it('бросает ApiError из problem+json неуспешного ответа', async () => {
    const response = new Response(
      JSON.stringify({ code: 'not_found', detail: 'Подписка не найдена' }),
      { status: 404, headers: { 'Content-Type': 'application/problem+json' } },
    );
    const error = await parseTransportResponse(response).catch((e: unknown) => e);
    expect(error).toBeInstanceOf(ApiError);
    const apiError = error as ApiError;
    expect(apiError.code).toBe('not_found');
    expect(apiError.detail).toBe('Подписка не найдена');
    expect(apiError.status).toBe(404);
  });

  it('бросает ApiError с фолбэком, когда тело неуспеха не problem+json', async () => {
    const response = new Response('oops', { status: 500 });
    const error = await parseTransportResponse(response).catch((e: unknown) => e);
    expect(error).toBeInstanceOf(ApiError);
    const apiError = error as ApiError;
    expect(apiError.code).toBe('unknown');
    expect(apiError.detail).toBe('Ошибка сервера (код 500)');
    expect(apiError.status).toBe(500);
  });
});
