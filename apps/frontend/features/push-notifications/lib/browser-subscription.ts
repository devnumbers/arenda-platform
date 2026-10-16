'use client';

/**
 * Единая точка чтения и снятия подписки браузера (serviceWorker.ready →
 * pushManager): проба подписки, heal и мастер-выключение экрана ходят сюда.
 * Явная подписка (request-push) остаётся своя — ей нужна сама registration
 * для pushManager.subscribe.
 */

/**
 * Живая подписка браузера или null. Бросает, когда service worker не
 * зарегистрировался — вызывающий решает свою деградацию.
 */
export async function getBrowserSubscription(): Promise<PushSubscription | null> {
  const registration = await navigator.serviceWorker.ready;
  return registration.pushManager.getSubscription();
}

/**
 * Жёсткая отписка браузера (спека #1028 §2): снимает живую подписку и
 * возвращает её endpoint — DELETE бекенду идёт по нему (если браузер успел
 * пересоздать подписку между пробой и кликом, снесётся её строка, а не
 * чужая). null — подписки не было или отписка не удалась: вызывающий
 * DELETE'ит по endpoint из пробы, хвост у push-сервиса добьёт 410-prune
 * диспетчера.
 */
export async function unsubscribeBrowserSubscription(): Promise<string | null> {
  try {
    const subscription = await getBrowserSubscription();
    if (!subscription) return null;
    const { endpoint } = subscription;
    await subscription.unsubscribe();
    return endpoint;
  } catch {
    return null;
  }
}
