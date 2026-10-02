import type { JSX } from 'react';
import {
  Skeleton,
  SkeletonListRow,
  skeletonBlockClass,
  skeletonRowWidths,
} from '@/shared/ui/design';
import { PaymentsRowsSkeleton } from './payments-sections';

/**
 * Скелетоны-архетипы зоны операций (#605, паритет — §7 DESIGN.md):
 * композиции примитивов #604 под реальные лейауты своих экранов — контент
 * занимает место скелетона без сдвига. Шапки (заголовок, пилюля поиска,
 * чипы) рендерятся вне фазы загрузки и скелетоном не подменяются.
 */

/** Приглушённый тон блоков внутри серых карточек (§7, skeletonBlockClass). */
const MUTED = skeletonBlockClass('muted');

/** Сводная карточка-заглушка: серая карточка OperationsSummaryCard
 * (Figma 1510-77101) — сумма 16/24, подпись 14/20, полоса разбивки 6px. */
function SkeletonSummaryCard(): JSX.Element {
  return (
    <div aria-hidden className="min-w-0 flex-1 rounded-card bg-surface-muted px-6 pb-6 pt-5">
      <span className="flex flex-col gap-0.5">
        <Skeleton className={`h-6 w-20 ${MUTED}`} />
        <Skeleton className={`h-5 w-16 ${MUTED}`} />
      </span>
      <Skeleton className={`mt-4 h-1.5 w-full ${MUTED}`} />
    </div>
  );
}

/**
 * Ряд карточек сводки: хаб — две карточки «Расходы»/«Доходы» в ряд,
 * страница направления — одна карточка во всю ширину колонки.
 */
export function OperationsSummarySkeleton({
  cards,
}: {
  readonly cards: 1 | 2;
}): JSX.Element {
  if (cards === 1) {
    return (
      <div aria-hidden className="px-6">
        <SkeletonSummaryCard />
      </div>
    );
  }
  return (
    <div aria-hidden className="flex gap-2 px-6">
      <SkeletonSummaryCard />
      <SkeletonSummaryCard />
    </div>
  );
}

/** Группа дат-заглушка: заголовок даты (PaymentsHeading 20/24, на белом —
 * базовый тон) и строки операций (PaymentRowButton px-3+px-3 py-3 →
 * вставка 24/12). */
function SkeletonDateGroup({
  rows,
}: {
  readonly rows: number;
}): JSX.Element {
  const widths = skeletonRowWidths(rows);
  return (
    <section aria-hidden className="flex flex-col">
      <div className="px-6">
        <Skeleton className={`h-6 w-24 ${skeletonBlockClass('base')}`} />
      </div>
      {widths.map((rowWidths, index) => (
        <SkeletonListRow
          key={index}
          value
          widths={rowWidths}
          className="px-6 py-3"
        />
      ))}
    </section>
  );
}

/**
 * Лента по датам-заглушка: каркас OperationsDateList — группы с зазором 8,
 * внутри заголовок и строки со знаковой суммой. Две группы (3+2 строки)
 * заполняют первый экран так же, как сеяная лента.
 */
export function OperationsDateFeedSkeleton(): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col gap-2">
      <SkeletonDateGroup rows={3} />
      <SkeletonDateGroup rows={2} />
    </div>
  );
}

/**
 * Строки выборщика категорий-заглушка (#544): ListRow «иконка + название +
 * сумма периода + чекбокс» — однострочные (без подзаголовка), как реальные
 * строки, чтобы высота совпадала.
 */
export function OperationsCategoriesSkeleton(): JSX.Element {
  const widths = skeletonRowWidths(8);
  return (
    <div aria-hidden className="flex flex-col">
      {widths.map((rowWidths, index) => (
        <SkeletonListRow
          key={index}
          subtitle={false}
          value
          trailing
          widths={rowWidths}
        />
      ))}
    </div>
  );
}

/**
 * Строки выборщика объектов-заглушка (#542): ListRow «фото + название +
 * адрес + чекбокс» — канон строки без правой колонки.
 */
export function OperationsObjectsSkeleton(): JSX.Element {
  const widths = skeletonRowWidths(4);
  return (
    <div aria-hidden className="flex flex-col">
      {widths.map((rowWidths, index) => (
        <SkeletonListRow key={index} trailing widths={rowWidths} />
      ))}
    </div>
  );
}

/**
 * Поисковая выдача-заглушка (#543/#476): контент — две секции («Категории» —
 * заголовок + чипы, «Операции»/«Платежи» — заголовок + ряды), скелетон
 * зеркалит их анатомию: заголовки-полоски, чипы-заглушки (высота канона
 * ChipButton), ряды канона #605. Пара «заголовок + контент» живёт в своей
 * секции, как <section aria-label> живой выдачи: зазор заголовок→контент
 * держит pt-2 контента (8px), gap-6 контейнера делит только секции —
 * иначе контент уезжает вверх на смене загрузки→выдача. Общий архетип
 * поисковых экранов операций и платежей. description — дата под суммой
 * в строках глобального поиска; в объектном (#476) строки без неё.
 */
export function SearchResultsSkeleton({
  description = false,
}: {
  readonly description?: boolean;
}): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col gap-6 pt-4">
      <section aria-hidden className="flex flex-col">
        <Skeleton className="ml-6 h-6 w-40" />
        <div className="flex flex-wrap gap-1.5 px-6 pt-2">
          <Skeleton className="h-11 w-28 rounded-full" />
          <Skeleton className="h-11 w-36 rounded-full" />
        </div>
      </section>
      <section aria-hidden className="flex flex-col">
        <Skeleton className="ml-6 h-6 w-40" />
        <div className="flex flex-col pt-2">
          <PaymentsRowsSkeleton description={description} />
        </div>
      </section>
    </div>
  );
}
