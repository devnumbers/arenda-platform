import { describe, expect, it } from 'vitest';
import type { PushDevicePreferences } from '@/features/push-notifications';
import type { RequestPushPermissionOutcome } from '@/features/push-notifications';
import {
  applyPushChange,
  resolvePushDisplay,
  resolvePushState,
  verdictFromOutcome,
  type PushChange,
} from './notification-settings';

const ALL_ON: PushDevicePreferences = {
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
  it('с подпиской категории приходят с сервера, мастер — факт подписки', () => {
    const display = resolvePushDisplay({
      ...settledInput,
      server: { categories: { ...ALL_ON.categories, rental: false } },
    });
    expect(display.masterOn).toBe(true);
    expect(display.categories.rental).toBe(false);
    // пока GET в воздухе — дефолт «всё включено»
    expect(
      resolvePushDisplay({ ...settledInput, server: undefined }).categories,
    ).toStrictEqual(ALL_ON.categories);
  });

  it('без подписки мастер выключен (строки нет = выключено, спека #1028 §0)', () => {
    const display = resolvePushDisplay({ ...settledInput, endpoint: null, local: null });
    expect(display.masterOn).toBe(false);
    expect(display.categories).toStrictEqual(ALL_ON.categories);

    // локальные выключатели хранятся до первой подписки, мастер они не включают
    const turnedOffCategory = resolvePushDisplay({
      ...settledInput,
      endpoint: null,
      local: { categories: { ...ALL_ON.categories, tasks: false } },
    });
    expect(turnedOffCategory.masterOn).toBe(false);
    expect(turnedOffCategory.categories.tasks).toBe(false);
  });

  it('браузер без пуша — колонка не рендерится вовсе', () => {
    const display = resolvePushDisplay({ ...settledInput, supported: false });
    expect(display.visible).toBe(false);
  });
});

describe('resolvePushState — источник категорий', () => {
  it('с подпиской — сервер, без — локальное состояние, дефолт один', () => {
    const server: PushDevicePreferences = {
      categories: { rental: false, payments_operations: true, tasks: true, shared_access: true },
    };
    const local: PushDevicePreferences = {
      categories: { rental: true, payments_operations: false, tasks: true, shared_access: true },
    };
    expect(resolvePushState('https://push.example/e1', server, local)).toBe(server);
    expect(resolvePushState(null, server, local)).toBe(local);
    expect(resolvePushState(null, undefined, null)).toStrictEqual(ALL_ON);
  });
});

describe('resolvePushDisplay — мастер и разрешения', () => {
  it('карточка «Разрешите пуши» — подписки нет и разрешение не выдано (2329-150165)', () => {
    expect(
      resolvePushDisplay({ ...settledInput, permissionGranted: false, endpoint: null })
        .needsPermission,
    ).toBe(true);
    // подписка есть — включать нечего, карточка не нужна
    expect(
      resolvePushDisplay({ ...settledInput, permissionGranted: false }).needsPermission,
    ).toBe(false);
    // проба браузера не осела — состояние неясно, карточку не показываем
    expect(
      resolvePushDisplay({
        ...settledInput,
        permissionGranted: false,
        endpoint: null,
        probeSettled: false,
      }).needsPermission,
    ).toBe(false);
  });
});

describe('applyPushChange', () => {
  it('мастер в тело PUT не выражается — состояние без изменений', () => {
    expect(applyPushChange(ALL_ON, { kind: 'master', value: false })).toBe(ALL_ON);
    expect(applyPushChange(ALL_ON, { kind: 'master', value: true })).toBe(ALL_ON);
  });

  it('двигает одну категорию', () => {
    const next = applyPushChange(ALL_ON, {
      kind: 'category',
      category: 'tasks',
      value: false,
    });
    expect(next.categories.tasks).toBe(false);
    expect(next.categories.rental).toBe(true);
    expect(ALL_ON.categories.tasks).toBe(true);
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
