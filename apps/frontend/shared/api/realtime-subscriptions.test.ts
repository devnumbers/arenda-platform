import { describe, expect, it, vi } from 'vitest';
import {
  realtimeEntitySubscribers,
  subscribeRealtimeEntity,
} from './realtime-subscriptions';

function frame(entity: string, propertyId: string | null = 'p1') {
  return { entity, propertyId };
}

describe('subscribeRealtimeEntity — точные потребители кадров (тикет #718)', () => {
  it('подписчик получает кадры своей сущности', () => {
    const onFrame = vi.fn();
    const unsubscribe = subscribeRealtimeEntity('history', { onFrame });

    realtimeEntitySubscribers('history')[0]?.onFrame?.(frame('history'));

    expect(onFrame).toHaveBeenCalledExactlyOnceWith(frame('history'));
    unsubscribe();
  });

  it('кадры чужих сущностей до подписчика не доходят', () => {
    const onFrame = vi.fn();
    const unsubscribe = subscribeRealtimeEntity('history', { onFrame });

    expect(realtimeEntitySubscribers('payments')).toStrictEqual([]);

    unsubscribe();
  });

  it('несколько подписчиков одной сущности получают кадр все', () => {
    const first = vi.fn();
    const second = vi.fn();
    const unsubscribes = [
      subscribeRealtimeEntity('operations', { onFrame: first }),
      subscribeRealtimeEntity('operations', { onFrame: second }),
    ];

    for (const handler of realtimeEntitySubscribers('operations')) {
      handler.onFrame?.(frame('operations'));
    }

    expect(first).toHaveBeenCalledExactlyOnceWith(frame('operations'));
    expect(second).toHaveBeenCalledExactlyOnceWith(frame('operations'));
    for (const unsubscribe of unsubscribes) unsubscribe();
  });

  it('отписка снимает подписчика — реестр пуст', () => {
    const unsubscribe = subscribeRealtimeEntity('history', {});

    unsubscribe();

    expect(realtimeEntitySubscribers('history')).toStrictEqual([]);
  });
});
