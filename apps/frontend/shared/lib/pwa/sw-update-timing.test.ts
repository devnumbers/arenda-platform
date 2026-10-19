import { describe, expect, it } from 'vitest';

import { POPSTATE_QUIET_MS, shouldActivateWaitingOnRouteChange } from './sw-update-timing';

// Тайминг активации ожидающего service worker при смене маршрута (#771).
// goBack/кнопка «назад» — это popstate-навигация, коммитящая переход на
// экране-приёмнике; мгновенный reload апдейтера разрывает переход посреди
// кадра (исторический потребитель — one-shot staged-флаг успеха, снесён
// заменой попапов на тосты 19.10.2026). Активация откладывается до
// следующего push-перехода или скрытия приложения.

describe('shouldActivateWaitingOnRouteChange', () => {
  const now = 1_000_000;

  it('без ожидающего SW активировать нечего — независимо от popstate', () => {
    expect(
      shouldActivateWaitingOnRouteChange({
        pendingSkipWaiting: false,
        lastPopNavigateAt: now - 1,
        now,
      }),
    ).toBe(false);
  });

  it('push-переход (popstate не стрелял) активирует ожидающий SW — триггер ADR 0032', () => {
    expect(
      shouldActivateWaitingOnRouteChange({
        pendingSkipWaiting: true,
        lastPopNavigateAt: 0,
        now,
      }),
    ).toBe(true);
  });

  it('смена маршрута сразу после popstate (goBack) активацию откладывает', () => {
    expect(
      shouldActivateWaitingOnRouteChange({
        pendingSkipWaiting: true,
        lastPopNavigateAt: now - 10,
        now,
      }),
    ).toBe(false);
  });

  it('после тихого окна с момента popstate активация снова разрешена', () => {
    expect(
      shouldActivateWaitingOnRouteChange({
        pendingSkipWaiting: true,
        lastPopNavigateAt: now - POPSTATE_QUIET_MS,
        now,
      }),
    ).toBe(true);
  });
});
