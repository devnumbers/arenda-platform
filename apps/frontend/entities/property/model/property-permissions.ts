import type { Property } from './types';

/** Центральные права читателя на объект (карта #692, тикет #703):
 * единый селектор из `property.access` (роль + статус) вместо точечных
 * проверок. Частичные флаги комбинируются на экранах; сами решают, что
 * скрывать — кнопки создания/правки, «Совместный доступ», «Покинуть». */
export type PropertyPermissions = {
  /** Мутации контента (создание/правка/удаление платежей, операций, задач,
   * контактов, аренд; CTA пустых секций): роль сильнее зрителя и объект
   * не в архиве (ADR 0028, финансовая история #446). Наследник
   * canMutateProperty (#703) — тот снесён. */
  readonly canEdit: boolean;
  /** Совместный доступ: владелец или полный доступ — «Совместный доступ»
   * и билдеры кебаба/«Управления» (их ролевой `canMutate` — тот же
   * предикат, архив билдер ветвит сам). Архив не гасит: отзыв на
   * архивном объекте разрешён (#700). */
  readonly canManageMembers: boolean;
  /** Участник чужого объекта — «Покинуть объект» (не для владельца).
   * Архив не гасит: выйти можно и из архивного (канон выхода #701). */
  readonly canLeave: boolean;
};

const DENIED: PropertyPermissions = {
  canEdit: false,
  canManageMembers: false,
  canLeave: false,
};

/**
 * Пока объект не загружен или не загрузился — прав нет: принимает уже
 * развёрнутое значение (экраны передают `propertyQuery.isSuccess ?
 * propertyQuery.data : undefined`); объект без контекста доступа —
 * страховка «только чтение» (сервер всё равно ответил бы отказом).
 * Нужны только роль и статус — ленты передают строку справочника
 * объектов целиком, срезы — `Pick` из него.
 */
export function propertyPermissions(
  property: Pick<Property, 'access' | 'status'> | undefined,
): PropertyPermissions {
  const role = property?.access?.role;
  if (property === undefined || role === undefined) {
    return DENIED;
  }
  return {
    canEdit: role !== 'viewer' && property.status !== 'archived',
    canManageMembers: role !== 'viewer',
    canLeave: role !== 'owner',
  };
}
