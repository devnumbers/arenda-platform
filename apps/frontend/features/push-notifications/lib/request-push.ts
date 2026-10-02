'use client';

import type { NotificationCategoryPreferences } from '@/entities/notification';
import {
  hasRequiredKeys,
  subscribeToPush,
  subscriptionToPayload,
} from './subscribe';
import {
  isPushSupported,
  readNotificationPermission,
  requiresInstallOnIos,
} from './platform';

/**
 * Явное включение пушей действием юзера (клик тумблера, авто-промпт — спека
 * #1028 §2–§3, слайс 2 #1038): системное окно (только когда разрешение ещё
 * `default`) → тихая подписка → POST. Фоновых подписок не существует —
 * единственный путь сюда из не-жестового кода — авто-промпт, и только пока
 * разрешение `default`.
 *
 * Возвращает структурированный исход, чтобы вызывающий показал нужную
 * реакцию (инструкция iOS, отказ, тост успеха).
 */

export type PushSubscribeErrorReason =
  | 'no-vapid-key'
  | 'subscribe-failed'
  | 'missing-keys'
  | 'network-error';

export type RequestPushPermissionOutcome =
  | { readonly outcome: 'ios-needs-install' }
  | { readonly outcome: 'unsupported' }
  | { readonly outcome: 'denied' }
  | { readonly outcome: 'subscribed'; readonly subscription: PushSubscription }
  | { readonly outcome: 'already-subscribed'; readonly subscription: PushSubscription }
  | { readonly outcome: 'error'; readonly reason: PushSubscribeErrorReason };

/** POST подписки на бекенд; `categories` уходят явно (спека §5 — фронт всегда
 * шлёт категории; omitted-дефолт «все ВКЛ» остаётся чужим клиентам). */
export type PostSubscriptionFn = (
  payload: ReturnType<typeof subscriptionToPayload>,
  categories?: NotificationCategoryPreferences,
) => Promise<void>;

/**
 * Запрашивает разрешение (если `default`), подписывает браузер и POSTит
 * подписку. `vapidKey` и `postSubscription` резолвит вызывающий (react-query
 * хуки). Живая подписка (already-subscribed) повторного POST не делает —
 * строка на бекенде ей соответствует, а их расхождение лечит heal при
 * загрузке.
 */
export async function requestPushPermissionAndSubscribe(
  vapidKey: string | undefined,
  postSubscription: PostSubscriptionFn,
  categories?: NotificationCategoryPreferences,
): Promise<RequestPushPermissionOutcome> {
  if (!isPushSupported()) {
    return { outcome: 'unsupported' };
  }

  // iOS доставляет пуши только установленной PWA; вместо обречённого
  // системного окна показываем инструкцию «На экран „Домой“».
  if (requiresInstallOnIos()) {
    return { outcome: 'ios-needs-install' };
  }

  if (readNotificationPermission() === 'denied') {
    return { outcome: 'denied' };
  }
  if (readNotificationPermission() === 'default') {
    const permission = await Notification.requestPermission();
    if (permission !== 'granted') {
      return { outcome: 'denied' };
    }
  }

  if (!vapidKey) {
    return { outcome: 'error', reason: 'no-vapid-key' };
  }

  const registration = await navigator.serviceWorker.ready;
  const existing = await registration.pushManager.getSubscription();
  if (existing) {
    return { outcome: 'already-subscribed', subscription: existing };
  }

  let subscription: PushSubscription;
  try {
    subscription = await subscribeToPush(registration, vapidKey);
  } catch {
    // subscribe() бросает при отказе или транзиентных ошибках; сохранять нечего.
    return { outcome: 'error', reason: 'subscribe-failed' };
  }

  const payload = subscriptionToPayload(subscription);
  if (!hasRequiredKeys(payload)) {
    return { outcome: 'error', reason: 'missing-keys' };
  }

  try {
    await postSubscription(payload, categories);
  } catch {
    return { outcome: 'error', reason: 'network-error' };
  }

  return { outcome: 'subscribed', subscription };
}
