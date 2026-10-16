'use client';

import { useMemo, useSyncExternalStore } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import {
  isPushSupported,
  readNotificationPermission,
  subscribeToPermissionChanges,
} from '../lib/platform';
import {
  PENDING_PUSH_STATUS,
  mergePushStatus,
  resolveInitialPushStatus,
  type PushSubscriptionStatus,
} from '../lib/push-subscription-status';
import { usePushSubscriptionProbe, pushSubscriptionKeys } from './hooks';

export type { PushSubscriptionStatus };

export type PushSubscriptionStatusResult = PushSubscriptionStatus & {
  /**
   * Re-probe the browser's subscription (permission and support re-read
   * live through the permissions store). Call after an explicit user action
   * that may have created or removed a subscription — otherwise the status
   * stays stale until the next invalidation.
   */
  readonly refresh: () => void;
};

// Синхронная часть пробы (поддержка + разрешение) — снапшот-хранилище с
// живыми обновлениями: браузер не уведомляет о смене разрешения через
// Notification.permission, но permissions.query({name:'notifications'})
// кидает change при оптине из Site Settings и автоотзыве Chrome (спека
// #1028 §6, слайс 2 #1038) — useSyncExternalStore перечитывает вердикт без
// перезагрузки. Подписки на смене разрешения НЕ влечёт автоподписки —
// осознанное отклонение от рекомендации исследования (дефолт ВЫКЛ).
// getServerSnapshot держит SSR и гидратацию в нейтральном pending —
// расхождение сервер/клиент React закрывает пере-рендером без ошибки
// гидратации (#418).
const subscribeToPermission = subscribeToPermissionChanges;
const getSyncPushStatus = (): PushSubscriptionStatus =>
  resolveInitialPushStatus({
    supported: isPushSupported(),
    permission: readNotificationPermission(),
  });
const getServerPushStatus = (): PushSubscriptionStatus => PENDING_PUSH_STATUS;

/**
 * Inspect the browser push capability and return a coarse-grained status the
 * notification-settings UI can branch on.
 *
 * The synchronous parts (support, permission) settle through
 * `useSyncExternalStore` right after hydration and update live on external
 * permission changes; the settings screen holds its loading archetype until
 * then — a late card insert shifted the sections (audit #877, CLS 0.15).
 * The subscription probe is a react-query on the shared
 * `pushSubscriptionKeys.subscription` key: heal-отписка и авто-подписка
 * стартового гейта обновляют все потребители (экран настроек, тост-гейт)
 * одной инвалидацией, без гонки «экран увидел подписку до heal'а».
 */
export function usePushSubscriptionStatus(): PushSubscriptionStatusResult {
  const syncStatus = useSyncExternalStore(
    subscribeToPermission,
    getSyncPushStatus,
    getServerPushStatus,
  );
  // Проба подписки смыслена только при выданном разрешении (как раньше:
  // не-выданное — синхронная ветка, подписку у браузера не ждём).
  const probe = usePushSubscriptionProbe({
    enabled: syncStatus.isPending,
  });
  const queryClient = useQueryClient();

  // Ручная мемоизация с комментарием (CODING_STANDARDS, React Compiler):
  // слияние синхронного вердикта и вердикта подписки — чистая функция,
  // проверяется модульно (mergePushStatus).
  const status = useMemo(
    () => mergePushStatus(syncStatus, probe.data),
    [syncStatus, probe.data],
  );

  return useMemo(
    () => ({
      ...status,
      refresh: () => {
        void queryClient.invalidateQueries({
          queryKey: pushSubscriptionKeys.subscription,
        });
      },
    }),
    [status, queryClient],
  );
}
