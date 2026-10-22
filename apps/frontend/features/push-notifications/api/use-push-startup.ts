'use client';

import { useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ApiError } from '@/shared/api/errors';
import { notificationKeys } from '@/shared/api/query-keys';
import { reportClientError } from '@/shared/lib/error-reporting/report-client-error';
import { notify } from '@/shared/lib/notifications';
import {
  healPushSubscription,
  type PushRowProbe,
} from '../lib/heal';
import { getBrowserSubscription } from '../lib/browser-subscription';
import { runAutoPromptStep } from '../lib/auto-prompt';
import {
  allowsGesturelessPermissionPrompt,
  isPushSupported,
  readNotificationPermission,
} from '../lib/platform';
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
 * слайс 2 #1038, спека #1028 §3–§4 в редакции аменды #1548). Два шага,
 * строго по порядку:
 *
 * 1. **Heal** — браузерная подписка есть, а строки в БД нет ⇒ молчаливая
 *    `unsubscribe()` (вариант А): UI честно показывает «выключено», хвост
 *    чужой строки у push-сервиса добивает 410-prune диспетчера. Сетевая
 *    ошибка пробы подписку не трогает. Проба подписки (react-query) и кэш
 *    настроек этого endpoint обновляются здесь же — экран настроек не
 *    успевает соврать.
 * 2. **Авто-промпт варианта (c)** (карта #1545, разрешение #1548 — решения
 *    владельца 22.10.2026, отмена «авто-промпт один раз»): разрешение ещё
 *    `default` ⇒ `requestPermission()` без жеста — **при каждом заходе**;
 *    памяти о показе нет (localStorage-флаг `push-prompt-shown` снесён,
 *    старые ключи игнорируются), единственный гейт повтора — `default`.
 *    grant → тихая подписка + POST со всеми категориями ВКЛ + тост; иначе —
 *    молчание: denied, резолв `default` без окна (подавление Chrome) и тихий
 *    reject Firefox (безжестовый промис реджектится — глотает шаг
 *    `runAutoPromptStep`) не ошибка и не репортинг. WebKit (Safari macOS,
 *    iOS PWA) безжестового вызова не получает вовсе — риск персистентного
 *    `denied` ⚠; iOS без установленной PWA молчит (`ios-needs-install`).
 *    Включение во всех этих браузерах — только жестовым путём, тумблером
 *    настроек.
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

      // 2. Авто-промпт варианта (c): единственный гейт повтора — разрешение
      // default при каждом заходе; WebKit безжестового вызова не получает,
      // тихий reject Firefox глотает шаг. VAPID-ключ ждём до окна (шаг):
      // grant без возможности подписаться потратил бы разрешение впустую.
      if (cancelled()) return;
      const step = await runAutoPromptStep({
        vapidKey,
        postSubscription,
        probePermission: readNotificationPermission,
        allowsGesturelessPrompt: allowsGesturelessPermissionPrompt,
        request: requestPushPermissionAndSubscribe,
      });
      if (cancelled()) return;
      if (step.kind === 'subscribed') {
        // POST-мутация уже посадила endpoint в пробу подписки — экран
        // настроек и тост-гейт обновились; остаётся тост.
        notify.scenarios.profile.pushEnabled();
      } else if (step.kind === 'error') {
        reportClientError(`push auto-prompt subscribe failed: ${step.reason}`);
      }
    })();

    return () => {
      flow.abort();
    };
  }, [vapidKey, postSubscription, queryClient]);
}
