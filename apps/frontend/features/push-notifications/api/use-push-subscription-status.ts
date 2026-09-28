'use client';

import { useCallback, useEffect, useMemo, useState, useSyncExternalStore } from 'react';
import {
  isPushSupported,
  readNotificationPermission,
} from '../lib/platform';
import {
  PENDING_PUSH_STATUS,
  resolveInitialPushStatus,
  type PushSubscriptionStatus,
} from '../lib/push-subscription-status';

export type { PushSubscriptionStatus };

export type PushSubscriptionStatusResult = PushSubscriptionStatus & {
  /**
   * Re-probe the browser's push capability, permission, and subscription.
   * The browser does not emit an event when any of these change, so call this
   * after an explicit user action that may affect the result (e.g. once the
   * "Разрешить пуши" button has created a subscription) — otherwise the status
   * stays stale until a page reload.
   */
  readonly refresh: () => void;
};

// Синхронная часть пробы (поддержка + разрешение) — снапшот-хранилище без
// событий: браузер не уведомляет об их изменении, обновление приходит
// ре-рендером после явных действий (refresh). getServerSnapshot держит SSR и
// гидратацию в нейтральном pending — расхождение сервер/клиент React
// закрывает пере-рендером без ошибки гидратации (#418).
const subscribeToNothing = (): (() => void) => () => {};
const getSyncPushStatus = (): PushSubscriptionStatus =>
  resolveInitialPushStatus({
    supported: isPushSupported(),
    permission: readNotificationPermission(),
  });
const getServerPushStatus = (): PushSubscriptionStatus => PENDING_PUSH_STATUS;

/** Вердикт асинхронной половины пробы — подписка браузера. */
type SubscriptionVerdict = {
  readonly probed: boolean;
  readonly endpoint: string | null;
};

const NOT_PROBED: SubscriptionVerdict = { probed: false, endpoint: null };

/**
 * Inspect the browser push capability and return a coarse-grained status the
 * notification-settings UI can branch on.
 *
 * The synchronous parts (support, permission) settle through
 * `useSyncExternalStore` right after hydration; the settings screen holds its
 * loading archetype until then — a late card insert shifted the sections
 * (audit #877, CLS 0.15). Only the subscription check is async: its setState
 * lives in `.then`/`setTimeout` callbacks, which the set-state-in-effect rule
 * treats as legitimate (state settles after an awaited operation, not
 * synchronously on mount).
 */
export function usePushSubscriptionStatus(): PushSubscriptionStatusResult {
  const syncStatus = useSyncExternalStore(
    subscribeToNothing,
    getSyncPushStatus,
    getServerPushStatus,
  );
  const [verdict, setVerdict] = useState<SubscriptionVerdict>(NOT_PROBED);

  // Ручная мемоизация с комментарием (CODING_STANDARDS, React Compiler):
  // слияние синхронного вердикта и вердикта подписки — зависимостей ровно
  // две, компилятору тут полагаться нечего кэшировать сверх этого.
  const status = useMemo<PushSubscriptionStatus>(() => {
    if (!syncStatus.isPending) {
      return syncStatus;
    }
    if (!verdict.probed) {
      return PENDING_PUSH_STATUS;
    }
    return {
      isUnsupported: false,
      needsPermission: false,
      permissionDenied: false,
      needsSubscription: verdict.endpoint === null,
      isReady: verdict.endpoint !== null,
      isPending: false,
      endpoint: verdict.endpoint,
    };
  }, [syncStatus, verdict]);

  // Re-read the browser push capability/permission/subscription. Explicit
  // useCallback, not the compiler's automatic memoization: exhaustive-deps
  // is static and cannot see it, so a plain function plus `[refresh]` deps
  // below is a lint error; with useCallback the identity is stable by
  // construction. `cancelled` is threaded in by the mount effect so a probe
  // that resolves after unmount does not call setState.
  const refresh = useCallback(async (cancelled: () => boolean = () => false): Promise<void> => {
    if (!isPushSupported()) return;
    // Не-выданное разрешение — синхронная ветка: её вердикт уже в снапшоте
    // (обновится следующим ре-рендером), подписку у браузера не ждём.
    if (readNotificationPermission() !== 'granted') return;

    try {
      const registration = await navigator.serviceWorker.ready;
      const subscription = await registration.pushManager.getSubscription();
      if (!cancelled()) {
        setVerdict({ probed: true, endpoint: subscription?.endpoint ?? null });
      }
    } catch {
      // Деградация: проба подписки не сошлась (SW не зарегистрировался) —
      // считаем подписки нет, тумблеры живут на локальном состоянии.
      if (!cancelled()) {
        setVerdict({ probed: true, endpoint: null });
      }
    }
  }, []);

  useEffect(() => {
    // The synchronous cases (unsupported, permission verdict) settle in the
    // snapshot; the mount probe adds the subscription verdict on top. The
    // async IIFE keeps the setState calls behind an await so ESLint's
    // react-hooks/set-state-in-effect rule does not fire.
    let cancelled = false;
    // Страховка вечного pending: если проба подписки не сошлась за 3с
    // (SW-регистрация зависла), экран выходит из архетипа с вердиктом «без
    // подписки»; поздний успех refresh всё равно обновит endpoint.
    const fallback = setTimeout(() => {
      if (!cancelled) {
        setVerdict({ probed: true, endpoint: null });
      }
    }, 3000);
    void (async () => {
      await refresh(() => cancelled);
      clearTimeout(fallback);
    })();
    return () => {
      cancelled = true;
      clearTimeout(fallback);
    };
  }, [refresh]);

  // Ручная мемоизация (CODING_STANDARDS, React Compiler): объект результата
  // собирается заново только при смене verdict'а или refresh.
  return useMemo(
    () => ({ ...status, refresh: () => void refresh() }),
    [status, refresh],
  );
}
