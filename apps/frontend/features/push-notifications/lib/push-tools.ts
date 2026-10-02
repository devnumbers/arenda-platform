'use client';

import { useCallback } from 'react';
import {
  useCreatePushSubscription,
  useVapidPublicKey,
} from '../api/hooks';
import type { PostSubscriptionFn } from './request-push';

/**
 * Инструменты подписки для явных флоу включения: VAPID-ключ бекенда и POST
 * /push/subscriptions. Единая фабрика, чтобы и хук экрана
 * (useSubscribePush), и стартовый гейт (авто-промпт) вызывали их из
 * эффектов, не перетаскивая правила хуков в асинхронные хелперы. Фоновых
 * подписок не существует (спека #1028 §4, слайс 2 #1038) — кроме
 * авто-промпта нового юзера это единственные точки POST'а.
 */
export function usePushSubscriptionTools(): {
  readonly vapidKey: string | undefined;
  readonly postSubscription: PostSubscriptionFn;
} {
  const { data: vapidKey } = useVapidPublicKey();
  const createSubscription = useCreatePushSubscription();
  // `useCallback` keeps the function referentially stable across renders so
  // the consumer's `useEffect` does not loop on every render.
  const postSubscription = useCallback<PostSubscriptionFn>(
    async (payload, categories) => {
      await createSubscription.mutateAsync(
        categories === undefined ? payload : { ...payload, categories },
      );
    },
    [createSubscription],
  );
  return { vapidKey, postSubscription };
}
