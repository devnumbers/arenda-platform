/**
 * Резолвер ссылок сегментов (тикет #713, ADR 0061 §7): отдельного
 * эндпоинта деталей нет — синий фрагмент ведёт на существующую страницу
 * сущности; удаляемые действия сервер пишет без ссылки (снапшот названия
 * остаётся), а сущность, удалённую позже, встретит 404 целевой страницы.
 * Неизвестный вид — null: фрагмент рендерится текстом.
 */

import { ROUTES } from '@/shared/config/routes';

import type { HistorySegmentLink } from './types';

export function historySegmentHref(
  link: HistorySegmentLink,
  propertyId: string,
): string | null {
  switch (link.kind) {
    case 'property':
      return ROUTES.property(link.id);
    // Страница аренды по id сама уводит незавершённую аренду на текущий экран.
    case 'rental':
      return ROUTES.propertyRentalCompleted(propertyId, link.id);
    case 'payment':
      return ROUTES.propertyPayment(propertyId, link.id);
    case 'operation':
      return ROUTES.propertyOperation(propertyId, link.id);
    case 'contact':
      return ROUTES.propertyContact(propertyId, link.id);
    // Страница у вхождения задачи одна — правка правила (канон списков задач).
    case 'task':
      return ROUTES.propertyTaskEdit(propertyId, link.id);
    case 'member':
      return ROUTES.participant(link.id);
    default:
      return null;
  }
}
