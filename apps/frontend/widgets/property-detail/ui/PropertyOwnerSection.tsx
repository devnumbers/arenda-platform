import type { JSX } from 'react';
import { BoldUser } from '@/shared/assets/icons';
import { CircleIcon } from '@/shared/ui/design';
import { PropertySectionCard } from './PropertySectionCard';

export type PropertyOwnerSectionProps = {
  readonly ownerName: string;
  /** Почта владельца (Figma 2200-97365 — контактный ряд владельца на
   * детали чужого объекта; owner_email в access-контракте). Пустая или
   * не пришедшая — сабтайтл не рисуется. */
  readonly ownerEmail?: string;
};

/**
 * Секция «Владелец объекта» (макеты 2235-100370, 2200-97365; тикет #703,
 * правка приёмки #757): на чужом объекте владелец назван статичным рядом
 * — под характеристиками, перед «Управлением». Ряд никуда не ведёт:
 * страница участника про выданные доступы, собственных ног у владельца
 * нет (решение #700). Анатомия по макету 2200-97365: белый круг 44 с
 * кольцом 2.5px в цвет карточки и тёмным Icon/Bold/User 24, имя R/500
 * 16/18 и почта M/400 14/16 серым сабтайтлом — сознательная экспозиция
 * этого экрана (прецедент suspended_shared.owner_email #702).
 */
export function PropertyOwnerSection({
  ownerName,
  ownerEmail,
}: PropertyOwnerSectionProps): JSX.Element {
  return (
    <PropertySectionCard title="Владелец объекта">
      <div className="flex items-center gap-3 px-6 pb-4 pt-4">
        <CircleIcon variant="muted" aria-hidden>
          <BoldUser className="h-6 w-6 text-content" />
        </CircleIcon>
        <span className="flex min-w-0 flex-col justify-center gap-1">
          <span className="truncate text-base font-medium leading-[18px] text-content">
            {ownerName}
          </span>
          {ownerEmail !== undefined && ownerEmail !== '' && (
            <span className="truncate text-sm leading-4 text-content-tertiary">{ownerEmail}</span>
          )}
        </span>
      </div>
    </PropertySectionCard>
  );
}
