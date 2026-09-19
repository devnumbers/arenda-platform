import type { JSX } from 'react';
import type { AccessRole } from '@/shared/model/access';
import { EditSmall, EyeSmall } from '@/shared/assets/icons';

export type PropertyAccessPillProps = {
  readonly role: AccessRole;
};

/** Пилюля доступа под заголовком чужого объекта (Figma 2200-97365,
 * компонент StatusHouseBadge): роль читающего — «Редактирование» с
 * Icon/S/Edit либо «Просмотр» с Icon/S/Eye (канон participantLegBadge
 * #698). Заменяет снесённый баннер «С вами делится…» (приёмка #757);
 * владелец пилюля не получает, suspended до детали не доходит
 * (лимит-экран). */
export function PropertyAccessPill({ role }: PropertyAccessPillProps): JSX.Element | null {
  if (role === 'owner') {
    return null;
  }
  const Icon = role === 'full_access' ? EditSmall : EyeSmall;
  return (
    <p className="m-0 mt-3 inline-flex items-center gap-1.5 rounded-pill bg-surface-muted py-2 pl-3 pr-4 text-sm font-medium leading-4 text-content">
      <Icon aria-hidden className="h-4 w-4" />
      {role === 'full_access' ? 'Редактирование' : 'Просмотр'}
    </p>
  );
}
