import { describe, expect, it } from 'vitest';
import type { PushDevicePreferences } from '@/features/push-notifications';
import type { RequestPushPermissionOutcome } from '@/features/push-notifications';
import {
  applyPushChange,
  resolvePushDisplay,
  verdictFromOutcome,
  type PushChange,
} from './notification-settings';

const ALL_ON: PushDevicePreferences = {
  enabled: true,
  categories: { rental: true, payments_operations: true, tasks: true, shared_access: true },
};

const settledInput = {
  probeSettled: true,
  supported: true,
  permissionGranted: true,
  endpoint: 'https://push.example/endpoint-1' as string | null,
  server: ALL_ON,
  local: null,
};

describe('resolvePushDisplay — источник состояния', () => {
  it('с подпиской состояние приходит с сервера', () => {
    const display = resolvePushDisplay({
      ...settledInput,
      server: { enabled: false, categories: { ...ALL_ON.categories, rental: false } },
    });
    expect(display.masterOn).toBe(false);
    expect(display.categories.rental).toBe(false);
    // пока GET в воздухе — дефолт «всё включено»
    expect(
      resolvePushDisplay({ ...settledInput, server: undefined }).masterOn,
    ).toBe(true);
  });

  it('без подписки состояние локальное с дефолтом «всё включено» (решение #738)', () => {
    const display = resolvePushDisplay({ ...settledInput, endpoint: null, local: null });
    expect(display.masterOn).toBe(true);
    expect(display.categories).toStrictEqual(ALL_ON.categories);

    const turnedOff = resolvePushDisplay({
      ...settledInput,
      endpoint: null,
      local: { enabled: false, categories: { ...ALL_ON.categories, tasks: false } },
    });
    expect(turnedOff.masterOn).toBe(false);
    expect(turnedOff.categories.tasks).toBe(false);
  });

  it('браузер без пуша — колонка не рендерится вовсе', () => {
    const display = resolvePushDisplay({ ...settledInput, supported: false });
    expect(display.visible).toBe(false);
  });
});

describe('resolvePushDisplay — мастер и разрешения', () => {
  it('мастер выключен — категорийные тумблеры затемнены (2333-180696), значения хранятся', () => {
    const display = resolvePushDisplay({
      ...settledInput,
      server: { enabled: false, categories: { ...ALL_ON.categories, rental: true } },
    });
    expect(display.categoriesInteractive).toBe(false);
    expect(display.categories.rental).toBe(true);
  });

  it('карточка «Разрешите пуши» — мастер включён и разрешение не выдано (2329-150165)', () => {
    expect(
      resolvePushDisplay({ ...settledInput, permissionGranted: false }).needsPermission,
    ).toBe(true);
    // мастер выключен — карточка не нужна
    expect(
      resolvePushDisplay({
        ...settledInput,
        permissionGranted: false,
        server: { ...ALL_ON, enabled: false },
      }).needsPermission,
    ).toBe(false);
    // проба браузера не осела — состояние неясно, карточку не показываем
    expect(
      resolvePushDisplay({ ...settledInput, permissionGranted: false, probeSettled: false })
        .needsPermission,
    ).toBe(false);
  });
});

describe('applyPushChange', () => {
  it('двигает мастер, не трогая категории', () => {
    const next = applyPushChange(ALL_ON, { kind: 'master', value: false });
    expect(next.enabled).toBe(false);
    expect(next.categories).toStrictEqual(ALL_ON.categories);
  });

  it('двигает одну категорию', () => {
    const next = applyPushChange(ALL_ON, {
      kind: 'category',
      category: 'tasks',
      value: false,
    });
    expect(next.enabled).toBe(true);
    expect(next.categories.tasks).toBe(false);
    expect(next.categories.rental).toBe(true);
  });
});

describe('verdictFromOutcome — флоу «Разрешите пуши» (#746)', () => {
  const subscription = { endpoint: 'https://push.example/endpoint-9' } as PushSubscription;

  it('подписка создана/уже была — применяем желаемое состояние на её endpoint', () => {
    for (const outcome of [
      { outcome: 'subscribed', subscription },
      { outcome: 'already-subscribed', subscription },
    ] as RequestPushPermissionOutcome[]) {
      expect(verdictFromOutcome(outcome)).toStrictEqual({
        kind: 'subscribe-success',
        endpoint: 'https://push.example/endpoint-9',
      });
    }
  });

  it('отказ, iOS-установка и ошибка — свои вердикты, ничего не применяем', () => {
    expect(verdictFromOutcome({ outcome: 'denied' })).toStrictEqual({ kind: 'denied' });
    expect(verdictFromOutcome({ outcome: 'ios-needs-install' })).toStrictEqual({
      kind: 'ios-needs-install',
    });
    expect(
      verdictFromOutcome({ outcome: 'error', reason: 'network-error' }),
    ).toStrictEqual({ kind: 'flow-error' });
    expect(verdictFromOutcome({ outcome: 'unsupported' })).toStrictEqual({ kind: 'noop' });
  });
});

describe('PushChange — тип изменений замкнут по каталогу настроек', () => {
  it('категория изменения — из каталога четырёх', () => {
    const change: PushChange = { kind: 'category', category: 'shared_access', value: true };
    expect(change.category).toBe('shared_access');
  });
});
