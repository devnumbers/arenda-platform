import type { PropertyPermissions } from '@/entities/property';

/** Состояние кнопки «Добавить» пустой секции детали объекта. */
export type PropertySectionCtaState = {
  /** Кнопка рисуется (onCta определён). */
  readonly visible: boolean;
  /** Кнопка глухая: объект в архиве — read-only (ADR 0028, канон #589). */
  readonly disabled: boolean;
};

/**
 * Create-CTA пустых секций детали (#774, решение владельца Q13=А «резать
 * консистентно»): зритель кнопки не видит вовсе — объектные формы ему
 * отвечают 403, а кнопка, ведущая в отказ, мёртвая (прецедент #757,
 * гарды #758; канон «+» операций #703). Владелец и редактор пользуются
 * кнопками как раньше. Архив не прячает кнопку, а глушит: у владельца
 * и редактора архивного объекта пустые секции несут глухие «Добавить»
 * (канон #589, #773) — поэтому «кто вообще мог бы создавать» и «можно
 * ли прямо сейчас» — два разных флага центральных прав (#703):
 * видимость по canManageMembers (роль сильнее зрителя), глухота по
 * !canEdit (архив). Пока объект не загружен, прав нет — CTA не рисуется
 * (нет вспышки кнопки на скелетоне).
 */
export function propertySectionCta(
  permissions: PropertyPermissions,
): PropertySectionCtaState {
  return {
    visible: permissions.canManageMembers,
    disabled: !permissions.canEdit,
  };
}
