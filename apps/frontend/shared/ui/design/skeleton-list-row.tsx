'use client';

import type { JSX } from 'react';
import { cn } from '@/shared/lib/cn';
import { Skeleton } from './skeleton';
import { SKELETON_ROW_WIDTHS_DEFAULT, skeletonBlockClass, type SkeletonRowWidths, type SkeletonTone } from './skeleton-parts';

/**
 * Скелетон строки списка (#604, API финализирован на хабах #605) —
 * плейсхолдер канонической строки (§6 DESIGN.md: ListRow и семейство
 * PaymentRowButton/ContactRowButton): круг ведущей иконки/аватара 44,
 * колонка заголовка и подзаголовка (24/20), правая колонка значения и
 * описания (сумма, дата — слотами `value`/`description`, как у ListRow) и
 * ведомый чекбокс выборщиков (`trailing`). Анатомия повторяет реальную
 * строку (px-6 py-2, зазор 4), чтобы контент занял место скелетона без
 * сдвига; нестандартные вставки строки (py-3 глобальной ленты, вставки
 * серой карточки) потребитель приносит через className. Тон `muted` —
 * строка внутри серой карточки. Ширины полей — `widths` (по умолчанию
 * первый шаг цикла `skeletonRowWidths`).
 */
export type SkeletonListRowProps = {
  /** Круг-плейсхолдер ведущей иконки/аватара 44×44. */
  readonly leading?: boolean;
  /** Подзаголовок-плейсхолдер; однострочные строки выборщиков (#544)
   * передают false — высота строки совпадает с реальной. */
  readonly subtitle?: boolean;
  /** Правая колонка, верхняя строка — значение (сумма). */
  readonly value?: boolean;
  /** Правая колонка, нижняя строка — описание (дата, срок). */
  readonly description?: boolean;
  /** Ведомый чекбокс-плейсхолдер (страницы-выборщики #542/#544). */
  readonly trailing?: boolean;
  /** Тон блоков: `base` — на белом, `muted` — внутри серой карточки. */
  readonly tone?: SkeletonTone;
  /** Ширины полей заголовка/подзаголовка (цикл для соседних строк). */
  readonly widths?: SkeletonRowWidths;
  readonly className?: string;
};

export function SkeletonListRow({
  leading = true,
  subtitle: withSubtitle = true,
  value = false,
  description = false,
  trailing = false,
  tone = 'base',
  widths = SKELETON_ROW_WIDTHS_DEFAULT,
  className,
}: SkeletonListRowProps): JSX.Element {
  const block = skeletonBlockClass(tone);
  return (
    <div aria-hidden className={cn('flex w-full items-center gap-3 px-6 py-2', className)}>
      {leading && <Skeleton className={cn('h-11 w-11 shrink-0', block)} />}
      <span className="flex min-w-0 flex-1 flex-col gap-1">
        <Skeleton className={cn('h-6', widths.title, block)} />
        {withSubtitle && <Skeleton className={cn('h-5', widths.subtitle, block)} />}
      </span>
      {(value || description) && (
        <span className="flex shrink-0 flex-col items-end gap-1">
          {value && <Skeleton className={cn('h-6 w-16', block)} />}
          {description && <Skeleton className={cn('h-5 w-12', block)} />}
        </span>
      )}
      {trailing && <Skeleton className={cn('h-5 w-5 shrink-0 rounded-lg', block)} />}
    </div>
  );
}
