'use client';

import type { JSX } from 'react';
import { BoldUser, SmallArrowRight } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';
import { useKeyboardActivation } from '@/shared/lib/hooks/useKeyboardActivation';
import type { Participant } from '../model/types';
import {
  participantRowSubtitle,
  participantRowTitle,
  participantStatusBadge,
} from '../lib/participant-display';
import { ParticipantStatusBadge } from './participant-status-badge';

/**
 * Строка участника списка «Ваши участники» (#697; Figma 2036-82971,
 * компонент Row Button 936:39348 с бейджем): аватар-круг 44 с BoldUser,
 * титул (имя или почта pending), подзаголовок-почта 14/16, под ним чип
 * агрегат-статуса; ведомый шеврон. Паддинг строки 12px 0 (макет), тап —
 * страница участника (#698). Каноническая строка срезов сущностей
 * (DESIGN.md §6, семейство ContactRowButton): живёт в entities, нужна
 * будет и живым экранам #719.
 */
export function ParticipantRowButton({
  participant,
  onSelect,
  className,
}: {
  readonly participant: Participant;
  readonly onSelect?: () => void;
  readonly className?: string;
}): JSX.Element {
  const activatorProps = useKeyboardActivation({ onSelect });
  const subtitle = participantRowSubtitle(participant);
  const badge = participantStatusBadge(participant);

  return (
    <div
      {...activatorProps}
      className={cn(
        'group/row flex w-full cursor-pointer items-center gap-3 py-3 text-left outline-none',
        'transition-opacity focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-white',
        'hover:opacity-80 active:opacity-80',
        onSelect === undefined && 'cursor-default',
        className,
      )}
    >
      <span
        aria-hidden
        className="flex h-11 w-11 shrink-0 items-center justify-center rounded-pill bg-surface-muted shadow-[0_0_0_2.5px_var(--dl-surface)]"
      >
        <BoldUser className="h-6 w-6" />
      </span>
      <span className="flex min-w-0 flex-1 flex-col justify-center gap-1">
        <span className="truncate text-base font-medium text-content">
          {participantRowTitle(participant)}
        </span>
        {subtitle !== undefined && (
          <span className="truncate text-sm text-content-secondary">{subtitle}</span>
        )}
        <span>
          <ParticipantStatusBadge badge={badge} />
        </span>
      </span>
      <SmallArrowRight className="h-6 w-6 shrink-0 text-content-tertiary" aria-hidden />
    </div>
  );
}
