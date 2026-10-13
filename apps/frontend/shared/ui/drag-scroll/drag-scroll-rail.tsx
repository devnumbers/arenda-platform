"use client";

import type { ReactNode } from "react";
import type { JSX } from "react";
import { cn } from "@/shared/lib/cn";
import { useDragScroll } from "./use-drag-scroll";

/** Канонные классы горизонтальной snap-ленты (хаб «Платежей», #578):
 * скрытый скроллбар поверх нативного overflow-скролла. */
const RAIL_CLASSES =
  "flex snap-x snap-mandatory gap-2 overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden";

/**
 * Горизонтальная snap-лента с mouse-drag прокси (#1349): нативный
 * overflow-скролл, перетаскивание мышью/пером с инерционным доездом к
 * snap-точке (useDragScroll; тач не трогается). Дети — snap-start
 * карточки; допклассы потребителя (bleed ≤560: `max-tablet:-mx-6
 * max-tablet:px-6 max-tablet:[scroll-padding-inline:24px]`) ложатся
 * поверх канонных.
 */
export function DragScrollRail({
  testId,
  className,
  children,
}: {
  readonly testId?: string;
  readonly className?: string;
  readonly children: ReactNode;
}): JSX.Element {
  const { ref, onPointerDown, onDragStart } = useDragScroll();
  return (
    <div
      ref={ref}
      onPointerDown={onPointerDown}
      onDragStart={onDragStart}
      data-testid={testId}
      className={cn(RAIL_CLASSES, className)}
    >
      {children}
    </div>
  );
}
