import type { JSX, ReactNode } from 'react';
import { Skeleton } from '@/shared/ui/design';

/**
 * Состояния ленты «Истории действий» (#709): скелет с паритетом финального
 * лейаута (макет 2157-56876) — плашка дня, шапка объекта, серые карточки
 * актёров со строками (§7 канона скелетонов: бары внутри серой карточки —
 * bg-surface-muted-hover, иначе тон бара сливается с карточкой). Пустое
 * состояние — на самом
 * экране (макет 2050-158499: только серая строка «Действий не было», без
 * иллюстрации). Скелетоны прибитых страниц — паритет их композиции
 * (решение владельца 24.09): «История объекта» (#1321, макет 3258-76984)
 * и пара «участник+объект» (#841, макет 3260-77196) — плашка дня и сразу
 * карточки актёров (объект — в сабтайтле шапки); общий двухгрупповый
 * остаётся только общей ленте — прежде он
 * стоял на всех страницах и заполнял весь viewport, схлопываясь в
 * короткую ленту. Паритет кадрам ленты (карта #1324, #1281): плашка дня
 * — в sticky-обёртке живой плашки, секции дня с pb-6, объектные группы с
 * gap-4 — pending- и загруженная композиции совпадают геометрией.
 */

const ROW_WIDTHS = ['w-11/12', 'w-2/3', 'w-3/4'] as const;

/** Строка ленты скелетона: полоска тона + S-иконка + текст + время —
 * анатомия строки внутри карточки актёра. */
function HistoryFeedSkeletonRow({ width }: { readonly width: string }): JSX.Element {
  return (
    <div className="flex items-end gap-2">
      <Skeleton className="h-4 w-[3px] shrink-0 self-stretch rounded-pill bg-surface-muted-hover" />
      <Skeleton className="h-4 w-4 shrink-0 rounded-pill bg-surface-muted-hover" />
      <Skeleton className={`h-[15px] bg-surface-muted-hover ${width}`} />
      <Skeleton className="ml-auto h-[15px] w-10 shrink-0 bg-surface-muted-hover" />
    </div>
  );
}

function HistoryFeedSkeletonActorCard({ rows }: { readonly rows: number }): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col gap-2 rounded-m bg-surface-muted p-3">
      <div className="flex items-center gap-2">
        <Skeleton className="h-6 w-6 shrink-0 rounded-pill bg-white" />
        <Skeleton className="h-[15px] w-28 bg-surface-muted-hover" />
      </div>
      {ROW_WIDTHS.slice(0, rows).map((width, index) => (
        <HistoryFeedSkeletonRow key={index} width={width} />
      ))}
    </div>
  );
}

/** Плашка дня скелетона: зазор до блоков дня — 24, как у реальной
 * липкой плашки ленты (§7 #604 — контент встаёт на место скелетона);
 * обёртка sticky с теми же классами, что у живой плашки, — паритет
 * композиции pending/загруженного кадра (#1281). */
function HistoryFeedSkeletonDayPill(): JSX.Element {
  return (
    <div className="sticky top-[calc(env(safe-area-inset-top)_+_24px)] z-20 mb-6 flex justify-center tablet:top-[96px]">
      <Skeleton className="h-[31px] w-28 rounded-pill" />
    </div>
  );
}

/** Шапка объекта внутри группы скелетона (общая лента: аватар 24 +
 * название + адрес). */
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

/** Секция дня скелетона — зеркало секции живой ленты: pb-6 после блоков
 * дня (следующая секция встаёт ровно на место, #604/#1281). */
function HistoryFeedSkeletonDay({ children }: { readonly children: ReactNode }): JSX.Element {
  return <section className="pb-6">{children}</section>;
}

function HistoryFeedSkeletonGroup(): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col gap-4">
      <HistoryFeedSkeletonObjectHeader />
      <div className="flex flex-col gap-1.5">
        <HistoryFeedSkeletonActorCard rows={3} />
        <HistoryFeedSkeletonActorCard rows={1} />
      </div>
    </div>
  );
}

/** Скелет общей ленты: две группы (плашка + шапка объекта + карточки
 * актёров) — лента по всем объектам реально длинная. */
export function HistoryFeedSkeleton(): JSX.Element {
  return (
    <div aria-hidden>
      <HistoryFeedSkeletonDay>
        <HistoryFeedSkeletonDayPill />
        <HistoryFeedSkeletonGroup />
      </HistoryFeedSkeletonDay>
      <HistoryFeedSkeletonDay>
        <HistoryFeedSkeletonDayPill />
        <HistoryFeedSkeletonGroup />
      </HistoryFeedSkeletonDay>
    </div>
  );
}

/** Скелет страниц с пином объекта — «Истории объекта» (#840, макет
 * 3258-76984) и пары «участник+объект» (#841, макет 3260-77196; D7):
 * объект живёт в сабтайтле шапки (TopNavTitle, вне фазы загрузки §7),
 * над лентой — сразу плашка дня и карточки актёров без объектной секции;
 * у пары карточка одна — актёр прибит страницей. */
export function HistoryPropertyFeedSkeleton(): JSX.Element {
  return (
    <div aria-hidden>
      <HistoryFeedSkeletonDay>
        <HistoryFeedSkeletonDayPill />
        <div className="flex flex-col gap-1.5">
          <HistoryFeedSkeletonActorCard rows={3} />
        </div>
      </HistoryFeedSkeletonDay>
    </div>
  );
}

/** Выбор скелетона по скоупу страницы — одна точка для живого экрана и
 * фолбэка Suspense-границы (#1281): хром фолбэка не разъезжается с
 * хромом экрана (канон screen-chrome); выбор ведёт пин объекта —
 * прибитые страницы (объект #840, пара #841) против общей ленты. */
export function historyFeedSkeleton(propertyId?: string): JSX.Element {
  if (propertyId !== undefined) {
    return <HistoryPropertyFeedSkeleton />;
  }
  return <HistoryFeedSkeleton />;
}

/**
 * Скелет шита «Настройки» (#711): паритет финального лейаута (макет
 * 2177-60527 — чип периода, четыре свёрнутые карточки групп, «Сбросить
 * фильтры»); рендерится, пока опции /history/filters в пути. На странице
 * пары (#841) группа «Участники» тоже рисуется (прибитый человек,
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
