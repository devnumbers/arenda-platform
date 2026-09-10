import type { JSX } from 'react';
import { Skeleton, skeletonBlockClass, skeletonRowWidths, type SkeletonRowWidths } from '@/shared/ui/design';

/**
 * Скелетоны-архетипы ленты задач (#605, паритет — §7 DESIGN.md):
 * композиции примитивов #604 под лейаут глобальной ленты — секции-карточки
 * TaskSectionCard (§6: строки TaskRow) и сворачиваемый блок выполненных
 * CollapsibleSection (по умолчанию свёрнут — в загрузке тоже свёрнут).
 * Шапки (заголовок, чипы сортировки/фильтра) рендерятся вне фазы загрузки.
 */

/** Приглушённый тон блоков внутри серых секций (§7, skeletonBlockClass). */
const MUTED = skeletonBlockClass('muted');

/** Строка задачи-заглушка: каркас TaskRow — кружок-чекбокс 24, колонка
 * «название 16/18 + строка объекта 14/16 + строка времени 14/16» с зазором
 * 6 (px-6 py-3 строки). */
function SkeletonTaskRow({
  widths,
}: {
  readonly widths: SkeletonRowWidths;
}): JSX.Element {
  return (
    <div aria-hidden className="flex w-full items-start gap-4 px-6 py-3">
      <Skeleton className={`mt-px h-6 w-6 shrink-0 rounded-pill ${MUTED}`} />
      <span className="flex min-w-0 flex-1 flex-col gap-1.5">
        <Skeleton className={`h-[18px] ${widths.title} ${MUTED}`} />
        <Skeleton className={`h-4 ${widths.subtitle} ${MUTED}`} />
        <Skeleton className={`h-4 w-1/4 ${MUTED}`} />
      </span>
    </div>
  );
}

/** Секция-карточка-заглушка: каркас TaskSectionCard — заголовок H3 20/24
 * (pt-6 pb-2) и строки задач, снизу баланс 8. */
function SkeletonSectionCard({ rows }: { readonly rows: number }): JSX.Element {
  const widths = skeletonRowWidths(rows);
  return (
    <section aria-hidden className="mx-6 rounded-card bg-surface-muted pb-2">
      <div className="px-6 pb-2 pt-6">
        <Skeleton className={`h-6 w-40 ${MUTED}`} />
      </div>
      <div className="flex flex-col">
        {widths.map((rowWidths, index) => (
          <SkeletonTaskRow key={index} widths={rowWidths} />
        ))}
      </div>
    </section>
  );
}

/** Свёрнутый блок выполненных-заглушка: каркас CollapsibleSection без
 * контента — заголовок со счётчиком и стрелка (24 + баланс 24). */
function SkeletonCompletedSection(): JSX.Element {
  return (
    <section
      aria-hidden
      className="mx-6 flex items-center justify-between gap-3 rounded-card bg-surface-muted px-6 pb-6 pt-6"
    >
      <Skeleton className={`h-6 w-44 ${MUTED}`} />
      <Skeleton className={`h-6 w-6 shrink-0 ${MUTED}`} />
    </section>
  );
}

/**
 * Скелетон ленты задач: активная секция с тремя строками (типовая
 * «Просроченные»/«Сегодня»), секция с одной и свёрнутые «Выполненные».
 * Рендерится внутри контейнера ленты (gap-6 между блоками даёт контейнер).
 */
export function TasksFeedSkeleton(): JSX.Element {
  return (
    <>
      <SkeletonSectionCard rows={3} />
      <SkeletonSectionCard rows={1} />
      <SkeletonCompletedSection />
    </>
  );
}
