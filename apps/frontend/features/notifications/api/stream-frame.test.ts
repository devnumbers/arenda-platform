import { describe, expect, it } from 'vitest';
import { parseStreamFrame, type StreamFrame } from './stream-frame';

/** Кадры стрима (#742, ADR 0058): конверт {v, occurredAt, payload} внутри
 * data-строки SSE-события. */
function frameData(payload: unknown, v = 1): string {
  return JSON.stringify({ v, occurredAt: '2026-09-19T10:40:00Z', payload });
}

describe('parseStreamFrame — кадр connected', () => {
  it('страрт-кадр бэка (event "connected", data "{}") даёт connected', () => {
    expect(parseStreamFrame('connected', '{}')).toEqual<StreamFrame>({
      kind: 'connected',
    });
  });
});

describe('parseStreamFrame — notification.created', () => {
  it('полный кадр даёт created с полями тоста', () => {
    const data = frameData({
      id: '00000000-0000-0000-0000-000000000001',
      category: 'rental',
      contextLabel: '2-комнатная на Ленина',
      title: 'Аренда завершена',
      body: 'Договор аренды завершён',
      url: '/notifications/00000000-0000-0000-0000-000000000001',
    });
    expect(parseStreamFrame('notification.created', data)).toEqual<StreamFrame>({
      kind: 'created',
      id: '00000000-0000-0000-0000-000000000001',
      category: 'rental',
      contextLabel: '2-комнатная на Ленина',
      title: 'Аренда завершена',
      body: 'Договор аренды завершён',
      url: '/notifications/00000000-0000-0000-0000-000000000001',
      occurredAt: '2026-09-19T10:40:00Z',
    });
  });

  it('contextLabel и url необязательны — их отсутствие даёт null', () => {
    const data = frameData({
      id: '00000000-0000-0000-0000-000000000001',
      category: 'system',
      title: 'Технические работы',
      body: '16 сентября будут работы',
    });
    const frame = parseStreamFrame('notification.created', data);
    expect(frame).toMatchObject({ kind: 'created', contextLabel: null, url: null });
  });

  it('url — только same-origin абсолютный путь (канон resolveClickTarget)', () => {
    const base = {
      id: '00000000-0000-0000-0000-000000000001',
      category: 'system',
      title: 't',
      body: 'b',
    };
    expect(parseStreamFrame('notification.created', frameData({ ...base, url: 'https://evil.example/x' }))?.kind).toBe('created');
    expect(
      (parseStreamFrame('notification.created', frameData({ ...base, url: 'https://evil.example/x' })) as Extract<StreamFrame, { kind: 'created' }>).url,
    ).toBeNull();
    expect(
      (parseStreamFrame('notification.created', frameData({ ...base, url: '//evil.example/x' })) as Extract<StreamFrame, { kind: 'created' }>).url,
    ).toBeNull();
    expect(
      (parseStreamFrame('notification.created', frameData({ ...base, url: '/ok/path' })) as Extract<StreamFrame, { kind: 'created' }>).url,
    ).toBe('/ok/path');
  });
});

describe('parseStreamFrame — notification.unread_count', () => {
  it('кадр счётчика даёт unread-count', () => {
    expect(parseStreamFrame('notification.unread_count', frameData({ count: 7 }))).toEqual<StreamFrame>({
      kind: 'unread-count',
      count: 7,
      occurredAt: '2026-09-19T10:40:00Z',
    });
  });

  it('отрицательный и нечисловой счётчик отбрасываются', () => {
    expect(parseStreamFrame('notification.unread_count', frameData({ count: -1 }))).toBeNull();
    expect(parseStreamFrame('notification.unread_count', frameData({ count: '7' }))).toBeNull();
  });
});

describe('parseStreamFrame — защита от мусора', () => {
  it('не-JSON data даёт null', () => {
    expect(parseStreamFrame('notification.created', 'не json')).toBeNull();
  });

  it('неизвестное имя события даёт null (клиент без слушателя его и не видит)', () => {
    expect(parseStreamFrame('future.event', frameData({}))).toBeNull();
  });

  it('ломающая версия конверта v>1 игнорируется, аддитивная v===1 читается', () => {
    expect(parseStreamFrame('notification.created', frameData({ id: 'x' }, 2))).toBeNull();
    expect(parseStreamFrame('notification.created', frameData({ id: 'x', category: 'tasks', title: 't', body: 'b' }, 1))?.kind).toBe('created');
  });

  it('created без id/title — мусор, null', () => {
    expect(parseStreamFrame('notification.created', frameData({}))).toBeNull();
    expect(parseStreamFrame('notification.created', frameData({ id: 'x' }))).toBeNull();
  });

  it('неизвестная категория деградирует в system — тост не теряем', () => {
    const frame = parseStreamFrame(
      'notification.created',
      frameData({ id: 'x', category: 'brand_new', title: 't', body: 'b' }),
    );
    expect(frame).toMatchObject({ kind: 'created', category: 'system' });
  });
});
