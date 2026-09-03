import type { Property } from '@/entities/property';

/**
 * Мутационный доступ к объекту (ADR 0028): смотрящий читает без кнопок
 * (история 47), архив read-only (финансовая история #446). Пока объект
 * не загружен или не загрузился — мутаций нет: helper принимает уже
 * развёрнутое значение, экраны передают `propertyQuery.isSuccess ?
 * propertyQuery.data : undefined`.
 *
 * Общий предикат кнопок правки/удаления/создания: платежи (#446, история
 * 47) и контакты (#508–#510). Четвёртая копия выражения — повод извлечь.
 */
export function canMutateProperty(property: Property | undefined): boolean {
  if (property === undefined) return false;
  const role = property.access?.role;
  return role !== undefined && role !== 'viewer' && property.status !== 'archived';
}
