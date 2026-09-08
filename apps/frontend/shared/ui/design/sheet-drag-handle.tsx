import type { JSX } from 'react';
import { cn } from '@/shared/lib/cn';

/** Ручка шита (Figma 847:11295 Drag Handle): серая пилюля 36×4 в полосе
 * высотой 16. Общая для StickyBottomBar и модалки-шита. Вариант `wide` —
 * пилюля 48×4 в полосе 24 — ручка шита «Еще» (Figma 1721:57140, тикет
 * #560). */
export type SheetDragHandleProps = {
  readonly className?: string;
  /** Широкая ручка 48×4 в полосе 24 — шит «Еще» TabBar. */
  readonly wide?: boolean;
};

export function SheetDragHandle({ className, wide = false }: SheetDragHandleProps): JSX.Element {
  return (
    <div
      className={cn('flex shrink-0 items-center justify-center', wide ? 'h-6' : 'h-4', className)}
      aria-hidden
    >
      <div className={cn('h-1 rounded-pill bg-input-border', wide ? 'w-12' : 'w-9')} />
    </div>
  );
}
