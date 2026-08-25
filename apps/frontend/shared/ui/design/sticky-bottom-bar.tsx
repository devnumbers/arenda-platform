import type { JSX, ReactNode } from 'react';
import { cn } from '@/shared/lib/cn';
import { SheetDragHandle } from './sheet-drag-handle';

/** Нижняя закреплённая панель дизайн-слоя (Figma 651:5800): белый «шит»
 * radius 40 сверху с drag-handle и слотом контента (паддинг 24, зазор 16);
 * на широких экранах контент — в колонке max-560. Низ уважает safe-area
 * (home indicator). PageContent даёт снизу 136px, чтобы контент не уходил
 * под панель. */
export type StickyBottomBarProps = {
  readonly children: ReactNode;
  readonly dragHandle?: boolean;
  readonly className?: string;
};

export function StickyBottomBar({ children, dragHandle = true, className }: StickyBottomBarProps): JSX.Element {
  return (
    <div className={cn('fixed inset-x-0 bottom-0 z-40 rounded-t-sheet bg-surface font-sans', className)}>
      {dragHandle && <SheetDragHandle />}
      <div className="mx-auto flex w-full max-w-[560px] flex-col gap-4 p-6 pb-[max(1.5rem,env(safe-area-inset-bottom))]">
        {children}
      </div>
    </div>
  );
}
