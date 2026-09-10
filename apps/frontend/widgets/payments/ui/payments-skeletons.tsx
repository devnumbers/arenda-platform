import type { JSX, ReactNode } from 'react';
import { Skeleton, SkeletonListRow, skeletonBlockClass, skeletonRowWidths } from '@/shared/ui/design';

/**
 * Скелетоны-архетипы детализаций платежей (#606, паритет — §7 DESIGN.md):
 * композиции примитивов #604 под финальный лейаут каждой страницы —
 * страница правила (hero-карточка, круглые кнопки, серые секции со
 * строками PaymentRowButton, плитки подэкранов), страница операции и
 * проекция (hero, «Данные операции», «Подробнее»), плоские группы
 * «заголовок + строки» истории и графика. Шапки (TopNav, заголовок) и
 * чип сортировки истории рендерятся вне фазы загрузки — контент занимает
 * место скелетона без сдвига.
 */

/** Приглушённый тон блоков внутри серых карточек (§7, skeletonBlockClass). */
const MUTED = skeletonBlockClass('muted');

/** Серая группа-заглушка: каркас PaymentsGroup — заголовок 20/24 (pt-6 pb-3)
 * и строки, снизу баланс 24; боковые поля экрана приносит секция (mx-6). */
function SkeletonPaymentsGroup({ children }: { readonly children: ReactNode }): JSX.Element {
  return (
    <section aria-hidden className="mx-6 rounded-card bg-surface-muted pb-6">
      <div className="px-6 pb-3 pt-6">
        <Skeleton className={`h-6 w-40 ${MUTED}`} />
      </div>
      <div className="flex flex-col">{children}</div>
    </section>
  );
}

/** Серая секция страницы операции-заглушка: каркас OperationSection —
 * заголовок H3 20/24 (px-6 pb-2) и строки без карточки-подложки. */
function SkeletonOperationSection({ children }: { readonly children: ReactNode }): JSX.Element {
  return (
    <section aria-hidden>
      <div className="px-6 pb-2">
        <Skeleton className="h-6 w-40" />
      </div>
      <div className="flex flex-col">{children}</div>
    </section>
  );
}

/** Круглая кнопка-заглушка: каркас RoundActionButton — круг 56 и подпись
 * 13/15 с зазором 8; тон базовый — реальный круг secondary сам серый. */
function SkeletonRoundAction(): JSX.Element {
  return (
    <span className="flex flex-col items-center gap-2">
      <Skeleton className="h-14 w-14 rounded-pill" />
      <Skeleton className="h-[15px] w-12" />
    </span>
  );
}

/** Плитка подэкранов-заглушка: каркас PaymentsTile — серая карточка
 * min-h 168.5 (p-6), иконка 40 сверху, подпись снизу. */
function SkeletonTile(): JSX.Element {
  return (
    <span
      aria-hidden
      className="flex min-h-[168.5px] flex-1 flex-col justify-between rounded-card bg-surface-muted p-6"
    >
      <Skeleton className={`h-10 w-10 ${MUTED}`} />
      <Skeleton className={`h-[18px] w-3/5 ${MUTED}`} />
    </span>
  );
}

/**
 * Скелетон страницы платежа (#606): hero-карточка правила, три круглые
 * кнопки (полный доступ), секции «Ближайшая операция» (строка со суммой)
 * и «Просроченные» (строки со суммой и сроком), плитки «График/История».
 */
export function PaymentDetailSkeleton(): JSX.Element {
  const overdueWidths = skeletonRowWidths(1);
  return (
    <>
      <div className="px-6">
        <section aria-hidden className="flex flex-col gap-3 rounded-card bg-surface-muted p-6">
          <span className="flex flex-col items-start gap-2">
            <Skeleton className={`h-[18px] w-2/5 ${MUTED}`} />
            <Skeleton className={`h-6 w-1/2 ${MUTED}`} />
            <Skeleton className={`h-4 w-1/4 ${MUTED}`} />
          </span>
          <span className="flex items-center gap-1.5">
            <Skeleton className={`h-4 w-4 shrink-0 ${MUTED}`} />
            <Skeleton className={`h-4 w-2/5 ${MUTED}`} />
          </span>
        </section>
      </div>

      <div className="px-6" aria-hidden>
        <div className="grid grid-cols-3">
          <SkeletonRoundAction />
          <SkeletonRoundAction />
          <SkeletonRoundAction />
        </div>
      </div>

      <div className="flex flex-col gap-6">
        <SkeletonPaymentsGroup>
          <SkeletonListRow value tone="muted" className="px-3" />
        </SkeletonPaymentsGroup>

        <SkeletonPaymentsGroup>
          {overdueWidths.map((rowWidths, index) => (
            <SkeletonListRow
              key={index}
              value
              description
              tone="muted"
              widths={rowWidths}
              className="px-3"
            />
          ))}
        </SkeletonPaymentsGroup>

        <div className="px-6" aria-hidden>
          <div className="flex gap-2">
            <SkeletonTile />
            <SkeletonTile />
          </div>
        </div>
      </div>
    </>
  );
}

/** Строка «Подробнее»-заглушка: каркас DetailRow — лейбл слева и значение
 * справа, 14/16 с py-1, прямо на белом. */
function SkeletonDetailRow(): JSX.Element {
  return (
    <span aria-hidden className="flex items-center justify-between gap-3 px-6 py-1">
      <Skeleton className="h-4 w-16" />
      <Skeleton className="h-4 w-24" />
    </span>
  );
}

/**
 * Скелетон страницы операции (#606): hero (круг категории 96, название,
 * чип, сумма 40, подпись срока), секции «Данные операции» (строки-ссылки
 * на правило и объект) и «Подробнее» (строки «лейбл — значение»). Служит
 * и проекционному просмотру — тот же OperationView. Sticky-кнопка оплаты
 * вне потока — скелетоном не зеркалится.
 */
export function OperationDetailSkeleton(): JSX.Element {
  return (
    <>
      <div aria-hidden className="flex flex-col items-center gap-3 px-6 pt-6 pb-6">
        <Skeleton className="h-24 w-24 rounded-pill" />
        <Skeleton className="h-6 w-2/5" />
        <Skeleton className="h-8 w-28 rounded-pill" />
        <Skeleton className="h-[44px] w-3/5" />
        <Skeleton className="h-[18px] w-1/4" />
      </div>

      <SkeletonOperationSection>
        <SkeletonListRow className="py-3" />
        <SkeletonListRow className="py-3" />
      </SkeletonOperationSection>

      <SkeletonOperationSection>
        <SkeletonDetailRow />
        <SkeletonDetailRow />
        <SkeletonDetailRow />
      </SkeletonOperationSection>
    </>
  );
}

/**
 * Скелетон плоской группы «заголовок + строки» (#606) — история операций
 * и график платежей: заголовок даты/секции и строки канона
 * PaymentRowButton px-3 py-3 со суммой. Число групп и строк — типовой
 * срез контента (первая порция).
 */
export function PaymentGroupedListSkeleton({
  groups,
}: {
  readonly groups: ReadonlyArray<number>;
}): JSX.Element {
  return (
    <>
      {groups.map((rows, groupIndex) => (
        <section key={groupIndex} aria-hidden className="flex flex-col">
          <Skeleton className="mx-6 h-6 w-24" />
          {skeletonRowWidths(rows).map((rowWidths, index) => (
            <SkeletonListRow key={index} value widths={rowWidths} className="px-3 py-3" />
          ))}
        </section>
      ))}
    </>
  );
}
