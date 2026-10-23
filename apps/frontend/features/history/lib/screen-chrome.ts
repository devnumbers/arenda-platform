import { ROUTES } from '@/shared/config/routes';

/**
 * Личность экрана «История действий» одним вычислением (карта #704:
 * #840/#841 — прибитые скоупы; глобальная страница «Действия участника»
 * #712 снята, #1361): заголовок и фолбэк «Назад» зависят только от пинов
 * страницы и нужны в двух местах — живому экрану (history-feed-screen) и
 * фолбэку Suspense-границы с хромом (#1281, history-feed-loading). Одна
 * точка — чтобы хром фолбэка не разъезжался с хромом экрана.
 */

/** Заголовок экрана по скоупу страницы. Страница участника без объекта —
 * «Действия участника» (карта #1582): скоуп «человек × его текущие ноги»,
 * переработка решения 22.10.2026. */
export function historyScreenTitle(
  participantId?: string,
  propertyId?: string,
): string {
  return participantId !== undefined && propertyId !== undefined
    ? 'Действия участника в объекте'
    : propertyId !== undefined
      ? 'История объекта'
      : participantId !== undefined
        ? 'Действия участника'
        : 'История действий';
}

/** Фолбэк «Назад» по скоупу страницы (общий предок источника входа,
 * #843). Страница участника возвращается на неё саму (карта #1582). */
export function historyScreenBackHref(
  participantId?: string,
  propertyId?: string,
): string {
  return participantId !== undefined && propertyId !== undefined
    ? ROUTES.participantRights(participantId, propertyId)
    : propertyId !== undefined
      ? ROUTES.propertyParticipants(propertyId)
      : participantId !== undefined
        ? ROUTES.participant(participantId)
        : ROUTES.participants;
}
