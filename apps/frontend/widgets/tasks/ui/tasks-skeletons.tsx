import type { JSX } from 'react';
import {
  Skeleton,
  SkeletonFormField,
  skeletonBlockClass,
  skeletonRowWidths,
  type SkeletonRowWidths,
} from '@/shared/ui/design';

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

/**
 * Скелетон формы правки задачи (#606, паритет — §7 DESIGN.md): каркас
 * TaskEditForm (анатомия шага 2 создания) — «Задача», «Комментарий»
 * (min-h-14 при пустом значении), пара полей «Дата/Время», чипы повтора.
 * Sticky-кнопка «Сохранить» вне потока — не зеркалится.
 */
export function TaskEditFormSkeleton(): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col gap-8 px-6">
      <SkeletonFormField labelWidth="w-20" />
      <SkeletonFormField labelWidth="w-36" />
      <span className="grid grid-cols-2 gap-2">
        <SkeletonFormField labelWidth="w-16" />
        <SkeletonFormField labelWidth="w-16" />
      </span>
      <span className="flex flex-col gap-3">
        <Skeleton className="h-[18px] w-40" />
        <span className="flex flex-wrap gap-1">
          <Skeleton className="h-11 w-20 rounded-pill" />
          <Skeleton className="h-11 w-24 rounded-pill" />
          <Skeleton className="h-11 w-24 rounded-pill" />
          <Skeleton className="h-11 w-20 rounded-pill" />
        </span>
      </span>
    </div>
  );
}

/**
 * Скелетон шага создания задачи (#607, паритет — §7 DESIGN.md): шаг 1 —
 * «Задача» и, на глобальном входе, поле «Объект» с подписью; шаг 2 —
 * «Комментарий», пара «Дата/Время», чипы повтора (анатомия шага 2 без
 * поля «Задача»). Контейнеры повторяют шаги формы, чтобы контент занял
 * место скелетона без сдвига.
 */
export function TaskCreateStepSkeleton({
  step,
  withObject = false,
}: {
  readonly step: 1 | 2;
  /** Поле «Объект» — только глобальный вход (#525). */
  readonly withObject?: boolean;
}): JSX.Element {
  if (step === 1) {
    return (
      <div aria-hidden className={withObject ? 'flex flex-col gap-8 px-6' : 'px-6'}>
        <SkeletonFormField labelWidth="w-20" />
        {withObject && (
          <span className="flex flex-col gap-2">
            <Skeleton className="h-[18px] w-16" />
            <Skeleton className="h-14 w-full rounded-button" />
            <Skeleton className="h-[15px] w-24" />
          </span>
        )}
      </div>
    );
  }
  return (
    <div aria-hidden className="flex flex-col gap-8 px-6">
      <SkeletonFormField labelWidth="w-36" />
      <span className="grid grid-cols-2 gap-2">
        <SkeletonFormField labelWidth="w-16" />
        <SkeletonFormField labelWidth="w-16" />
      </span>
      <span className="flex flex-col gap-3">
        <Skeleton className="h-[18px] w-40" />
        <span className="flex flex-wrap gap-1">
          <Skeleton className="h-11 w-20 rounded-pill" />
          <Skeleton className="h-11 w-24 rounded-pill" />
          <Skeleton className="h-11 w-24 rounded-pill" />
          <Skeleton className="h-11 w-20 rounded-pill" />
        </span>
      </span>
    </div>
  );
}
