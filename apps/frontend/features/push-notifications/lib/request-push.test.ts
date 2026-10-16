import { afterEach, describe, expect, it, vi } from 'vitest';
import {
    requestPushPermissionAndSubscribe,
    type PostSubscriptionFn,
} from './request-push';

// 27 символов — корректный base64url (длина % 4 = 3; группа из одного
// символа невалидна для atob)
const VAPID_KEY = 'B_vapid_key_base64url_value';
const ENDPOINT = 'https://fcm.googleapis.com/fcm/send/abc';

function makeBrowserSubscription(): PushSubscription {
    return {
        endpoint: ENDPOINT,
        expirationTime: null,
        getKey(name: 'p256dh' | 'auth') {
            if (name === 'p256dh') return new ArrayBuffer(65);
            return new ArrayBuffer(16);
        },
        toJSON: () => ({}),
    } as unknown as PushSubscription;
}

type BrowserStub = {
    readonly permission: NotificationPermission;
    readonly requestPermissionResult: NotificationPermission;
    readonly existingSubscription: PushSubscription | null;
    readonly subscribeError?: Error;
    readonly subscribe: ReturnType<typeof vi.fn>;
    readonly getSubscription: ReturnType<typeof vi.fn>;
};

/** Собирает фейковый браузер: Notification + serviceWorker + PushManager. */
function stubBrowser({
    permission = 'default',
    requestPermissionResult = 'granted',
    existingSubscription = null,
    subscribeError,
    subscribeResult,
}: Partial<BrowserStub> & { readonly subscribeResult?: Promise<PushSubscription> } = {}): BrowserStub & { readonly requestPermission: ReturnType<typeof vi.fn> } {
    const requestPermission = vi.fn().mockResolvedValue(requestPermissionResult);
    const subscribe = vi.fn(() => {
        if (subscribeError) return Promise.reject(subscribeError);
        if (subscribeResult) return subscribeResult;
        return Promise.resolve(makeBrowserSubscription());
    });
    const getSubscription = vi.fn().mockResolvedValue(existingSubscription);

    const notificationApi = { permission, requestPermission };
    vi.stubGlobal('Notification', notificationApi);
    vi.stubGlobal('window', {
        PushManager: {},
        Notification: notificationApi,
    });
    vi.stubGlobal('navigator', {
        // desktop UA: не iOS — requiresInstallOnIos() false
        userAgent:
            'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4.1 Safari/605.1.15',
        serviceWorker: {
            ready: Promise.resolve({
                pushManager: { subscribe, getSubscription },
            }),
        },
    });
    // платформенный детектор читает 'ontouchend' in document (мак-ветка UA)
    vi.stubGlobal('document', {});
    return { permission, requestPermissionResult, existingSubscription, subscribe, getSubscription, requestPermission };
}

function makePost(): ReturnType<typeof vi.fn<PostSubscriptionFn>> {
    return vi.fn((): Promise<void> => Promise.resolve());
}

afterEach(() => {
    vi.unstubAllGlobals();
});

