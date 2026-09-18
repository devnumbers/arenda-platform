import type { JSX } from 'react';
import { BoldUser } from '@/shared/assets/icons';
import { PropertySectionCard } from './PropertySectionCard';

/**
 * Секция «Владелец объекта» (макет 2235-100370, тикет #703): на чужом
 * объекте владелец назван статичным рядом с аватаром — под
 * характеристиками, перед «Управлением». Ряд никуда не ведёт: страница
 * участника про выданные доступы, собственных ног у владельца нет
 * (решение #700). Почты владельца в контракте доступа нет сознательно
 * (privacy-канон «never email», спека PropertyAccessContext) — в макете
 * адрес-плейсхолдер, вопрос приёмки.
 */
export function PropertyOwnerSection({
  ownerName,
}: {
  readonly ownerName: string;
}): JSX.Element {
  return (
    <PropertySectionCard title="Владелец объекта">
      <div className="flex items-center gap-3 px-6 pb-4 pt-4">
        <span
          aria-hidden
          className="flex h-11 w-11 shrink-0 items-center justify-center rounded-pill bg-surface-muted shadow-[0_0_0_2.5px_var(--dl-surface)]"
        >
          <BoldUser className="h-6 w-6 text-content-tertiary" />
        </span>
        <span className="min-w-0 truncate text-base font-medium leading-[18px] text-content">
          {ownerName}
        </span>
      </div>
    </PropertySectionCard>
  );
}
