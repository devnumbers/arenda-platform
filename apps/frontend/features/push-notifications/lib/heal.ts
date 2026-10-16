/**
 * Правда-проверка push-подписки при авторизованной загрузке (спека #1028 §4,
 * слайс 2 #1038): браузер помнит подписку, а строки `push_subscriptions` в БД
 * нет — аномалия №7 (строка снесена выключением на другом флоу, миграцией
 * 000147, prune'ом диспетчера). Лечится молчаливой отпиской (вариант А): UI
 * честно показывает «выключено», хвост чужой строки у push-сервиса добивает
 * существующий 410-prune диспетчера (`worker_push.go`).
 */

/** Минимальный хэндл подписки, нужный heal'у (срез браузерного PushSubscription). */
export type SubscriptionHandle = {
  readonly endpoint: string;
  unsubscribe(): Promise<boolean>;
};

/** Ответ GET /push/subscriptions/preferences?endpoint для живой подписки. */
export type PushRowProbe = 'row-exists' | 'row-missing' | 'probe-failed';

export type HealResult =
  | { readonly outcome: 'no-subscription'; readonly endpoint: null }
  | { readonly outcome: 'kept'; readonly endpoint: string }
  | { readonly outcome: 'unsubscribed'; readonly endpoint: string };

/**
 * Silently unsubscribe when the browser's subscription has no row in the
 * database. A probe failure (network, 5xx) is NOT a 404: the truth is
 * unknown, so the subscription stays — an unsubscribe on a flaky network
 * would silently turn push off for a subscribed user.
 *
 * `probeRow` must be total (map every error to `'probe-failed'`); an
 * unexpected throw propagates to the caller's error reporting.
 */
export async function healPushSubscription(deps: {
  readonly getSubscription: () => Promise<SubscriptionHandle | null>;
  readonly probeRow: (endpoint: string) => Promise<PushRowProbe>;
}): Promise<HealResult> {
  const subscription = await deps.getSubscription();
  if (!subscription) {
    return { outcome: 'no-subscription', endpoint: null };
  }
  const probe = await deps.probeRow(subscription.endpoint);
  if (probe === 'row-missing') {
    await subscription.unsubscribe();
    return { outcome: 'unsubscribed', endpoint: subscription.endpoint };
  }
  return { outcome: 'kept', endpoint: subscription.endpoint };
}
