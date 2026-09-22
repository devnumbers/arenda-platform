import type { JSX } from 'react';
import { EditSmall, EyeSmall, LockSmall } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';
import type { ParticipantRowBadge, ParticipantRowBadgeIcon } from '../lib/participant-badge';

function RowBadgeIcon({ icon }: { readonly icon: ParticipantRowBadgeIcon }): JSX.Element {
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
 * Чип строки участника — общая анатомия «Row Button Bage» (Figma
 * 2036:83107): пилюля radius 8, паддинг 4×8, зазор 4, текст 14/16
 * Regular; warning — жёлтая подложка (канон --color-warning-bg; макет
 * несёт близкий #FFB900@10% — §12/§13: токен сильнее цвета макета).
 * Рендерит и агрегат-чип строки (#697), и чип ноги доступа (#698):
 * различалась только иконка. Иконки — контурный S-стиль 16×16
 * currentColor.
 */
export function ParticipantRowBadge({ badge }: { readonly badge: ParticipantRowBadge }): JSX.Element {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-lg px-2 py-1 text-sm leading-4 text-content',
        badge.tone === 'warning' ? 'bg-warning-bg' : 'bg-surface-muted',
      )}
    >
      {badge.icon !== undefined && (
        <span className="flex h-4 w-4 shrink-0 items-center justify-center">
          <RowBadgeIcon icon={badge.icon} />
        </span>
      )}
      {badge.label}
    </span>
  );
}
