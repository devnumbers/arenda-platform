import { ROUTES } from '@/shared/config/routes';
import type { Property } from '@/entities/property';

/** Куда и как называется пункт «Объекты» единого хрома: подпись + адрес.
 * Подпись живёт рядом с адресом, потому что обе решаются одним правилом. */
export type PropertiesNavItem = {
  readonly label: string;
  readonly href: string;
};

/**
 * Пункт «Объекты» хрома (карта #984, решение владельца 30.09): ведение на
 * закреплённый объект снесено. Единственный прямой переход — у базового
 * тарифа: ровно один живой объект в списке, он свой и архив пуст — пункт
 * называется «Объект» и ведёт сразу на его страницу. Все остальные случаи
 * (платные тарифы всегда, у basic — шаринг, архив, пустая книга, аномалии)
 * — «Объекты» на список. Пока списки не загружены — список (безопасный
 * фолбэк).
 *
 * Данные: главный список несёт активные и «в ремонте» (свои + чужие),
 * архив — отдельная ручка; считаются оба. Чужой опознаётся по
 * access.role, свой — role 'owner'; без access — «не свой» (safe).
 */
export function resolvePropertiesNavItem(
  tariffName: string | null | undefined,
  properties: readonly Property[] | undefined,
  archivedProperties: readonly Property[] | undefined,
): PropertiesNavItem {
  const objectsItem: PropertiesNavItem = { label: 'Объекты', href: ROUTES.properties };

  if (tariffName !== 'basic') return objectsItem;
  if (properties === undefined || archivedProperties === undefined) return objectsItem;
  if (properties.length !== 1 || archivedProperties.length !== 0) return objectsItem;

  const [only] = properties;
  if (only === undefined) return objectsItem;
  if (only.status === 'archived') return objectsItem;
  if (only.access?.role !== 'owner') return objectsItem;

  return { label: 'Объект', href: ROUTES.property(only.id) };
}
