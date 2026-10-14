"use client";

import type { JSX } from "react";
import type { ReactNode } from "react";
import { cn } from "@/shared/lib/cn";
import { useSnapRailDrag } from "./use-snap-rail-drag";

/** Канонные классы горизонтальной snap-ленты: нативный overflow-скролл со
 * скрытым скроллбаром, snap-mandatory (тач/колесо браузера), axis-lock —
 * вертикальный свайп по ленте крутит страницу (`touch-action: pan-y
 * pinch-zoom`); горизонтальную ось скроллер ведёт сам. Дети — snap-start
 * ([&>*]: карточкам не нужно помнить канон): без snap-align у детей
 * браузерный снап мёртв. */
const RAIL_CLASSES =
  "flex snap-x snap-mandatory gap-2 overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden [&>*]:snap-start [touch-action:pan-y_pinch-zoom]";

/**
 * Горизонтальная snap-лента (#1349; решение владельца 14.10 — возврат с
 * Embla на своё решение): нативный скролл — тач, колесо и перо ведёт
 * браузер (momentum, резинка и снап UIScrollView на iOS — оригинал, не
 * имитация); мышь/перо тащат ленту drag-прокси с инерцией к ближайшей
 * snap-точке проекции остановки и резинкой у краёв (useSnapRailDrag,
 * физика Apple — snap-rail-physics). Дети — snap-start карточки
 * фиксированной ширины со shrink-0; допклассы потребителя (bleed ≤560:
 * `max-tablet:-mx-6 max-tablet:px-6 max-tablet:[scroll-padding-inline:
 * 24px]`) ложатся на скроллер поверх канонных — scroll-padding и есть
 * линия снапа. Вживую: ленты секций хаба «Платежей»
 * (widgets/payments/ui/payments-global-screen.tsx), витрина /ui-kit.
 */
export function SnapRail({
  testId,
  className,
  children,
}: {
  readonly testId?: string;
  readonly className?: string;
  readonly children: ReactNode;
}): JSX.Element {
  const { railRef, contentRef, onPointerDown, onDragStart } = useSnapRailDrag();

  return (
    <div
      ref={railRef}
      onPointerDown={onPointerDown}
      onDragStart={onDragStart}
      data-testid={testId}
      className={cn(RAIL_CLASSES, className)}
    >
      <div ref={contentRef} className="flex gap-2">
        {children}
      </div>
    </div>
  );
}
