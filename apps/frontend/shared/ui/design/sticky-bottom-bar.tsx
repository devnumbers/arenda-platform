import type { JSX, ReactNode } from 'react';
import { cn } from '@/shared/lib/cn';
import { SheetDragHandle } from './sheet-drag-handle';

/** Нижняя закреплённая панель дизайн-слоя (Figma 1043:60106): белый «шит»
 * radius 40 сверху со слотом контента (паддинг 24, зазор 16); каноника —
 * без grabber-ручки (Show Grabber: false), проп `dragHandle` оставлен для
 * включения, если канва вернёт ручку. На широких экранах контент — в
 * колонке max-560. Низ уважает safe-area (home indicator). PageContent даёт
 * снизу 136px, чтобы контент не уходил под панель. */
export type StickyBottomBarProps = {
  readonly children: ReactNode;
  readonly dragHandle?: boolean;
  readonly className?: string;
};

export function StickyBottomBar({ children, dragHandle = false, className }: StickyBottomBarProps): JSX.Element {
  return (
    <div className={cn('fixed inset-x-0 bottom-0 z-40 rounded-t-sheet bg-surface font-sans', className)}>
      {dragHandle && <SheetDragHandle />}
      <div className="mx-auto flex w-full max-w-[560px] flex-col gap-4 p-6 pb-[max(1.5rem,env(safe-area-inset-bottom))]">
        {children}
      </div>
    </div>
  );
}
