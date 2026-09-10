import type { JSX } from 'react';
import {
  Skeleton,
  SkeletonListRow,
  SkeletonRoundAction,
  skeletonBlockClass,
} from '@/shared/ui/design';

/**
 * Скелетон экрана «Аренда» (#606, паритет — §7 DESIGN.md): композиция
 * примитивов #604 под каркас RentalDetailBody активной аренды —
 * картинка-ключ 96 с «Оплачено N из M», пара круглых действий, секции
 * «Платеж» (строка правила), прогресс-карточка, «Условия аренды» (3 строки
 * «метка — значение»), «Арендатор» и «Управление» (строки ListRow).
 * Шапка рендерится вне фазы загрузки.
 */

/** Приглушённый тон блоков внутри серых карточек (§7, skeletonBlockClass). */
const MUTED = skeletonBlockClass('muted');

/** Заголовок секции-заглушки: каркас RentalGroup — H3 20/24 и стрелка. */
function SkeletonGroupHeader({ titleWidth, arrow = true }: {
  readonly titleWidth: string;
  readonly arrow?: boolean;
}): JSX.Element {
  return (
    <span className="flex items-center gap-3">
      <Skeleton className={`h-6 ${titleWidth} ${MUTED}`} />
      {arrow && <Skeleton className={`h-6 w-6 shrink-0 ${MUTED}`} />}
    </span>
  );
}

export function RentalDetailSkeleton(): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col gap-12">
      {/* Картинка-ключ 96 (1232:61491) и «Оплачено N из M». */}
      <div className="flex flex-col items-center gap-4">
        <Skeleton className="h-24 w-24" />
        <div className="flex flex-col items-center gap-2">
          <Skeleton className="h-[18px] w-20" />
          <Skeleton className="h-8 w-3/5" />
        </div>
      </div>

      {/* Пара круглых действий равными колонками (1232:61272). */}
      <div className="grid grid-cols-2 justify-items-center">
        <SkeletonRoundAction />
        <SkeletonRoundAction />
      </div>

      <div className="flex flex-col gap-4">
        {/* «Платеж»: строка правила p-3 на серой карточке (pb-3). */}
        <section className="mx-6 flex flex-col gap-2 rounded-card bg-surface-muted pb-3">
          <div className="px-6 pt-6">
            <SkeletonGroupHeader titleWidth="w-24" />
          </div>
          <SkeletonListRow value tone="muted" className="px-3 py-3" />
        </section>

        {/* Прогресс: строка следующего платежа, бар, строка остатка. */}
        <section className="mx-6 rounded-card bg-surface-muted p-6">
          <div className="flex flex-col gap-5">
            <div className="flex flex-col gap-3">
              <Skeleton className={`h-4 w-2/5 ${MUTED}`} />
              <Skeleton className={`h-1.5 w-full rounded-pill ${MUTED}`} />
            </div>
            <Skeleton className={`h-4 w-1/3 ${MUTED}`} />
          </div>
        </section>

        {/* «Условия аренды»: 3 строки «метка — значение» (зазор секции 16). */}
        <section className="mx-6 flex flex-col gap-4 rounded-card bg-surface-muted pb-6">
          <div className="px-6 pt-6">
            <SkeletonGroupHeader titleWidth="w-40" />
          </div>
          <div className="flex flex-col gap-2 px-6">
            <span className="flex gap-3">
              <Skeleton className={`h-4 w-20 shrink-0 ${MUTED}`} />
              <Skeleton className={`h-4 w-24 ${MUTED}`} />
            </span>
            <span className="flex gap-3">
              <Skeleton className={`h-4 w-20 shrink-0 ${MUTED}`} />
              <Skeleton className={`h-4 w-16 ${MUTED}`} />
            </span>
            <span className="flex gap-3">
              <Skeleton className={`h-4 w-20 shrink-0 ${MUTED}`} />
              <Skeleton className={`h-4 w-1/3 ${MUTED}`} />
            </span>
          </div>
        </section>

        {/* «Арендатор»: строка канона px-3 py-2 (pb-4). */}
        <section className="mx-6 flex flex-col gap-2 rounded-card bg-surface-muted pb-4">
          <div className="px-6 pt-6">
            <SkeletonGroupHeader titleWidth="w-28" />
          </div>
          <SkeletonListRow tone="muted" className="px-3 py-2" />
        </section>

        {/* «Управление»: строки ListRow (иконка 24 + заголовок), без стрелки. */}
        <section className="mx-6 flex flex-col gap-2 rounded-card bg-surface-muted pb-3">
          <div className="px-6 pt-6">
            <SkeletonGroupHeader titleWidth="w-32" arrow={false} />
          </div>
          <div className="flex flex-col">
            {Array.from({ length: 4 }, (_, index) => (
              <span key={index} className="flex items-center gap-3 px-6 py-2">
                <Skeleton className={`h-6 w-6 shrink-0 ${MUTED}`} />
                <Skeleton className={`h-6 w-1/2 ${MUTED}`} />
              </span>
            ))}
          </div>
        </section>
      </div>
    </div>
  );
}
