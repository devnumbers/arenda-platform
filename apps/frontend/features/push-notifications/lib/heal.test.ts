import { describe, expect, it, vi } from 'vitest';
import { healPushSubscription, type SubscriptionHandle } from './heal';

function makeSubscription(
  overrides: Partial<SubscriptionHandle> = {},
): SubscriptionHandle {
  return {
    endpoint: 'https://fcm.googleapis.com/fcm/send/abc',
    unsubscribe: vi.fn().mockResolvedValue(undefined),
    ...overrides,
  };
}

describe('healPushSubscription — правда-проверка (спека #1028 §4)', () => {
  it('подписки в браузере нет — проверять нечего, проба не зовётся', async () => {
    const probeRow = vi.fn();
    const result = await healPushSubscription({
      getSubscription: () => Promise.resolve(null),
      probeRow,
    });
    expect(result).toStrictEqual({ outcome: 'no-subscription', endpoint: null });
    expect(probeRow).not.toHaveBeenCalled();
  });

  it('строка в БД есть — аномалии нет, подписка живёт', async () => {
    const subscription = makeSubscription();
    const result = await healPushSubscription({
      getSubscription: () => Promise.resolve(subscription),
      probeRow: () => Promise.resolve('row-exists'),
    });
    expect(result).toStrictEqual({ outcome: 'kept', endpoint: subscription.endpoint });
    expect(subscription.unsubscribe).not.toHaveBeenCalled();
  });

  it('браузер помнит подписку, строки в БД нет — молчаливая отписка (вариант А)', async () => {
    const subscription = makeSubscription();
    let probedEndpoint = '';
    const result = await healPushSubscription({
      getSubscription: () => Promise.resolve(subscription),
      probeRow: (endpoint) => {
        probedEndpoint = endpoint;
        return Promise.resolve('row-missing');
      },
    });
    expect(result).toStrictEqual({ outcome: 'unsubscribed', endpoint: subscription.endpoint });
    expect(probedEndpoint).toBe(subscription.endpoint);
    expect(subscription.unsubscribe).toHaveBeenCalledTimes(1);
  });

  it('сетевая ошибка пробы — не 404: подписку не трогаем', async () => {
    const subscription = makeSubscription();
    const result = await healPushSubscription({
      getSubscription: () => Promise.resolve(subscription),
      probeRow: () => Promise.resolve('probe-failed'),
    });
    expect(result).toStrictEqual({ outcome: 'kept', endpoint: subscription.endpoint });
    expect(subscription.unsubscribe).not.toHaveBeenCalled();
  });
});
