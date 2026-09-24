import type { JSX } from 'react';
import { Skeleton } from '@/shared/ui/design';

/**
 * Состояния ленты «Истории действий» (#709): скелет с паритетом финального
 * лейаута (макет 2157-56876) — плашка дня, шапка объекта, серые карточки
 * актёров со строками (§7 канона скелетонов: бары внутри серой карточки —
 * bg-surface-muted-hover, иначе тон бара сливается с карточкой). Пустое
 * состояние — на самом
 * экране (макет 2050-158499: только серая строка «Действий не было», без
 * иллюстрации). Скелетоны прибитых страниц — паритет их композиции
 * (решение владельца 24.09): «Действия участника» — одна группа, «История
 * объекта» и пара (#840/#841) — шапка-карточка объекта + одна группа без
 * объектной секции; общий двухгрупповый остаётся только общей ленте —
 * прежде он стоял на всех страницах и заполнял весь viewport, схлопываясь
 * в короткую ленту.
 */

const ROW_WIDTHS = ['w-11/12', 'w-2/3', 'w-3/4'] as const;

function HistoryFeedSkeletonActorCard({ rows }: { readonly rows: number }): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col gap-2 rounded-m bg-surface-muted p-3">
      <div className="flex items-center gap-2">
        <Skeleton className="h-6 w-6 shrink-0 rounded-pill bg-white" />
        <Skeleton className="h-[15px] w-28 bg-surface-muted-hover" />
      </div>
      {ROW_WIDTHS.slice(0, rows).map((width, index) => (
        <div key={index} className="flex items-end gap-2">
          <Skeleton className="h-4 w-[3px] shrink-0 self-stretch rounded-pill bg-surface-muted-hover" />
          <Skeleton className="h-4 w-4 shrink-0 rounded-pill bg-surface-muted-hover" />
          <Skeleton className={`h-[15px] bg-surface-muted-hover ${width}`} />
          <Skeleton className="ml-auto h-[15px] w-10 shrink-0 bg-surface-muted-hover" />
        </div>
      ))}
    </div>
  );
}

/** Плашка дня скелетона: зазор до блоков дня — 24, как у реальной
 * липкой плашки ленты (§7 #604 — контент встаёт на место скелетона). */
function HistoryFeedSkeletonDayPill(): JSX.Element {
  return (
    <div className="mb-6 flex justify-center">
      <Skeleton className="h-[31px] w-28 rounded-pill" />
    </div>
  );
}

/** Шапка объекта внутри группы скелетона (общая лента и «Действия
 * участника»: аватар 24 + название + адрес). */
function HistoryFeedSkeletonObjectHeader(): JSX.Element {
  return (
    <div className="flex items-center gap-2">
      <Skeleton className="h-6 w-6 shrink-0 rounded-pill" />
      <div className="min-w-0">
        <Skeleton className="h-[15px] w-36" />
        <Skeleton className="mt-1 h-[15px] w-24" />
      </div>
    </div>
  );
}

/** Шапка-карточка прибитого объекта скелетона («История объекта» и пара,
 * #840/#841): то же содержимое объектной шапки, но над всей лентой —
 * зеркало PinnedPropertyHeader. */
function HistoryFeedSkeletonPinnedProperty(): JSX.Element {
  return (
    <div aria-hidden className="mb-3 flex min-w-0 items-center gap-2">
      <HistoryFeedSkeletonObjectHeader />
    </div>
  );
}

function HistoryFeedSkeletonGroup(): JSX.Element {
  return (
    <div aria-hidden>
      <HistoryFeedSkeletonDayPill />
      <div className="flex flex-col gap-3">
        <HistoryFeedSkeletonObjectHeader />
        <div className="flex flex-col gap-1.5">
          <HistoryFeedSkeletonActorCard rows={3} />
          <HistoryFeedSkeletonActorCard rows={1} />
        </div>
      </div>
    </div>
  );
}

/** Скелет общей ленты: две группы (плашка + шапка объекта + карточки
 * актёров) — лента по всем объектам реально длинная. */
export function HistoryFeedSkeleton(): JSX.Element {
  return (
    <div aria-hidden>
      <HistoryFeedSkeletonGroup />
      <HistoryFeedSkeletonGroup />
    </div>
  );
}

/** Скелет «Действий участника» (#712): одна группа — в пределах одного
 * человека день даёт одну-две объектные секции, двухгрупповый общий
 * скелетон заполнял весь viewport и схлопывался (решение владельца
 * 24.09). */
export function HistoryMemberFeedSkeleton(): JSX.Element {
  return (
    <div aria-hidden>
      <HistoryFeedSkeletonDayPill />
      <div className="flex flex-col gap-3">
        <HistoryFeedSkeletonObjectHeader />
        <div className="flex flex-col gap-1.5">
          <HistoryFeedSkeletonActorCard rows={3} />
        </div>
      </div>
    </div>
  );
}

/** Скелет «Истории объекта» (#840) и «Действий участника в объекте»
 * (#841): шапка-карточка прибитого объекта над лентой + одна группа без
 * объектной секции — внутри объекта дни открываются сразу карточками
 * актёров. */
export function HistoryPropertyFeedSkeleton(): JSX.Element {
  return (
    <div aria-hidden>
      <HistoryFeedSkeletonPinnedProperty />
      <HistoryFeedSkeletonDayPill />
      <div className="flex flex-col gap-1.5">
        <HistoryFeedSkeletonActorCard rows={3} />
      </div>
    </div>
  );
}

/**
 * Скелет шита «Настройки» (#711): паритет финального лейаута (макет
 * 2177-60527 — чип периода, четыре свёрнутые карточки групп, «Сбросить
 * фильтры»); рендерится, пока опции /history/filters в пути. На «Действиях
 * участника» (#712) группа «Участники» тоже рисуется (прибитый человек,
 * макет 2184-92510) — четыре карточки в обоих режимах.
 */
export function HistoryFiltersSheetSkeleton(): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col gap-4">
      <Skeleton className="h-11 w-40 rounded-pill" />
      {[0, 1, 2, 3].map((index) => (
        <div key={index} className="rounded-3xl bg-surface-muted py-1">
          <div className="flex min-h-14 items-center gap-2 py-4 pl-5 pr-5">
            <Skeleton className="h-6 w-6 shrink-0 rounded-lg bg-surface-muted-hover" />
            <Skeleton className="h-[18px] w-36 bg-surface-muted-hover" />
            <Skeleton className="ml-auto h-[15px] w-10 bg-surface-muted-hover" />
          </div>
        </div>
      ))}
      <Skeleton className="h-14 w-full rounded-pill" />
    </div>
  );
}
