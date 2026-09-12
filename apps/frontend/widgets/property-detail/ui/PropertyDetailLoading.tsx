import type { JSX } from 'react';
import { Skeleton } from '@/shared/ui/design';

const CONTENT_SECTIONS = 6;
const MANAGE_ROWS = 7;

/** Скелетон детали объекта — геометрия нового каркаса #588 (медиа-блок,
 * шесть секций-карточек с заголовком и «пустым» содержимым, карточка
 * «Управления» с рядами строк). Внутри серых карточек приглушение до
 * bg-surface-muted-hover (§7). Пульс до гидратации данных, aria-busy —
 * на корневом блоке. */
export function PropertyDetailLoading(): JSX.Element {
  return (
    <div aria-busy="true" aria-label="Загрузка объекта">
      <div className="flex flex-col items-center pt-6">
        <Skeleton className="h-24 w-24 rounded-pill" />
        <Skeleton className="mt-6 h-8 w-[280px]" />
        <Skeleton className="mt-3 h-4 w-[180px]" />
      </div>
      {Array.from({ length: CONTENT_SECTIONS }, (_, index) => (
        <div
          key={index}
          className={`${index === 0 ? 'mt-20' : 'mt-4'} rounded-card bg-surface-muted px-6 pb-6 pt-3`}
        >
          <div className="flex items-center justify-between">
            <Skeleton className="h-5 w-36 bg-surface-muted-hover" />
            <Skeleton className="h-6 w-6 bg-surface-muted-hover" />
          </div>
          <div className="flex flex-col items-center pt-4">
            <Skeleton className="h-16 w-16 bg-surface-muted-hover" />
            <Skeleton className="mt-4 h-5 w-48 bg-surface-muted-hover" />
            <Skeleton className="mt-2 h-4 w-56 bg-surface-muted-hover" />
            <Skeleton className="mt-4 h-11 w-[105px] bg-surface-muted-hover" />
          </div>
        </div>
      ))}
      <div className="mt-4 rounded-card bg-surface-muted px-6 pb-6 pt-3">
        <Skeleton className="h-5 w-36 bg-surface-muted-hover" />
        <div className="flex flex-col gap-2 pt-4">
          {Array.from({ length: MANAGE_ROWS }, (_, index) => (
            <Skeleton key={index} className="h-[52px] bg-surface-muted-hover" />
          ))}
        </div>
      </div>
    </div>
  );
}
