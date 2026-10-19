"use client";

import type { JSX } from "react";
import type { ReactNode } from "react";
import { cn } from "@/shared/lib/cn";
import { useSnapRailDrag } from "./use-snap-rail-drag";

/** Канонные классы горизонтальной ленты: нативный overflow-скролл со
 * скрытым скроллбаром, БЕЗ снапа (решение владельца 19.10 — «магнитность
 * убрать, пусть просто скроллится»; supersedes proximity 18.10 и
 * mandatory 14.10) — свободный скролл с инерцией и резинкой браузера.
 * touch-action `pan-x pan-y pinch-zoom`: pan-x разрешает браузеру вести
 * горизонтальную ось (нативный скролл ленты на таче работает, только если
 * ось разрешена touch-action — одиночный pan-y по образцу JS-карусели
 * делал ось мёртвой), pan-y держит вертикальный свайп за страницей; выбор
 * оси — первичная ось жеста, браузер разбирает сам. */
const RAIL_CLASSES =
  "flex gap-2 overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden [touch-action:pan-x_pan-y_pinch-zoom]";

/** Классы ширины fluid-плитки ленты (решение владельца 18.10 — «2 на
 * мобилке, 3 на планшете/ПК»): проценты резолвятся по контент-боксу ленты —
 * с bleed ≤560 это вьюпорт минус 2×24, выше — колонка минус 2×24. Единый
 * источник для плитки `PaymentCardButton`, её скелетона и витрин — сетка
 * 24/8 не расползается. */
export const SNAP_TILE_CLASSES =
  "w-[calc((100%-8px)/2)] tablet:w-[calc((100%-16px)/3)]";

/**
 * Горизонтальная лента (#1349; без снапа — решение владельца 19.10,
 * supersedes proximity 18.10 и mandatory 14.10): нативный скролл — тач,
 * колесо и перо ведут браузер (momentum и резинка UIScrollView на iOS —
 * оригинал, не имитация); мышь/перо тащат ленту drag-прокси с инерционным
 * доездом по проекции остановки (без привязки к карточным линиям) и
 * резинкой у краёв (useSnapRailDrag, физика Apple — snap-rail-physics).
 * Дети — карточки fluid-ширины со shrink-0 (плитка растягивается: 2
 * карточки на мобайл-ярусе, 3 на планшете/ПК — SNAP_TILE_CLASSES,
 * решение владельца 18.10); допклассы потребителя (bleed ≤560:
 * `max-tablet:-mx-6 max-tablet:px-6`) ложатся на скроллер поверх
 * канонных. Вживую: ленты секций хаба «Платежей»
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
      {/* w-full обязателен: контент-див — флекс-айтем с auto-шириной
       * (max-content) — без явной ширины проценты fluid-плиток резолвятся
       * циклически. w-full = контент-бокс ленты, против него и считают
       * плитки. */}
      <div ref={contentRef} className="flex w-full gap-2">
        {children}
      </div>
    </div>
  );
}
