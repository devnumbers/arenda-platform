import type { JSX } from 'react';
import {
  Skeleton,
  skeletonRowWidths,
  type SkeletonRowWidths,
} from '@/shared/ui/design';

/**
 * Скелетон ряда списка участников (#697, паритет — §7 DESIGN.md):
 * каркас ParticipantRowButton на белом — аватар-круг 44, титул 18,
 * подзаголовок 16 и чип-бейдж 24 (со своим радиусом; статусные бейджи
 * есть не у всех рядов — D3 #1313, скелетон держит место), ведомый
 * шеврон; паддинг строки 12px 0, как у реальной. Ширины —
 * детерминированный цикл.
 */
export function ParticipantsListSkeleton(): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col px-6">
      {skeletonRowWidths(4).map((widths, index) => (
        <ParticipantRowSkeleton key={index} widths={widths} />
      ))}
    </div>
  );
}

function ParticipantRowSkeleton({ widths }: { readonly widths: SkeletonRowWidths }): JSX.Element {
  return (
    <span className="flex w-full items-center gap-3 py-3">
      <Skeleton className="h-11 w-11 shrink-0 rounded-pill" />
      <span className="flex min-w-0 flex-1 flex-col gap-1">
        <Skeleton className={`h-[18px] ${widths.title}`} />
        <Skeleton className={`h-4 ${widths.subtitle}`} />
        <Skeleton className="h-6 w-44 rounded-lg" />
      </span>
      <Skeleton className="h-6 w-6 shrink-0" />
    </span>
  );
}
