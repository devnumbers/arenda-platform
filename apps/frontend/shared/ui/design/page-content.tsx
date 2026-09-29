import type { JSX, ReactNode } from 'react';
import { cn } from '@/shared/lib/cn';

/** Колонка контента новых экранов (Figma 948:47567): центрированная колонка
 * max-width 560 (токен `--container-column`, утилита max-w-column — тикет
 * #865); mx-auto центрирует её по вьюпорту на любом ярусе, сайдбар на ПК
 * fixed и сдвига контента не несёт — правее сайдбара на ПК живёт только
 * белый лист полноэкранных поверхностей (канон DESIGN.md §1). Вертикальные
 * отступы 24/136 — 24 от хедера до контента (решение задачи #460), 136
 * снизу под StickyBottomBar/TabBar. Горизонтальный паддинг не вкладывается
 * — строки списка приносят свой 24px, остальной контент оборачивается
 * экраном. */
export type PageContentProps = {
  readonly children: ReactNode;
  readonly className?: string;
};

export function PageContent({ children, className }: PageContentProps): JSX.Element {
  return (
    <div className={cn('mx-auto flex w-full max-w-column flex-col pt-6 pb-[136px] font-sans', className)}>
      {children}
    </div>
  );
}
