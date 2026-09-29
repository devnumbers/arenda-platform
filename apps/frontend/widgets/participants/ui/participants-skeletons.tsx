import type { JSX } from 'react';
import { Skeleton, skeletonRowWidths, type SkeletonRowWidths } from '@/shared/ui/design';

/**
 * Скелетон страницы участника (#698, паритет — §7 DESIGN.md): центр-блок
 * «аватар 96 — имя 32 — почта 16 — чип 24» и ряды «Доступных объектов»
 * (аватар 44, титул 18, адрес 16, бейдж 24, чеврон). Ширины —
 * детерминированный цикл, как у списка #697.
 */
export function ParticipantScreenSkeleton(): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col gap-6 pt-4">
      <div className="flex flex-col items-center gap-2">
        <Skeleton className="h-24 w-24 rounded-pill" />
        <Skeleton className="h-8 w-40" />
        <Skeleton className="h-4 w-48" />
        <Skeleton className="mt-2 h-6 w-44 rounded-lg" />
      </div>
      <Skeleton className="h-6 w-48" />
      <div className="flex flex-col">
        {skeletonRowWidths(3).map((widths, index) => (
          <LegRowSkeleton key={index} widths={widths} />
        ))}
      </div>
    </div>
  );
}

function LegRowSkeleton({ widths }: { readonly widths: SkeletonRowWidths }): JSX.Element {
  return (
    <span className="flex w-full items-center gap-3 py-3">
      <Skeleton className="h-11 w-11 shrink-0 rounded-pill" />
      <span className="flex min-w-0 flex-1 flex-col gap-1">
        <Skeleton className={`h-[18px] ${widths.title}`} />
        <Skeleton className={`h-4 ${widths.subtitle}`} />
        <Skeleton className="h-6 w-40 rounded-lg" />
      </span>
      <Skeleton className="h-6 w-6 shrink-0" />
    </span>
  );
}

/** Скелетон экрана «Права участника» (#698, паритет — §7 DESIGN.md): строка
 * объекта с бейджем роли, сегмент роли (высота контейнера 52 = кнопки 48 +
 * паддинг капсулы 2×2) и оба ряда действий («Действия участника в объекте»
 * есть у зарегистрированных — обычный случай; «Отозвать» — всегда). */
export function ParticipantRightsSkeleton(): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col gap-6">
      <span className="flex w-full items-center gap-3 py-3">
        <Skeleton className="h-11 w-11 shrink-0 rounded-pill" />
        <span className="flex min-w-0 flex-1 flex-col gap-1">
          <Skeleton className="h-[18px] w-2/5" />
          <Skeleton className="h-4 w-3/5" />
          <Skeleton className="h-6 w-40 rounded-lg" />
        </span>
      </span>
      <Skeleton className="h-[52px] w-full rounded-2xl" />
      <div className="flex flex-col">
        <span className="flex w-full items-center rounded-button py-1">
          <Skeleton className="h-11 w-11 shrink-0 rounded-pill" />
          <Skeleton className="ml-1 h-[18px] w-56" />
        </span>
        <span className="flex w-full items-center rounded-button py-1">
          <Skeleton className="h-11 w-11 shrink-0 rounded-pill" />
          <Skeleton className="ml-1 h-[18px] w-48" />
        </span>
      </div>
    </div>
  );
}

/** Скелетон экрана «Пригласить в объект» (#698): строка «Все объекты»,
 * разделитель и чек-ряды. */
export function ParticipantInviteSkeleton(): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col">
      {[0, 1, 2, 3].map((index) => (
        <span key={index} className="flex w-full items-center gap-3 py-3">
          <Skeleton className="h-11 w-11 shrink-0 rounded-pill" />
          <span className="flex min-w-0 flex-1 flex-col gap-1">
            <Skeleton className="h-[18px] w-2/5" />
            <Skeleton className="h-4 w-3/5" />
          </span>
          <Skeleton className="h-6 w-6 shrink-0 rounded-lg" />
        </span>
      ))}
    </div>
  );
}

/** Скелетон экрана «Пригласите участника» (#699, паритет — §7 DESIGN.md):
 * иллюстрация 96 — заголовок 32 — описание 16×3 без зазора (переносится
 * на три строки в мобайл-колонке; на планшете/ПК их две — блок длиннее
 * реального на 16px) — поле 56 — сегмент 52 — строка выбора 68. */
export function ParticipantsInviteSkeleton(): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col pt-4">
      <Skeleton className="h-24 w-24 self-center rounded-pill" />
      <Skeleton className="mt-8 h-8 w-64" />
      <span className="mt-2 flex flex-col">
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-4/5" />
      </span>
      <Skeleton className="mt-8 h-14 w-full rounded-button" />
      <Skeleton className="mt-8 h-[52px] w-full rounded-2xl" />
      <span className="mt-6 flex w-full items-center gap-3 py-3">
        <Skeleton className="h-11 w-11 shrink-0 rounded-pill" />
        <span className="flex min-w-0 flex-1 flex-col gap-1">
          <Skeleton className="h-[18px] w-2/5" />
          <Skeleton className="h-4 w-3/5" />
        </span>
        <Skeleton className="h-6 w-6 shrink-0" />
      </span>
    </div>
  );
}
