import { afterEach, describe, expect, it, vi } from 'vitest';

import { deviceTimezone } from './device-timezone';

const stubResolvedOptions = (timeZone: string | undefined) => {
  vi.spyOn(Intl, 'DateTimeFormat').mockImplementation(
    () =>
      ({
        resolvedOptions: () => ({ timeZone }),
      }) as unknown as Intl.DateTimeFormat,
  );
};

afterEach(() => {
  vi.restoreAllMocks();
});

describe('deviceTimezone', () => {
  it('возвращает зону устройства', () => {
    stubResolvedOptions('Asia/Yekaterinburg');
    expect(deviceTimezone()).toBe('Asia/Yekaterinburg');
  });

  it('undefined, когда браузер не отдал зону, — поле не отправляется', () => {
    stubResolvedOptions(undefined);
    expect(deviceTimezone()).toBeUndefined();
  });

  it('undefined для пустой строки', () => {
    stubResolvedOptions('');
    expect(deviceTimezone()).toBeUndefined();
  });
});
