import type { Property } from '@/entities/property';

/**
 * Имя владельца для ряда карточки объекта (решение владельца по итогам
 * обхода #756): ряд носят только ЧУЖИЕ объекты — карточка показывает, чей
 * это объект и кто выдал доступ. Свои объекты ряда не ведут (владелец —
 * сам читающий), как и строки без контекста доступа или без обогащения
 * (write-пути, stale-кэш).
 */
export function cardOwnerName(property: Property): string | null {
  if (property.access === undefined || property.access.role === 'owner') {
    return null;
  }
  return property.access.ownerName ?? null;
}
