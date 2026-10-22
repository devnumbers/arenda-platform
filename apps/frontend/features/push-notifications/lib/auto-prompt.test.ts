import { describe, expect, it, vi } from 'vitest';
import { allCategoriesEnabled } from '@/entities/notification';
import {
  autoPromptShouldRequest,
  runAutoPromptStep,
  type AutoPromptStepDeps,
} from './auto-prompt';
import type { NotificationPermissionState } from './platform';
import type { RequestPushPermissionOutcome } from './request-push';

describe('autoPromptShouldRequest — правило авто-промпта (c), #1548', () => {
  it('default: окно запрашивается — при каждом заходе, память о показе не хранится', () => {
    expect(
      autoPromptShouldRequest({
        permission: 'default',
        allowsGesturelessPrompt: true,
      }),
    ).toBe(true);
  });

  it('denied: молчание — пользователь сказал «нет», окно больше не поднимается', () => {
    expect(
      autoPromptShouldRequest({
        permission: 'denied',
        allowsGesturelessPrompt: true,
      }),
    ).toBe(false);
  });

  it('granted: молчание — пуши уже разрешены, окно не нужно', () => {
    expect(
      autoPromptShouldRequest({
        permission: 'granted',
        allowsGesturelessPrompt: true,
      }),
    ).toBe(false);
  });

  it('unsupported: молчание (поддержка проверяется выше, правило ничего не знает о ней)', () => {
    expect(
      autoPromptShouldRequest({
        permission: 'unsupported',
        allowsGesturelessPrompt: true,
      }),
    ).toBe(false);
  });

  it('WebKit (Safari macOS, iOS PWA): безжестового вызова нет даже при default — риск персистентного denied', () => {
    expect(
      autoPromptShouldRequest({
        permission: 'default',
        allowsGesturelessPrompt: false,
      }),
    ).toBe(false);
  });
});

const VAPID_KEY = 'B_vapid_key_base64url_value';
const SUBSCRIPTION = { endpoint: 'https://fcm.googleapis.com/fcm/send/abc' } as unknown as PushSubscription;

/** Сборка мира шага: платформа/разрешение и исход запроса — инъекцией,
 * без браузерных глобалов. */
function makeStepDeps(
  overrides: {
    readonly permission?: NotificationPermissionState;
    readonly gestureless?: boolean;
    readonly vapidKey?: string | undefined;
    readonly outcome?: RequestPushPermissionOutcome;
    readonly throwInstead?: Error;
  } = {},
): {
  readonly deps: AutoPromptStepDeps;
  readonly request: ReturnType<typeof vi.fn<() => Promise<RequestPushPermissionOutcome>>>;
  readonly post: ReturnType<typeof vi.fn<() => Promise<void>>>;
} {
  const post = vi.fn((): Promise<void> => Promise.resolve());
  const request = vi.fn((): Promise<RequestPushPermissionOutcome> => {
    if (overrides.throwInstead) return Promise.reject(overrides.throwInstead);
    return Promise.resolve(overrides.outcome ?? { outcome: 'denied' });
  });
  return {
    deps: {
      // 'in', а не ??: явно переданный undefined («ключ ещё не пришёл») —
      // валидный сценарий мира, ?? подменил бы его дефолтом.
      vapidKey: 'vapidKey' in overrides ? overrides.vapidKey : VAPID_KEY,
      postSubscription: post,
      probePermission: () => overrides.permission ?? 'default',
      allowsGesturelessPrompt: () => overrides.gestureless ?? true,
      request: request,
    },
    request,
    post,
  };
}

describe('runAutoPromptStep — шаг 2 стартового гейта, вариант (c) (#1548)', () => {
  it('default на Chromium — окно запрашивается с тост-исходом subscribed, категории — все ВКЛ', async () => {
    const stub = makeStepDeps({
      outcome: { outcome: 'subscribed', subscription: SUBSCRIPTION },
    });
    await expect(runAutoPromptStep(stub.deps)).resolves.toStrictEqual({
      kind: 'subscribed',
    });
    expect(stub.request).toHaveBeenCalledWith(
      VAPID_KEY,
      stub.deps.postSubscription,
      allCategoriesEnabled(),
    );
  });

  it('denied/granted/unsupported — молчание, запрос даже не начинается', async () => {
    for (const permission of ['denied', 'granted', 'unsupported'] as const) {
      const stub = makeStepDeps({ permission });
      await expect(runAutoPromptStep(stub.deps)).resolves.toStrictEqual({
        kind: 'silent',
      });
      expect(stub.request).not.toHaveBeenCalled();
    }
  });

  it('WebKit (гейт false) — молчание без вызова даже при default', async () => {
    const stub = makeStepDeps({ gestureless: false });
    await expect(runAutoPromptStep(stub.deps)).resolves.toStrictEqual({
      kind: 'silent',
    });
    expect(stub.request).not.toHaveBeenCalled();
  });

  it('VAPID-ключ ещё не пришёл — молчание без вызова (grant без подписки тратил бы разрешение)', async () => {
    const stub = makeStepDeps({ vapidKey: undefined });
    await expect(runAutoPromptStep(stub.deps)).resolves.toStrictEqual({
      kind: 'silent',
    });
    expect(stub.request).not.toHaveBeenCalled();
  });

  it('исход denied (отказ в окне, резолв default без окна) — молчание, не ошибка', async () => {
    const stub = makeStepDeps({ outcome: { outcome: 'denied' } });
    await expect(runAutoPromptStep(stub.deps)).resolves.toStrictEqual({
      kind: 'silent',
    });
  });

  it('ios-needs-install — молчание (граница спеки сохранена)', async () => {
    const stub = makeStepDeps({ outcome: { outcome: 'ios-needs-install' } });
    await expect(runAutoPromptStep(stub.deps)).resolves.toStrictEqual({
      kind: 'silent',
    });
  });

  it('структурная ошибка подписки — наружу с причиной (репортинг вызывающего)', async () => {
    const stub = makeStepDeps({
      outcome: { outcome: 'error', reason: 'subscribe-failed' },
    });
    await expect(runAutoPromptStep(stub.deps)).resolves.toStrictEqual({
      kind: 'error',
      reason: 'subscribe-failed',
    });
  });

  it('error no-vapid-key — молчание (не репортится, как и раньше)', async () => {
    const stub = makeStepDeps({
      outcome: { outcome: 'error', reason: 'no-vapid-key' },
    });
    await expect(runAutoPromptStep(stub.deps)).resolves.toStrictEqual({
      kind: 'silent',
    });
  });

  it('запрос реджектит (Firefox 72+ инстантный reject безжестового вызова) — тихий no-op: не бросает, не ошибка, не репортинг', async () => {
    const stub = makeStepDeps({
      throwInstead: new Error(
        'The Notification permission may only be requested from inside a short running user-generated event handler',
      ),
    });
    await expect(runAutoPromptStep(stub.deps)).resolves.toStrictEqual({
      kind: 'silent',
    });
    expect(stub.post).not.toHaveBeenCalled();
  });
});
