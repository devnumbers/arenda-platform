import type { JSX } from 'react';
import type { AccessRole } from '@/shared/model/access';
import { EditSmall, EyeSmall } from '@/shared/assets/icons';
import { propertyPillClass } from './property-pill-class';

export type PropertyAccessPillProps = {
  readonly role: AccessRole;
};

/** Пилюля доступа под заголовком чужого объекта (Figma 2200-97365,
 * компонент StatusHouseBadge): роль читающего — «Редактирование» с
 * Icon/S/Edit либо «Просмотр» с Icon/S/Eye (канон participantLegBadge
 * #698). Заменяет снесённый баннер «С вами делится…» (приёмка #757);
 * владелец пилюля не получает, suspended до детали не доходит
 * (лимит-экран). Отступ ряда пилюль держит обёртка страницы
 * (propertyHeaderPills #773). */
export function PropertyAccessPill({ role }: PropertyAccessPillProps): JSX.Element | null {
  if (role === 'owner') {
    return null;
  }
  const Icon = role === 'full_access' ? EditSmall : EyeSmall;
  return (
    <p className={propertyPillClass}>
      <Icon aria-hidden className="h-4 w-4" />
      {role === 'full_access' ? 'Редактирование' : 'Просмотр'}
    </p>
  );
}
