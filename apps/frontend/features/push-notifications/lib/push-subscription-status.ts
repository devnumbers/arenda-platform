/**
 * Начальный статус пробы браузера для пуш-UI (экран «Настроить
 * уведомления» #746, гейт стрима #769). Поддержка Web Push и выданное
 * разрешение читаются синхронно: их вердикт известен сразу после
 * гидратации — useSyncExternalStore закрывает расхождение сервер/клиент
 * пере-рендером без ошибки гидратации. Экран настроек до этого момента
 * держит архетип загрузки: поздняя вставка карточки «Разрешите пуши»
 * между секциями сдвигала бы их (аудит #877 — CLS 0.15 на холодном
 * входе). Асинхронной остаётся только подписка (endpoint) — она меняет
 * тумблеры 1:1, каркас не двигает.
 *
 * Все исходы — синглтоны: `useSyncExternalStore` сравнивает снапшоты через
 * Object.is, новый объект на каждый вызов getSnapshot зациклил бы рендер.
 */

import type { NotificationPermissionState } from './platform';

export type PushSubscriptionStatus = {
    /** The browser cannot receive push at all. */
    readonly isUnsupported: boolean;
    /** Notification permission has not been granted. */
    readonly needsPermission: boolean;
    /** The user explicitly denied notification permission — the system prompt cannot be re-shown. */
    readonly permissionDenied: boolean;
    /** Permission granted but no active subscription on this device. */
    readonly needsSubscription: boolean;
    /** Push is supported, permission granted, and an active subscription exists. */
    readonly isReady: boolean;
    /** Initial SSR-safe state — no decision yet. */
    readonly isPending: boolean;
    /**
     * Endpoint URL of the active browser subscription — the device key of the
     * per-device preferences API (#743, решение #738). null until a live
     * subscription is probed (pending / unsupported / no subscription).
     */
    readonly endpoint: string | null;
};

export const PENDING_PUSH_STATUS: PushSubscriptionStatus = {
    isUnsupported: false,
    needsPermission: false,
    permissionDenied: false,
    needsSubscription: false,
    isReady: false,
    isPending: true,
    endpoint: null,
};

export const UNSUPPORTED_PUSH_STATUS: PushSubscriptionStatus = {
    isUnsupported: true,
    needsPermission: false,
    permissionDenied: false,
    needsSubscription: false,
    isReady: false,
    isPending: false,
    endpoint: null,
};

const PERMISSION_DEFAULT_PUSH_STATUS: PushSubscriptionStatus = {
    isUnsupported: false,
    needsPermission: true,
    permissionDenied: false,
    needsSubscription: false,
    isReady: false,
    isPending: false,
    endpoint: null,
};

const PERMISSION_DENIED_PUSH_STATUS: PushSubscriptionStatus = {
    isUnsupported: false,
    needsPermission: true,
    permissionDenied: true,
    needsSubscription: false,
    isReady: false,
    isPending: false,
    endpoint: null,
};

/** Синхронная часть пробы: поддержка Web Push и выданное разрешение.
 * `'unsupported'` в правах означает отсутствие Notification API — при
 * поддержке контейнера такое не встречается, трактуем как «не выдано». */
export function resolveInitialPushStatus(input: {
    readonly supported: boolean;
    readonly permission: NotificationPermissionState;
}): PushSubscriptionStatus {
    if (!input.supported) {
        return UNSUPPORTED_PUSH_STATUS;
    }
    if (input.permission === 'granted') {
        return PENDING_PUSH_STATUS;
    }
    return input.permission === 'denied'
        ? PERMISSION_DENIED_PUSH_STATUS
        : PERMISSION_DEFAULT_PUSH_STATUS;
}
