import type { AccessRole } from '@/shared/model/access';
import type { Property } from '@/entities/property';

/** Пилюля шапки детали объекта: роль читающего (канон 2200-97365) или
 * «В архиве» (канон StatusHouseBadge 1603-92103, экран «Архивные объекты»). */
export type PropertyHeaderPill =
  | { readonly kind: 'access'; readonly role: AccessRole }
  | { readonly kind: 'archived' };

/**
 * Пилюли под заголовком детали (#773): архив — признак статуса объекта
 * для любого читателя (deep-link на чужой архивный открывается читаемым,
 * а списком его не увидеть — ленты фильтруют archived), роль — грант
 * чужого объекта. Архивная первая: состояние объекта важнее роли;
 * «Редактирование» у архивного full_access — правда о гранте, read-only
 * объясняет архивная пилюля и статусное пустое аренды (ADR 0028).
 * Владелец пилюли роли не получает (канон PropertyAccessPill).
 */
export function propertyHeaderPills(
  property: Pick<Property, 'status' | 'access'> | undefined,
): ReadonlyArray<PropertyHeaderPill> {
  if (property === undefined) {
    return [];
  }
  const pills: PropertyHeaderPill[] = [];
  if (property.status === 'archived') {
    pills.push({ kind: 'archived' });
  }
  const role = property.access?.role;
  if (role !== undefined && role !== 'owner') {
    pills.push({ kind: 'access', role });
  }
  return pills;
}
