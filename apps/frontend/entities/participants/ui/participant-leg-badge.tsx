import type { JSX } from 'react';
import { EditSmall, EyeSmall, LockSmall } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';
import type { ParticipantLegBadge } from '../lib/participant-legs';

function LegBadgeIcon({ icon }: { readonly icon: NonNullable<ParticipantLegBadge['icon']> }): JSX.Element {
  switch (icon) {
    case 'edit':
      return <EditSmall className="h-4 w-4" aria-hidden />;
    case 'eye':
      return <EyeSmall className="h-4 w-4" aria-hidden />;
    case 'lock':
      return <LockSmall className="h-4 w-4" aria-hidden />;
  }
}

/**
 * Чип ноги доступа (страница участника #698) — та же анатомия «Row Button
 * Bage», что у агрегат-чипа ParticipantStatusBadge (#697): пилюля radius 8,
 * паддинг 4×8, зазор 4, текст 14/16; warning — жёлтая подложка
 * (--color-warning-bg). Иконки — контурный S-стиль 16×16 currentColor.
 */
export function ParticipantLegBadge({ badge }: { readonly badge: ParticipantLegBadge }): JSX.Element {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-lg px-2 py-1 text-sm leading-4 text-content',
        badge.tone === 'warning' ? 'bg-warning-bg' : 'bg-surface-muted',
      )}
    >
      {badge.icon !== undefined && (
        <span className="flex h-4 w-4 shrink-0 items-center justify-center">
          <LegBadgeIcon icon={badge.icon} />
        </span>
      )}
      {badge.label}
    </span>
  );
}
