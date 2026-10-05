import type { JSX } from "react";
import { cn } from "@/lib/cn";

/** Ручка шита (Figma 847:11295 Drag Handle) — перенос из apps/frontend
 * (shared/ui/design/sheet-drag-handle.tsx): серая пилюля 36×4 в полосе
 * высотой 16. Общая для модалки-шита; вариант `wide` — пилюля 48×4 в
 * полосе 24 — ручка шита «Еще» TabBar фронта, лендингу не нужна, оставлена
 * для 1:1 с исходником. */
export type SheetDragHandleProps = {
  readonly className?: string;
  /** Широкая ручка 48×4 в полосе 24 — шит «Еще» TabBar фронта. */
  readonly wide?: boolean;
};

export function SheetDragHandle({ className, wide = false }: SheetDragHandleProps): JSX.Element {
  return (
    <div
      className={cn("flex shrink-0 items-center justify-center", wide ? "h-6" : "h-4", className)}
      aria-hidden
    >
      <div className={cn("h-1 rounded-pill bg-input-border", wide ? "w-12" : "w-9")} />
    </div>
  );
}
