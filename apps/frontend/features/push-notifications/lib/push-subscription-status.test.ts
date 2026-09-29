import { describe, expect, it } from 'vitest';
import {
  PENDING_PUSH_STATUS,
  UNSUPPORTED_PUSH_STATUS,
  resolveInitialPushStatus,
} from './push-subscription-status';

describe('resolveInitialPushStatus', () => {
  it('браузер без Web Push — unsupported, решение осело в первом кадре', () => {
    expect(resolveInitialPushStatus({ supported: false, permission: 'default' })).toBe(
      UNSUPPORTED_PUSH_STATUS,
    );
  });

  it('поддержка есть, разрешение не выдано — карточка «Разрешите пуши» ждёт с первого кадра', () => {
    const status = resolveInitialPushStatus({ supported: true, permission: 'default' });
    expect(status.isPending).toBe(false);
    expect(status.isUnsupported).toBe(false);
    expect(status.needsPermission).toBe(true);
    expect(status.permissionDenied).toBe(false);
    expect(status.endpoint).toBeNull();
  });

  it('явный отказ — та же ветка с флагом denied', () => {
    const status = resolveInitialPushStatus({ supported: true, permission: 'denied' });
    expect(status.needsPermission).toBe(true);
    expect(status.permissionDenied).toBe(true);
  });

  it('разрешение выдано — остаётся pending до асинхронной проверки подписки', () => {
    expect(resolveInitialPushStatus({ supported: true, permission: 'granted' })).toBe(
      PENDING_PUSH_STATUS,
    );
  });

  it('исходы — синглтоны: getSnapshot сравнивается через Object.is', () => {
    expect(resolveInitialPushStatus({ supported: true, permission: 'default' })).toBe(
      resolveInitialPushStatus({ supported: true, permission: 'default' }),
    );
    expect(resolveInitialPushStatus({ supported: true, permission: 'denied' })).toBe(
      resolveInitialPushStatus({ supported: true, permission: 'denied' }),
    );
    expect(resolveInitialPushStatus({ supported: false, permission: 'granted' })).toBe(
      resolveInitialPushStatus({ supported: false, permission: 'unsupported' }),
    );
  });
});