describe('requestPushPermissionAndSubscribe — явное включение (слайс 2, спека #1028 §2–§3)', () => {
    it('браузер без Web Push — unsupported, ничего не трогаем', async () => {
        vi.stubGlobal('window', {});
        vi.stubGlobal('navigator', {});
        const post = makePost();
        const outcome = await requestPushPermissionAndSubscribe(VAPID_KEY, post);
        expect(outcome).toStrictEqual({ outcome: 'unsupported' });
        expect(post).not.toHaveBeenCalled();
    });

    it('iOS без установленной PWA — инструкция установки, системного окна нет', async () => {
        const notificationApi = { permission: 'default', requestPermission: vi.fn() };
        vi.stubGlobal('Notification', notificationApi);
        vi.stubGlobal('window', {
            PushManager: {},
            Notification: notificationApi,
            matchMedia: () => ({ matches: false }),
        });
        vi.stubGlobal('navigator', {
            userAgent:
                'Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1',
            serviceWorker: { ready: Promise.resolve({ pushManager: {} }) },
        });
        vi.stubGlobal('document', { ontouchend: null });
        const post = makePost();
        const outcome = await requestPushPermissionAndSubscribe(VAPID_KEY, post);
        expect(outcome).toStrictEqual({ outcome: 'ios-needs-install' });
        expect(notificationApi.requestPermission).not.toHaveBeenCalled();
    });

    it('разрешение уже отклонено — denied без системного окна', async () => {
        stubBrowser({ permission: 'denied' });
        const post = makePost();
        const outcome = await requestPushPermissionAndSubscribe(VAPID_KEY, post);
        expect(outcome).toStrictEqual({ outcome: 'denied' });
        expect(post).not.toHaveBeenCalled();
    });

    it('default: промпт отклонён — denied, подписки нет', async () => {
        stubBrowser({ permission: 'default', requestPermissionResult: 'denied' });
        const post = makePost();
        const outcome = await requestPushPermissionAndSubscribe(VAPID_KEY, post);
        expect(outcome).toStrictEqual({ outcome: 'denied' });
        expect(post).not.toHaveBeenCalled();
    });

    it('default: промпт выдан — тихая подписка + POST (полученные категории уходят явно)', async () => {
        const browser = stubBrowser({ permission: 'default', requestPermissionResult: 'granted' });
        const post = makePost();
        const categories = { rental: true, payments_operations: false, tasks: false, shared_access: false };
        const outcome = await requestPushPermissionAndSubscribe(VAPID_KEY, post, categories);
        expect(browser.subscribe).toHaveBeenCalledWith(
            expect.objectContaining({ userVisibleOnly: true }),
        );
        expect(outcome.outcome).toBe('subscribed');
        if (outcome.outcome !== 'subscribed') throw new Error('unreachable');
        expect(outcome.subscription.endpoint).toBe(ENDPOINT);
        expect(post).toHaveBeenCalledTimes(1);
        const firstCall = post.mock.calls[0];
        if (!firstCall) throw new Error('POST не вызван');
        const payload = firstCall[0];
        expect(payload.endpoint).toBe(ENDPOINT);
        expect(firstCall[1]).toStrictEqual(categories);
    });

    it('разрешение уже granted — окно не показывается, подписка создаётся молча', async () => {
        const browser = stubBrowser({ permission: 'granted' });
        const post = makePost();
        const outcome = await requestPushPermissionAndSubscribe(VAPID_KEY, post);
        expect(browser.requestPermission).not.toHaveBeenCalled();
        expect(outcome.outcome).toBe('subscribed');
    });

    it('живая подписка уже есть — повторной подписки и POST нет (already-subscribed)', async () => {
        const existing = makeBrowserSubscription();
        const browser = stubBrowser({ permission: 'granted', existingSubscription: existing });
        const post = makePost();
        const outcome = await requestPushPermissionAndSubscribe(VAPID_KEY, post);
        expect(browser.subscribe).not.toHaveBeenCalled();
        expect(post).not.toHaveBeenCalled();
        expect(outcome).toStrictEqual({ outcome: 'already-subscribed', subscription: existing });
    });

    it('subscribe бросил (отказ/транзиент) — error subscribe-failed', async () => {
        stubBrowser({ permission: 'granted', subscribeError: new DOMException('denied', 'NotAllowedError') });
        const post = makePost();
        const outcome = await requestPushPermissionAndSubscribe(VAPID_KEY, post);
        expect(outcome).toStrictEqual({ outcome: 'error', reason: 'subscribe-failed' });
        expect(post).not.toHaveBeenCalled();
    });

    it('браузер скрыл p256dh/auth — error missing-keys, POST не уходит', async () => {
        const broken = makeBrowserSubscription();
        (broken as unknown as { getKey: () => null }).getKey = () => null;
        stubBrowser({ permission: 'granted', subscribeResult: Promise.resolve(broken) });
        const post = makePost();
        const outcome = await requestPushPermissionAndSubscribe(VAPID_KEY, post);
        expect(outcome).toStrictEqual({ outcome: 'error', reason: 'missing-keys' });
        expect(post).not.toHaveBeenCalled();
    });

    it('POST упал (сеть/5xx) — error network-error', async () => {
        stubBrowser({ permission: 'granted' });
        const post = makePost();
        post.mockRejectedValueOnce(new Error('boom'));
        const outcome = await requestPushPermissionAndSubscribe(VAPID_KEY, post);
        expect(outcome).toStrictEqual({ outcome: 'error', reason: 'network-error' });
    });

    it('VAPID-ключа нет — error no-vapid-key, подписка не создаётся', async () => {
        const browser = stubBrowser({ permission: 'granted' });
        const post = makePost();
        const outcome = await requestPushPermissionAndSubscribe(undefined, post);
        expect(outcome).toStrictEqual({ outcome: 'error', reason: 'no-vapid-key' });
        expect(browser.subscribe).not.toHaveBeenCalled();
    });
});
