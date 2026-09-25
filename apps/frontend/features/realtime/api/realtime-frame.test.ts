import { describe, expect, it } from 'vitest';
import { REALTIME_ENTITY_NAMES, parseRealtimeFrame } from './realtime-frame';

/** data-строка кадра entity.changed: конверт v1 (ADR 0060 §5) с payload
 * {propertyId, entity} (ADR 0062 §2). */
function frameData(payload: unknown, v = 1): string {
  return JSON.stringify({ v, occurredAt: '2026-09-25T10:00:00Z', payload });
}

describe('Словарь сущностей — контракт с бэком (ADR 0062 §2)', () => {
  it('восемь имён в каноническом порядке словаря бэка', () => {
    expect(REALTIME_ENTITY_NAMES).toEqual([
      'payments',
      'operations',
      'tasks',
      'contacts',
      'rentals',
      'property',
      'access',
      'history',
    ]);
  });
});

describe('parseRealtimeFrame — разбор кадра entity.changed', () => {
  it('кадр по объекту разбирается в сущность и propertyId', () => {
    const frame = parseRealtimeFrame(frameData({ propertyId: 'p1', entity: 'payments' }));
    expect(frame).toStrictEqual({ entity: 'payments', propertyId: 'p1' });
  });

  it('кадр безобъектной книги владельца: propertyId null разбирается в null', () => {
    const frame = parseRealtimeFrame(frameData({ propertyId: null, entity: 'tasks' }));
    expect(frame).toStrictEqual({ entity: 'tasks', propertyId: null });
  });

  it('отсутствующий propertyId читается как безобъектная книга', () => {
    const frame = parseRealtimeFrame(frameData({ entity: 'contacts' }));
    expect(frame).toStrictEqual({ entity: 'contacts', propertyId: null });
  });

  it('неизвестная сущность — кадр не наш (добавление сущности назад-совместимо)', () => {
    expect(parseRealtimeFrame(frameData({ propertyId: 'p1', entity: 'future-entity' }))).toBeNull();
  });

  it('ломающая версия конверта игнорируется', () => {
    expect(
      parseRealtimeFrame(frameData({ propertyId: 'p1', entity: 'payments' }, 2)),
    ).toBeNull();
  });

  it('конверт без версии игнорируется', () => {
    expect(parseRealtimeFrame('{"occurredAt":"2026-09-25T10:00:00Z","payload":{}}')).toBeNull();
  });

  it('мусор вместо JSON — null', () => {
    expect(parseRealtimeFrame('не json')).toBeNull();
  });

  it('не-объект JSON — null', () => {
    expect(parseRealtimeFrame('[1,2,3]')).toBeNull();
  });

  it('payload не объект — null', () => {
    expect(parseRealtimeFrame('{"v":1,"payload":"payments"}')).toBeNull();
  });

  it('entity не строка — null', () => {
    expect(parseRealtimeFrame(frameData({ propertyId: 'p1', entity: 42 }))).toBeNull();
  });

  it('propertyId не строка и не null — null', () => {
    expect(parseRealtimeFrame(frameData({ propertyId: 42, entity: 'payments' }))).toBeNull();
  });

  it('пустой propertyId — null (мусорный кадр, а не безобъектная книга)', () => {
    expect(parseRealtimeFrame(frameData({ propertyId: '', entity: 'payments' }))).toBeNull();
  });
});
