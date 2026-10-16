import type { JSX } from 'react';
import { Skeleton } from '@/shared/ui/design';
import { PropertyOperationsSkeleton } from './PropertyOperationsBlock';

const MANAGE_ROWS = 7;
/** Анатомия тела каждой из шести секций (аренда, платежи, операции,
 * контакты, задачи, тип): зеркалит фазу «деталь приехала» — до прихода
 * данных секции показывают те же тела (аренда/контакты/задачи — чаще
 * всего пустые, платежи — ленту иконок, операции — сводку). */
const SECTION_BODIES = ['empty', 'payments', 'operations', 'empty', 'empty', 'empty'] as const;

const SECTION_BODY_SKELETONS = {
  empty: PropertySectionEmptySkeleton,
  payments: PropertyPaymentsStripSkeleton,
  operations: PropertyOperationsSkeleton,
} as const;

/**
 * Скелетон детали объекта — геометрия нового каркаса #588. Карточки
 * секций повторяют PropertySectionCard буквально (§7, паритет #604):
 * карточка без собственных паддингов, строка заголовка px-6 pt-6, тело
 * несёт свой паддинг — смена фаз «страница целиком» → «деталь приехала,
 * секции едут» → «секции загружены» не двигает контент. Внутри серых
 * карточек приглушение до bg-surface-muted-hover (§7). Пульс до
 * гидратации данных, aria-busy — на корневом блоке.
 */
export function PropertyDetailLoading(): JSX.Element {
  return (
    <div aria-busy="true" aria-label="Загрузка объекта">
      <div className="flex flex-col items-center pt-6">
        <Skeleton className="h-24 w-24 rounded-pill" />
        <Skeleton className="mt-6 h-8 w-[280px]" />
        <Skeleton className="mt-3 h-4 w-[180px]" />
      </div>
      {SECTION_BODIES.map((body, index) => {
        const BodySkeleton = SECTION_BODY_SKELETONS[body];
        return (
          <div
            key={index}
            className={`${index === 0 ? 'mt-20' : 'mt-4'} rounded-card bg-surface-muted`}
          >
            <PropertySectionHeaderSkeleton />
            <BodySkeleton />
          </div>
        );
      })}
      <div className="mt-4 rounded-card bg-surface-muted">
        <div className="flex items-center px-6 pt-6" aria-hidden>
          <Skeleton className="h-6 w-36 bg-surface-muted-hover" />
        </div>
        <div className="flex flex-col gap-2 px-6 pb-6 pt-4">
          {Array.from({ length: MANAGE_ROWS }, (_, index) => (
            <Skeleton key={index} className="h-[52px] bg-surface-muted-hover" />
          ))}
        </div>
      </div>
    </div>
  );
}

/** Строка заголовка секции — геометрия шапки PropertySectionCard
 * (название 24px линии + шеврон 24 у края). */
function PropertySectionHeaderSkeleton(): JSX.Element {
  return (
    <div className="flex w-full items-center px-6 pt-6" aria-hidden>
      <Skeleton className="h-6 w-36 bg-surface-muted-hover" />
      <Skeleton className="ml-auto h-6 w-6 shrink-0 bg-surface-muted-hover" />
    </div>
  );
}

/**
 * Тело секции в фазе «данные едут» — анатомия короткого пустого
 * состояния (PropertySectionEmpty без описания): иллюстрация 64, одна
 * строка текста 16, кнопка 44 — px-12 py-6, зазор 24 до кнопки.
 */
export function PropertySectionEmptySkeleton(): JSX.Element {
  return (
    <div className="flex flex-col items-center px-12 py-6" aria-hidden>
      <Skeleton className="h-16 w-16 rounded-pill bg-surface-muted-hover" />
      <Skeleton className="mt-4 h-4 w-48 bg-surface-muted-hover" />
      <Skeleton className="mt-6 h-11 w-[105px] rounded-button bg-surface-muted-hover" />
    </div>
  );
}

/**
 * Лента иконок «Регулярных платежей» в фазе загрузки — анатомия
 * PropertyPaymentsIconsBlock (Figma 1185:40820): две колонки 50/50 —
 * подпись 14/16 и нахлёст кругов 44 (−12px).
 */
export function PropertyPaymentsStripSkeleton(): JSX.Element {
  return (
    <div className="flex gap-4 px-6 pb-6 pt-4" aria-hidden>
      {[0, 1].map((column) => (
        <div key={column} className="flex min-w-0 flex-1 flex-col gap-2">
          <Skeleton className="h-4 w-24 bg-surface-muted-hover" />
          <div className="flex -space-x-3">
            <Skeleton className="h-11 w-11 rounded-pill bg-surface-muted-hover" />
            <Skeleton className="h-11 w-11 rounded-pill bg-surface-muted-hover" />
            <Skeleton className="h-11 w-11 rounded-pill bg-surface-muted-hover" />
          </div>
        </div>
      ))}
    </div>
  );
}
