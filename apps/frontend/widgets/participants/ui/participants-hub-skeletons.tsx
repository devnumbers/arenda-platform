import type { JSX } from 'react';
import { Skeleton } from '@/shared/ui/design';

/** Скелетон хаба «Совместный доступ» (#696): паритет двух карточек
 * (макет 2008-47013) — та же геометрия, что у контента; шапка экрана
 * и нижняя CTA рендерятся вне фазы загрузки (§7 DESIGN.md). */
export function ParticipantsHubSkeleton(): JSX.Element {
  return (
    <div role="status" aria-label="Загрузка раздела" className="flex flex-col gap-4 px-6">
      <div className="flex flex-col gap-6 rounded-[32px] bg-surface-muted p-8">
        <div className="flex items-start gap-6">
          <div className="flex min-w-0 flex-1 flex-col gap-2">
            <Skeleton className="h-6 w-40" />
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-4/5" />
          </div>
          <Skeleton className="h-12 w-12 shrink-0" />
        </div>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Skeleton className="h-6 w-6" />
            <Skeleton className="h-[18px] w-28" />
          </div>
          <Skeleton className="h-6 w-6" />
        </div>
      </div>
      <div className="flex flex-col gap-6 rounded-[32px] bg-surface-muted p-8">
        <div className="flex items-start gap-6">
          <div className="flex min-w-0 flex-1 flex-col gap-2">
            <Skeleton className="h-6 w-44" />
            <Skeleton className="h-6 w-40" />
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-3/5" />
          </div>
          <Skeleton className="h-12 w-12 shrink-0" />
        </div>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Skeleton className="h-6 w-6" />
            <Skeleton className="h-[18px] w-24" />
          </div>
          <Skeleton className="h-6 w-6" />
        </div>
      </div>
    </div>
  );
}
