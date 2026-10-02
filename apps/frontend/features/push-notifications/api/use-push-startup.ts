'use client';

import { useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ApiError } from '@/shared/api/errors';
import { notificationKeys } from '@/shared/api/query-keys';
import { reportClientError } from '@/shared/lib/error-reporting/report-client-error';
import { notify } from '@/shared/lib/notifications';
import { allCategoriesEnabled } from '@/entities/notification';
import {
  healPushSubscription,
  type PushRowProbe,
} from '../lib/heal';
import { getBrowserSubscription } from '../lib/browser-subscription';
import { isPushSupported, readNotificationPermission } from '../lib/platform';
import {
  hasPushPromptBeenShown,
  markPushPromptShown,
} from '../lib/push-prompt-flag';
import { requestPushPermissionAndSubscribe } from '../lib/request-push';
import { usePushSubscriptionTools } from '../lib/push-tools';
import { pushSubscriptionKeys } from './hooks';

/** GET /push/subscriptions/preferences?endpoint, переведённый в вердикт
 * правды: 200 — строка есть, 404 — строки нет (аналогия №7), прочее
 * (сеть/5xx) — правда неизвестна, подписку не трогаем. */
async function probePushRow(endpoint: string): Promise<PushRowProbe> {
  const query = new URLSearchParams({ endpoint });
  try {
    await apiClient<unknown>(
      `/push/subscriptions/preferences?${query.toString()}`,
    );
    return 'row-exists';
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) {
      return 'row-missing';
    }
    return 'probe-failed';
  }
}

/**
 * Стартовый гейт пушей (маунтится в ScreenLayout — авторизованная зона,
 * слайс 2 #1038, спека #1028 §3–§4). Два шага, строго по порядку:
 *
 * 1. **Heal** — браузерная подписка есть, а строки в БД нет ⇒ молчаливая
 *    `unsubscribe()` (вариант А): UI честно показывает «выключено», хвост
 *    чужой строки у push-сервиса добивает 410-prune диспетчера. Сетевая
 *    ошибка пробы подписку не трогает. Проба подписки (react-query) и кэш
 *    настроек этого endpoint обновляются здесь же — экран настроек не
 *    успевает соврать.
 * 2. **Авто-промпт новому юзеру** — разрешение ещё `default` и флага
 *    `push-prompt-shown` нет в localStorage: `requestPermission()` без
 *    жеста; флаг ставится до окна и независимо от исхода — повторов нет.
 *    grant → тихая подписка + POST со всеми категориями ВКЛ + тост; иначе —
 *    ничего. iOS без установленной PWA и Firefox окно не покажут (ограничение
 *    принято в спеке) — флаг всё равно ставится.
 *
 * Логаут подписку НЕ отпишивает (per-device, осознанно): смена аккаунта в
 * том же браузере лечится этим же heal'ом. `ensureActiveSubscription` с
 * фоновым ресабскрайбом снесён: подписка возникает только из явного
 * действия.
 */
export function usePushStartup(): void {
  const queryClient = useQueryClient();
  const { vapidKey, postSubscription } = usePushSubscriptionTools();

  useEffect(() => {
    if (!isPushSupported()) return;
    // Гейт потока. Чтение — через функцию: TS пережимает свойство
    // signal.aborted литералом после первого if'а и не видит abort() из
    // cleanup, прямой второй if линт честно считает мёртвым
    // (no-unnecessary-condition) — и прав: abort() внешняя мутация.
    const flow = new AbortController();
    const cancelled = (): boolean => flow.signal.aborted;

    void (async () => {
      // 1. Heal: подписка без строки БД — молчаливая отписка.
      try {
        const result = await healPushSubscription({
          getSubscription: getBrowserSubscription,
          probeRow: probePushRow,
        });
        if (cancelled()) return;
        if (result.outcome === 'unsubscribed') {
          // Проба подписки и кэш настроек этого endpoint: экран настроек и
          // тост-гейт видят «выключено» сразу, кэш не переживает повторную
          // подписку того же endpoint (FCM выдаёт стабильный URL).
          queryClient.setQueryData(pushSubscriptionKeys.subscription, null);
          queryClient.removeQueries({
            queryKey: notificationKeys.pushPreferences(result.endpoint),
          });
        }
      } catch (error) {
        if (!cancelled()) {
          reportClientError(
            'push heal threw',
            error instanceof Error ? error.stack : undefined,
          );
        }
        return;
      }

      // 2. Авто-промпт: только пока разрешение default и окно ещё не
      // показывали этому браузеру. VAPID-ключ ждём до окна: grant без
      // возможности подписаться потратил бы разрешение впустую.
      if (cancelled() || vapidKey === undefined) return;
      if (readNotificationPermission() !== 'default') return;
      if (hasPushPromptBeenShown()) return;
      markPushPromptShown();

      const outcome = await requestPushPermissionAndSubscribe(
        vapidKey,
        postSubscription,
        allCategoriesEnabled(),
      );
      if (cancelled()) return;
      if (outcome.outcome === 'subscribed') {
        // POST-мутация уже посадила endpoint в пробу подписки — экран
        // настроек и тост-гейт обновились; остаётся тост.
        notify.scenarios.profile.pushEnabled();
      } else if (outcome.outcome === 'error' && outcome.reason !== 'no-vapid-key') {
        reportClientError(`push auto-prompt subscribe failed: ${outcome.reason}`);
      }
    })();

    return () => {
      flow.abort();
    };
  }, [vapidKey, postSubscription, queryClient]);
}
