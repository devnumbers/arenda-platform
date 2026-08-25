import type { JSX } from 'react';
import { cn } from '@/shared/lib/cn';

/** Ручка шита (Figma 847:11295 Drag Handle): серая пилюля 36×4 в полосе
 * высотой 16. Общая для StickyBottomBar и модалки-шита. */
export type SheetDragHandleProps = {
  readonly className?: string;
};

export function SheetDragHandle({ className }: SheetDragHandleProps): JSX.Element {
  return (
    <div className={cn('flex h-4 shrink-0 items-center justify-center', className)} aria-hidden>
      <div className="h-1 w-9 rounded-pill bg-input-border" />
    </div>
  );
}
