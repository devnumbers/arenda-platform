import { ROUTES } from '@/shared/config/routes';

/**
 * Личность экрана «История действий» одним вычислением (карта #704:
 * #712/#840/#841 — прибитые скоупы): заголовок и фолбэк «Назад» зависят
 * только от пинов страницы и нужны в двух местах — живому экрану
 * (history-feed-screen) и фолбэку Suspense-границы с хромом (#1281,
 * history-feed-loading). Одна точка — чтобы хром фолбэка не разъезжался
 * с хромом экрана.
 */

/** Заголовок экрана по скоупу страницы. */
export function historyScreenTitle(
  participantId?: string,
  propertyId?: string,
): string {
  const isMemberPage = participantId !== undefined;
  const isPropertyPage = propertyId !== undefined;
  return isMemberPage && isPropertyPage
    ? 'Действия участника в объекте'
    : isMemberPage
      ? 'Действия участника'
      : isPropertyPage
        ? 'История объекта'
        : 'История действий';
}

/** Фолбэк «Назад» по скоупу страницы (общий предок источника входа,
 * #843). */
export function historyScreenBackHref(
  participantId?: string,
  propertyId?: string,
): string {
  const isMemberPage = participantId !== undefined;
  const isPropertyPage = propertyId !== undefined;
  return isMemberPage && isPropertyPage
    ? ROUTES.participantRights(participantId, propertyId)
    : isMemberPage
      ? ROUTES.history
      : isPropertyPage
        ? ROUTES.propertyParticipants(propertyId)
        : ROUTES.participants;
}
