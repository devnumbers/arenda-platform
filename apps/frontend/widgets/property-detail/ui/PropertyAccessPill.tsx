import type { JSX } from 'react';
import { ACCESS_ROLE_ICON_NAMES, ACCESS_ROLE_LABELS } from '@/entities/access';
import type { AccessRole } from '@/shared/model/access';
import { EditSmall, EyeSmall } from '@/shared/assets/icons';
import { LiveValue } from '@/shared/ui/live-value';
import { propertyPillClass } from './property-pill-class';

/** Имя иконки канона → компонент Icon/S (ассеты — забота слоя
 * отображения, словарь ролей их не импортирует). */
const ROLE_ICON_COMPONENTS = {
  edit: EditSmall,
  eye: EyeSmall,
} as const;

export type PropertyAccessPillProps = {
  readonly role: AccessRole;
  /** Роль в полёте перечитывания (кадр access realtime-стрима
   * инвалидирует семейства — #719): канон C гасит пилюлю до доставки. */
  readonly refreshing?: boolean;
};

/** Пилюля доступа под заголовком чужого объекта (Figma 2200-97365,
 * компонент StatusHouseBadge): роль читающего — «Редактирование» с
 * Icon/S/Edit либо «Просмотр» с Icon/S/Eye (канон participantLegBadge
 * #698). Заменяет снесённый баннер «С вами делится…» (приёмка #757);
 * владелец пилюлю не получает, suspended до детали не доходит
 * (лимит-экран). Отступ ряда пилюль держит обёртка страницы
 * (propertyHeaderPills #773). Смена роли чужой рукой оживает каноном C
 * (#880): dim → кроссфейд + вспышка — пилюля может смениться только
 * извне (свою роль читающему не поменять), поэтому анимация всегда
 * уместна. */
export function PropertyAccessPill({ role, refreshing = false }: PropertyAccessPillProps): JSX.Element | null {
  if (role === 'owner') {
    return null;
  }
  const Icon = ROLE_ICON_COMPONENTS[ACCESS_ROLE_ICON_NAMES[role]];
  return (
    <p className={propertyPillClass}>
      {/* Flex-раскладка внутри: корень LiveValue — inline-block (модуль),
       * дети оверлея снимаются целиком. */}
      <LiveValue valueKey={role} refreshing={refreshing}>
        <span className="flex items-center gap-1.5">
          <Icon aria-hidden className="h-4 w-4" />
          {ACCESS_ROLE_LABELS[role]}
        </span>
      </LiveValue>
    </p>
  );
}
