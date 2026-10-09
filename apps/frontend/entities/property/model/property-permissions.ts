import type { Property } from './types';

/** Центральные права читателя на объект (карта #692, тикет #703):
 * единый селектор из `property.access` (роль + статус) вместо точечных
 * проверок. Частичные флаги комбинируются на экранах; сами решают, что
 * скрывать — кнопки создания/правки, «Совместный доступ», «Покинуть». */
export type PropertyPermissions = {
  /** Мутации контента (создание/правка/удаление платежей, операций, задач,
   * контактов, аренд): роль сильнее зрителя и объект не в архиве
   * (ADR 0028, финансовая история #446). Наследник canMutateProperty
   * (#703) — тот снесён. Create-CTA пустых секций детали на одном этом
   * флаге не держится: видимость — canMutate (архив глушит, а не прячет,
   * канон #589/#773), глухота — здесь (#774, шов propertySectionCta;
   * D8 #1304 отвязал CTA от canManageMembers — тот стал owner-only). */
  readonly canEdit: boolean;
  /** Ролевой мутационный доступ: роль сильнее зрителя, архив не гасит —
   * билдеры «Управления»/кебаба ветвят архив сами (#703). Наследник
   * прежнего смысла canManageMembers: до D8 тот был «роль сильнее
   * зрителя», пока не стал зеркалом owner-only политики участников. */
  readonly canMutate: boolean;
  /** Совместный доступ: только собственник (зеркало owner-only политики
   * участников, D8 #1304) — приглашение, меню, смена роли, отзыв. Архив
   * не гасит: отзыв на архивном объекте разрешён (#700). */
  readonly canManageMembers: boolean;
  /** Участник чужого объекта — «Покинуть объект» (не для владельца).
   * Архив не гасит: выйти можно и из архивного (канон выхода #701). */
  readonly canLeave: boolean;
  /** Жизненный цикл объекта — архив/возврат/удаление (зеркало
   * sharedpolicy.CanLifecycle): только владелец. Гейтит мёртвые кнопки
   * у участника («Перевести в архив», «Удалить объект» — сервер отвечает
   * 403, приёмка #757: кнопок, которые нельзя сделать, быть не должно). */
  readonly canLifecycle: boolean;
};

const DENIED: PropertyPermissions = {
  canEdit: false,
  canMutate: false,
  canManageMembers: false,
  canLeave: false,
  canLifecycle: false,
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
  if (property === undefined) {
    return DENIED;
  }
  const role = property.access?.role;
  if (role === undefined) {
    return DENIED;
  }
  return {
    canEdit: role !== 'viewer' && property.status !== 'archived',
    canMutate: role !== 'viewer',
    canManageMembers: role === 'owner',
    canLeave: role !== 'owner',
    canLifecycle: role === 'owner',
  };
}

/** Строки справочника с правом правки — пикеры привязки объекта
 * (задача/контакт/операция): привязка к зрительскому объекту упёрлась бы
 * в отказ сервера, архив гасится тем же флагом. */
export function filterEditableProperties<T extends Pick<Property, 'access' | 'status'>>(
  properties: ReadonlyArray<T>,
): ReadonlyArray<T> {
  return properties.filter((property) => propertyPermissions(property).canEdit);
}
