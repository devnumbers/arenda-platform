/**
 * Правило авто-промпта разрешения, вариант (c) (карта #1545, разрешение
 * #1548 — решения владельца 22.10.2026; отмена «авто-промпт один раз» —
 * аменда §3 спеки #1028): нативное окно `Notification.requestPermission()`
 * запрашивается **при каждом заходе**, пока разрешение `default`, — без
 * жеста и без in-app призыва. Память о показе не хранится нигде:
 * localStorage-флаг `push-prompt-shown` снесён (старые ключи в хранилищах
 * пользователей просто перестают читаться), единственный гейт повтора —
 * `permission === 'default'`. Исходы окна в правило не входят: закрытое без
 * ответа окно оставляет `default` и повторяется на следующем заходе;
 * подавление Chrome (quieter UI, эмбарго) — дело браузера, исход
 * «`default` без окна» для вызывающего — мягкий отказ.
 */

import { allCategoriesEnabled } from '@/entities/notification';
import type { NotificationPermissionState } from './platform';
import type {
  requestPushPermissionAndSubscribe,
  PostSubscriptionFn,
  PushSubscribeErrorReason,
  RequestPushPermissionOutcome,
} from './request-push';

/** Мир, в котором правило решает судьбу авто-промпта. */
export type AutoPromptWorld = {
  /** Текущее разрешение браузера (проба `platform.ts`). */
  readonly permission: NotificationPermissionState;
  /** Платформа допускает безжестовый запрос окна (WebKit — нет). */
  readonly allowsGesturelessPrompt: boolean;
};

/** True — окно запрашивается (с последующей тихой подпиской при grant);
 * false — молчание: на WebKit вызов не делается вовсе (риск персистентного
 * `denied` ⚠ ломает жестовый путь), при `denied`/`granted` повтора нет.
 * На Firefox правило возвращает true — вызов делается, но инстантный reject
 * ловится тихо (шаг ниже), включение остаётся за тумблером. */
export function autoPromptShouldRequest(world: AutoPromptWorld): boolean {
  return world.allowsGesturelessPrompt && world.permission === 'default';
}

/** Исход шага авто-промпта для вызывающего: `subscribed` — тост «Пуши
 * включены»; `error` — reportClientError с причиной; `silent` — ничего:
 * разрешение было не `default`, окна не было/не дали (denied, резолв
 * `default` без окна — подавление Chrome, тихий reject Firefox), ключа
 * VAPID ещё нет, iOS без PWA, или репортить нечего (`no-vapid-key`). */
export type AutoPromptStepOutcome =
  | { readonly kind: 'subscribed' }
  | { readonly kind: 'silent' }
  | { readonly kind: 'error'; readonly reason: PushSubscribeErrorReason };

/** Мир шага 2 стартового гейта (use-push-startup): всё внешнее — инъекцией,
 * по прецеденту heal (спека #1028 §4). */
export type AutoPromptStepDeps = {
  /** VAPID-ключ бекенда; undefined — ключ ещё в воздухе, окно не тратим. */
  readonly vapidKey: string | undefined;
  readonly postSubscription: PostSubscriptionFn;
  /** Проба разрешения (`readNotificationPermission`). */
  readonly probePermission: () => NotificationPermissionState;
  /** Платформенный гейт (`allowsGesturelessPermissionPrompt`). */
  readonly allowsGesturelessPrompt: () => boolean;
  readonly request: typeof requestPushPermissionAndSubscribe;
};

/**
 * Шаг 2 стартового гейта — авто-промпт варианта (c) (карта #1545,
 * разрешение #1548, аменда §3 спеки #1028): при каждом заходе, пока
 * разрешение `default`, окно запрашивается без жеста; grant → тихая
 * подписка + POST со всеми категориями ВКЛ. Сознательно глотает **любой**
 * бросок запроса: Firefox 72+ инстантно реджектит безжестовый
 * `requestPermission()` — исход тихий (не ошибка, не репортинг,
 * `default` не трогается, включение — тумблером), а прочие броски
 * (отказ serviceWorker) авто-промпту тоже не выдается наружу — вежливость
 * при заходе; структурные ошибки подписки остаются исходом `error`.
 * Жестовый путь (тумблер) этот контракт не разделяет: request-push
 * держит его реджект строгим (клик — событие юзера, окно показывается).
 */
export async function runAutoPromptStep(
  deps: AutoPromptStepDeps,
): Promise<AutoPromptStepOutcome> {
  // VAPID-ключ ждём до окна: grant без возможности подписаться потратил бы
  // разрешение впустую.
  if (deps.vapidKey === undefined) return { kind: 'silent' };
  if (
    !autoPromptShouldRequest({
      permission: deps.probePermission(),
      allowsGesturelessPrompt: deps.allowsGesturelessPrompt(),
    })
  ) {
    return { kind: 'silent' };
  }

  let outcome: RequestPushPermissionOutcome;
  try {
    outcome = await deps.request(
      deps.vapidKey,
      deps.postSubscription,
      allCategoriesEnabled(),
    );
  } catch {
    return { kind: 'silent' };
  }

  if (outcome.outcome === 'subscribed') return { kind: 'subscribed' };
  if (outcome.outcome === 'error' && outcome.reason !== 'no-vapid-key') {
    return { kind: 'error', reason: outcome.reason };
  }
  return { kind: 'silent' };
}
