/**
 * Тайминг активации ожидающего service worker при смене маршрута (#771,
 * правка контракта ADR 0032). Вынесено в чистую функцию: vitest-окружение
 * фронтенда — node без DOM, поведенческий тест компонента невозможен.
 */

/**
 * Тихое окно после popstate-перехода (goBack, кнопка/жест «назад»), в
 * течение которого смена маршрута не активирует ожидающий SW. popstate
 * стреляет до обработки навигации роутером, поэтому timestamp всегда
 * успевает встать до эффекта usePathname; окно с запасом покрывает
 * коммит перехода. Случайное «перепроглатывание» окна (push в течение
 * секунды после «назад») безвредно — активация уедет на следующий push
 * или на скрытие приложения.
 */
export const POPSTATE_QUIET_MS = 1_000;

/**
 * Активировать ли ожидающий SW на очередной смене маршрута: ADR 0032
 * активирует по смене маршрута, НО popstate-навигация несёт one-shot
 * состояние перехода (staged-флаг попапа «Участник приглашен» и
 * одноклассники) — мгновенный reload апдейтера на экране-приёмнике стирает
 * его до показа. Поэтому после popstate активация откладывается до
 * следующего push-перехода (или до скрытия приложения — тот триггер не
 * здесь и не тронут).
 */
export function shouldActivateWaitingOnRouteChange(input: {
  /** Есть ли ожидающий (waiting) worker, готовый к SKIP_WAITING. */
  readonly pendingSkipWaiting: boolean;
  /** Date.now() последнего popstate-перехода; 0 — не наблюдался. */
  readonly lastPopNavigateAt: number;
  readonly now: number;
}): boolean {
  if (!input.pendingSkipWaiting) {
    return false;
  }
  return input.now - input.lastPopNavigateAt >= POPSTATE_QUIET_MS;
}
