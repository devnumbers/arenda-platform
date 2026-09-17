import type { JSX } from 'react';
import { LockSmall } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';
import type { ParticipantBadge } from '../lib/participant-display';

/**
 * Чип агрегат-статуса в строке участника — Figma «Row Button Bage»
 * (компонент 2036:83107 в анатомии Row Button): пилюля radius 8,
 * паддинг 4×8, зазор 4, текст 14/16 Regular. Warning («Превышен лимит
 * объектов») — жёлтая подложка (канон --color-warning-bg; макет несёт
 * близкий #FFB900@10% — §12/§13: токен сильнее цвета макета) и
 * Icon/S/Lock; neutral («Доступ ко всем объектам» / «Доступно N
 * объектов») — серая подложка без иконки (макеты 2008-82943).
 */
export function ParticipantStatusBadge({ badge }: { readonly badge: ParticipantBadge }): JSX.Element {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-lg px-2 py-1 text-sm leading-4 text-content',
        badge.tone === 'warning' ? 'bg-warning-bg' : 'bg-surface-muted',
      )}
    >
      {badge.withLock && <LockSmall className="h-4 w-4 shrink-0" aria-hidden />}
      {badge.label}
    </span>
  );
}
